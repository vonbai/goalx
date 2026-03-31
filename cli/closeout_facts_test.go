package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCloseoutFactsReadyToFinalize(t *testing.T) {
	tests := []struct {
		name  string
		facts RunCloseoutFacts
		want  bool
	}{
		{
			name:  "incomplete closeout is not ready",
			facts: RunCloseoutFacts{Complete: false, MasterUnread: 0},
			want:  false,
		},
		{
			name:  "complete closeout with no unread inbox is ready",
			facts: RunCloseoutFacts{Complete: true, MasterUnread: 0, ObjectiveIntegrityOK: true},
			want:  true,
		},
		{
			name: "complete closeout with success plane present and zero quality debt is ready",
			facts: RunCloseoutFacts{
				Complete:             true,
				MasterUnread:         0,
				ObjectiveIntegrityOK: true,
				SuccessPlanePresent:  true,
				SuccessModelExists:   true,
				ProofPlanExists:      true,
				WorkflowPlanExists:   true,
				QualityDebtPresent:   true,
				QualityDebtZero:      true,
			},
			want: true,
		},
		{
			name:  "complete closeout with unread inbox stays open",
			facts: RunCloseoutFacts{Complete: true, MasterUnread: 1, ObjectiveIntegrityOK: true},
			want:  false,
		},
		{
			name:  "complete closeout with unlocked objective contract stays open",
			facts: RunCloseoutFacts{Complete: true, MasterUnread: 0, ObjectiveContractPresent: true, ObjectiveContractLocked: false, ObjectiveIntegrityOK: false},
			want:  false,
		},
		{
			name:  "complete closeout with missing objective coverage stays open",
			facts: RunCloseoutFacts{Complete: true, MasterUnread: 0, ObjectiveContractPresent: true, ObjectiveContractLocked: true, ObjectiveIntegrityOK: false},
			want:  false,
		},
		{
			name: "complete closeout with success-plane quality debt still finalizes lifecycle",
			facts: RunCloseoutFacts{
				Complete:             true,
				MasterUnread:         0,
				ObjectiveIntegrityOK: true,
				SuccessPlanePresent:  true,
				SuccessModelExists:   true,
				ProofPlanExists:      true,
				WorkflowPlanExists:   true,
				QualityDebtPresent:   true,
				QualityDebtZero:      false,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.facts.ReadyToFinalize(); got != tt.want {
				t.Fatalf("ReadyToFinalize() = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestRunCloseoutFactsMaintenanceAction(t *testing.T) {
	tests := []struct {
		name   string
		facts  RunCloseoutFacts
		master TargetPresenceFacts
		want   RunCloseoutMaintenanceAction
	}{
		{
			name:   "incomplete closeout does nothing",
			facts:  RunCloseoutFacts{Complete: false, MasterUnread: 0},
			master: TargetPresenceFacts{State: TargetPresencePresent},
			want:   RunCloseoutMaintenanceActionNone,
		},
		{
			name:   "ready closeout finalizes even with live master",
			facts:  RunCloseoutFacts{Complete: true, MasterUnread: 0, ObjectiveIntegrityOK: true},
			master: TargetPresenceFacts{State: TargetPresencePresent},
			want:   RunCloseoutMaintenanceActionFinalize,
		},
		{
			name:   "unread inbox with live master stays open",
			facts:  RunCloseoutFacts{Complete: true, MasterUnread: 1, ObjectiveIntegrityOK: true},
			master: TargetPresenceFacts{State: TargetPresencePresent},
			want:   RunCloseoutMaintenanceActionNone,
		},
		{
			name:   "unread inbox with missing master requests recovery",
			facts:  RunCloseoutFacts{Complete: true, MasterUnread: 1, ObjectiveIntegrityOK: true},
			master: TargetPresenceFacts{State: TargetPresenceWindowMissing},
			want:   RunCloseoutMaintenanceActionRecoverMaster,
		},
		{
			name:   "objective integrity gap with missing master requests recovery",
			facts:  RunCloseoutFacts{Complete: true, MasterUnread: 0, ObjectiveContractPresent: true, ObjectiveContractLocked: true, ObjectiveIntegrityOK: false},
			master: TargetPresenceFacts{State: TargetPresenceWindowMissing},
			want:   RunCloseoutMaintenanceActionRecoverMaster,
		},
		{
			name: "success-plane debt with missing master still finalizes",
			facts: RunCloseoutFacts{
				Complete:             true,
				MasterUnread:         0,
				ObjectiveIntegrityOK: true,
				SuccessPlanePresent:  true,
				SuccessModelExists:   true,
				ProofPlanExists:      true,
				WorkflowPlanExists:   true,
				QualityDebtPresent:   true,
				QualityDebtZero:      false,
			},
			master: TargetPresenceFacts{State: TargetPresenceWindowMissing},
			want:   RunCloseoutMaintenanceActionFinalize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.facts.MaintenanceAction(tt.master); got != tt.want {
				t.Fatalf("MaintenanceAction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildRunCloseoutFactsBlocksEvolveFinalizeWithoutManagedStop(t *testing.T) {
	_, runDir, _, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
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
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)

	facts, err := BuildRunCloseoutFacts(runDir)
	if err != nil {
		t.Fatalf("BuildRunCloseoutFacts: %v", err)
	}
	if !facts.Complete {
		t.Fatalf("Complete = false, want true: %+v", facts)
	}
	if facts.ReadyToFinalize() {
		t.Fatalf("ReadyToFinalize() = true, want false for evolve run with open frontier: %+v", facts)
	}
}

func TestBuildRunCloseoutFactsAllowsFinalizeWhenOnlyQualityDebtRemains(t *testing.T) {
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
		Checks:      []AcceptanceCheck{{ID: "chk-1", Label: "acceptance", Command: "printf ok", State: acceptanceCheckStateActive}},
		LastResult:  AcceptanceResult{CheckedAt: "2026-03-31T02:00:00Z"},
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
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"updated_at":"2026-03-31T02:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
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

	facts, err := BuildRunCloseoutFacts(runDir)
	if err != nil {
		t.Fatalf("BuildRunCloseoutFacts: %v", err)
	}
	if !facts.Complete || !facts.SuccessPlanePresent {
		t.Fatalf("closeout facts = %+v, want complete run with success plane", facts)
	}
	if facts.QualityDebtZero {
		t.Fatalf("QualityDebtZero = true, want false: %+v", facts)
	}
	if !facts.ReadyToFinalize() {
		t.Fatalf("ReadyToFinalize() = false, want true when only quality debt remains: %+v", facts)
	}
}
