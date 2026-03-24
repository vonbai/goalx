package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IdentityFence struct {
	Version          int    `json:"version"`
	RunID            string `json:"run_id,omitempty"`
	Epoch            int    `json:"epoch,omitempty"`
	CharterHash      string `json:"charter_hash,omitempty"`
	GoalHash         string `json:"goal_hash,omitempty"`
	AcceptanceHash   string `json:"acceptance_hash,omitempty"`
	CoordinationHash string `json:"coordination_hash,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

func IdentityFencePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "identity-fence.json")
}

func NewIdentityFence(runDir string, meta *RunMetadata) (*IdentityFence, error) {
	if meta == nil {
		var err error
		meta, err = LoadRunMetadata(RunMetadataPath(runDir))
		if err != nil {
			return nil, err
		}
	}
	if meta == nil {
		return nil, fmt.Errorf("run metadata is nil")
	}

	fence := &IdentityFence{
		Version: 1,
		RunID:   meta.RunID,
		Epoch:   meta.Epoch,
	}
	charterHash, err := hashFileContents(RunCharterPath(runDir))
	if err != nil {
		return nil, err
	}
	goalHash, err := hashFileContents(GoalPath(runDir))
	if err != nil {
		return nil, err
	}
	acceptanceHash, err := hashFileContents(AcceptanceStatePath(runDir))
	if err != nil {
		return nil, err
	}
	coordinationHash, err := hashFileContents(CoordinationPath(runDir))
	if err != nil {
		return nil, err
	}
	fence.CharterHash = charterHash
	fence.GoalHash = goalHash
	fence.AcceptanceHash = acceptanceHash
	fence.CoordinationHash = coordinationHash
	fence.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return fence, nil
}

func LoadIdentityFence(path string) (*IdentityFence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var fence IdentityFence
	if len(strings.TrimSpace(string(data))) == 0 {
		fence.Version = 1
		return &fence, nil
	}
	if err := json.Unmarshal(data, &fence); err != nil {
		return nil, fmt.Errorf("parse identity fence: %w", err)
	}
	if fence.Version <= 0 {
		fence.Version = 1
	}
	return &fence, nil
}

func SaveIdentityFence(path string, fence *IdentityFence) error {
	if fence == nil {
		return fmt.Errorf("identity fence is nil")
	}
	if fence.Version <= 0 {
		fence.Version = 1
	}
	if fence.UpdatedAt == "" {
		fence.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(fence, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func hashFileContents(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
