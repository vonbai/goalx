package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestEnsureGoalContractStateCreatesDefaultFile(t *testing.T) {
	runDir := t.TempDir()

	state, err := EnsureGoalContractState(runDir, "ship auth flow")
	if err != nil {
		t.Fatalf("EnsureGoalContractState: %v", err)
	}
	if state.Objective != "ship auth flow" {
		t.Fatalf("objective = %q, want ship auth flow", state.Objective)
	}
	if state.Version != 1 {
		t.Fatalf("version = %d, want 1", state.Version)
	}
	if len(state.Items) != 0 {
		t.Fatalf("items = %#v, want empty", state.Items)
	}
	if _, err := os.Stat(GoalContractPath(runDir)); err != nil {
		t.Fatalf("goal contract file missing: %v", err)
	}
}

func TestVerifyFailsWhenGoalContractHasUnfinishedRequiredItems(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-contract"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-contract
mode: develop
objective: ship feature
target:
  files: ["README.md"]
harness:
  command: "printf 'gate ok\n'"
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	contract := []byte(`{
  "version": 1,
  "objective": "ship feature",
  "items": [
    {
      "id": "req-1",
      "kind": "user_required",
      "requirement": "ship the feature end to end",
      "status": "delegated"
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "goal contract") {
		t.Fatalf("Verify error = %v, want goal contract failure", err)
	}

	stateData, readErr := os.ReadFile(filepath.Join(runDir, "acceptance.json"))
	if readErr != nil {
		t.Fatalf("read acceptance state: %v", readErr)
	}
	stateText := string(stateData)
	for _, want := range []string{
		`"status": "failed"`,
		`"command_source": "acceptance"`,
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}

	evidenceData, readErr := os.ReadFile(filepath.Join(runDir, "acceptance-last.txt"))
	if readErr != nil {
		t.Fatalf("read acceptance evidence: %v", readErr)
	}
	if !strings.Contains(string(evidenceData), "goal contract") {
		t.Fatalf("acceptance evidence missing goal contract failure:\n%s", evidenceData)
	}

	statusData, readErr := os.ReadFile(ProjectStatusCachePath(repo))
	if readErr != nil {
		t.Fatalf("read status.json: %v", readErr)
	}
	statusText := string(statusData)
	for _, want := range []string{
		`"goal_contract_status":"pending"`,
		`"goal_required_remaining":1`,
	} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("status.json missing %q:\n%s", want, statusText)
		}
	}
}

func TestVerifyPassesWhenRequiredGoalContractItemsAreResolved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-contract-pass"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-contract-pass
mode: develop
objective: ship feature
target:
  files: ["README.md"]
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	contract := []byte(`{
  "version": 1,
  "objective": "ship feature",
  "items": [
    {
      "id": "req-1",
      "kind": "user_required",
      "requirement": "ship the feature end to end",
      "status": "done",
      "satisfaction_basis": "preexisting",
      "evidence": ["/tmp/e2e.txt"],
      "evidence_class": "artifact",
      "counter_evidence": ["checked current HEAD for missing end-to-end path"],
      "semantic_match": "exact"
    },
    {
      "id": "req-2",
      "kind": "goal_necessary",
      "requirement": "document migration path",
      "status": "waived",
      "user_approved": true
    },
    {
      "id": "enh-1",
      "kind": "goal_enhancement",
      "requirement": "polish UX copy",
      "status": "queued"
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	statusData, err := os.ReadFile(ProjectStatusCachePath(repo))
	if err != nil {
		t.Fatalf("read status.json: %v", err)
	}
	statusText := string(statusData)
	for _, want := range []string{
		`"acceptance_status":"passed"`,
		`"goal_contract_status":"satisfied"`,
		`"goal_required_remaining":0`,
		`"goal_enhancement_open":1`,
	} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("status.json missing %q:\n%s", want, statusText)
		}
	}
}

func TestVerifyFailsWhenRequiredGoalContractItemLacksSatisfactionBasis(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-contract-basis"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-contract-basis
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	contract := []byte(`{
  "version": 1,
  "objective": "ship feature",
  "items": [
    {
      "id": "req-1",
      "kind": "user_required",
      "requirement": "ship the feature end to end",
      "status": "done",
      "evidence": ["/tmp/e2e.txt"],
      "evidence_class": "artifact",
      "counter_evidence": ["checked current HEAD for missing end-to-end path"],
      "semantic_match": "exact"
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "satisfaction_basis") {
		t.Fatalf("Verify error = %v, want satisfaction_basis failure", err)
	}
}

func TestVerifyFailsWhenRunChangeClaimHasNoChangesSinceRunStart(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName := "verify-contract-run-change"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-contract-run-change
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	contract := []byte(`{
  "version": 1,
  "objective": "ship feature",
  "items": [
    {
      "id": "req-1",
      "kind": "user_required",
      "requirement": "ship the feature end to end",
      "status": "done",
      "satisfaction_basis": "run_change",
      "evidence": ["/tmp/e2e.txt"]
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "run_change") {
		t.Fatalf("Verify error = %v, want run_change consistency failure", err)
	}
}
