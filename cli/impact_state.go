package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ImpactState struct {
	Version          int      `json:"version"`
	Scope            string   `json:"scope"`
	BaselineRevision string   `json:"baseline_revision"`
	HeadRevision     string   `json:"head_revision"`
	ResolverKind     string   `json:"resolver_kind"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	ChangedSymbols   []string `json:"changed_symbols,omitempty"`
	ChangedProcesses []string `json:"changed_processes,omitempty"`
	UpdatedAt        string   `json:"updated_at,omitempty"`
}

func ImpactStatePath(runDir string) string {
	return filepath.Join(runDir, "impact-state.json")
}

func LoadImpactState(path string) (*ImpactState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseImpactState(data)
	if err != nil {
		return nil, fmt.Errorf("parse impact state: %w", err)
	}
	return state, nil
}

func SaveImpactState(path string, state *ImpactState) error {
	if state == nil {
		return fmt.Errorf("impact state is nil")
	}
	if err := validateImpactStateInput(state); err != nil {
		return err
	}
	normalizeImpactState(state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func parseImpactState(data []byte) (*ImpactState, error) {
	var state ImpactState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceImpactState, err)
	}
	if err := validateImpactStateInput(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceImpactState, err)
	}
	normalizeImpactState(&state)
	return &state, nil
}

func validateImpactStateInput(state *ImpactState) error {
	if state == nil {
		return fmt.Errorf("impact state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("impact state version must be positive")
	}
	if strings.TrimSpace(state.Scope) == "" {
		return fmt.Errorf("impact state scope is required")
	}
	if strings.TrimSpace(state.BaselineRevision) == "" {
		return fmt.Errorf("impact state baseline_revision is required")
	}
	if strings.TrimSpace(state.HeadRevision) == "" {
		return fmt.Errorf("impact state head_revision is required")
	}
	switch strings.TrimSpace(state.ResolverKind) {
	case "repo-native", "gitnexus", "file_only", "none":
	default:
		return fmt.Errorf("impact state resolver_kind %q is invalid", state.ResolverKind)
	}
	return nil
}

func normalizeImpactState(state *ImpactState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	state.Scope = strings.TrimSpace(state.Scope)
	state.BaselineRevision = strings.TrimSpace(state.BaselineRevision)
	state.HeadRevision = strings.TrimSpace(state.HeadRevision)
	state.ResolverKind = strings.TrimSpace(state.ResolverKind)
	state.ChangedFiles = compactStrings(state.ChangedFiles)
	state.ChangedSymbols = compactStrings(state.ChangedSymbols)
	state.ChangedProcesses = compactStrings(state.ChangedProcesses)
	state.UpdatedAt = strings.TrimSpace(state.UpdatedAt)
}
