package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestHasUrgentUnreadReturnsTrueForUnreadUrgentMessage(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "drop everything and triage", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	if !hasUrgentUnread(runDir) {
		t.Fatal("hasUrgentUnread = false, want true")
	}
}

func TestHasUrgentUnreadReturnsFalseWhenNoUrgent(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "normal priority message"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}

	if hasUrgentUnread(runDir) {
		t.Fatal("hasUrgentUnread = true, want false")
	}
}

func TestRunSidecarTickSendsEscapeOnUrgentUnread(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "drop everything and triage", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
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

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()
	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
	if gotTarget != wantTarget || gotEngine != "codex" {
		t.Fatalf("sendAgentNudge target=%q engine=%q, want %q codex", gotTarget, gotEngine, wantTarget)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "send-keys -t "+wantTarget+" Escape") {
		t.Fatalf("tmux log missing urgent Escape interrupt:\n%s", string(logData))
	}
}
