package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goalx "github.com/vonbai/goalx"
)

type CoordinationState struct {
	Version       int                            `json:"version"`
	PlanSummary   []string                       `json:"plan_summary,omitempty"`
	Owners        map[string]string              `json:"owners,omitempty"`
	Sessions      map[string]CoordinationSession `json:"sessions,omitempty"`
	Decision      *CoordinationDecision          `json:"decision,omitempty"`
	Blocked       []string                       `json:"blocked,omitempty"`
	OpenQuestions []string                       `json:"open_questions,omitempty"`
	UpdatedAt     string                         `json:"updated_at,omitempty"`
}

type CoordinationSession struct {
	State              string                    `json:"state,omitempty"`
	ExecutionState     string                    `json:"execution_state,omitempty"`
	Scope              string                    `json:"scope,omitempty"`
	BlockedBy          string                    `json:"blocked_by,omitempty"`
	DispatchableSlices []goalx.DispatchableSlice `json:"dispatchable_slices,omitempty"`
	LastRound          int                       `json:"last_round,omitempty"`
	UpdatedAt          string                    `json:"updated_at,omitempty"`
}

type CoordinationDecision struct {
	RootCause        string `json:"root_cause,omitempty"`
	LocalPath        string `json:"local_path,omitempty"`
	CompatiblePath   string `json:"compatible_path,omitempty"`
	ArchitecturePath string `json:"architecture_path,omitempty"`
	ChosenPath       string `json:"chosen_path,omitempty"`
	ChosenPathReason string `json:"chosen_path_reason,omitempty"`
}

func CoordinationPath(runDir string) string {
	return filepath.Join(runDir, "coordination.json")
}

func LoadCoordinationState(path string) (*CoordinationState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseCoordinationState(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return state, nil
}

func SaveCoordinationState(path string, state *CoordinationState) error {
	if err := validateCoordinationState(state); err != nil {
		return err
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func EnsureCoordinationState(runDir, objective string) (*CoordinationState, error) {
	path := CoordinationPath(runDir)
	state, err := LoadCoordinationState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = &CoordinationState{
			Version:   1,
			Owners:    map[string]string{},
			Sessions:  map[string]CoordinationSession{},
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := SaveCoordinationState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	return state, nil
}

func parseCoordinationState(data []byte) (*CoordinationState, error) {
	var state CoordinationState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCoordination, err)
	}
	if err := validateCoordinationState(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCoordination, err)
	}
	return &state, nil
}

// normalizeCoordinationState ensures structural consistency without
// truncating or modifying master-written content.
func normalizeCoordinationState(state *CoordinationState) {
	if state == nil {
		return
	}
	if state.Sessions == nil {
		state.Sessions = map[string]CoordinationSession{}
	}
}

func validateCoordinationState(state *CoordinationState) error {
	if state == nil {
		return fmt.Errorf("coordination state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("coordination state version must be positive")
	}
	normalizeCoordinationState(state)
	return nil
}
