package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CoordinationState struct {
	Version       int               `json:"version"`
	Objective     string            `json:"objective,omitempty"`
	PlanSummary   []string          `json:"plan_summary,omitempty"`
	Owners        map[string]string `json:"owners,omitempty"`
	Blocked       []string          `json:"blocked,omitempty"`
	OpenQuestions []string          `json:"open_questions,omitempty"`
	UpdatedAt     string            `json:"updated_at,omitempty"`
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
	var state CoordinationState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &state, nil
}

func SaveCoordinationState(path string, state *CoordinationState) error {
	if state == nil {
		return fmt.Errorf("coordination state is nil")
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
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
			Objective: objective,
			Owners:    map[string]string{},
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := SaveCoordinationState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Objective == "" {
		state.Objective = objective
	}
	if state.Owners == nil {
		state.Owners = map[string]string{}
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := SaveCoordinationState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}
