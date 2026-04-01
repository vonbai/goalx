package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	if index.TransportFactsPath != TransportFactsPath(runDir) {
		t.Fatalf("transport_facts_path = %q, want %q", index.TransportFactsPath, TransportFactsPath(runDir))
	}
	if index.ExperimentsLogPath != ExperimentsLogPath(runDir) {
		t.Fatalf("experiments_log_path = %q, want %q", index.ExperimentsLogPath, ExperimentsLogPath(runDir))
	}
	if index.IntegrationStatePath != IntegrationStatePath(runDir) {
		t.Fatalf("integration_state_path = %q, want %q", index.IntegrationStatePath, IntegrationStatePath(runDir))
	}
	if index.Master.Engine != "codex" || index.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %+v, want codex/gpt-5.4", index.Master)
	}
}

func TestBuildContextIndexIncludesImmutableRunIdentity(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	charter, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	if index.RunIdentity.RunID != meta.RunID {
		t.Fatalf("run identity run_id = %q, want %q", index.RunIdentity.RunID, meta.RunID)
	}
	if index.RunIdentity.RootRunID != meta.RootRunID {
		t.Fatalf("run identity root_run_id = %q, want %q", index.RunIdentity.RootRunID, meta.RootRunID)
	}
	if index.RunIdentity.Objective != cfg.Objective {
		t.Fatalf("run identity objective = %q, want %q", index.RunIdentity.Objective, cfg.Objective)
	}
	if index.RunIdentity.Mode != string(cfg.Mode) {
		t.Fatalf("run identity mode = %q, want %q", index.RunIdentity.Mode, cfg.Mode)
	}
	if index.RunIdentity.Intent != runIntentEvolve {
		t.Fatalf("run identity intent = %q, want %q", index.RunIdentity.Intent, runIntentEvolve)
	}
	if index.RunIdentity.RoleContracts.Master == nil || index.RunIdentity.RoleContracts.Master.Kind != "master" {
		t.Fatalf("run identity master role contract = %+v, want master contract", index.RunIdentity.RoleContracts.Master)
	}
	if charter != nil && index.RunIdentity.CharterID != charter.CharterID {
		t.Fatalf("run identity charter_id = %q, want %q", index.RunIdentity.CharterID, charter.CharterID)
	}
	if !strings.Contains(renderContextIndex(index), "Intent: `evolve`") {
		t.Fatalf("rendered context missing intent:\n%s", renderContextIndex(index))
	}
}

func TestBuildContextIndexIncludesDeclaredReadonlyBoundary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Target = goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	seedGuidanceSessionFixture(t, runDir, cfg)
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session identity missing")
	}
	identity.Target = goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}}
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if got, want := index.TargetFiles, []string{"report.md"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("target_files = %#v, want %#v", got, want)
	}
	if got, want := index.ReadonlyPaths, []string{"."}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("readonly_paths = %#v, want %#v", got, want)
	}
	if len(index.Sessions) != 1 {
		t.Fatalf("sessions = %#v, want one session", index.Sessions)
	}
	if got, want := index.Sessions[0].ReadonlyPaths, []string{"."}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("session readonly_paths = %#v, want %#v", got, want)
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Run Boundary",
		"Target files: `report.md`",
		"Readonly paths: `.`",
		"readonly paths: `.`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesDeclaredContextFilesAndRefs(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	externalContext := filepath.Join(t.TempDir(), "brief.md")
	if err := os.WriteFile(externalContext, []byte("# brief\n"), 0o644); err != nil {
		t.Fatalf("write external context: %v", err)
	}
	cfg.Context = goalx.ContextConfig{
		Files: []string{externalContext},
		Refs:  []string{"https://example.com/spec", "ticket-123"},
	}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}

	if got, want := index.ContextFiles, []string{externalContext}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("context_files = %#v, want %#v", got, want)
	}
	if got, want := index.ContextRefs, []string{"https://example.com/spec", "ticket-123"}; len(got) != len(want) || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("context_refs = %#v, want %#v", got, want)
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Declared Context",
		"Context files: `" + externalContext + "`",
		"Context refs: `https://example.com/spec`, `ticket-123`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesEvolveFactsOnlyForEvolveRuns(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`)
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:             1,
		CurrentExperimentID: "exp-1",
		CurrentBranch:       "goalx/guidance-run/root",
		CurrentCommit:       "abc123",
		UpdatedAt:           "2026-03-28T10:00:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}
	if err := RefreshEvolveFacts(runDir); err != nil {
		t.Fatalf("RefreshEvolveFacts: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.EvolveFactsPath != EvolveFactsPath(runDir) {
		t.Fatalf("evolve_facts_path = %q, want %q", index.EvolveFactsPath, EvolveFactsPath(runDir))
	}
	if index.Evolve == nil {
		t.Fatal("evolve summary missing")
	}
	if index.Evolve.FrontierState != EvolveFrontierActive {
		t.Fatalf("evolve frontier_state = %q, want %q", index.Evolve.FrontierState, EvolveFrontierActive)
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Evolve",
		"Evolve facts",
		"Frontier state: `active`",
		"Best experiment: `exp-1`",
		"Open candidate count: `0`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}

	repo2, runDir2, cfg2, _ := writeGuidanceRunFixture(t)
	index2, err := BuildContextIndex(repo2, cfg2.Name, runDir2)
	if err != nil {
		t.Fatalf("BuildContextIndex non-evolve: %v", err)
	}
	if index2.EvolveFactsPath != "" {
		t.Fatalf("non-evolve evolve_facts_path = %q, want empty", index2.EvolveFactsPath)
	}
	if index2.Evolve != nil {
		t.Fatalf("non-evolve evolve summary = %+v, want nil", index2.Evolve)
	}
	if strings.Contains(renderContextIndex(index2), "## Evolve") {
		t.Fatalf("rendered context unexpectedly exposed evolve section:\n%s", renderContextIndex(index2))
	}
}

func TestBuildContextIndexIncludesSelectionSnapshotFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	writeSelectionSnapshotFixture(t, runDir, testSelectionSnapshot{
		Version:           1,
		ExplicitSelection: true,
		Policy: goalx.EffectiveSelectionPolicy{
			DisabledEngines:  []string{"aider"},
			DisabledTargets:  []string{"claude-code/sonnet"},
			MasterCandidates: []string{"codex/gpt-5.4", "claude-code/opus"},
			WorkerCandidates: []string{"codex/gpt-5.4-mini", "codex/gpt-5.4", "claude-code/opus"},
			MasterEffort:     goalx.EffortHigh,
			WorkerEffort:     goalx.EffortMedium,
		},
		Master: goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
		Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini", Effort: goalx.EffortMedium},
	})

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.SelectionSnapshotPath != SelectionSnapshotPath(runDir) {
		t.Fatalf("selection_snapshot_path = %q, want %q", index.SelectionSnapshotPath, SelectionSnapshotPath(runDir))
	}
	if index.Selection == nil {
		t.Fatal("selection facts missing")
	}
	if len(index.Selection.MasterCandidates) != 2 || index.Selection.MasterCandidates[0] != "codex/gpt-5.4" {
		t.Fatalf("master_candidates = %#v, want codex first", index.Selection.MasterCandidates)
	}
	if len(index.Selection.DisabledTargets) != 1 || index.Selection.DisabledTargets[0] != "claude-code/sonnet" {
		t.Fatalf("disabled_targets = %#v, want claude-code/sonnet", index.Selection.DisabledTargets)
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{
		"Selection snapshot",
		"Master candidates: `codex/gpt-5.4, claude-code/opus`",
		"Worker candidates: `codex/gpt-5.4-mini, codex/gpt-5.4, claude-code/opus`",
		"Disabled targets: `claude-code/sonnet`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesGoalBoundarySummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	state := &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "deliver live capability", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "add missing backend support", Source: goalItemSourceMaster, Role: goalItemRoleEnabler, State: goalItemStateClaimed},
			{ID: "req-3", Text: "prove user journey", Source: goalItemSourceUser, Role: goalItemRoleProof, State: goalItemStateWaived, ApprovalRef: "master-inbox:1"},
		},
		Optional: []GoalItem{
			{ID: "opt-1", Text: "nice to have polish", Source: goalItemSourceMaster, Role: goalItemRoleGuardrail, State: goalItemStateOpen},
		},
	}
	if err := SaveGoalState(GoalPath(runDir), state); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.GoalBoundary == nil {
		t.Fatal("goal boundary summary missing")
	}
	if index.GoalBoundary.RequiredCount != 3 {
		t.Fatalf("required_count = %d, want 3", index.GoalBoundary.RequiredCount)
	}
	if index.GoalBoundary.OptionalCount != 1 {
		t.Fatalf("optional_count = %d, want 1", index.GoalBoundary.OptionalCount)
	}
	if got := index.GoalBoundary.RequiredBySource[goalItemSourceUser]; got != 2 {
		t.Fatalf("required_by_source[user] = %d, want 2", got)
	}
	if got := index.GoalBoundary.RequiredBySource[goalItemSourceMaster]; got != 1 {
		t.Fatalf("required_by_source[master] = %d, want 1", got)
	}
	if got := index.GoalBoundary.RequiredByRole[goalItemRoleOutcome]; got != 1 {
		t.Fatalf("required_by_role[outcome] = %d, want 1", got)
	}
	if got := index.GoalBoundary.RequiredByRole[goalItemRoleEnabler]; got != 1 {
		t.Fatalf("required_by_role[enabler] = %d, want 1", got)
	}
	if got := index.GoalBoundary.RequiredByRole[goalItemRoleProof]; got != 1 {
		t.Fatalf("required_by_role[proof] = %d, want 1", got)
	}
	if got := index.GoalBoundary.RequiredByState[goalItemStateOpen]; got != 1 {
		t.Fatalf("required_by_state[open] = %d, want 1", got)
	}
	if got := index.GoalBoundary.RequiredByState[goalItemStateClaimed]; got != 1 {
		t.Fatalf("required_by_state[claimed] = %d, want 1", got)
	}
	if got := index.GoalBoundary.RequiredByState[goalItemStateWaived]; got != 1 {
		t.Fatalf("required_by_state[waived] = %d, want 1", got)
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Goal Boundary",
		"Required items: `3`",
		"Optional items: `1`",
		"Required by source: `master=1, user=2`",
		"Required by role: `enabler=1, outcome=1, proof=1`",
		"Required by state: `claimed=1, open=1, waived=1`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesObjectiveIntegritySummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), &ObjectiveContract{
		Version:       1,
		ObjectiveHash: "sha256:demo",
		State:         objectiveContractStateLocked,
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-1",
				Text:             "ship the live experience",
				Kind:             objectiveClauseKindDelivery,
				SourceExcerpt:    "ship the live experience",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
			{
				ID:               "ucl-2",
				Text:             "verify the live path",
				Kind:             objectiveClauseKindVerification,
				SourceExcerpt:    "verify the live path",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceAcceptance},
			},
		},
	}); err != nil {
		t.Fatalf("SaveObjectiveContract: %v", err)
	}
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{
				ID:     "req-1",
				Text:   "ship the live experience",
				Source: goalItemSourceUser,
				Role:   goalItemRoleOutcome,
				Covers: []string{"ucl-1"},
				State:  goalItemStateOpen,
				Note:   "boundary only",
			},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:     2,
		GoalVersion: 1,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Label: "live verification", Command: "printf ok", Covers: []string{"ucl-2"}, State: acceptanceCheckStateActive},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.ObjectiveContractPath != ObjectiveContractPath(runDir) {
		t.Fatalf("objective_contract_path = %q, want %q", index.ObjectiveContractPath, ObjectiveContractPath(runDir))
	}
	if index.ObjectiveIntegrity == nil {
		t.Fatal("objective integrity summary missing")
	}
	if !index.ObjectiveIntegrity.ContractLocked || !index.ObjectiveIntegrity.IntegrityReady || !index.ObjectiveIntegrity.IntegrityOK {
		t.Fatalf("objective integrity = %+v, want locked/ready/ok", index.ObjectiveIntegrity)
	}
	if index.ObjectiveIntegrity.GoalCoveredCount != 1 || index.ObjectiveIntegrity.AcceptanceCoveredCount != 1 {
		t.Fatalf("objective coverage = %+v, want 1/1", index.ObjectiveIntegrity)
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{
		"Objective contract",
		"## Objective Contract",
		"State: `locked`",
		"Goal clause coverage: `1/1`",
		"Acceptance clause coverage: `1/1`",
		"Integrity OK: `true`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestContextIndexIncludesQualityDebtSummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship cockpit", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "ship research spine", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-5",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfaceActive,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:     2,
		GoalVersion: 1,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Label: "acceptance", Command: "printf ok", State: acceptanceCheckStateActive},
		},
		LastResult: AcceptanceResult{CheckedAt: "2026-03-31T02:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	if err := SaveSuccessModel(SuccessModelPath(runDir), &SuccessModel{
		Version:               1,
		ObjectiveContractHash: "sha256:objective",
		GoalHash:              "sha256:goal",
		Dimensions: []SuccessDimension{
			{ID: "req-1", Kind: "outcome", Text: "ship cockpit", Required: true},
			{ID: "req-2", Kind: "outcome", Text: "ship research spine", Required: true},
		},
	}); err != nil {
		t.Fatalf("SaveSuccessModel: %v", err)
	}
	if err := SaveProofPlan(ProofPlanPath(runDir), &ProofPlan{
		Version: 1,
		Items: []ProofPlanItem{
			{ID: "proof-acceptance", CoversDimensions: []string{"req-1"}, Kind: "acceptance_check", Required: true, SourceSurface: "acceptance"},
			{ID: "proof-report", CoversDimensions: []string{"req-2"}, Kind: "report", Required: true, SourceSurface: "report"},
		},
	}); err != nil {
		t.Fatalf("SaveProofPlan: %v", err)
	}
	if err := SaveWorkflowPlan(WorkflowPlanPath(runDir), &WorkflowPlan{
		Version: 1,
		RequiredRoles: []WorkflowRoleRequirement{
			{ID: "critic", Required: true},
			{ID: "finisher", Required: true},
		},
		Gates: []string{"critic_review_present", "finisher_pass_present"},
	}); err != nil {
		t.Fatalf("SaveWorkflowPlan: %v", err)
	}
	if err := SaveDomainPack(DomainPackPath(runDir), &DomainPack{Version: 1, Domain: "generic"}); err != nil {
		t.Fatalf("SaveDomainPack: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.QualityDebt == nil {
		t.Fatal("quality debt summary missing")
	}
	if index.QualityDebt.Zero {
		t.Fatalf("quality debt unexpectedly zero: %+v", index.QualityDebt)
	}
	if len(index.QualityDebt.SuccessDimensionUnowned) != 1 || index.QualityDebt.SuccessDimensionUnowned[0] != "req-2" {
		t.Fatalf("success_dimension_unowned = %#v, want req-2", index.QualityDebt.SuccessDimensionUnowned)
	}
	if !index.QualityDebt.CriticGateMissing || !index.QualityDebt.FinisherGateMissing {
		t.Fatalf("quality debt gates = %+v, want critic/finisher missing", index.QualityDebt)
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Quality Debt",
		"Success dimensions unowned: `req-2`",
		"Proof plan gaps: `proof-report`",
		"Critic gate missing: `true`",
		"Finisher gate missing: `true`",
		"Only correctness evidence present: `true`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesRunStatusAcceptanceAndCloseoutSummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	evidencePath := filepath.Join(runDir, "evidence.txt")
	if err := os.WriteFile(evidencePath, []byte("evidence\n"), 0o644); err != nil {
		t.Fatalf("write evidence: %v", err)
	}
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{
				ID:            "req-1",
				Text:          "ship feature",
				Source:        goalItemSourceUser,
				Role:          goalItemRoleOutcome,
				State:         goalItemStateClaimed,
				EvidencePaths: []string{evidencePath},
			},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:     2,
		GoalVersion: 1,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Label: "verify", Command: "printf ok", State: acceptanceCheckStateActive},
		},
		LastResult: AcceptanceResult{
			CheckedAt:    "2026-03-28T10:00:00Z",
			ExitCode:     intPtr(0),
			EvidencePath: AcceptanceEvidencePath(runDir),
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	requiredRemaining := 0
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseComplete,
		RequiredRemaining: &requiredRemaining,
		LastVerifiedAt:    "2026-03-28T10:00:00Z",
		UpdatedAt:         "2026-03-28T10:05:00Z",
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(CompletionStatePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir proof dir: %v", err)
	}
	if err := os.WriteFile(CompletionStatePath(runDir), []byte(`{"verdict":"complete"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.RunStatus == nil {
		t.Fatal("run status summary missing")
	}
	if index.RunStatus.RequiredRemaining != 0 || index.RunStatus.GoalRequiredRemaining != 0 || !index.RunStatus.RequiredRemainingMatch {
		t.Fatalf("run status summary = %+v, want aligned zero remaining", index.RunStatus)
	}
	if index.RunStatus.StatusOpenRequiredIDsRecorded {
		t.Fatalf("StatusOpenRequiredIDsRecorded = true, want false when status omitted IDs: %+v", index.RunStatus)
	}
	if index.Acceptance == nil {
		t.Fatal("acceptance summary missing")
	}
	if index.Acceptance.ActiveCheckCount != 1 || index.Acceptance.LastExitCode == nil || *index.Acceptance.LastExitCode != 0 {
		t.Fatalf("acceptance summary = %+v, want one passing active check", index.Acceptance)
	}
	if index.Closeout == nil {
		t.Fatal("closeout summary missing")
	}
	if !index.Closeout.SummaryExists || !index.Closeout.CompletionProofExists || !index.Closeout.ReadyToFinalize {
		t.Fatalf("closeout summary = %+v, want ready closeout", index.Closeout)
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Run Status",
		"Required remaining (status): `0`",
		"Required remaining (goal): `0`",
		"Required remaining match: `true`",
		"Status open required IDs recorded: `false`",
		"## Acceptance",
		"Active checks: `1`",
		"Last exit code: `0`",
		"## Closeout",
		"Summary exists: `true`",
		"Completion proof exists: `true`",
		"Ready to finalize: `true`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexOmitsProviderBootstrapForCodexMaster(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.Master.Engine != "codex" {
		t.Fatalf("master engine = %q, want codex", index.Master.Engine)
	}
	if index.Master.ProviderBootstrap != nil {
		t.Fatalf("master provider bootstrap = %+v, want nil for codex", index.Master.ProviderBootstrap)
	}
}

func TestBuildContextIndexIncludesExecutionSurfaceFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Master.Effort = goalx.EffortHigh
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	writeSelectionSnapshotFixture(t, runDir, testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterEffort: goalx.EffortHigh,
		},
		Master: goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
	})
	seedGuidanceSessionFixture(t, runDir, cfg)
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	identity.RequestedEffort = goalx.EffortHigh
	identity.EffectiveEffort = "xhigh"
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity for rewrite: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity rewrite: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.Master.SurfaceKind != "root_master" {
		t.Fatalf("master surface_kind = %q, want root_master", index.Master.SurfaceKind)
	}
	if index.Master.WorktreeKind != "run_root_shared" {
		t.Fatalf("master worktree_kind = %q, want run_root_shared", index.Master.WorktreeKind)
	}
	if index.Master.MergeableOutputSurface {
		t.Fatalf("master mergeable_output_surface = true, want false")
	}
	if index.Master.RequestedEffort != goalx.EffortHigh {
		t.Fatalf("master requested_effort = %q, want high", index.Master.RequestedEffort)
	}
	if index.Master.EffectiveEffort != "high" {
		t.Fatalf("master effective_effort = %q, want high", index.Master.EffectiveEffort)
	}
	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	session := index.Sessions[0]
	if session.RoleKind != "develop" {
		t.Fatalf("session role_kind = %q, want develop", session.RoleKind)
	}
	if session.Engine != "codex" || session.Model != "gpt-5.4-mini" {
		t.Fatalf("session engine/model = %s/%s, want codex/gpt-5.4-mini", session.Engine, session.Model)
	}
	if session.RequestedEffort != goalx.EffortHigh {
		t.Fatalf("session requested_effort = %q, want high", session.RequestedEffort)
	}
	if session.EffectiveEffort != "xhigh" {
		t.Fatalf("session effective_effort = %q, want xhigh", session.EffectiveEffort)
	}
	if session.SurfaceKind != "durable_session" {
		t.Fatalf("session surface_kind = %q, want durable_session", session.SurfaceKind)
	}
	if session.WorktreeKind != "dedicated" {
		t.Fatalf("session worktree_kind = %q, want dedicated", session.WorktreeKind)
	}
	if !session.MergeableOutputSurface {
		t.Fatalf("session mergeable_output_surface = false, want true")
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{
		"Surface: `root_master`",
		"Worktree kind: `run_root_shared`",
		"Mergeable output surface: `false`",
		"Requested effort: `high`",
		"Effective effort: `high`",
		"role: `develop`",
		"surface: `durable_session`",
		"worktree kind: `dedicated`",
		"mergeable output: `true`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesCompilerReportPath(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	if err := EnsureSuccessCompilation(repo, runDir, cfg, &RunMetadata{Version: 1, ProjectRoot: repo}); err != nil {
		t.Fatalf("EnsureSuccessCompilation: %v", err)
	}
	if err := SaveCompilerReport(CompilerReportPath(runDir), &CompilerReport{
		Version:         1,
		CompilerVersion: "compiler-v2",
		SelectedPriorRefs: []string{
			"prior/operator-cockpit",
		},
	}); err != nil {
		t.Fatalf("SaveCompilerReport: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.CompilerInputPath != CompilerInputPath(runDir) {
		t.Fatalf("compiler_input_path = %q, want %q", index.CompilerInputPath, CompilerInputPath(runDir))
	}
	if index.CompilerReportPath != CompilerReportPath(runDir) {
		t.Fatalf("compiler_report_path = %q, want %q", index.CompilerReportPath, CompilerReportPath(runDir))
	}
	if index.ProtocolComposition == nil {
		t.Fatal("protocol_composition = nil, want summary")
	}
	if len(index.ProtocolComposition.Philosophy) == 0 {
		t.Fatalf("protocol composition philosophy = %v, want non-empty", index.ProtocolComposition.Philosophy)
	}
	if len(index.ProtocolComposition.SelectedPriorRefs) != 1 || index.ProtocolComposition.SelectedPriorRefs[0] != "prior/operator-cockpit" {
		t.Fatalf("protocol composition selected priors = %v, want compiler report prior", index.ProtocolComposition.SelectedPriorRefs)
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{"Compiler input", "Compiler report", "## Protocol Composition", "Selected prior refs"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildContextIndexIncludesIntakePathWhenPresent(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveRunIntake(IntakePath(runDir), &RunIntake{
		Version:   1,
		Objective: cfg.Objective,
		Intent:    runIntentDeliver,
	}); err != nil {
		t.Fatalf("SaveRunIntake: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.IntakePath != IntakePath(runDir) {
		t.Fatalf("intake_path = %q, want %q", index.IntakePath, IntakePath(runDir))
	}

	rendered := renderContextIndex(index)
	if !strings.Contains(rendered, "Intake") {
		t.Fatalf("rendered context missing intake line:\n%s", rendered)
	}
}

func TestBuildContextIndexIncludesBudgetSummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = 2 * time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	startedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       cfg.Name,
		Mode:      string(cfg.Mode),
		Active:    true,
		StartedAt: startedAt.Format(time.RFC3339),
		UpdatedAt: startedAt.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.Budget == nil {
		t.Fatal("context index budget missing")
	}
	if index.Budget.MaxDurationSeconds != int64((2*time.Hour)/time.Second) {
		t.Fatalf("budget max_duration_seconds = %d, want %d", index.Budget.MaxDurationSeconds, int64((2*time.Hour)/time.Second))
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{"## Budget", "max_duration", "deadline_at"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
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

func TestContextIndexIncludesSessionWorktreeLineage(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	identity.BaseBranchSelector = "run-root"
	identity.BaseBranch = "goalx/" + cfg.Name + "/root"
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity for rewrite: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity rewrite: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	session := index.Sessions[0]
	if session.BaseBranchSelector != "run-root" {
		t.Fatalf("BaseBranchSelector = %q, want run-root", session.BaseBranchSelector)
	}
	if session.BaseBranch != "goalx/"+cfg.Name+"/root" {
		t.Fatalf("BaseBranch = %q, want %q", session.BaseBranch, "goalx/"+cfg.Name+"/root")
	}
}

func TestProviderRuntimeFactsIncludeOnlyFrameworkOwnedRuntimeFacts(t *testing.T) {
	claudeFacts := providerRuntimeFactsForEngine("master", "claude-code")
	if len(claudeFacts) == 0 {
		t.Fatalf("claude provider facts missing")
	}
	claudeText := joinProviderFactText(claudeFacts)
	for _, want := range []string{
		"tmux + interactive TUI",
		"GoalX provider runtime does not change durable ownership boundaries",
		"PermissionRequest hook",
		"Elicitation hook",
		"urgent master-inbox fact through a Notification hook",
		"cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions",
	} {
		if !strings.Contains(claudeText, want) {
			t.Fatalf("claude provider facts missing %q:\n%s", want, claudeText)
		}
	}
	for _, unwanted := range []string{"skills", "plugins", "mcp", "route", "routing", "dispatch", "prefer"} {
		if strings.Contains(strings.ToLower(claudeText), unwanted) {
			t.Fatalf("claude provider facts should not encode %q:\n%s", unwanted, claudeText)
		}
	}

	codexFacts := providerRuntimeFactsForEngine("master", "codex")
	if len(codexFacts) == 0 {
		t.Fatalf("codex provider facts missing")
	}
	codexText := joinProviderFactText(codexFacts)
	for _, want := range []string{
		"tmux + interactive TUI",
		"GoalX provider runtime does not change durable ownership boundaries",
	} {
		if !strings.Contains(codexText, want) {
			t.Fatalf("codex provider facts missing %q:\n%s", want, codexText)
		}
	}
	for _, unwanted := range []string{"skills", "plugins", "mcp", "route", "routing", "dispatch", "prefer"} {
		if strings.Contains(strings.ToLower(codexText), unwanted) {
			t.Fatalf("codex provider facts should not encode %q:\n%s", unwanted, codexText)
		}
	}
}

func TestBuildContextIndexIncludesClaudeBootstrapFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Master.Engine = "claude-code"
	cfg.Master.Model = "opus"
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	if err := os.MkdirAll(RunWorktreePath(runDir), 0o755); err != nil {
		t.Fatalf("mkdir run worktree: %v", err)
	}
	if err := EnsureEngineTrusted("claude-code", RunWorktreePath(runDir)); err != nil {
		t.Fatalf("EnsureEngineTrusted: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.Master.ProviderBootstrap == nil {
		t.Fatal("master provider bootstrap missing")
	}
	if index.Master.ProviderBootstrap.PermissionMode != "auto" {
		t.Fatalf("master permission_mode = %q, want auto", index.Master.ProviderBootstrap.PermissionMode)
	}
	if !index.Master.ProviderBootstrap.PermissionRequestHookBootstrapped {
		t.Fatalf("master permission_request_hook_bootstrapped = false, want true")
	}
	if !index.Master.ProviderBootstrap.ElicitationHookBootstrapped {
		t.Fatalf("master elicitation_hook_bootstrapped = false, want true")
	}
	if !index.Master.ProviderBootstrap.NotificationHookBootstrapped {
		t.Fatalf("master notification_hook_bootstrapped = false, want true")
	}
	if !index.Master.ProviderBootstrap.BootstrapVerified {
		t.Fatalf("master bootstrap_verified = false, want true")
	}

	rendered := renderContextIndex(index)
	for _, want := range []string{
		"Permission mode: `auto`",
		"Provider bootstrap verified: `true`",
		"permission_request_hook_bootstrapped: `true`",
		"elicitation_hook_bootstrapped: `true`",
		"notification_hook_bootstrapped: `true`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
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
		ExperimentID:    "exp_guidance_shared_session_1",
		RoleKind:        "develop",
		Mode:            string(goalx.ModeWorker),
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
		Mode:         string(goalx.ModeWorker),
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

func joinProviderFactText(facts []ProviderRuntimeFact) string {
	parts := make([]string, 0, len(facts))
	for _, fact := range facts {
		parts = append(parts, fact.Fact)
	}
	return strings.Join(parts, "\n")
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

func TestBuildContextIndexFailsWithoutRunCharter(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := os.Remove(RunCharterPath(runDir)); err != nil {
		t.Fatalf("remove run charter: %v", err)
	}

	_, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err == nil || !strings.Contains(err.Error(), "run charter missing") {
		t.Fatalf("BuildContextIndex error = %v, want missing charter error", err)
	}
}
