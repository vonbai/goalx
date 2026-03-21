package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
    target="$2"
    if [ "${GOALX_FAKE_TMUX_FAIL_TARGET:-}" = "$target" ]; then
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
	t.Setenv("GOALX_FAKE_TMUX_FAIL_TARGET", tmuxSess+":master")

	err = Start(repo, nil)
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
