package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestStartCleansUpOnLaunchFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Engine:    "codex",
		Model:     "codex",
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "codex",
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	tmuxPath := filepath.Join(binDir, "tmux")
	script := `#!/bin/sh
set -eu
state="${GOALX_FAKE_TMUX_STATE:?}"
mkdir -p "$state"
log="$state/log"
cmd="$1"
shift
echo "$cmd $*" >> "$log"
case "$cmd" in
  has-session)
    target="$2"
    if [ -f "$state/session_$target" ]; then
      exit 0
    fi
    exit 1
    ;;
  new-session)
    name=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-s" ]; then
        shift
        name="$1"
        break
      fi
      shift
    done
    : > "$state/session_$name"
    exit 0
    ;;
  kill-session)
    target="$2"
    rm -f "$state/session_$target"
    exit 0
    ;;
  send-keys)
    if [ "${GOALX_FAKE_TMUX_FAIL_TARGET:-}" = "send-keys" ]; then
      exit 1
    fi
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)
	tmuxSess := goalx.TmuxSessionName(repo, cfg.Name)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error {
		return os.ErrPermission
	}

	err = Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")})
	if err == nil {
		t.Fatal("expected Start to fail")
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	if _, statErr := os.Stat(runDir); !os.IsNotExist(statErr) {
		t.Fatalf("run dir should be removed, stat err = %v", statErr)
	}

	branch := "goalx/demo/1"
	if err := exec.Command("git", "-C", repo, "rev-parse", "--verify", branch).Run(); err == nil {
		t.Fatalf("branch %s should be deleted during cleanup", branch)
	}

	logData, err := os.ReadFile(filepath.Join(stateDir, "log"))
	if err != nil {
		t.Fatalf("read fake tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "kill-session -t "+tmuxSess) {
		t.Fatalf("cleanup log missing kill-session for %s:\n%s", tmuxSess, string(logData))
	}
}

func TestStartLaunchesOnlyMaster(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Engine:    "codex",
		Model:     "codex",
		Parallel:  2,
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "codex",
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	tmuxPath := filepath.Join(binDir, "tmux")
	script := `#!/bin/sh
set -eu
state="${GOALX_FAKE_TMUX_STATE:?}"
mkdir -p "$state"
log="$state/log"
cmd="$1"
shift
echo "$cmd $*" >> "$log"
case "$cmd" in
  has-session)
    target="$2"
    if [ -f "$state/session_$target" ]; then
      exit 0
    fi
    exit 1
    ;;
  new-session)
    name=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-s" ]; then
        shift
        name="$1"
        break
      fi
      shift
    done
    : > "$state/session_$name"
    exit 0
    ;;
  kill-session)
    target="$2"
    rm -f "$state/session_$target"
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	var gotSidecarProjectRoot, gotSidecarRunName string
	var gotSidecarInterval time.Duration
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error {
		gotSidecarProjectRoot, gotSidecarRunName, gotSidecarInterval = projectRoot, runName, interval
		return nil
	}

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	charter, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	if err := ValidateRunCharterLinkage(meta, charter); err != nil {
		t.Fatalf("ValidateRunCharterLinkage: %v", err)
	}
	if meta.CharterID == "" || meta.CharterHash == "" {
		t.Fatalf("run metadata missing charter linkage: %+v", meta)
	}
	controlIdentity, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunIdentity: %v", err)
	}
	if controlIdentity.CharterID != meta.CharterID || controlIdentity.CharterDigest != meta.CharterHash {
		t.Fatalf("control run identity charter linkage = %+v, metadata = %+v", controlIdentity, meta)
	}
	fence, err := LoadIdentityFence(IdentityFencePath(runDir))
	if err != nil {
		t.Fatalf("LoadIdentityFence: %v", err)
	}
	if fence.RunID != meta.RunID || fence.Epoch != meta.Epoch || fence.CharterHash != meta.CharterHash {
		t.Fatalf("identity fence linkage = %+v, metadata = %+v", fence, meta)
	}
	for _, path := range []string{
		filepath.Join(runDir, "master.md"),
		filepath.Join(runDir, "master.jsonl"),
		filepath.Join(runDir, "run-charter.json"),
		filepath.Join(runDir, "acceptance.json"),
		filepath.Join(runDir, "goal.json"),
		filepath.Join(runDir, "goal-log.jsonl"),
		RunMetadataPath(runDir),
		filepath.Join(runDir, "control", "identity-fence.json"),
		filepath.Join(runDir, "artifacts.json"),
		filepath.Join(runDir, "coordination.json"),
		MasterInboxPath(runDir),
		MasterCursorPath(runDir),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(runDir, "acceptance.md"),
		filepath.Join(runDir, "goal-contract.json"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent, stat err = %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(runDir, "program-1.md"),
		filepath.Join(runDir, "journals", "session-1.jsonl"),
		WorktreePath(runDir, cfg.Name, 1),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent, stat err = %v", path, err)
		}
	}

	logData, err := os.ReadFile(filepath.Join(stateDir, "log"))
	if err != nil {
		t.Fatalf("read fake tmux log: %v", err)
	}
	logText := string(logData)
	if strings.Contains(logText, "heartbeat") {
		t.Fatalf("start should not create legacy heartbeat window:\n%s", logText)
	}
	if !strings.Contains(logText, "new-session -d -s "+goalx.TmuxSessionName(repo, cfg.Name)+" -n master") {
		t.Fatalf("start log missing master session creation:\n%s", logText)
	}
	if strings.Contains(logText, "send-keys -t "+goalx.TmuxSessionName(repo, cfg.Name)+":master") {
		t.Fatalf("start should launch master directly, not via send-keys:\n%s", logText)
	}
	if gotSidecarProjectRoot != repo || gotSidecarRunName != cfg.Name {
		t.Fatalf("launchRunSidecar got (%q, %q), want (%q, %q)", gotSidecarProjectRoot, gotSidecarRunName, repo, cfg.Name)
	}
	if gotSidecarInterval <= 0 {
		t.Fatalf("launchRunSidecar interval = %v, want > 0", gotSidecarInterval)
	}

	stateData, err := os.ReadFile(filepath.Join(runDir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance state: %v", err)
	}
	stateText := string(stateData)
	for _, want := range []string{
		`"status": "pending"`,
		`"default_command": "test -f README.md"`,
		`"effective_command": "test -f README.md"`,
		`"change_kind": "same"`,
		`"goal_version": 1`,
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}
}

func TestStartRendersMasterPreferences(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Engine:    "codex",
		Model:     "codex",
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "codex",
		},
		Preferences: goalx.PreferencesConfig{
			Research: goalx.PreferencePolicy{
				Engines:  []string{"claude-code/opus", "codex/gpt-5.4"},
				Strategy: "multi-perspective",
			},
			Develop: goalx.PreferencePolicy{
				Engines:  []string{"codex/gpt-5.4"},
				Strategy: "speed",
			},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	tmuxPath := filepath.Join(binDir, "tmux")
	script := `#!/bin/sh
set -eu
state="${GOALX_FAKE_TMUX_STATE:?}"
mkdir -p "$state"
log="$state/log"
cmd="$1"
shift
echo "$cmd $*" >> "$log"
case "$cmd" in
  has-session)
    target="$2"
    if [ -f "$state/session_$target" ]; then
      exit 0
    fi
    exit 1
    ;;
  new-session)
    name=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-s" ]; then
        shift
        name="$1"
        break
      fi
      shift
    done
    : > "$state/session_$name"
    exit 0
    ;;
  kill-session)
    target="$2"
    rm -f "$state/session_$target"
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	masterProtocol, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read master.md: %v", err)
	}
	text := string(masterProtocol)
	for _, want := range []string{
		"## User Preferences",
		"| Research | claude-code/opus, codex/gpt-5.4 | multi-perspective |",
		"| Develop | codex/gpt-5.4 | speed |",
		"CLI overrides take precedence.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("master.md missing %q:\n%s", want, text)
		}
	}
}

func TestStartLaunchesMasterWithRuntimeEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Engine:    "codex",
		Model:     "codex",
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "codex",
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	tmuxPath := filepath.Join(binDir, "tmux")
	script := `#!/bin/sh
set -eu
state="${GOALX_FAKE_TMUX_STATE:?}"
mkdir -p "$state"
log="$state/log"
cmd="$1"
shift
echo "$cmd $*" >> "$log"
case "$cmd" in
  has-session)
    target="$2"
    if [ -f "$state/session_$target" ]; then
      exit 0
    fi
    exit 1
    ;;
  new-session)
    name=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-s" ]; then
        shift
        name="$1"
        break
      fi
      shift
    done
    : > "$state/session_$name"
    exit 0
    ;;
  kill-session)
    target="$2"
    rm -f "$state/session_$target"
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", binDir+":"+"/tmp/goalx-bin:/usr/bin")
	t.Setenv("OPENAI_API_KEY", "sk-goalx")
	t.Setenv("FOO_TOOLCHAIN_ROOT", "/opt/goalx-toolchain")
	t.Setenv("TMUX_PANE", "%99")
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	logData, err := os.ReadFile(filepath.Join(stateDir, "log"))
	if err != nil {
		t.Fatalf("read fake tmux log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"new-session -d -s " + goalx.TmuxSessionName(repo, cfg.Name) + " -n master -c " + repo + " env ",
		"/bin/bash -c ",
		"lease-loop --run",
		"--holder",
		"master",
		"FOO_TOOLCHAIN_ROOT='/opt/goalx-toolchain'",
		"HOME='" + home + "'",
		"PATH='" + binDir + ":/tmp/goalx-bin:/usr/bin'",
		"OPENAI_API_KEY='sk-goalx'",
		"codex -m gpt-5.4 -a never -s danger-full-access",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("start launch log missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "TMUX_PANE='%99'") {
		t.Fatalf("start launch should not propagate TMUX_PANE:\n%s", logText)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	launchEnvData, err := os.ReadFile(filepath.Join(runDir, "control", "launch-env.json"))
	if err != nil {
		t.Fatalf("read launch env snapshot: %v", err)
	}
	launchEnvText := string(launchEnvData)
	for _, want := range []string{
		`"FOO_TOOLCHAIN_ROOT": "/opt/goalx-toolchain"`,
		`"HOME": "` + home + `"`,
		`"OPENAI_API_KEY": "sk-goalx"`,
	} {
		if !strings.Contains(launchEnvText, want) {
			t.Fatalf("launch env snapshot missing %q:\n%s", want, launchEnvText)
		}
	}
	if strings.Contains(launchEnvText, `"TMUX_PANE"`) {
		t.Fatalf("launch env snapshot should not persist TMUX_PANE:\n%s", launchEnvText)
	}
}
