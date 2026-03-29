package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestBuildRequiredCoverageTreatsMissingRequiredFrontierAsUnknown(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if coverage.RequiredPresent {
		t.Fatal("required_present = true, want false")
	}
	if got, want := coverage.OpenRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("open_required_ids = %v, want %v", got, want)
	}
	if len(coverage.MappedRequiredIDs) != 0 {
		t.Fatalf("mapped_required_ids = %v, want empty", coverage.MappedRequiredIDs)
	}
	if len(coverage.UnmappedRequiredIDs) != 0 {
		t.Fatalf("unmapped_required_ids = %v, want empty when required frontier missing", coverage.UnmappedRequiredIDs)
	}
}

func TestBuildRequiredCoverageOnlyCountsOpenRequiredItems(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "open item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "claimed item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateClaimed, EvidencePaths: []string{"/tmp/evidence.txt"}},
			{ID: "req-3", Text: "waived item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateWaived, ApprovalRef: "master-inbox:1"},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "db-investigation",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.OpenRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("open_required_ids = %v, want %v", got, want)
	}
	if got, want := coverage.MappedRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mapped_required_ids = %v, want %v", got, want)
	}
	if got, want := coverage.ProbingRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("probing_required_ids = %v, want %v", got, want)
	}
}

func TestBuildRequiredCoverageDetectsMissingSessionOwner(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-9",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.MappedRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("mapped_required_ids = %v, want %v", got, want)
	}
	if got, want := coverage.SessionOwnerMissingIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("session_owner_missing_ids = %v, want %v", got, want)
	}
}

func TestBuildRequiredCoverageDoesNotReuseOpenOwnerSessions(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "owned item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-1",
				ExecutionState: coordinationRequiredExecutionStateActive,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceAvailable,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "idle"}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-2", State: "idle"}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-2: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.IdleReusableSessions, []string{"session-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("idle_reusable_sessions = %v, want %v", got, want)
	}
}

func TestBuildRequiredCoverageMarksMasterOrphanedRequiredWhenReusableSessionExists(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "master",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceExhausted,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "parked"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "parked"}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.MasterOwnedRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("master_owned_required_ids = %v, want %v", got, want)
	}
	if got, want := coverage.MasterOrphanedRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("master_orphaned_required_ids = %v, want %v", got, want)
	}
	if got, want := coverage.ParkedReusableSessions, []string{"session-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parked_reusable_sessions = %v, want %v", got, want)
	}
}

func TestBuildRequiredCoverageDoesNotMarkMasterOrphanedRequiredWhenActiveSessionExists(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "master",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceExhausted,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "active"},
			"session-2": {State: "parked"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "active"}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-2", State: "parked"}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-2: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if len(coverage.MasterOrphanedRequiredIDs) != 0 {
		t.Fatalf("master_orphaned_required_ids = %v, want empty while active session exists", coverage.MasterOrphanedRequiredIDs)
	}
}

func TestBuildRequiredCoverageClassifiesBlockedAndPrematureBlockedItems(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "prematurely blocked item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "fully blocked item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	payload := `{
  "version": 1,
  "required": {
    "req-1": {
      "owner": "master",
      "execution_state": "blocked",
      "blocked_by": "claimed blocker before runtime exhausted",
      "surfaces": {
        "repo": "exhausted",
        "runtime": "pending",
        "run_artifacts": "exhausted",
        "web_research": "exhausted",
        "external_system": "not_applicable"
      }
    },
    "req-2": {
      "owner": "master",
      "execution_state": "blocked",
      "blocked_by": "all machine surfaces exhausted",
      "surfaces": {
        "repo": "exhausted",
        "runtime": "exhausted",
        "run_artifacts": "exhausted",
        "web_research": "exhausted",
        "external_system": "unreachable"
      }
    }
  }
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(payload), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.PrematureBlockedRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("premature_blocked_required_ids = %v, want %v", got, want)
	}
	if got, want := coverage.BlockedRequiredIDs, []string{"req-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("blocked_required_ids = %v, want %v", got, want)
	}
}
