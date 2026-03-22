package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	goalx "github.com/vonbai/goalx"
)

const (
	acceptanceStatusPending = "pending"
	acceptanceStatusPassed  = "passed"
	acceptanceStatusFailed  = "failed"
)

type AcceptanceState struct {
	Version       int    `json:"version"`
	Command       string `json:"command,omitempty"`
	CommandSource string `json:"command_source,omitempty"`
	Status        string `json:"status"`
	UpdatedAt     string `json:"updated_at,omitempty"`
	CheckedAt     string `json:"checked_at,omitempty"`
	LastExitCode  *int   `json:"last_exit_code,omitempty"`
	EvidencePath  string `json:"evidence_path,omitempty"`
}

func AcceptanceChecklistPath(runDir string) string {
	return filepath.Join(runDir, "acceptance.md")
}

func AcceptanceStatePath(runDir string) string {
	return filepath.Join(runDir, "acceptance.json")
}

func AcceptanceEvidencePath(runDir string) string {
	return filepath.Join(runDir, "acceptance-last.txt")
}

func NewAcceptanceState(cfg *goalx.Config) *AcceptanceState {
	cmd, source := goalx.ResolveAcceptanceCommandSource(cfg)
	return &AcceptanceState{
		Version:       1,
		Command:       cmd,
		CommandSource: source,
		Status:        acceptanceStatusPending,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
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
	return &state, nil
}

func SaveAcceptanceState(path string, state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
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

func EnsureAcceptanceState(runDir string, cfg *goalx.Config) (*AcceptanceState, error) {
	path := AcceptanceStatePath(runDir)
	state, err := LoadAcceptanceState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = NewAcceptanceState(cfg)
		if err := SaveAcceptanceState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}

	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Status == "" {
		state.Status = acceptanceStatusPending
	}
	if state.Command == "" {
		state.Command, state.CommandSource = goalx.ResolveAcceptanceCommandSource(cfg)
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := SaveAcceptanceState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}

func updateStatusWithAcceptance(statusPath string, state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}

	payload := map[string]any{}
	if data, err := os.ReadFile(statusPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("parse status %s: %w", statusPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	payload["acceptance_status"] = state.Status
	payload["acceptance_command"] = state.Command
	payload["acceptance_command_source"] = state.CommandSource
	payload["acceptance_checked_at"] = state.CheckedAt
	if state.LastExitCode != nil {
		payload["acceptance_exit_code"] = *state.LastExitCode
	}
	if state.EvidencePath != "" {
		payload["acceptance_evidence_path"] = state.EvidencePath
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(statusPath, data, 0o644)
}
