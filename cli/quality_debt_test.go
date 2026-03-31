package cli

import (
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildQualityDebtDetectsStructuralGaps(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
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
		LastResult: AcceptanceResult{
			CheckedAt: "2026-03-31T02:00:00Z",
		},
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
			{ID: "builder", Required: true},
			{ID: "critic", Required: true},
			{ID: "finisher", Required: true},
		},
		Gates: []string{"builder_result_present", "critic_review_present", "finisher_pass_present"},
	}); err != nil {
		t.Fatalf("SaveWorkflowPlan: %v", err)
	}
	if err := SaveDomainPack(DomainPackPath(runDir), &DomainPack{Version: 1, Domain: "generic"}); err != nil {
		t.Fatalf("SaveDomainPack: %v", err)
	}

	debt, err := BuildQualityDebt(runDir)
	if err != nil {
		t.Fatalf("BuildQualityDebt: %v", err)
	}
	if debt == nil {
		t.Fatal("BuildQualityDebt returned nil")
	}
	if len(debt.SuccessDimensionUnowned) != 1 || debt.SuccessDimensionUnowned[0] != "req-2" {
		t.Fatalf("success_dimension_unowned = %#v, want req-2", debt.SuccessDimensionUnowned)
	}
	if len(debt.ProofPlanGap) != 1 || debt.ProofPlanGap[0] != "proof-report" {
		t.Fatalf("proof_plan_gap = %#v, want proof-report", debt.ProofPlanGap)
	}
	if !debt.CriticGateMissing {
		t.Fatal("critic gate should be missing")
	}
	if !debt.FinisherGateMissing {
		t.Fatal("finisher gate should be missing")
	}
	if !debt.OnlyCorrectnessEvidence {
		t.Fatal("only_correctness_evidence_present should be true")
	}
}

func TestBuildQualityDebtReturnsZeroWhenSatisfied(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship cockpit", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-critic",
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
		},
	}); err != nil {
		t.Fatalf("SaveSuccessModel: %v", err)
	}
	if err := SaveProofPlan(ProofPlanPath(runDir), &ProofPlan{
		Version: 1,
		Items: []ProofPlanItem{
			{ID: "proof-acceptance", CoversDimensions: []string{"req-1"}, Kind: "acceptance_check", Required: true, SourceSurface: "acceptance"},
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
	if err := writeFileAtomic(SummaryPath(runDir), []byte("# Summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := writeFileAtomic(CompletionStatePath(runDir), []byte(`{"notes":"ok"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}
	for _, session := range []string{"session-critic", "session-finisher"} {
		identity, err := NewSessionIdentity(runDir, session, session, goalx.ModeWorker, "codex", "gpt-5.4", "", "", "", goalx.TargetConfig{})
		if err != nil {
			t.Fatalf("NewSessionIdentity %s: %v", session, err)
		}
		if err := SaveSessionIdentity(SessionIdentityPath(runDir, session), identity); err != nil {
			t.Fatalf("SaveSessionIdentity %s: %v", session, err)
		}
	}

	debt, err := BuildQualityDebt(runDir)
	if err != nil {
		t.Fatalf("BuildQualityDebt: %v", err)
	}
	if debt == nil || !debt.Zero() {
		t.Fatalf("quality debt = %+v, want zero debt", debt)
	}
}
