package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentityFencePathAndDerivation(t *testing.T) {
	runDir := t.TempDir()
	if got, want := IdentityFencePath(runDir), filepath.Join(runDir, "control", "identity-fence.json"); got != want {
		t.Fatalf("IdentityFencePath = %q, want %q", got, want)
	}

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")

	meta := &RunMetadata{
		Version:         1,
		Objective:       "ship feature",
		ProjectRoot:     repo,
		ProtocolVersion: 2,
		RunID:           "run_abc123",
		RootRunID:       "run_root123",
		Epoch:           2,
		CharterID:       "charter_abc123",
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	charter, err := NewRunCharter(runDir, "demo", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	goal := NewGoalState()
	goal.Required = []GoalItem{{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, State: goalItemStateClaimed, EvidencePaths: []string{"/tmp/evidence.txt"}}}
	if err := SaveGoalState(GoalPath(runDir), goal); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:          1,
		GoalVersion:      goal.Version,
		DefaultCommand:   "go test ./...",
		EffectiveCommand: "go test ./...",
		ChangeKind:       acceptanceChangeSame,
		LastResult:       AcceptanceResult{Status: acceptanceStatusPending},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	coord := &CoordinationState{Version: 1, Objective: "ship feature"}
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	fence, err := NewIdentityFence(runDir, meta)
	if err != nil {
		t.Fatalf("NewIdentityFence: %v", err)
	}
	if fence.RunID != meta.RunID || fence.Epoch != meta.Epoch {
		t.Fatalf("fence run identity = %+v, want run_id %q epoch %d", fence, meta.RunID, meta.Epoch)
	}
	if fence.CharterHash == "" || fence.GoalHash == "" || fence.AcceptanceHash == "" || fence.CoordinationHash == "" {
		t.Fatalf("fence hashes must be populated: %+v", fence)
	}
	wantCharterHash, err := hashFileContents(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents charter: %v", err)
	}
	if fence.CharterHash != wantCharterHash {
		t.Fatalf("CharterHash = %q, want %q", fence.CharterHash, wantCharterHash)
	}
	wantGoalHash, err := hashFileContents(GoalPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents goal: %v", err)
	}
	if fence.GoalHash != wantGoalHash {
		t.Fatalf("GoalHash = %q, want %q", fence.GoalHash, wantGoalHash)
	}
	wantAcceptanceHash, err := hashFileContents(AcceptanceStatePath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents acceptance: %v", err)
	}
	if fence.AcceptanceHash != wantAcceptanceHash {
		t.Fatalf("AcceptanceHash = %q, want %q", fence.AcceptanceHash, wantAcceptanceHash)
	}
	wantCoordinationHash, err := hashFileContents(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents coordination: %v", err)
	}
	if fence.CoordinationHash != wantCoordinationHash {
		t.Fatalf("CoordinationHash = %q, want %q", fence.CoordinationHash, wantCoordinationHash)
	}

	if err := SaveIdentityFence(IdentityFencePath(runDir), fence); err != nil {
		t.Fatalf("SaveIdentityFence: %v", err)
	}
	reloaded, err := LoadIdentityFence(IdentityFencePath(runDir))
	if err != nil {
		t.Fatalf("LoadIdentityFence: %v", err)
	}
	if reloaded == nil {
		t.Fatal("reloaded fence is nil")
	}
	if reloaded.CharterHash != fence.CharterHash || reloaded.GoalHash != fence.GoalHash || reloaded.AcceptanceHash != fence.AcceptanceHash || reloaded.CoordinationHash != fence.CoordinationHash {
		t.Fatalf("reloaded fence = %+v, want %+v", reloaded, fence)
	}
}

func TestIdentityFenceContentHashChangesWithFileContent(t *testing.T) {
	runDir := t.TempDir()
	path := filepath.Join(runDir, "content.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	first, err := hashFileContents(path)
	if err != nil {
		t.Fatalf("hashFileContents first: %v", err)
	}
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatalf("rewrite file: %v", err)
	}
	second, err := hashFileContents(path)
	if err != nil {
		t.Fatalf("hashFileContents second: %v", err)
	}
	if first == "" || second == "" {
		t.Fatal("hashes should not be empty")
	}
	if first == second {
		t.Fatalf("content hash did not change: %q", first)
	}
}
