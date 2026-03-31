package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestNextSessionIndexStartsAtOneWithNoJournals(t *testing.T) {
	runDir := t.TempDir()

	got, err := nextSessionIndex(runDir)
	if err != nil {
		t.Fatalf("nextSessionIndex: %v", err)
	}
	if got != 1 {
		t.Fatalf("nextSessionIndex = %d, want 1", got)
	}
}

func TestNextSessionIndexSkipsOccupiedWorktreeSlot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	runName := "slot-run"
	runDir := writeRunSpecFixture(t, projectRoot, &goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	})

	if err := os.MkdirAll(WorktreePath(runDir, runName, 1), 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	got, err := nextSessionIndex(runDir)
	if err != nil {
		t.Fatalf("nextSessionIndex: %v", err)
	}
	if got != 2 {
		t.Fatalf("nextSessionIndex = %d, want 2", got)
	}
}

func TestNextAvailableSessionIndexSkipsOccupiedWorktreeSlot(t *testing.T) {
	projectRoot := initGitRepo(t)
	runName := "slot-run"
	runDir := writeRunSpecFixture(t, projectRoot, &goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	})

	if err := os.MkdirAll(WorktreePath(runDir, runName, 2), 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	got, err := nextAvailableSessionIndex(projectRoot, runDir, runName)
	if err != nil {
		t.Fatalf("nextAvailableSessionIndex: %v", err)
	}
	if got != 3 {
		t.Fatalf("nextAvailableSessionIndex = %d, want 3", got)
	}
}

func TestNextAvailableSessionIndexSkipsSessionIdentityWithoutWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, projectRoot)
	if err := os.Remove(JournalPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session journal: %v", err)
	}
	if err := os.RemoveAll(WorktreePath(runDir, runName, 1)); err != nil {
		t.Fatalf("remove session worktree: %v", err)
	}

	got, err := nextAvailableSessionIndex(projectRoot, runDir, runName)
	if err != nil {
		t.Fatalf("nextAvailableSessionIndex: %v", err)
	}
	if got != 2 {
		t.Fatalf("nextAvailableSessionIndex = %d, want 2", got)
	}
}

func TestNextAvailableSessionIndexSkipsConfiguredProjectWorktreeSlot(t *testing.T) {
	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")

	runName := "slot-run"
	cfg := &goalx.Config{
		Name:         runName,
		Mode:         goalx.ModeWorker,
		Objective:    "ship feature",
		WorktreeRoot: ".worktrees",
		Master:       goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, projectRoot, cfg)
	if _, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}

	if err := os.MkdirAll(WorktreePath(runDir, runName, 2), 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	got, err := nextAvailableSessionIndex(projectRoot, runDir, runName)
	if err != nil {
		t.Fatalf("nextAvailableSessionIndex: %v", err)
	}
	if got != 3 {
		t.Fatalf("nextAvailableSessionIndex = %d, want 3", got)
	}
}
