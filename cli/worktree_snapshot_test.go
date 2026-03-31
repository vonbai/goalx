package cli

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func seedForkedWorktreeLineageFixture(t *testing.T, repo, runDir string, cfg *goalx.Config) (string, string, string) {
	t.Helper()

	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+cfg.Name+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	writeAndCommit(t, runWT, "root.txt", "root\n", "root change")

	session1WT := WorktreePath(runDir, cfg.Name, 1)
	if err := CreateWorktree(runWT, session1WT, "goalx/"+cfg.Name+"/1"); err != nil {
		t.Fatalf("CreateWorktree session-1: %v", err)
	}
	writeAndCommit(t, session1WT, "one.txt", "one\n", "session-1 change")

	session2WT := WorktreePath(runDir, cfg.Name, 2)
	if err := CreateWorktree(runWT, session2WT, "goalx/"+cfg.Name+"/2", "goalx/"+cfg.Name+"/1"); err != nil {
		t.Fatalf("CreateWorktree session-2: %v", err)
	}
	writeAndCommit(t, session2WT, "two.txt", "two\n", "session-2 change")

	for _, sess := range []struct {
		name         string
		worktreePath string
		branch       string
		baseSelector string
		baseBranch   string
	}{
		{
			name:         "session-1",
			worktreePath: session1WT,
			branch:       "goalx/" + cfg.Name + "/1",
			baseSelector: "run-root",
			baseBranch:   "goalx/" + cfg.Name + "/root",
		},
		{
			name:         "session-2",
			worktreePath: session2WT,
			branch:       "goalx/" + cfg.Name + "/2",
			baseSelector: "session-1",
			baseBranch:   "goalx/" + cfg.Name + "/1",
		},
	} {
		if err := EnsureSessionControl(runDir, sess.name); err != nil {
			t.Fatalf("EnsureSessionControl %s: %v", sess.name, err)
		}
		if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
			Name:         sess.name,
			State:        "active",
			Mode:         string(goalx.ModeWorker),
			Branch:       sess.branch,
			WorktreePath: sess.worktreePath,
		}); err != nil {
			t.Fatalf("UpsertSessionRuntimeState %s: %v", sess.name, err)
		}
		identity, err := NewSessionIdentity(runDir, sess.name, sessionRoleKind(goalx.ModeWorker), goalx.ModeWorker, "codex", "gpt-5.4", "", "", "", goalx.TargetConfig{})
		if err != nil {
			t.Fatalf("NewSessionIdentity %s: %v", sess.name, err)
		}
		identity.BaseBranchSelector = sess.baseSelector
		identity.BaseBranch = sess.baseBranch
		if err := SaveSessionIdentity(SessionIdentityPath(runDir, sess.name), identity); err != nil {
			t.Fatalf("SaveSessionIdentity %s: %v", sess.name, err)
		}
		if err := os.WriteFile(JournalPath(runDir, sess.name), []byte("{\"round\":1,\"status\":\"active\",\"desc\":\"working\"}\n"), 0o644); err != nil {
			t.Fatalf("write %s journal: %v", sess.name, err)
		}
	}

	return runWT, session1WT, session2WT
}

func TestSnapshotWorktreesReportsDirtyRootAndSessionStats(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "one\ntwo\n", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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

func TestSnapshotWorktreesIncludesForkedLineageFacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	_, _, _ = seedForkedWorktreeLineageFixture(t, repo, runDir, cfg)

	snapshot, err := SnapshotWorktrees(runDir)
	if err != nil {
		t.Fatalf("SnapshotWorktrees: %v", err)
	}

	if snapshot.RootLineage.ParentSelector != "source-root" {
		t.Fatalf("root parent selector = %q, want source-root", snapshot.RootLineage.ParentSelector)
	}
	if snapshot.RootLineage.AheadCommits < 1 {
		t.Fatalf("root ahead commits = %d, want >= 1", snapshot.RootLineage.AheadCommits)
	}
	session2 := snapshot.SessionLineage["session-2"]
	if session2.ParentSelector != "session-1" {
		t.Fatalf("session-2 parent selector = %q, want session-1", session2.ParentSelector)
	}
	if session2.ParentRef != "goalx/"+cfg.Name+"/1" {
		t.Fatalf("session-2 parent ref = %q, want %q", session2.ParentRef, "goalx/"+cfg.Name+"/1")
	}
	if session2.AheadCommits < 1 {
		t.Fatalf("session-2 ahead commits = %d, want >= 1", session2.AheadCommits)
	}
}

func TestRefreshRunGuidanceWritesSessionLineageSnapshot(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedForkedWorktreeLineageFixture(t, repo, runDir, cfg)

	if _, err := RefreshRunGuidance(repo, cfg.Name, runDir); err != nil {
		t.Fatalf("RefreshRunGuidance: %v", err)
	}

	snapshot, err := LoadWorktreeSnapshot(WorktreeSnapshotPath(runDir))
	if err != nil {
		t.Fatalf("LoadWorktreeSnapshot: %v", err)
	}
	if snapshot == nil || snapshot.RootLineage == nil {
		t.Fatalf("root lineage missing after refresh: %+v", snapshot)
	}
	if got := snapshot.RootLineage.ParentSelector; got != "source-root" {
		t.Fatalf("root parent selector = %q, want source-root", got)
	}
	session2, ok := snapshot.SessionLineage["session-2"]
	if !ok {
		t.Fatalf("session-2 lineage missing after refresh: %+v", snapshot.SessionLineage)
	}
	if session2.ParentSelector != "session-1" {
		t.Fatalf("session-2 parent selector = %q, want session-1", session2.ParentSelector)
	}
	if session2.ParentRef != "goalx/"+cfg.Name+"/1" {
		t.Fatalf("session-2 parent ref = %q, want %q", session2.ParentRef, "goalx/"+cfg.Name+"/1")
	}
}

func TestRunRuntimeHostTickWritesWorktreeSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	snapshot, err := LoadWorktreeSnapshot(WorktreeSnapshotPath(runDir))
	if err != nil {
		t.Fatalf("LoadWorktreeSnapshot: %v", err)
	}
	if snapshot == nil || snapshot.CheckedAt == "" {
		t.Fatalf("expected worktree snapshot to be written, got %+v", snapshot)
	}
}

func TestHashUntrackedPathHandlesDirectories(t *testing.T) {
	worktreePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktreePath, ".review", "71495365"), 0o755); err != nil {
		t.Fatalf("mkdir untracked dir: %v", err)
	}

	hasher := sha256.New()
	if err := hashUntrackedPath(hasher, worktreePath, ".review/71495365"); err != nil {
		t.Fatalf("hashUntrackedPath: %v", err)
	}
	if got := hasher.Sum(nil); len(got) == 0 {
		t.Fatal("hashUntrackedPath wrote no fingerprint data")
	}
}
