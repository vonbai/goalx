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
	charter, err := NewRunCharter(runDir, "demo", "demo objective", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	goal := NewGoalState()
	goal.Required = []GoalItem{{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateClaimed, EvidencePaths: []string{"/tmp/evidence.txt"}}}
	if err := writeBoundaryFixture(t, runDir, goal); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := writeAssuranceFixture(t, runDir, &AcceptanceState{
		Version:     2,
		GoalVersion: goal.Version,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Command: "go test ./...", State: acceptanceCheckStateActive},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	coord := &CoordinationState{Version: 1}
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
	if fence.CharterHash == "" || fence.ObligationModelHash == "" || fence.AssurancePlanHash == "" || fence.CoordinationHash == "" {
		t.Fatalf("fence hashes must be populated: %+v", fence)
	}
	wantCharterHash, err := hashFileContents(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents charter: %v", err)
	}
	if fence.CharterHash != wantCharterHash {
		t.Fatalf("CharterHash = %q, want %q", fence.CharterHash, wantCharterHash)
	}
	wantGoalHash, err := hashFileContents(ObligationModelPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents goal: %v", err)
	}
	if fence.ObligationModelHash != wantGoalHash {
		t.Fatalf("ObligationModelHash = %q, want %q", fence.ObligationModelHash, wantGoalHash)
	}
	wantAcceptanceHash, err := hashFileContents(AssurancePlanPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents assurance plan: %v", err)
	}
	if fence.AssurancePlanHash != wantAcceptanceHash {
		t.Fatalf("AssurancePlanHash = %q, want %q", fence.AssurancePlanHash, wantAcceptanceHash)
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
	if reloaded.CharterHash != fence.CharterHash || reloaded.ObligationModelHash != fence.ObligationModelHash || reloaded.AssurancePlanHash != fence.AssurancePlanHash || reloaded.CoordinationHash != fence.CoordinationHash {
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

func TestRefreshIdentityFenceDetectsChangedGoalContent(t *testing.T) {
	runDir := t.TempDir()
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
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	charter, err := NewRunCharter(runDir, "demo", "demo objective", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		t.Fatalf("hashRunCharter: %v", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata charter linkage: %v", err)
	}
	goal := NewGoalState()
	if err := writeBoundaryFixture(t, runDir, goal); err != nil {
		t.Fatalf("SaveGoalState initial: %v", err)
	}
	if err := writeAssuranceFixture(t, runDir, &AcceptanceState{
		Version:     2,
		GoalVersion: goal.Version,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Command: "go test ./...", State: acceptanceCheckStateActive},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{Version: 1}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	initial, err := NewIdentityFence(runDir, meta)
	if err != nil {
		t.Fatalf("NewIdentityFence initial: %v", err)
	}
	if err := SaveIdentityFence(IdentityFencePath(runDir), initial); err != nil {
		t.Fatalf("SaveIdentityFence initial: %v", err)
	}

	goal.Required = append(goal.Required, GoalItem{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen})
	goal.Version++
	if err := writeBoundaryFixture(t, runDir, goal); err != nil {
		t.Fatalf("SaveGoalState updated: %v", err)
	}

	updated, changed, err := RefreshIdentityFence(runDir, meta)
	if err != nil {
		t.Fatalf("RefreshIdentityFence: %v", err)
	}
	if !changed {
		t.Fatal("RefreshIdentityFence should report changed goal content")
	}
	if updated.ObligationModelHash == initial.ObligationModelHash {
		t.Fatalf("ObligationModelHash = %q, want change from %q", updated.ObligationModelHash, initial.ObligationModelHash)
	}
	reloaded, err := LoadIdentityFence(IdentityFencePath(runDir))
	if err != nil {
		t.Fatalf("LoadIdentityFence: %v", err)
	}
	if reloaded.ObligationModelHash != updated.ObligationModelHash {
		t.Fatalf("reloaded fence obligation hash = %q, want %q", reloaded.ObligationModelHash, updated.ObligationModelHash)
	}
}
