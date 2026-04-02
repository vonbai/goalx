package cli

import (
	"os"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestLoadRunStatusRecordParsesCanonicalShape(t *testing.T) {
	runDir := t.TempDir()
	path := RunStatusPath(runDir)
	data := `{
  "version": 1,
  "phase": "working",
  "required_remaining": 2,
  "open_required_ids": ["req-1", "req-2"],
  "active_sessions": ["session-1"],
  "keep_session": "session-2",
  "last_verified_at": "2026-03-28T10:00:00Z",
  "updated_at": "2026-03-28T10:05:00Z"
}`
	if err := writeFileAtomic(path, []byte(data), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	record, err := LoadRunStatusRecord(path)
	if err != nil {
		t.Fatalf("LoadRunStatusRecord: %v", err)
	}
	if record == nil || record.RequiredRemaining == nil {
		t.Fatal("LoadRunStatusRecord returned nil record or missing required_remaining")
	}
	if record.Version != 1 || record.Phase != runStatusPhaseWorking || *record.RequiredRemaining != 2 {
		t.Fatalf("unexpected record: %#v", record)
	}
}

func TestLoadRunStatusRecordRejectsUnknownFields(t *testing.T) {
	runDir := t.TempDir()
	path := RunStatusPath(runDir)
	if err := writeFileAtomic(path, []byte(`{"version":1,"phase":"working","required_remaining":1,"run":"demo"}`), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	_, err := LoadRunStatusRecord(path)
	if err == nil {
		t.Fatal("LoadRunStatusRecord should fail")
	}
	for _, want := range []string{"unknown field", "goalx schema status"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadRunStatusRecord error = %v, want %q", err, want)
		}
	}
}

func TestLoadRunStatusRecordRejectsMissingVersion(t *testing.T) {
	runDir := t.TempDir()
	path := RunStatusPath(runDir)
	if err := writeFileAtomic(path, []byte(`{"phase":"working","required_remaining":1}`), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	_, err := LoadRunStatusRecord(path)
	if err == nil || !strings.Contains(err.Error(), "version must be positive") {
		t.Fatalf("LoadRunStatusRecord error = %v, want version failure", err)
	}
}

func TestSaveRunStatusRecordRejectsRequiredRemainingDriftFromGoal(t *testing.T) {
	runDir := t.TempDir()
	if err := writeBoundaryFixture(t, runDir, &GoalState{
		Version: 1,
		Required: []GoalItem{
			{
				ID:     "req-1",
				Text:   "ship it",
				Source: goalItemSourceUser,
				Role:   goalItemRoleOutcome,
				State:  goalItemStateOpen,
			},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	requiredRemaining := 0
	err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseReview,
		RequiredRemaining: &requiredRemaining,
	})
	if err == nil {
		t.Fatal("SaveRunStatusRecord should reject required_remaining drift")
	}
	if !strings.Contains(err.Error(), "required_remaining=0 does not match boundary required_remaining=1") {
		t.Fatalf("SaveRunStatusRecord error = %v, want required_remaining drift", err)
	}
}

func TestBuildRunStatusComparisonIncludesRuntimeActiveSessionDrift(t *testing.T) {
	runDir := t.TempDir()
	requiredRemaining := 1
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseWorking,
		RequiredRemaining: &requiredRemaining,
		ActiveSessions:    []string{"session-3", "session-4"},
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "active", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-3", State: "parked", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-3: %v", err)
	}

	comparison, err := BuildRunStatusComparison(runDir)
	if err != nil {
		t.Fatalf("BuildRunStatusComparison: %v", err)
	}
	if comparison == nil {
		t.Fatal("BuildRunStatusComparison returned nil")
	}
	if comparison.ActiveSessionsMatch {
		t.Fatalf("ActiveSessionsMatch = true, want false: %+v", comparison)
	}
	if got := strings.Join(comparison.StatusActiveSessions, ","); got != "session-3,session-4" {
		t.Fatalf("StatusActiveSessions = %q, want session-3,session-4", got)
	}
	if got := strings.Join(comparison.RuntimeActiveSessions, ","); got != "session-1" {
		t.Fatalf("RuntimeActiveSessions = %q, want session-1", got)
	}
}

func TestBuildRunStatusComparisonDoesNotTreatMissingActiveSessionsAsDrift(t *testing.T) {
	runDir := t.TempDir()
	requiredRemaining := 1
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseWorking,
		RequiredRemaining: &requiredRemaining,
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "active", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}

	comparison, err := BuildRunStatusComparison(runDir)
	if err != nil {
		t.Fatalf("BuildRunStatusComparison: %v", err)
	}
	if comparison == nil {
		t.Fatal("BuildRunStatusComparison returned nil")
	}
	if comparison.StatusActiveSessionsRecorded {
		t.Fatalf("StatusActiveSessionsRecorded = true, want false: %+v", comparison)
	}
	if !comparison.ActiveSessionsMatch {
		t.Fatalf("ActiveSessionsMatch = false, want true when status omits active_sessions: %+v", comparison)
	}
}

func TestBuildRunStatusComparisonDetectsOpenRequiredIDDriftEvenWhenCountsMatch(t *testing.T) {
	runDir := t.TempDir()
	if err := writeBoundaryFixture(t, runDir, &GoalState{
		Version: 1,
		Required: []GoalItem{
			{
				ID:     "req-1",
				Text:   "ship feature",
				Source: goalItemSourceUser,
				Role:   goalItemRoleOutcome,
				State:  goalItemStateOpen,
			},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{
  "version": 1,
  "phase": "working",
  "required_remaining": 1,
  "open_required_ids": ["req-2"],
  "updated_at": "2026-03-28T10:10:00Z"
}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}

	comparison, err := BuildRunStatusComparison(runDir)
	if err != nil {
		t.Fatalf("BuildRunStatusComparison: %v", err)
	}
	if comparison == nil {
		t.Fatal("BuildRunStatusComparison returned nil")
	}
	if !comparison.RequiredRemainingMatch {
		t.Fatalf("RequiredRemainingMatch = false, want true when counts align: %+v", comparison)
	}
	if !comparison.StatusOpenRequiredIDsRecorded {
		t.Fatalf("StatusOpenRequiredIDsRecorded = false, want true: %+v", comparison)
	}
	if comparison.OpenRequiredIDsMatch {
		t.Fatalf("OpenRequiredIDsMatch = true, want false when IDs drift: %+v", comparison)
	}
	if got := strings.Join(comparison.StatusOpenRequiredIDs, ","); got != "req-2" {
		t.Fatalf("StatusOpenRequiredIDs = %q, want req-2", got)
	}
	if got := strings.Join(comparison.GoalRemainingRequiredIDs, ","); got != "req-1" {
		t.Fatalf("GoalRemainingRequiredIDs = %q, want req-1", got)
	}
}

func TestBuildRunStatusComparisonUsesObligationModelWhenGoalMissing(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveObligationModel(ObligationModelPath(runDir), &ObligationModel{
		Version:               1,
		ObjectiveContractHash: "sha256:objective",
		Required: []ObligationItem{
			{ID: "obl-1", Text: "ship feature", Kind: "outcome", CoversClauses: []string{"ucl-1"}},
		},
	}); err != nil {
		t.Fatalf("SaveObligationModel: %v", err)
	}
	requiredRemaining := 1
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseWorking,
		RequiredRemaining: &requiredRemaining,
		OpenRequiredIDs:   []string{"obl-1"},
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}

	comparison, err := BuildRunStatusComparison(runDir)
	if err != nil {
		t.Fatalf("BuildRunStatusComparison: %v", err)
	}
	if comparison == nil || comparison.GoalRequiredRemaining == nil || *comparison.GoalRequiredRemaining != 1 {
		t.Fatalf("comparison = %+v, want obligation-backed required_remaining=1", comparison)
	}
}
