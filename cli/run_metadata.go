package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type RunMetadata struct {
	Version      int    `json:"version"`
	Objective    string `json:"objective,omitempty"`
	BaseRevision string `json:"base_revision,omitempty"`
	StartedAt    string `json:"started_at,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

func RunMetadataPath(runDir string) string {
	return filepath.Join(runDir, "run-metadata.json")
}

func LoadRunMetadata(path string) (*RunMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var meta RunMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &meta, nil
}

func SaveRunMetadata(path string, meta *RunMetadata) error {
	if meta == nil {
		return fmt.Errorf("run metadata is nil")
	}
	if meta.Version <= 0 {
		meta.Version = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if meta.StartedAt == "" {
		meta.StartedAt = now
	}
	meta.UpdatedAt = now
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EnsureRunMetadata(runDir, projectRoot, objective string) (*RunMetadata, error) {
	path := RunMetadataPath(runDir)
	meta, err := LoadRunMetadata(path)
	if err != nil {
		return nil, err
	}
	if meta == nil {
		baseRevision, revErr := gitHeadRevision(projectRoot)
		if revErr != nil {
			return nil, revErr
		}
		meta = &RunMetadata{
			Version:      1,
			Objective:    objective,
			BaseRevision: baseRevision,
		}
		if err := SaveRunMetadata(path, meta); err != nil {
			return nil, err
		}
		return meta, nil
	}
	changed := false
	if meta.Version <= 0 {
		meta.Version = 1
		changed = true
	}
	if meta.Objective == "" {
		meta.Objective = objective
		changed = true
	}
	if meta.BaseRevision == "" {
		baseRevision, revErr := gitHeadRevision(projectRoot)
		if revErr != nil {
			return nil, revErr
		}
		meta.BaseRevision = baseRevision
		changed = true
	}
	if changed {
		if err := SaveRunMetadata(path, meta); err != nil {
			return nil, err
		}
	}
	return meta, nil
}

func gitHeadRevision(projectRoot string) (string, error) {
	out, err := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve git HEAD for %s: %w\n%s", projectRoot, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
