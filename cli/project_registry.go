package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type ProjectRunRef struct {
	Name      string `json:"name"`
	Mode      string `json:"mode,omitempty"`
	Objective string `json:"objective,omitempty"`
	State     string `json:"state,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ProjectRegistry struct {
	Version    int                      `json:"version"`
	FocusedRun string                   `json:"focused_run,omitempty"`
	ActiveRuns map[string]ProjectRunRef `json:"active_runs,omitempty"`
	SavedRuns  map[string]ProjectRunRef `json:"saved_runs,omitempty"`
	UpdatedAt  string                   `json:"updated_at,omitempty"`
}

func ProjectRegistryPath(projectRoot string) string {
	return filepath.Join(ProjectDataDir(projectRoot), "registry.json")
}

func LoadProjectRegistry(projectRoot string) (*ProjectRegistry, error) {
	data, err := os.ReadFile(ProjectRegistryPath(projectRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return &ProjectRegistry{
				Version:    1,
				ActiveRuns: map[string]ProjectRunRef{},
				SavedRuns:  map[string]ProjectRunRef{},
			}, nil
		}
		return nil, fmt.Errorf("read project registry: %w", err)
	}
	reg := &ProjectRegistry{}
	if len(strings.TrimSpace(string(data))) == 0 {
		reg.Version = 1
		reg.ActiveRuns = map[string]ProjectRunRef{}
		reg.SavedRuns = map[string]ProjectRunRef{}
		return reg, nil
	}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("parse project registry: %w", err)
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.ActiveRuns == nil {
		reg.ActiveRuns = map[string]ProjectRunRef{}
	}
	if reg.SavedRuns == nil {
		reg.SavedRuns = map[string]ProjectRunRef{}
	}
	return reg, nil
}

func SaveProjectRegistry(projectRoot string, reg *ProjectRegistry) error {
	if reg == nil {
		return fmt.Errorf("project registry is nil")
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.ActiveRuns == nil {
		reg.ActiveRuns = map[string]ProjectRunRef{}
	}
	if reg.SavedRuns == nil {
		reg.SavedRuns = map[string]ProjectRunRef{}
	}
	for name, ref := range reg.ActiveRuns {
		ref.Objective = ""
		reg.ActiveRuns[name] = ref
	}
	for name, ref := range reg.SavedRuns {
		ref.Objective = ""
		reg.SavedRuns[name] = ref
	}
	reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal project registry: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(ProjectRegistryPath(projectRoot)), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(ProjectRegistryPath(projectRoot), data, 0o644); err != nil {
		return fmt.Errorf("write project registry: %w", err)
	}
	return nil
}

func RegisterActiveRun(projectRoot string, cfg *goalx.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	reg.ActiveRuns[cfg.Name] = ProjectRunRef{
		Name:      cfg.Name,
		Mode:      string(cfg.Mode),
		State:     "active",
		UpdatedAt: now,
	}
	if reg.FocusedRun == "" {
		reg.FocusedRun = cfg.Name
	}
	if err := SaveProjectRegistry(projectRoot, reg); err != nil {
		return err
	}
	return UpsertGlobalRun(projectRoot, cfg, "active")
}

func MarkRunInactive(projectRoot, runName string) error {
	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		return err
	}
	delete(reg.ActiveRuns, runName)
	if reg.FocusedRun == runName {
		reg.FocusedRun = ""
		if len(reg.ActiveRuns) == 1 {
			for name := range reg.ActiveRuns {
				reg.FocusedRun = name
			}
		}
	}
	if err := SaveProjectRegistry(projectRoot, reg); err != nil {
		return err
	}
	return UpdateGlobalRunState(projectRoot, runName, "inactive")
}

func RegisterSavedRun(projectRoot string, cfg *goalx.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		return err
	}
	reg.SavedRuns[cfg.Name] = ProjectRunRef{
		Name:      cfg.Name,
		Mode:      string(cfg.Mode),
		State:     "saved",
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := SaveProjectRegistry(projectRoot, reg); err != nil {
		return err
	}
	return UpsertGlobalRun(projectRoot, cfg, "saved")
}

func RemoveRunRegistration(projectRoot, runName string) error {
	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		return err
	}
	delete(reg.ActiveRuns, runName)
	delete(reg.SavedRuns, runName)
	if reg.FocusedRun == runName {
		reg.FocusedRun = ""
	}
	if err := SaveProjectRegistry(projectRoot, reg); err != nil {
		return err
	}
	return RemoveGlobalRun(projectRoot, runName)
}

func ResolveDefaultRunName(projectRoot string) (string, error) {
	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		return "", err
	}
	if reg.FocusedRun != "" {
		if state, err := loadDerivedRunState(projectRoot, goalx.RunDir(projectRoot, reg.FocusedRun)); err == nil && state != nil && (state.Status == "active" || state.Status == "degraded") {
			return reg.FocusedRun, nil
		}
		if _, ok := reg.ActiveRuns[reg.FocusedRun]; ok {
			return reg.FocusedRun, nil
		}
	}
	if states, err := listDerivedRunStates(projectRoot); err == nil {
		openNames := make([]string, 0)
		for _, state := range states {
			if state.Status == "active" || state.Status == "degraded" {
				openNames = append(openNames, state.Name)
			}
		}
		switch len(openNames) {
		case 1:
			return openNames[0], nil
		case 0:
		default:
			sort.Strings(openNames)
			return "", fmt.Errorf("multiple active runs: %s (specify --run)", strings.Join(openNames, ", "))
		}
	}
	if len(reg.ActiveRuns) > 1 {
		return "", fmt.Errorf("multiple active runs: %s (specify --run)", strings.Join(sortedRunNames(reg.ActiveRuns), ", "))
	}

	runsDir := ProjectDataDir(projectRoot)
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("no runs found")
		}
		return "", err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != "saved" {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	switch len(names) {
	case 0:
		return "", fmt.Errorf("no runs found")
	case 1:
		return names[0], nil
	default:
		return "", fmt.Errorf("multiple runs: %s (specify --run)", strings.Join(names, ", "))
	}
}

func sortedRunNames(m map[string]ProjectRunRef) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
