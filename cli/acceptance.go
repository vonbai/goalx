package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

// No status or change_kind constants — the framework records raw facts.
// Interpretation of exit codes, classification of gate changes, and
// governance policy (e.g. narrowed gates requiring approval) belong
// to the master agent, not the framework.

// AcceptanceResult records raw verification output — exit code, timestamp,
// and path to captured output. No derived status or verdict.
type AcceptanceResult struct {
	CheckedAt    string `json:"checked_at,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	EvidencePath string `json:"evidence_path,omitempty"`
}

type AcceptanceState struct {
	Version          int              `json:"version"`
	GoalVersion      int              `json:"goal_version,omitempty"`
	DefaultCommand   string           `json:"default_command,omitempty"`
	EffectiveCommand string           `json:"effective_command,omitempty"`
	ChangeKind       string           `json:"change_kind,omitempty"`
	ChangeReason     string           `json:"change_reason,omitempty"`
	UserApproved     bool             `json:"user_approved,omitempty"`
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
	var state AcceptanceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	return &state, nil
}

func SaveAcceptanceState(path string, state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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

	changed := false
	defaultCommand := goalx.ResolveAcceptanceCommand(cfg)
	if strings.TrimSpace(state.DefaultCommand) == "" && strings.TrimSpace(defaultCommand) != "" {
		state.DefaultCommand = defaultCommand
		changed = true
	}
	if strings.TrimSpace(state.EffectiveCommand) == "" && strings.TrimSpace(state.DefaultCommand) != "" {
		state.EffectiveCommand = state.DefaultCommand
		changed = true
	}
	if state.GoalVersion <= 0 && goalVersion > 0 {
		state.GoalVersion = goalVersion
		changed = true
	}
	if changed {
		if err := SaveAcceptanceState(path, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

// ValidateAcceptanceStateForVerification and normalizeAcceptanceState were
// removed: they encoded governance policy (change_kind validation, narrowed
// gate approval) and silently mutated agent-written data. Per facts-not-judgments,
// the master agent owns all interpretation of acceptance state.
