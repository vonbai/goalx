package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestVerifyBootstrapsAssurancePlanFromConfigAndWritesEvidenceLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(ReportsDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir reports dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-run
mode: worker
objective: ship feature
target:
  files: ["README.md"]
local_validation:
  command: "test -f DOES-NOT-EXIST"
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	plan, err := LoadAssurancePlan(AssurancePlanPath(runDir))
	if err != nil {
		t.Fatalf("read assurance plan: %v", err)
	}
	if plan == nil || len(plan.Scenarios) != 1 {
		t.Fatalf("assurance plan = %+v, want one scenario", plan)
	}
	if plan.Scenarios[0].Harness.Command != "printf 'e2e ok\n'" {
		t.Fatalf("assurance plan command = %q, want printf e2e ok", plan.Scenarios[0].Harness.Command)
	}
	events, err := LoadEvidenceLog(EvidenceLogPath(runDir))
	if err != nil {
		t.Fatalf("read evidence log: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("evidence events = %d, want 1", len(events))
	}
}

func TestVerifyUsesAssurancePlanAndWritesEvidenceLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), []byte(`name: verify-run
mode: worker
objective: ship feature
`), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	if err := writeBoundaryFixture(t, runDir, &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: "user", Role: "outcome", State: "claimed", EvidencePaths: []string{"/tmp/e2e.txt"}},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveAssurancePlan(AssurancePlanPath(runDir), &AssurancePlan{
		Version:        1,
		ObligationRefs: []string{"obl-1"},
		Scenarios: []AssuranceScenario{
			{
				ID:                "scenario-cli-first-run",
				CoversObligations: []string{"obl-1"},
				Harness:           AssuranceHarness{Kind: "cli", Command: "printf 'ok\\n'"},
				Oracle: AssuranceOracle{
					Kind:             "exit_code",
					CheckDefinitions: []AssuranceOracleCheck{{Kind: "exit_code", Equals: "0"}},
				},
				Evidence:   []AssuranceEvidenceRequirement{{Kind: "stdout"}},
				GatePolicy: AssuranceGatePolicy{VerifyLane: "required", RequiredCognitionTier: "repo-native", Closeout: "required"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveAssurancePlan: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	events, err := LoadEvidenceLog(EvidenceLogPath(runDir))
	if err != nil {
		t.Fatalf("LoadEvidenceLog: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("evidence events = %#v, want one", events)
	}
	var body EvidenceEventBody
	if err := decodeStrictJSON(events[0].Body, &body); err != nil {
		t.Fatalf("decode event body: %v", err)
	}
	if body.ScenarioID != "scenario-cli-first-run" || body.HarnessKind != "cli" {
		t.Fatalf("event body = %+v, want scenario-cli-first-run/cli", body)
	}
}

func TestVerifyRunsAcceptanceInsideRunWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	snapshot := []byte(`name: verify-run
mode: worker
objective: ship feature
acceptance:
  command: "test -f run-worktree-only.txt"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	if err := writeBoundaryFixture(t, runDir, &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateClaimed, EvidencePaths: []string{"/tmp/e2e.txt"}},
		},
	}); err != nil {
		t.Fatalf("write boundary fixture: %v", err)
	}

	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runWT, "run-worktree-only.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write run-worktree-only.txt: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyRequiresAcceptanceChecks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	snapshot := []byte(`name: verify-run
mode: worker
objective: ship feature
target:
  files: ["README.md"]
local_validation:
  command: "test -f DOES-NOT-EXIST"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "role": "outcome",
      "state": "claimed",
      "evidence_paths": ["/tmp/e2e.txt"]
    }
  ],
  "optional": []
}`)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	var goalState GoalState
	if err := json.Unmarshal(goal, &goalState); err != nil {
		t.Fatalf("unmarshal goal state: %v", err)
	}
	if err := writeBoundaryFixture(t, runDir, &goalState); err != nil {
		t.Fatalf("write boundary fixture: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "no assurance scenarios configured") {
		t.Fatalf("Verify error = %v, want missing acceptance checks", err)
	}

}

func TestVerifyDoesNotRewriteGoalState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-goal-readonly"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	snapshot := []byte(`name: verify-goal-readonly
mode: worker
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	goalBefore := []byte(`{
  "version": 1,
  "updated_at": "2026-03-27T00:00:00Z",
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "role": "outcome",
      "state": "claimed",
      "evidence_paths": ["/tmp/e2e.txt"]
    }
  ],
  "optional": []
}`)
	var goalState GoalState
	if err := json.Unmarshal(goalBefore, &goalState); err != nil {
		t.Fatalf("unmarshal goal state: %v", err)
	}
	if err := writeBoundaryFixture(t, runDir, &goalState); err != nil {
		t.Fatalf("write boundary fixture: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	after, err := os.ReadFile(ObligationModelPath(runDir))
	if err != nil {
		t.Fatalf("read obligation model: %v", err)
	}
	if len(after) == 0 {
		t.Fatal("obligation model unexpectedly empty")
	}
}

func TestVerifyDoesNotGateOnMissingReportEvidenceManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-research-manifest-missing"
	runDir := seedResearchVerifyRun(t, repo, runName)

	if err := os.WriteFile(filepath.Join(ReportsDir(runDir), "architecture-options-comparison.md"), []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyDoesNotGateOnMalformedReportEvidenceManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-research-manifest-invalid"
	runDir := seedResearchVerifyRun(t, repo, runName)

	reportPath := filepath.Join(ReportsDir(runDir), "architecture-options-comparison.md")
	if err := os.WriteFile(reportPath, []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(ReportEvidenceManifestPath(reportPath), []byte(fmt.Sprintf(`{
  "version": 1,
  "report_path": %q,
  "covers": ["ucl-research"],
  "repo_evidence_paths": [],
  "external_refs": [],
  "unexpected": true
}`, reportPath)), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyDoesNotGateOnMissingExternalRefsInReportEvidenceManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-research-external-refs-missing"
	runDir := seedResearchVerifyRun(t, repo, runName)

	reportPath := filepath.Join(ReportsDir(runDir), "architecture-options-comparison.md")
	evidencePath := filepath.Join(runDir, "source.txt")
	if err := os.WriteFile(reportPath, []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(evidencePath, []byte("evidence\n"), 0o644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	if err := os.WriteFile(ReportEvidenceManifestPath(reportPath), []byte(fmt.Sprintf(`{
  "version": 1,
  "report_path": %q,
  "covers": ["ucl-research"],
  "repo_evidence_paths": [%q],
  "external_refs": []
}`, reportPath, evidencePath)), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyResearchPassesWithStructuredEvidence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-research-structured"
	runDir := seedResearchVerifyRun(t, repo, runName)

	reportPath := filepath.Join(ReportsDir(runDir), "architecture-options-comparison.md")
	evidencePath := filepath.Join(runDir, "source.txt")
	if err := os.WriteFile(reportPath, []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(evidencePath, []byte("evidence\n"), 0o644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	if err := os.WriteFile(ReportEvidenceManifestPath(reportPath), []byte(fmt.Sprintf(`{
  "version": 1,
  "report_path": %q,
  "covers": ["ucl-research"],
  "repo_evidence_paths": [%q],
  "external_refs": ["https://example.com/reference"]
}`, reportPath, evidencePath)), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func seedResearchVerifyRun(t *testing.T, repo, runName string) string {
	t.Helper()

	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(ReportsDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir reports dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: ` + runName + `
mode: worker
objective: compare external reference architectures
acceptance:
  command: "printf 'research e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	contract := &ObjectiveContract{
		Version:       1,
		ObjectiveHash: "sha256:research",
		State:         objectiveContractStateLocked,
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-research",
				Text:             "compare external reference architectures",
				Kind:             objectiveClauseKindVerification,
				SourceExcerpt:    "compare external reference architectures",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
		},
	}
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), contract); err != nil {
		t.Fatalf("SaveObjectiveContract: %v", err)
	}
	goal := &GoalState{
		Version: 1,
		Required: []GoalItem{
			{
				ID:            "req-1",
				Text:          "compare external reference architectures",
				Source:        goalItemSourceUser,
				Role:          goalItemRoleOutcome,
				State:         goalItemStateClaimed,
				Covers:        []string{"ucl-research"},
				EvidencePaths: []string{ensureSharedProofEvidence(t)},
			},
		},
		Optional: []GoalItem{},
	}
	if err := writeBoundaryFixture(t, runDir, goal); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "compare external reference architectures",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)

	return runDir
}

func TestVerifyRecordsAssuranceEvidenceWhenRunChangedCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	ensureSharedProofEvidence(t)

	runName := "verify-code-changed"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-code-changed
mode: worker
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: baseRevision,
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)

	writeAndCommit(t, repo, "feature.txt", "feature", "run change")

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	events, err := LoadEvidenceLog(EvidenceLogPath(runDir))
	if err != nil {
		t.Fatalf("read evidence log: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("evidence events = %d, want 1", len(events))
	}
}

func seedRunCharterForTests(t *testing.T, runDir, runName, projectRoot string) {
	t.Helper()

	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if meta == nil {
		t.Fatal("run metadata missing")
	}
	if meta.ProtocolVersion == 0 {
		meta.ProtocolVersion = 2
	}
	if meta.ProjectRoot == "" {
		meta.ProjectRoot = projectRoot
	}
	if meta.RunID == "" {
		meta.RunID = newRunID()
	}
	if meta.RootRunID == "" {
		meta.RootRunID = meta.RunID
	}
	if meta.Epoch == 0 {
		meta.Epoch = 1
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata normalize: %v", err)
	}
	charter, err := NewRunCharter(runDir, runName, "", meta)
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
}
