package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	goalx "github.com/vonbai/goalx"
)

// No status or governance constants — the framework records raw facts.
// Interpretation of exit codes and governance policy belong
// to the master agent, not the framework.

// AcceptanceResult records raw verification output — exit code, timestamp,
// and path to captured output. No derived status or verdict.
type AcceptanceResult struct {
	CheckedAt    string `json:"checked_at,omitempty"`
	Command      string `json:"command,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	EvidencePath string `json:"evidence_path,omitempty"`
}

type AcceptanceState struct {
	Version          int              `json:"version"`
	GoalVersion      int              `json:"goal_version,omitempty"`
	DefaultCommand   string           `json:"default_command,omitempty"`
	EffectiveCommand string           `json:"effective_command,omitempty"`
	LastResult       AcceptanceResult `json:"last_result,omitempty"`
	UpdatedAt        string           `json:"updated_at,omitempty"`
}

func AcceptanceNotesPath(runDir string) string {
	return filepath.Join(runDir, "acceptance.md")
}

func AcceptanceStatePath(runDir string) string {
	return filepath.Join(runDir, "acceptance.json")
}

func AcceptanceEvidencePath(runDir string) string {
	return filepath.Join(runDir, "acceptance-last.txt")
}

func NewAcceptanceState(cfg *goalx.Config, goalVersion int) *AcceptanceState {
	cmd := goalx.ResolveAcceptanceCommand(cfg)
	return &AcceptanceState{
		Version:          1,
		GoalVersion:      goalVersion,
		DefaultCommand:   cmd,
		EffectiveCommand: cmd,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
}

func LoadAcceptanceState(path string) (*AcceptanceState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseAcceptanceState(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return state, nil
}

func SaveAcceptanceState(path string, state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	if err := validateAcceptanceState(state); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func EnsureAcceptanceState(runDir string, cfg *goalx.Config, goalVersion int) (*AcceptanceState, error) {
	path := AcceptanceStatePath(runDir)
	state, err := LoadAcceptanceState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = NewAcceptanceState(cfg, goalVersion)
		if err := SaveAcceptanceState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	return state, nil
}

// ValidateAcceptanceStateForVerification and normalizeAcceptanceState were
// removed: they encoded governance policy
// gate approval) and silently mutated agent-written data. Per facts-not-judgments,
// the master agent owns all interpretation of acceptance state.

func parseAcceptanceState(data []byte) (*AcceptanceState, error) {
	var state AcceptanceState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAcceptance, err)
	}
	if err := validateAcceptanceState(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAcceptance, err)
	}
	return &state, nil
}

func validateAcceptanceState(state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("acceptance state version must be positive")
	}
	return nil
}
