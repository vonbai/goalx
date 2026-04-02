package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	runStatusPhaseWorking  = "working"
	runStatusPhaseReview   = "review"
	runStatusPhaseComplete = "complete"
)

type RunStatusRecord struct {
	Version           int      `json:"version"`
	Phase             string   `json:"phase"`
	RequiredRemaining *int     `json:"required_remaining"`
	OpenRequiredIDs   []string `json:"open_required_ids,omitempty"`
	ActiveSessions    []string `json:"active_sessions,omitempty"`
	KeepSession       string   `json:"keep_session,omitempty"`
	LastVerifiedAt    string   `json:"last_verified_at,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
}

type RunStatusComparison struct {
	Phase                         string   `json:"phase,omitempty"`
	StatusRequiredRemaining       *int     `json:"status_required_remaining,omitempty"`
	GoalRequiredRemaining         *int     `json:"boundary_required_remaining,omitempty"`
	StatusOpenRequiredIDs         []string `json:"status_open_required_ids,omitempty"`
	GoalRemainingRequiredIDs      []string `json:"boundary_remaining_required_ids,omitempty"`
	StatusOpenRequiredIDsRecorded bool     `json:"status_open_required_ids_recorded,omitempty"`
	StatusActiveSessions          []string `json:"status_active_sessions,omitempty"`
	RuntimeActiveSessions         []string `json:"runtime_active_sessions,omitempty"`
	StatusActiveSessionsRecorded  bool     `json:"status_active_sessions_recorded,omitempty"`
	RequiredRemainingMatch        bool     `json:"required_remaining_match,omitempty"`
	OpenRequiredIDsMatch          bool     `json:"open_required_ids_match,omitempty"`
	ActiveSessionsMatch           bool     `json:"active_sessions_match,omitempty"`
	LastVerifiedAt                string   `json:"last_verified_at,omitempty"`
}

func LoadRunStatusRecord(path string) (*RunStatusRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	record, err := parseRunStatusRecord(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return record, nil
}

func SaveRunStatusRecord(path string, record *RunStatusRecord) error {
	if err := validateRunStatusRecord(record); err != nil {
		return err
	}
	if err := validateRunStatusRecordAgainstGoal(path, record); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseRunStatusRecord(data []byte) (*RunStatusRecord, error) {
	var record RunStatusRecord
	if err := decodeStrictJSON(data, &record); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceStatus, err)
	}
	if err := validateRunStatusRecord(&record); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceStatus, err)
	}
	return &record, nil
}

func validateRunStatusRecord(record *RunStatusRecord) error {
	if record == nil {
		return fmt.Errorf("run status record is nil")
	}
	if record.Version <= 0 {
		return fmt.Errorf("run status record version must be positive")
	}
	switch strings.TrimSpace(record.Phase) {
	case runStatusPhaseWorking, runStatusPhaseReview, runStatusPhaseComplete:
	default:
		return fmt.Errorf("invalid run status phase %q", record.Phase)
	}
	if record.RequiredRemaining == nil {
		return fmt.Errorf("run status record missing required_remaining")
	}
	if *record.RequiredRemaining < 0 {
		return fmt.Errorf("run status record required_remaining must be non-negative")
	}
	return nil
}

func validateRunStatusRecordAgainstGoal(path string, record *RunStatusRecord) error {
	if record == nil || strings.TrimSpace(path) == "" {
		return nil
	}
	goalState, err := LoadCanonicalGoalState(filepath.Dir(path))
	if err != nil {
		return err
	}
	if goalState == nil || record.RequiredRemaining == nil {
		return nil
	}
	summary := SummarizeGoalState(goalState)
	if *record.RequiredRemaining != summary.RequiredRemaining {
		return fmt.Errorf("run status record required_remaining=%d does not match boundary required_remaining=%d", *record.RequiredRemaining, summary.RequiredRemaining)
	}
	remainingIDs := goalRemainingRequiredIDs(goalState)
	if len(record.OpenRequiredIDs) > 0 && !slices.Equal(record.OpenRequiredIDs, remainingIDs) {
		return fmt.Errorf("run status record open_required_ids=%q does not match boundary remaining_required_ids=%q", strings.Join(record.OpenRequiredIDs, ","), strings.Join(remainingIDs, ","))
	}
	return nil
}

func BuildRunStatusComparison(runDir string) (*RunStatusComparison, error) {
	status, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		return nil, err
	}
	goalState, err := LoadCanonicalGoalState(runDir)
	if err != nil {
		return nil, err
	}
	if status == nil && goalState == nil {
		return nil, nil
	}
	comparison := &RunStatusComparison{}
	if status != nil {
		comparison.Phase = strings.TrimSpace(status.Phase)
		comparison.StatusRequiredRemaining = status.RequiredRemaining
		comparison.StatusOpenRequiredIDsRecorded = status.OpenRequiredIDs != nil
		comparison.StatusOpenRequiredIDs = append([]string(nil), status.OpenRequiredIDs...)
		slices.Sort(comparison.StatusOpenRequiredIDs)
		comparison.StatusActiveSessionsRecorded = status.ActiveSessions != nil
		comparison.StatusActiveSessions = append([]string(nil), status.ActiveSessions...)
		slices.Sort(comparison.StatusActiveSessions)
		comparison.LastVerifiedAt = strings.TrimSpace(status.LastVerifiedAt)
	}
	if goalState != nil {
		summary := SummarizeGoalState(goalState)
		comparison.GoalRequiredRemaining = intPtr(summary.RequiredRemaining)
		comparison.GoalRemainingRequiredIDs = goalRemainingRequiredIDs(goalState)
	}
	sessionState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	comparison.RuntimeActiveSessions = runtimeActiveSessionNames(sessionState)
	if comparison.StatusRequiredRemaining != nil && comparison.GoalRequiredRemaining != nil {
		comparison.RequiredRemainingMatch = *comparison.StatusRequiredRemaining == *comparison.GoalRequiredRemaining
	}
	comparison.OpenRequiredIDsMatch = !comparison.StatusOpenRequiredIDsRecorded || slices.Equal(comparison.StatusOpenRequiredIDs, comparison.GoalRemainingRequiredIDs)
	comparison.ActiveSessionsMatch = !comparison.StatusActiveSessionsRecorded || slices.Equal(comparison.StatusActiveSessions, comparison.RuntimeActiveSessions)
	return comparison, nil
}

func runtimeActiveSessionNames(state *SessionsRuntimeState) []string {
	if state == nil || state.Sessions == nil {
		return nil
	}
	names := make([]string, 0, len(state.Sessions))
	for name, session := range state.Sessions {
		switch strings.TrimSpace(session.State) {
		case "active", "progress", "working", "idle":
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}
