package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestAttachUsesSavedTmuxLocatorSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "attach-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveTmuxLocator(TmuxLocatorPath(runDir), &TmuxLocator{
		Version: 1,
		Session: "gx-custom-attach",
	}); err != nil {
		t.Fatalf("SaveTmuxLocator: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "TMPDIR=$TMUX_TMPDIR $@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  attach-session)
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
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := Attach(repo, []string{"--run", cfg.Name, "master"}); err != nil {
		t.Fatalf("Attach: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	if !strings.Contains(logText, "attach-session -t gx-custom-attach:master") {
		t.Fatalf("attach should target saved tmux locator session:\n%s", string(logData))
	}
	if !strings.Contains(logText, "TMPDIR="+resolveRunTmuxSocketDir(repo, runDir, cfg.Name)) {
		t.Fatalf("attach should use run-owned TMUX_TMPDIR:\n%s", logText)
	}
}
