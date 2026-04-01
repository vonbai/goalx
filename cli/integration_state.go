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

type IntegrationState struct {
	Version               int      `json:"version"`
	CurrentExperimentID   string   `json:"current_experiment_id"`
	CurrentBranch         string   `json:"current_branch"`
	CurrentCommit         string   `json:"current_commit"`
	LastIntegrationID     string   `json:"last_integration_id,omitempty"`
	LastMethod            string   `json:"last_method,omitempty"`
	LastSourceExperimentIDs []string `json:"last_source_experiment_ids,omitempty"`
	UpdatedAt             string   `json:"updated_at"`
}

func IntegrationStatePath(runDir string) string {
	return filepath.Join(runDir, "integration.json")
}

func LoadIntegrationState(path string) (*IntegrationState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state IntegrationState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, fmt.Errorf("parse integration state: %w", err)
	}
	normalizeIntegrationState(&state)
	if err := validateIntegrationState(&state); err != nil {
		return nil, err
	}
	return &state, nil
}

func SaveIntegrationState(path string, state *IntegrationState) error {
	if state == nil {
		return fmt.Errorf("integration state is nil")
	}
	normalizeIntegrationState(state)
	if err := validateIntegrationState(state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func ResolveIntegrationState(projectRoot, runName string) (*IntegrationState, error) {
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load config layers: %w", err)
	}
	return ResolveIntegrationStateWithConfig(projectRoot, runName, &layers.Config)
}

// ResolveIntegrationStateWithConfig resolves integration state with fallback support.
// Fallback order:
// 1) configured saved_run_root
// 2) user-scoped saved root (when configured saved root is set)
// 3) legacy project-local saved root
// 4) active run directories (configured run_root first, then legacy)
func ResolveIntegrationStateWithConfig(projectRoot, runName string, cfg *goalx.Config) (*IntegrationState, error) {
	candidates := []string{
		filepath.Join(goalx.ResolveSavedRunDir(projectRoot, runName, cfg), "integration.json"),
	}
	if cfg != nil && cfg.SavedRunRoot != "" {
		candidates = append(candidates, filepath.Join(SavedRunDir(projectRoot, runName), "integration.json"))
	}
	candidates = append(candidates, filepath.Join(LegacySavedRunDir(projectRoot, runName), "integration.json"))
	if rc, err := resolveLocalRun(projectRoot, runName); err == nil && rc != nil {
		candidates = append(candidates, filepath.Join(rc.RunDir, "integration.json"))
	} else {
		for _, runDir := range resolveRunDirCandidates(projectRoot, runName) {
			candidates = append(candidates, filepath.Join(runDir, "integration.json"))
		}
	}
	for _, path := range candidates {
		state, err := LoadIntegrationState(path)
		if err != nil {
			return nil, err
		}
		if state != nil {
			return state, nil
		}
	}
	return nil, nil
}

func normalizeIntegrationState(state *IntegrationState) {
	if state == nil {
		return
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	state.CurrentExperimentID = strings.TrimSpace(state.CurrentExperimentID)
	state.CurrentBranch = strings.TrimSpace(state.CurrentBranch)
	state.CurrentCommit = strings.TrimSpace(state.CurrentCommit)
	state.LastIntegrationID = strings.TrimSpace(state.LastIntegrationID)
	state.LastMethod = strings.TrimSpace(state.LastMethod)
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
}

func validateIntegrationState(state *IntegrationState) error {
	if state.Version <= 0 {
		return fmt.Errorf("integration state version must be positive")
	}
	if state.CurrentExperimentID == "" {
		return fmt.Errorf("integration state current_experiment_id is required")
	}
	if state.CurrentBranch == "" {
		return fmt.Errorf("integration state current_branch is required")
	}
	if state.CurrentCommit == "" {
		return fmt.Errorf("integration state current_commit is required")
	}
	if strings.TrimSpace(state.UpdatedAt) == "" {
		return fmt.Errorf("integration state updated_at is required")
	}
	return nil
}
