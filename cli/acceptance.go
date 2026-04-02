package cli

import (
	"fmt"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const (
	acceptanceCheckStateActive = "active"
	acceptanceCheckStateWaived = "waived"
)

// AcceptanceResult records raw verification output — exit code, timestamp,
// and paths to captured output. No derived status or verdict.
type AcceptanceResult struct {
	CheckedAt    string                  `json:"checked_at,omitempty"`
	ExitCode     *int                    `json:"exit_code,omitempty"`
	EvidencePath string                  `json:"evidence_path,omitempty"`
	CheckResults []AcceptanceCheckResult `json:"check_results,omitempty"`
}

type AcceptanceCheckResult struct {
	ID           string `json:"id"`
	Command      string `json:"command,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	EvidencePath string `json:"evidence_path,omitempty"`
}

type AcceptanceCheck struct {
	ID          string   `json:"id"`
	Label       string   `json:"label,omitempty"`
	Command     string   `json:"command,omitempty"`
	Covers      []string `json:"covers,omitempty"`
	State       string   `json:"state,omitempty"`
	ApprovalRef string   `json:"approval_ref,omitempty"`
}

type AcceptanceState struct {
	Version     int              `json:"version"`
	GoalVersion int              `json:"goal_version,omitempty"`
	Checks      []AcceptanceCheck `json:"checks,omitempty"`
	LastResult  AcceptanceResult `json:"last_result,omitempty"`
	UpdatedAt   string           `json:"updated_at,omitempty"`
}

func NewAcceptanceState(cfg *goalx.Config, goalVersion int) *AcceptanceState {
	state := &AcceptanceState{
		Version:     2,
		GoalVersion: goalVersion,
		Checks:      []AcceptanceCheck{},
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	if cmd := goalx.ResolveAcceptanceCommand(cfg); strings.TrimSpace(cmd) != "" {
		state.Checks = append(state.Checks, AcceptanceCheck{
			ID:      "chk-1",
			Label:   "bootstrap acceptance",
			Command: cmd,
			State:   acceptanceCheckStateActive,
		})
	}
	normalizeAcceptanceState(state)
	return state
}

func parseAcceptanceState(data []byte) (*AcceptanceState, error) {
	var state AcceptanceState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAssurancePlan, err)
	}
	if err := validateAcceptanceState(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAssurancePlan, err)
	}
	normalizeAcceptanceState(&state)
	return &state, nil
}

func validateAcceptanceState(state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("acceptance state version must be positive")
	}
	seen := map[string]struct{}{}
	for _, check := range state.Checks {
		if strings.TrimSpace(check.ID) == "" {
			return fmt.Errorf("acceptance check id is required")
		}
		if _, ok := seen[check.ID]; ok {
			return fmt.Errorf("duplicate acceptance check id %q", check.ID)
		}
		seen[check.ID] = struct{}{}
		switch normalizeAcceptanceCheckState(check.State) {
		case acceptanceCheckStateActive:
			if strings.TrimSpace(check.Command) == "" {
				return fmt.Errorf("acceptance check %s is missing command", check.ID)
			}
		case acceptanceCheckStateWaived:
			if strings.TrimSpace(check.ApprovalRef) == "" {
				return fmt.Errorf("acceptance check %s is waived without explicit approval_ref", check.ID)
			}
		default:
			return fmt.Errorf("acceptance check %s has invalid state %q", check.ID, check.State)
		}
	}
	return nil
}

func normalizeAcceptanceState(state *AcceptanceState) {
	if state.Version <= 0 {
		state.Version = 2
	}
	if state.Checks == nil {
		state.Checks = []AcceptanceCheck{}
	}
	for i := range state.Checks {
		normalizeAcceptanceCheck(&state.Checks[i])
	}
	if state.LastResult.CheckResults == nil {
		state.LastResult.CheckResults = []AcceptanceCheckResult{}
	}
}

func normalizeAcceptanceCheck(check *AcceptanceCheck) {
	if check == nil {
		return
	}
	check.ID = strings.TrimSpace(check.ID)
	check.Label = strings.TrimSpace(check.Label)
	check.Command = strings.TrimSpace(check.Command)
	check.Covers = trimmedGoalCovers(check.Covers)
	check.State = normalizeAcceptanceCheckState(check.State)
	check.ApprovalRef = strings.TrimSpace(check.ApprovalRef)
}

func normalizeAcceptanceCheckState(state string) string {
	switch strings.TrimSpace(state) {
	case "", acceptanceCheckStateActive:
		return acceptanceCheckStateActive
	case acceptanceCheckStateWaived:
		return acceptanceCheckStateWaived
	default:
		return strings.TrimSpace(state)
	}
}
