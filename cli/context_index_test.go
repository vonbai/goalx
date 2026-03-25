package cli

import (
	"encoding/json"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildContextIndexIncludesRunAnchors(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}

	if index.ProjectRoot != repo {
		t.Fatalf("project_root = %q, want %q", index.ProjectRoot, repo)
	}
	if index.RunDir != runDir {
		t.Fatalf("run_dir = %q, want %q", index.RunDir, runDir)
	}
	if index.RunWorktree != RunWorktreePath(runDir) {
		t.Fatalf("run_worktree = %q, want %q", index.RunWorktree, RunWorktreePath(runDir))
	}
	if index.CharterPath != RunCharterPath(runDir) {
		t.Fatalf("charter_path = %q, want %q", index.CharterPath, RunCharterPath(runDir))
	}
	if index.Master.Engine != "codex" || index.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %+v, want codex/gpt-5.4", index.Master)
	}
}

func TestContextIndexIncludesSessionRoster(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}

	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	session := index.Sessions[0]
	if session.Name != "session-1" {
		t.Fatalf("session name = %q, want session-1", session.Name)
	}
	if session.InboxPath != ControlInboxPath(runDir, "session-1") {
		t.Fatalf("session inbox = %q, want %q", session.InboxPath, ControlInboxPath(runDir, "session-1"))
	}
	if session.WorktreePath != WorktreePath(runDir, cfg.Name, 1) {
		t.Fatalf("session worktree = %q, want %q", session.WorktreePath, WorktreePath(runDir, cfg.Name, 1))
	}
}

func TestContextIndexUsesRunWorktreeForSharedSession(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	sessionName := "session-1"
	if err := EnsureSessionControl(runDir, sessionName); err != nil {
		t.Fatalf("EnsureSessionControl: %v", err)
	}
	identity := &SessionIdentity{
		Version:         1,
		SessionName:     sessionName,
		RoleKind:        "develop",
		Mode:            string(goalx.ModeDevelop),
		Engine:          "codex",
		Model:           "gpt-5.4-mini",
		OriginCharterID: loadCharterIDForTests(t, runDir),
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, sessionName), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         sessionName,
		State:        "active",
		Mode:         string(goalx.ModeDevelop),
		WorktreePath: "",
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	if index.Sessions[0].WorktreePath != RunWorktreePath(runDir) {
		t.Fatalf("shared session worktree = %q, want %q", index.Sessions[0].WorktreePath, RunWorktreePath(runDir))
	}
}

func TestContextIndexExcludesRawEnvSnapshot(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, unwanted := range []string{"raw_env_path", "captured_path"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("context index should not expose %q:\n%s", unwanted, text)
		}
	}
}
