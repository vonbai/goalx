package cli

import (
	"encoding/json"
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
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)
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

	branch := "goalx/demo/root"
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
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Parallel: 2,
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)
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
	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	if activity == nil || activity.Run.RunName != cfg.Name {
		t.Fatalf("activity snapshot not written correctly: %+v", activity)
	}
	index, err := LoadContextIndex(ContextIndexPath(runDir))
	if err != nil {
		t.Fatalf("LoadContextIndex: %v", err)
	}
	if index == nil || index.RunName != cfg.Name {
		t.Fatalf("context index not written correctly: %+v", index)
	}
	affordances, err := LoadAffordances(AffordancesJSONPath(runDir))
	if err != nil {
		t.Fatalf("LoadAffordances: %v", err)
	}
	if affordances == nil || affordances.RunName != cfg.Name {
		t.Fatalf("affordances not written correctly: %+v", affordances)
	}

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
		testSelectionSnapshotPath(runDir),
		filepath.Join(runDir, "acceptance.json"),
		filepath.Join(runDir, "goal.json"),
		filepath.Join(runDir, "goal-log.jsonl"),
		filepath.Join(runDir, "reports"),
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
	snapshot := readSelectionSnapshotFixture(t, runDir)
	if snapshot.Master.Engine != "codex" || snapshot.Master.Model != "gpt-5.4" {
		t.Fatalf("selection snapshot master = %s/%s, want codex/gpt-5.4", snapshot.Master.Engine, snapshot.Master.Model)
	}
	if snapshot.Research.Engine != "codex" || snapshot.Research.Model != "gpt-5.4" {
		t.Fatalf("selection snapshot research = %s/%s, want codex/gpt-5.4", snapshot.Research.Engine, snapshot.Research.Model)
	}
	if snapshot.Policy.MasterCandidates[0] != "codex/gpt-5.4" {
		t.Fatalf("selection snapshot master_candidates = %#v, want codex first", snapshot.Policy.MasterCandidates)
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
	if strings.Contains(stateText, `"default_command"`) {
		t.Fatalf("acceptance state must not synthesize default_command from local_validation:\n%s", stateText)
	}
	if strings.Contains(stateText, `"effective_command"`) {
		t.Fatalf("acceptance state must not synthesize effective_command from local_validation:\n%s", stateText)
	}
	if !strings.Contains(stateText, `"goal_version": 1`) {
		t.Fatalf("acceptance state missing goal_version:\n%s", stateText)
	}
	// Framework must not auto-derive status or governance fields.
	if strings.Contains(stateText, `"status"`) {
		t.Fatalf("acceptance state must not contain derived status field:\n%s", stateText)
	}
	for _, unwanted := range []string{`"change_kind"`, `"change_reason"`, `"user_approved"`} {
		if strings.Contains(stateText, unwanted) {
			t.Fatalf("acceptance state must not contain governance field %q:\n%s", unwanted, stateText)
		}
	}
}

func TestStartWithManualDraftPreservesImplicitSelectionPolicySnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	draft := []byte(`
name: demo
mode: auto
objective: audit auth flow
target:
  files: ["README.md"]
local_validation:
  command: test -f README.md
`)
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), draft, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	binDir := t.TempDir()
	stateDir := t.TempDir()
	for _, name := range []string{"codex", "claude"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write %s shim: %v", name, err)
		}
	}
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
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, "demo")
	snapshot := readSelectionSnapshotFixture(t, runDir)
	if snapshot.Master.Engine != "codex" || snapshot.Master.Model != "gpt-5.4" {
		t.Fatalf("snapshot master = %s/%s, want codex/gpt-5.4", snapshot.Master.Engine, snapshot.Master.Model)
	}
	if len(snapshot.Policy.MasterCandidates) < 2 || snapshot.Policy.MasterCandidates[0] != "codex/gpt-5.4" || snapshot.Policy.MasterCandidates[1] != "claude-code/opus" {
		t.Fatalf("snapshot master_candidates = %#v, want codex then claude", snapshot.Policy.MasterCandidates)
	}
	if len(snapshot.Policy.ResearchCandidates) == 0 || snapshot.Policy.ResearchCandidates[0] != "claude-code/opus" {
		t.Fatalf("snapshot research_candidates = %#v, want claude-code/opus first", snapshot.Policy.ResearchCandidates)
	}
}

func TestStartInitializesRootExperimentLineage(t *testing.T) {
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
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
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
cmd="$1"
shift
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
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error {
		return nil
	}

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) != 1 || events[0].Kind != "experiment.created" {
		t.Fatalf("unexpected experiment events: %#v", events)
	}
	state, err := LoadIntegrationState(IntegrationStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadIntegrationState: %v", err)
	}
	if state == nil {
		t.Fatal("integration state missing")
	}
	if strings.TrimSpace(state.CurrentExperimentID) == "" {
		t.Fatal("CurrentExperimentID empty")
	}
	if state.CurrentBranch != "goalx/demo/root" {
		t.Fatalf("CurrentBranch = %q, want goalx/demo/root", state.CurrentBranch)
	}
}

func TestStartRefreshesActivityAfterMasterLaunch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:            "guidance-start",
		Mode:            goalx.ModeDevelop,
		Objective:       "ship it",
		Target:          goalx.TargetConfig{Files: []string{"README.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master:          goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
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
cmd="$1"
shift
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
  capture-pane)
    printf 'master launched\n'
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	if activity == nil || !activity.Actors["master"].PanePresent {
		t.Fatalf("master pane facts missing after start: %+v", activity)
	}
}

func TestStartFocusesNewestRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	writeDraft := func(name, objective string) string {
		t.Helper()
		cfg := goalx.Config{
			Name:      name,
			Mode:      goalx.ModeDevelop,
			Objective: objective,
			Roles: goalx.RoleDefaultsConfig{
				Develop: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
			},
			Target: goalx.TargetConfig{Files: []string{"README.md"}},
			LocalValidation: goalx.LocalValidationConfig{
				Command: "test -f README.md",
			},
			Master: goalx.MasterConfig{
				Engine: "codex",
				Model:  "gpt-5.4",
			},
		}
		data, err := yaml.Marshal(&cfg)
		if err != nil {
			t.Fatalf("marshal config: %v", err)
		}
		path := filepath.Join(goalxDir, "goalx.yaml")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write goalx.yaml: %v", err)
		}
		return path
	}

	firstDraft := writeDraft("alpha", "ship alpha")
	if err := Start(repo, []string{"--config", firstDraft}); err != nil {
		t.Fatalf("Start alpha: %v", err)
	}
	secondDraft := writeDraft("beta", "ship beta")
	if err := Start(repo, []string{"--config", secondDraft}); err != nil {
		t.Fatalf("Start beta: %v", err)
	}

	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	if reg.FocusedRun != "beta" {
		t.Fatalf("focused run = %q, want beta", reg.FocusedRun)
	}
}

func TestStartBuildLaunchConfigMatchesResolverDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
name: shared
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeDetectedPresetPath(t)
	t.Setenv("PATH", pathDir+":"+os.Getenv("PATH"))

	opts, err := parseStartArgs([]string{"ship it"})
	if err != nil {
		t.Fatalf("parseStartArgs: %v", err)
	}
	resolvedCfg, err := resolveLaunchConfig(projectRoot, opts.launchOptions)
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}

	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	resolved, err := goalx.ResolveConfig(layers, goalx.ResolveRequest{
		Objective: "ship it",
		Mode:      goalx.ModeDevelop,
	})
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	cfg := resolvedCfg.Config

	if cfg.Master.Engine != resolved.Config.Master.Engine || cfg.Master.Model != resolved.Config.Master.Model {
		t.Fatalf("master = %s/%s, want %s/%s", cfg.Master.Engine, cfg.Master.Model, resolved.Config.Master.Engine, resolved.Config.Master.Model)
	}
	if cfg.Roles.Research.Engine != resolved.Config.Roles.Research.Engine || cfg.Roles.Research.Model != resolved.Config.Roles.Research.Model {
		t.Fatalf("research = %s/%s, want %s/%s", cfg.Roles.Research.Engine, cfg.Roles.Research.Model, resolved.Config.Roles.Research.Engine, resolved.Config.Roles.Research.Model)
	}
	if cfg.Roles.Develop.Engine != resolved.Config.Roles.Develop.Engine || cfg.Roles.Develop.Model != resolved.Config.Roles.Develop.Model {
		t.Fatalf("develop = %s/%s, want %s/%s", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model, resolved.Config.Roles.Develop.Engine, resolved.Config.Roles.Develop.Model)
	}
}

func TestStartPreservesExplicitCodexTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	writeLaunchConfigProjectFile(t, repo, `
master:
  engine: codex
  model: gpt-5.4
roles:
  research:
    engine: codex
    model: gpt-5.4
  develop:
    engine: codex
    model: gpt-5.4
target:
  files: ["README.md"]
local_validation:
  command: test -f README.md
`)

	binDir := makeDetectedPresetPath(t)
	tmuxPath := filepath.Join(binDir, "tmux")
	script := `#!/bin/sh
set -eu
cmd="$1"
shift
case "$cmd" in
  has-session)
    exit 1
    ;;
  new-session|kill-session|send-keys)
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error {
		return nil
	}

	if err := Start(repo, []string{"ship it"}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, goalx.Slugify("ship it"))
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want codex/gpt-5.4", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Research.Engine != "codex" || cfg.Roles.Research.Model != "gpt-5.4" {
		t.Fatalf("research = %s/%s, want codex/gpt-5.4", cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "gpt-5.4" {
		t.Fatalf("develop = %s/%s, want codex/gpt-5.4", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
}

func TestStartAddsClaudeMasterInboxHook(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "claude-master",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "claude-code",
			Model:  "opus",
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
cmd="$1"
shift
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
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	hooksPath := filepath.Join(RunWorktreePath(runDir), ".claude", "hooks.json")
	data, err = os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var doc struct {
		Hooks []struct {
			Event   string `json:"event"`
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal hooks.json: %v", err)
	}
	if len(doc.Hooks) != 1 {
		t.Fatalf("len(Hooks) = %d, want 1", len(doc.Hooks))
	}
	if doc.Hooks[0].Event != "Stop" {
		t.Fatalf("hook event = %q, want Stop", doc.Hooks[0].Event)
	}
	for _, want := range []string{
		"INBOX PENDING",
		MasterInboxPath(runDir),
		MasterCursorPath(runDir),
	} {
		if !strings.Contains(doc.Hooks[0].Command, want) {
			t.Fatalf("hook command missing %q:\n%s", want, doc.Hooks[0].Command)
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
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
		},
		Preferences: goalx.PreferencesConfig{
			Research: goalx.PreferencePolicy{
				Guidance: "multi-perspective",
			},
			Develop: goalx.PreferencePolicy{
				Guidance: "speed",
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

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

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
		"### Preferences",
		"| Research | multi-perspective |",
		"| Develop | speed |",
		"Prefer policy-based session launches.",
		"Explicit `--engine/--model` is an override.",
		"Liveness state: `" + LivenessPath(runDir) + "`",
		"Worktree snapshot: `" + WorktreeSnapshotPath(runDir) + "`",
		"Sessions without dedicated worktrees share the run worktree.",
		"Use `goalx add --worktree` for parallel isolation.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("master.md missing %q:\n%s", want, text)
		}
	}
}

func TestStartLaunchesMasterWithCurrentProcessEnvWithoutSnapshot(t *testing.T) {
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
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
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
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", binDir+":"+origPath)
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
	runDir := goalx.RunDir(repo, cfg.Name)
	runWT := RunWorktreePath(runDir)
	logText := string(logData)
	for _, want := range []string{
		"new-session -d -s " + goalx.TmuxSessionName(repo, cfg.Name) + " -n master -c " + runWT + " env ",
		"/bin/bash -c ",
		"lease-loop --run",
		"--holder",
		"master",
		"FOO_TOOLCHAIN_ROOT='/opt/goalx-toolchain'",
		"HOME='" + home + "'",
		"PATH='" + binDir + ":" + origPath + "'",
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
	if _, err := os.Stat(filepath.Join(runDir, "control", "launch-env.json")); !os.IsNotExist(err) {
		t.Fatalf("launch env snapshot should not be written, stat err = %v", err)
	}
	if _, err := os.Stat(runWT); err != nil {
		t.Fatalf("run worktree missing: %v", err)
	}
}

func TestStartAutoSnapshotsTrackedChangesIntoRunWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("dirty tracked\n"), 0o644); err != nil {
		t.Fatalf("write dirty README: %v", err)
	}

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:            "snapshot-demo",
		Mode:            goalx.ModeDevelop,
		Objective:       "ship feature",
		Target:          goalx.TargetConfig{Files: []string{"README.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
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
cmd="$1"
shift
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
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if meta.SnapshotCommit == "" {
		t.Fatal("SnapshotCommit empty, want auto-snapshot commit hash")
	}
	head := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	if meta.SnapshotCommit != head {
		t.Fatalf("SnapshotCommit = %q, want HEAD %q", meta.SnapshotCommit, head)
	}
	if got := strings.TrimSpace(gitOutput(t, repo, "log", "-1", "--pretty=%s")); got != "goalx: snapshot before snapshot-demo" {
		t.Fatalf("latest commit subject = %q", got)
	}

	runWT := RunWorktreePath(runDir)
	data, err = os.ReadFile(filepath.Join(runWT, "README.md"))
	if err != nil {
		t.Fatalf("read run worktree README: %v", err)
	}
	if string(data) != "dirty tracked\n" {
		t.Fatalf("run worktree README = %q, want dirty tracked contents", string(data))
	}
}

func TestStartCopiesGitignoredFilesToRunWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	writeAndCommit(t, repo, ".gitignore", "CLAUDE.md\ndocs/\n", "add ignore rules")
	writeTestFile(t, repo, "CLAUDE.md", "master instructions\n")
	writeTestFile(t, repo, "docs/plan.md", "mirror plan\n")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:            "mirror-demo",
		Mode:            goalx.ModeDevelop,
		Objective:       "mirror ignored files",
		Target:          goalx.TargetConfig{Files: []string{"README.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "gpt-5.4",
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
cmd="$1"
shift
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
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runWT := RunWorktreePath(goalx.RunDir(repo, cfg.Name))
	for path, want := range map[string]string{
		"CLAUDE.md":    "master instructions\n",
		"docs/plan.md": "mirror plan\n",
	} {
		data, err := os.ReadFile(filepath.Join(runWT, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", path, string(data), want)
		}
	}
}
