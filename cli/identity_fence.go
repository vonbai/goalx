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
	Version              int    `json:"version"`
	RunID                string `json:"run_id,omitempty"`
	Epoch                int    `json:"epoch,omitempty"`
	CharterHash          string `json:"charter_hash,omitempty"`
	ObligationModelHash  string `json:"obligation_model_hash,omitempty"`
	AssurancePlanHash    string `json:"assurance_plan_hash,omitempty"`
	CoordinationHash     string `json:"coordination_hash,omitempty"`
	UpdatedAt            string `json:"updated_at,omitempty"`
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
	obligationModelHash, err := hashOptionalCanonicalBoundary(runDir)
	if err != nil {
		return nil, err
	}
	assurancePlanHash, err := hashOptionalCanonicalAssurance(runDir)
	if err != nil {
		return nil, err
	}
	coordinationHash, err := hashFileContents(CoordinationPath(runDir))
	if err != nil {
		return nil, err
	}
	fence.CharterHash = charterHash
	fence.ObligationModelHash = obligationModelHash
	fence.AssurancePlanHash = assurancePlanHash
	fence.CoordinationHash = coordinationHash
	fence.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return fence, nil
}

func RefreshIdentityFence(runDir string, meta *RunMetadata) (*IdentityFence, bool, error) {
	path := IdentityFencePath(runDir)
	current, err := LoadIdentityFence(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, false, err
	}
	next, err := NewIdentityFence(runDir, meta)
	if err != nil {
		return nil, false, err
	}
	changed := current == nil || !sameIdentityFence(current, next)
	if changed {
		if err := SaveIdentityFence(path, next); err != nil {
			return nil, false, err
		}
	}
	return next, changed, nil
}

func LoadIdentityFence(path string) (*IdentityFence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type identityFenceCompat struct {
		Version             int    `json:"version"`
		RunID               string `json:"run_id,omitempty"`
		Epoch               int    `json:"epoch,omitempty"`
		CharterHash         string `json:"charter_hash,omitempty"`
		ObligationModelHash string `json:"obligation_model_hash,omitempty"`
		AssurancePlanHash   string `json:"assurance_plan_hash,omitempty"`
		CoordinationHash    string `json:"coordination_hash,omitempty"`
		UpdatedAt           string `json:"updated_at,omitempty"`
	}
	var payload identityFenceCompat
	if len(strings.TrimSpace(string(data))) == 0 {
		return &IdentityFence{Version: 1}, nil
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse identity fence: %w", err)
	}
	fence := &IdentityFence{
		Version:             payload.Version,
		RunID:               payload.RunID,
		Epoch:               payload.Epoch,
		CharterHash:         payload.CharterHash,
		ObligationModelHash: strings.TrimSpace(payload.ObligationModelHash),
		AssurancePlanHash:   strings.TrimSpace(payload.AssurancePlanHash),
		CoordinationHash:    payload.CoordinationHash,
		UpdatedAt:           payload.UpdatedAt,
	}
	if fence.Version <= 0 {
		fence.Version = 1
	}
	return fence, nil
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

func sameIdentityFence(a, b *IdentityFence) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.RunID == b.RunID &&
		a.Epoch == b.Epoch &&
		a.CharterHash == b.CharterHash &&
		a.ObligationModelHash == b.ObligationModelHash &&
		a.AssurancePlanHash == b.AssurancePlanHash &&
		a.CoordinationHash == b.CoordinationHash
}
