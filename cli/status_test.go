package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestStatusHelpDoesNotResolveRun(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Status(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Status --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx status [NAME] [session-N]") {
		t.Fatalf("status help output = %q", out)
	}
}

func TestStatusShowsUnreadInboxAndLegacyProtocolDrift(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	cfg := goalx.Config{
		Name:      "status-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, &cfg)
	if err := SaveProjectRegistry(repo, &ProjectRegistry{
		Version:    1,
		FocusedRun: cfg.Name,
		ActiveRuns: map[string]ProjectRunRef{
			cfg.Name: {Name: cfg.Name, State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveProjectRegistry: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, &cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "do work"); err != nil {
			t.Fatalf("AppendMasterInboxMessage: %v", err)
		}
	}
	if err := SaveMasterState(MasterStatePath(runDir), &MasterState{LastSeenID: 1, LastHeartbeatSeq: 5, HeartbeatLag: 2, WakePending: true, StaleSince: "2026-03-23T00:00:00Z"}); err != nil {
		t.Fatalf("SaveMasterState: %v", err)
	}
	if err := SaveHeartbeatState(HeartbeatStatePath(runDir), &HeartbeatState{Seq: 7}); err != nil {
		t.Fatalf("SaveHeartbeatState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Run: status-run",
		"Control:",
		"unread_inbox=2",
		"heartbeat_lag=2",
		"wake_pending=true",
		"stale=true",
		"legacy protocol",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}
