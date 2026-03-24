package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestScanLivenessReportsHealthyMasterAndSession(t *testing.T) {
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

	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+cfg.Name+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	sessionWT := WorktreePath(runDir, cfg.Name, 1)
	if err := os.MkdirAll(sessionWT, 0o755); err != nil {
		t.Fatalf("mkdir session worktree: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		WorktreePath: sessionWT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease session: %v", err)
	}

	state, err := ScanLiveness(runDir)
	if err != nil {
		t.Fatalf("ScanLiveness: %v", err)
	}
	if state.Master.Lease != "healthy" || !state.Master.PIDAlive || !state.Master.HasWorktree {
		t.Fatalf("unexpected master liveness: %+v", state.Master)
	}
	session := state.Sessions["session-1"]
	if session.Lease != "healthy" || !session.PIDAlive || !session.HasWorktree {
		t.Fatalf("unexpected session liveness: %+v", session)
	}
}

func TestScanLivenessNotifiesMasterWhenSessionDies(t *testing.T) {
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

	sessionWT := WorktreePath(runDir, cfg.Name, 1)
	if err := os.MkdirAll(sessionWT, 0o755); err != nil {
		t.Fatalf("mkdir session worktree: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		WorktreePath: sessionWT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if err := SaveControlLease(ControlLeasePath(runDir, "session-1"), &ControlLease{
		Version:   1,
		Holder:    "session-1",
		RunID:     meta.RunID,
		Epoch:     meta.Epoch,
		RenewedAt: time.Now().UTC().Add(-2 * time.Minute).Format(time.RFC3339),
		ExpiresAt: time.Now().UTC().Add(-time.Minute).Format(time.RFC3339),
		PID:       999999,
		Transport: "tmux",
	}); err != nil {
		t.Fatalf("SaveControlLease: %v", err)
	}

	state, err := ScanLiveness(runDir)
	if err != nil {
		t.Fatalf("ScanLiveness: %v", err)
	}
	session := state.Sessions["session-1"]
	if session.Lease != "expired" || session.PIDAlive || !session.HasWorktree {
		t.Fatalf("unexpected session liveness: %+v", session)
	}

	inbox, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	text := string(inbox)
	for _, want := range []string{`"type":"session-died"`, `session-1`} {
		if !strings.Contains(text, want) {
			t.Fatalf("master inbox missing %q:\n%s", want, text)
		}
	}
}

func TestRunSidecarTickWritesLivenessState(t *testing.T) {
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
	if err := CreateWorktree(repo, RunWorktreePath(runDir), "goalx/"+cfg.Name+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	state, err := LoadLivenessState(LivenessPath(runDir))
	if err != nil {
		t.Fatalf("LoadLivenessState: %v", err)
	}
	if state == nil || state.CheckedAt == "" {
		t.Fatalf("expected liveness state to be written, got %+v", state)
	}
	if !state.Master.HasWorktree {
		t.Fatalf("expected master liveness to report run worktree: %+v", state.Master)
	}
}
