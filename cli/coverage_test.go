package cli

import (
	"reflect"
	"testing"
)

func TestBuildRequiredCoverageTreatsMissingOwnersAsUnknown(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if coverage.OwnersPresent {
		t.Fatal("owners_present = true, want false")
	}
	if got, want := coverage.OpenRequiredIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("open_required_ids = %v, want %v", got, want)
	}
	if len(coverage.OwnedOpenIDs) != 0 {
		t.Fatalf("owned_open_ids = %v, want empty", coverage.OwnedOpenIDs)
	}
	if len(coverage.UnmappedOpenIDs) != 0 {
		t.Fatalf("unmapped_open_ids = %v, want empty when owners missing", coverage.UnmappedOpenIDs)
	}
}

func TestBuildRequiredCoverageOnlyCountsOpenRequiredItems(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "open item", State: goalItemStateOpen},
			{ID: "req-2", Text: "claimed item", State: goalItemStateClaimed, EvidencePaths: []string{"/tmp/evidence.txt"}},
			{ID: "req-3", Text: "waived item", State: goalItemStateWaived, UserApproved: true},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Owners: map[string]string{
			"req-1": "workstream-a",
			"req-2": "session-2",
			"req-3": "session-3",
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
	if got, want := coverage.OwnedOpenIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owned_open_ids = %v, want %v", got, want)
	}
	if len(coverage.UnmappedOpenIDs) != 0 {
		t.Fatalf("unmapped_open_ids = %v, want empty", coverage.UnmappedOpenIDs)
	}
}

func TestBuildRequiredCoverageDetectsMissingSessionOwner(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Owners: map[string]string{
			"req-1": "session-9",
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.OwnedOpenIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owned_open_ids = %v, want %v", got, want)
	}
	if got, want := coverage.OwnerSessionMissingIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owner_session_missing_ids = %v, want %v", got, want)
	}
}

func TestBuildRequiredCoverageTreatsOpaqueOwnerTokenAsExplicitCoverage(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Owners: map[string]string{
			"req-1": "db-investigation",
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.OwnedOpenIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owned_open_ids = %v, want %v", got, want)
	}
	if len(coverage.OwnerSessionMissingIDs) != 0 {
		t.Fatalf("owner_session_missing_ids = %v, want empty for opaque owner token", coverage.OwnerSessionMissingIDs)
	}
}

func TestBuildRequiredCoverageIgnoresStaleCoordinationSessionRoster(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Owners: map[string]string{
			"req-1": "session-9",
		},
		Sessions: map[string]CoordinationSession{
			"session-9": {State: "idle"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if got, want := coverage.OwnerSessionMissingIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("owner_session_missing_ids = %v, want %v", got, want)
	}
	if len(coverage.IdleReusableSessions) != 0 {
		t.Fatalf("idle_reusable_sessions = %v, want empty for stale coordination-only roster", coverage.IdleReusableSessions)
	}
	if len(coverage.ParkedReusableSessions) != 0 {
		t.Fatalf("parked_reusable_sessions = %v, want empty for stale coordination-only roster", coverage.ParkedReusableSessions)
	}
}
