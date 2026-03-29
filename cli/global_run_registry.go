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

type GlobalRunRegistry struct {
	Version   int                     `json:"version"`
	Runs      map[string]GlobalRunRef `json:"runs,omitempty"`
	UpdatedAt string                  `json:"updated_at,omitempty"`
}

type GlobalRunRef struct {
	Key         string `json:"key,omitempty"`
	Name        string `json:"name"`
	ProjectID   string `json:"project_id"`
	ProjectRoot string `json:"project_root,omitempty"`
	RunDir      string `json:"run_dir,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	TmuxSession string `json:"tmux_session,omitempty"`
	Mode        string `json:"mode,omitempty"`
	Objective   string `json:"objective,omitempty"`
	State       string `json:"state,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

func GlobalRunRegistryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".goalx", "runs", "index.json")
}

func LoadGlobalRunRegistry() (*GlobalRunRegistry, error) {
	data, err := os.ReadFile(GlobalRunRegistryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalRunRegistry{
				Version: 1,
				Runs:    map[string]GlobalRunRef{},
			}, nil
		}
		return nil, fmt.Errorf("read global run registry: %w", err)
	}
	reg := &GlobalRunRegistry{}
	if len(strings.TrimSpace(string(data))) == 0 {
		reg.Version = 1
		reg.Runs = map[string]GlobalRunRef{}
		return reg, nil
	}
	if err := json.Unmarshal(data, reg); err != nil {
		return nil, fmt.Errorf("parse global run registry: %w", err)
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.Runs == nil {
		reg.Runs = map[string]GlobalRunRef{}
	}
	return reg, nil
}

func SaveGlobalRunRegistry(reg *GlobalRunRegistry) error {
	if reg == nil {
		return fmt.Errorf("global run registry is nil")
	}
	if reg.Version == 0 {
		reg.Version = 1
	}
	if reg.Runs == nil {
		reg.Runs = map[string]GlobalRunRef{}
	}
	for key, ref := range reg.Runs {
		ref.Objective = ""
		reg.Runs[key] = ref
	}
	reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal global run registry: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(GlobalRunRegistryPath()), 0o755); err != nil {
		return err
	}
	if err := writeFileAtomic(GlobalRunRegistryPath(), data, 0o644); err != nil {
		return fmt.Errorf("write global run registry: %w", err)
	}
	return nil
}

func mutateGlobalRunRegistry(mutate func(*GlobalRunRegistry) error) error {
	return mutateStructuredFile(
		GlobalRunRegistryPath(),
		0o644,
		func(data []byte) (*GlobalRunRegistry, error) {
			reg, err := LoadGlobalRunRegistry()
			if err != nil {
				return nil, err
			}
			return reg, nil
		},
		func() *GlobalRunRegistry {
			return &GlobalRunRegistry{
				Version: 1,
				Runs:    map[string]GlobalRunRef{},
			}
		},
		func(reg *GlobalRunRegistry) error {
			return mutate(reg)
		},
		func(reg *GlobalRunRegistry) ([]byte, error) {
			if reg == nil {
				return nil, fmt.Errorf("global run registry is nil")
			}
			if reg.Version == 0 {
				reg.Version = 1
			}
			if reg.Runs == nil {
				reg.Runs = map[string]GlobalRunRef{}
			}
			for key, ref := range reg.Runs {
				ref.Objective = ""
				reg.Runs[key] = ref
			}
			reg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			data, err := json.MarshalIndent(reg, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal global run registry: %w", err)
			}
			return data, nil
		},
	)
}

func globalRunKey(projectRoot, runName string) string {
	return projectRoot + "::" + runName
}

func UpsertGlobalRun(projectRoot string, cfg *goalx.Config, state string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	key := globalRunKey(projectRoot, cfg.Name)
	return mutateGlobalRunRegistry(func(reg *GlobalRunRegistry) error {
		reg.Runs[key] = GlobalRunRef{
			Key:         key,
			Name:        cfg.Name,
			ProjectID:   goalx.ProjectID(projectRoot),
			ProjectRoot: projectRoot,
			RunDir:      goalx.RunDir(projectRoot, cfg.Name),
			TmuxSession: goalx.TmuxSessionName(projectRoot, cfg.Name),
			Mode:        string(cfg.Mode),
			State:       state,
			UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		}
		reg.Runs[key] = hydrateGlobalRunIdentity(reg.Runs[key])
		return nil
	})
}

func UpdateGlobalRunState(projectRoot, runName, state string) error {
	key := globalRunKey(projectRoot, runName)
	return mutateGlobalRunRegistry(func(reg *GlobalRunRegistry) error {
		ref, ok := reg.Runs[key]
		if !ok {
			ref = GlobalRunRef{
				Key:         key,
				Name:        runName,
				ProjectID:   goalx.ProjectID(projectRoot),
				ProjectRoot: projectRoot,
				RunDir:      goalx.RunDir(projectRoot, runName),
				TmuxSession: goalx.TmuxSessionName(projectRoot, runName),
			}
			if cfg, err := LoadRunSpec(ref.RunDir); err == nil && cfg != nil {
				ref.Mode = string(cfg.Mode)
			}
		}
		ref.State = state
		ref.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		reg.Runs[key] = hydrateGlobalRunIdentity(ref)
		return nil
	})
}

func RemoveGlobalRun(projectRoot, runName string) error {
	return mutateGlobalRunRegistry(func(reg *GlobalRunRegistry) error {
		delete(reg.Runs, globalRunKey(projectRoot, runName))
		return nil
	})
}

func LookupGlobalRuns(selector string) ([]GlobalRunRef, error) {
	reg, err := LoadGlobalRunRegistry()
	if err != nil {
		return nil, err
	}
	if isRunIDSelector(selector) {
		for _, ref := range reg.Runs {
			ref = hydrateGlobalRunIdentity(ref)
			if ref.RunID != selector {
				continue
			}
			ref.Key = globalRunKey(ref.ProjectRoot, ref.Name)
			return []GlobalRunRef{ref}, nil
		}
		return nil, nil
	}

	projectID, runName := parseRunSelector(selector)

	matches := make([]GlobalRunRef, 0, len(reg.Runs))
	for _, ref := range reg.Runs {
		if ref.Name != runName {
			continue
		}
		if projectID != "" && ref.ProjectID != projectID {
			continue
		}
		ref.Key = globalRunKey(ref.ProjectRoot, ref.Name)
		matches = append(matches, ref)
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].ProjectID == matches[j].ProjectID {
			return matches[i].Name < matches[j].Name
		}
		return matches[i].ProjectID < matches[j].ProjectID
	})
	return matches, nil
}

func hydrateGlobalRunIdentity(ref GlobalRunRef) GlobalRunRef {
	if ref.RunID != "" || ref.RunDir == "" {
		return ref
	}
	meta, err := LoadRunMetadata(RunMetadataPath(ref.RunDir))
	if err != nil || meta == nil {
		return ref
	}
	ref.RunID = meta.RunID
	return ref
}

func parseRunSelector(selector string) (projectID, runName string) {
	parts := strings.SplitN(selector, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1]
	}
	return "", selector
}
