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

type SessionIdentity struct {
	Version         int                 `json:"version"`
	SessionName     string              `json:"session_name,omitempty"`
	RoleKind        string              `json:"role_kind,omitempty"`
	Mode            string              `json:"mode,omitempty"`
	Engine          string              `json:"engine,omitempty"`
	Model           string              `json:"model,omitempty"`
	Target          goalx.TargetConfig  `json:"target,omitempty"`
	Harness         goalx.HarnessConfig `json:"harness,omitempty"`
	OriginCharterID string              `json:"origin_charter_id,omitempty"`
	CreatedAt       string              `json:"created_at,omitempty"`
}

func SessionIdentityPath(runDir, sessionName string) string {
	return filepath.Join(runDir, "sessions", sessionName, "identity.json")
}

func RequireSessionIdentity(runDir, sessionName string) (*SessionIdentity, error) {
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, sessionName))
	if err != nil {
		return nil, err
	}
	if identity == nil {
		return nil, fmt.Errorf("session identity missing at %s", SessionIdentityPath(runDir, sessionName))
	}
	charter, err := RequireRunCharter(runDir)
	if err != nil {
		return nil, err
	}
	if err := ValidateSessionIdentityLinkage(identity, charter); err != nil {
		return nil, err
	}
	return identity, nil
}

func NewSessionIdentity(runDir, sessionName, roleKind string, mode goalx.Mode, engine, model string, target goalx.TargetConfig, harness goalx.HarnessConfig) (*SessionIdentity, error) {
	charter, err := RequireRunCharter(runDir)
	if err != nil {
		return nil, err
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	if err := ValidateRunCharterLinkage(meta, charter); err != nil {
		return nil, err
	}

	identity := &SessionIdentity{
		Version:         1,
		SessionName:     sessionName,
		RoleKind:        roleKind,
		Mode:            string(mode),
		Engine:          engine,
		Model:           model,
		Target:          target,
		Harness:         harness,
		OriginCharterID: charter.CharterID,
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := ValidateSessionIdentityLinkage(identity, charter); err != nil {
		return nil, err
	}
	normalizeSessionIdentity(identity)
	return identity, nil
}

func LoadSessionIdentity(path string) (*SessionIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var identity SessionIdentity
	if len(strings.TrimSpace(string(data))) == 0 {
		identity.Version = 1
		normalizeSessionIdentity(&identity)
		return &identity, nil
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return nil, fmt.Errorf("parse session identity: %w", err)
	}
	normalizeSessionIdentity(&identity)
	return &identity, nil
}

func SaveSessionIdentity(path string, identity *SessionIdentity) error {
	if identity == nil {
		return fmt.Errorf("session identity is nil")
	}
	normalizeSessionIdentity(identity)
	if identity.CreatedAt == "" {
		identity.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(identity, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("session identity already exists at %s", path)
		}
		return err
	}
	defer file.Close()
	_, err = file.Write(data)
	return err
}

func normalizeSessionIdentity(identity *SessionIdentity) {
	if identity == nil {
		return
	}
	if identity.Version <= 0 {
		identity.Version = 1
	}
	if strings.TrimSpace(identity.Mode) == "" {
		identity.Mode = string(goalx.ModeDevelop)
	}
}

func sessionRoleKind(mode goalx.Mode) string {
	switch mode {
	case goalx.ModeResearch:
		return "master-derived-research"
	case goalx.ModeDevelop:
		return "master-derived-develop"
	default:
		if trimmed := strings.TrimSpace(string(mode)); trimmed != "" {
			return "master-derived-" + trimmed
		}
		return "master-derived"
	}
}
