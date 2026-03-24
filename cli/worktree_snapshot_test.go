package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestSnapshotWorktreesReportsDirtyRootAndSessionStats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "one\ntwo\n", "base commit")
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
	if err := CreateWorktree(runWT, sessionWT, "goalx/"+cfg.Name+"/1"); err != nil {
		t.Fatalf("CreateWorktree session: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Branch:       "goalx/" + cfg.Name + "/1",
		WorktreePath: sessionWT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	if err := os.WriteFile(filepath.Join(runWT, "README.md"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatalf("write run README: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sessionWT, "README.md"), []byte("one\n"), 0o644); err != nil {
		t.Fatalf("write session README: %v", err)
	}

	snapshot, err := SnapshotWorktrees(runDir)
	if err != nil {
		t.Fatalf("SnapshotWorktrees: %v", err)
	}
	if snapshot.Root.DirtyFiles != 1 || snapshot.Root.Insertions != 1 || snapshot.Root.Deletions != 0 {
		t.Fatalf("unexpected root snapshot: %+v", snapshot.Root)
	}
	session := snapshot.Sessions["session-1"]
	if session.DirtyFiles != 1 || session.Insertions != 0 || session.Deletions != 1 {
		t.Fatalf("unexpected session snapshot: %+v", session)
	}
}

func TestRunSidecarTickWritesWorktreeSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
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

	snapshot, err := LoadWorktreeSnapshot(WorktreeSnapshotPath(runDir))
	if err != nil {
		t.Fatalf("LoadWorktreeSnapshot: %v", err)
	}
	if snapshot == nil || snapshot.CheckedAt == "" {
		t.Fatalf("expected worktree snapshot to be written, got %+v", snapshot)
	}
}
