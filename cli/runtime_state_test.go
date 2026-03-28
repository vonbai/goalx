package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestSaveRunRuntimeStateDoesNotPersistRecommendationField(t *testing.T) {
	runDir := t.TempDir()
	path := RunRuntimeStatePath(runDir)
	if err := SaveRunRuntimeState(path, &RunRuntimeState{
		Version:   1,
		Run:       "demo",
		Mode:      "develop",
		Active:    true,
		Phase:     "working",
		UpdatedAt: "2026-03-26T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read run runtime state: %v", err)
	}
	text := string(data)
	if strings.Contains(text, `"recommendation"`) {
		t.Fatalf("run runtime state should not persist recommendation:\n%s", text)
	}
}

func TestSnapshotSessionRuntimeDoesNotProjectAckSessionAsLifecycleState(t *testing.T) {
	runDir := t.TempDir()
	worktreePath := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(JournalPath(runDir, "session-1")), 0o755); err != nil {
		t.Fatalf("mkdir journals dir: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte("{\"round\":2,\"status\":\"ack-session\",\"desc\":\"read inbox\",\"owner_scope\":\"fix queue drift\"}\n"), 0o644); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	snapshot, err := SnapshotSessionRuntime(runDir, "session-1", worktreePath)
	if err != nil {
		t.Fatalf("SnapshotSessionRuntime: %v", err)
	}
	if snapshot.State != "" {
		t.Fatalf("snapshot state = %q, want empty for control-only ack status", snapshot.State)
	}
	if snapshot.LastJournalState != "ack-session" {
		t.Fatalf("last journal state = %q, want ack-session", snapshot.LastJournalState)
	}
	if snapshot.OwnerScope != "fix queue drift" {
		t.Fatalf("owner scope = %q, want fix queue drift", snapshot.OwnerScope)
	}
	if snapshot.LastRound != 2 {
		t.Fatalf("last round = %d, want 2", snapshot.LastRound)
	}
}

func TestRefreshSessionRuntimeProjectionPreservesParkedState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")

	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "parked",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte("{\"round\":3,\"status\":\"progress\",\"desc\":\"still working\",\"owner_scope\":\"ui slice\"}\n"), 0o644); err != nil {
		t.Fatalf("write journal: %v", err)
	}

	if err := RefreshSessionRuntimeProjection(runDir, runName); err != nil {
		t.Fatalf("RefreshSessionRuntimeProjection: %v", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := state.Sessions["session-1"].State; got != "parked" {
		t.Fatalf("session-1 state = %q, want parked", got)
	}
}
