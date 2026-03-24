package cli

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	currentProtocolVersion = 2
	runIDPrefix            = "run_"
)

type RunMetadata struct {
	Version         int    `json:"version"`
	Objective       string `json:"objective,omitempty"`
	ProjectRoot     string `json:"project_root,omitempty"`
	ProtocolVersion int    `json:"protocol_version,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	RootRunID       string `json:"root_run_id,omitempty"`
	Epoch           int    `json:"epoch,omitempty"`
	BaseRevision    string `json:"base_revision,omitempty"`
	PhaseKind       string `json:"phase_kind,omitempty"`
	SourceRun       string `json:"source_run,omitempty"`
	SourcePhase     string `json:"source_phase,omitempty"`
	ParentRun       string `json:"parent_run,omitempty"`
	CharterID       string `json:"charter_id,omitempty"`
	CharterHash     string `json:"charter_hash,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
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
			Version:         1,
			Objective:       objective,
			ProjectRoot:     projectRoot,
			ProtocolVersion: currentProtocolVersion,
			RunID:           newRunID(),
			RootRunID:       "",
			Epoch:           1,
			BaseRevision:    baseRevision,
		}
		meta.RootRunID = meta.RunID
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
	if meta.ProjectRoot == "" && projectRoot != "" {
		meta.ProjectRoot = projectRoot
		changed = true
	}
	if meta.ProtocolVersion <= 0 {
		meta.ProtocolVersion = 1
		changed = true
	}
	if meta.ProtocolVersion >= 2 {
		if meta.RunID == "" {
			meta.RunID = newRunID()
			changed = true
		}
		if meta.RootRunID == "" {
			meta.RootRunID = meta.RunID
			changed = true
		}
		if meta.Epoch <= 0 {
			meta.Epoch = 1
			changed = true
		}
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

func newRunID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s%d", runIDPrefix, time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("%s%x", runIDPrefix, buf)
}

func isRunIDSelector(selector string) bool {
	return strings.HasPrefix(selector, runIDPrefix)
}

func gitHeadRevision(projectRoot string) (string, error) {
	out, err := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("resolve git HEAD for %s: %w\n%s", projectRoot, err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
