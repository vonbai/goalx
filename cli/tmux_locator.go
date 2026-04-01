package cli

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

type TmuxLocator struct {
	Version   int    `json:"version"`
	Session   string `json:"session,omitempty"`
	SocketDir string `json:"socket_dir,omitempty"`
}

func TmuxLocatorPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "tmux-locator.json")
}

func LoadTmuxLocator(path string) (*TmuxLocator, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	locator := &TmuxLocator{}
	if len(strings.TrimSpace(string(data))) != 0 {
		if err := json.Unmarshal(data, locator); err != nil {
			return nil, fmt.Errorf("parse tmux locator: %w", err)
		}
	}
	if locator.Version == 0 {
		locator.Version = 1
	}
	return locator, nil
}

func SaveTmuxLocator(path string, locator *TmuxLocator) error {
	if locator == nil {
		return fmt.Errorf("tmux locator is nil")
	}
	if locator.Version == 0 {
		locator.Version = 1
	}
	return writeJSONFile(path, locator)
}

func resolveRunTmuxSession(projectRoot, runDir, runName string) string {
	if locator, err := LoadTmuxLocator(TmuxLocatorPath(runDir)); err == nil && locator != nil {
		if session := strings.TrimSpace(locator.Session); session != "" {
			return session
		}
	}
	return goalx.TmuxSessionName(projectRoot, runName)
}

func resolveRunTmuxSocketDir(_ string, runDir, _ string) string {
	if strings.TrimSpace(runDir) == "" {
		return ""
	}
	if locator, err := LoadTmuxLocator(TmuxLocatorPath(runDir)); err == nil && locator != nil {
		if socketDir := strings.TrimSpace(locator.SocketDir); socketDir != "" && !tmuxSocketPathTooLong(socketDir) {
			return socketDir
		}
	}
	return defaultRunTmuxSocketDir(runDir)
}

func ensureRunTmuxLocator(projectRoot, runDir, runName string) (string, error) {
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		return "", err
	}
	session := resolveRunTmuxSession(projectRoot, runDir, runName)
	socketDir := resolveRunTmuxSocketDir(projectRoot, runDir, runName)
	if err := os.MkdirAll(socketDir, 0o755); err != nil {
		return "", err
	}
	if err := SaveTmuxLocator(TmuxLocatorPath(runDir), &TmuxLocator{
		Version:   1,
		Session:   session,
		SocketDir: socketDir,
	}); err != nil {
		return "", err
	}
	return session, nil
}

func defaultRunTmuxSocketDir(runDir string) string {
	cleanRunDir := filepath.Clean(strings.TrimSpace(runDir))
	if cleanRunDir == "" || cleanRunDir == "." {
		return ""
	}
	sum := sha1.Sum([]byte(cleanRunDir))
	key := hex.EncodeToString(sum[:8])
	// Keep the transport root short enough for the eventual tmux socket path.
	return filepath.Join(os.TempDir(), "goalx-tmux", key)
}

func tmuxSocketPathTooLong(socketDir string) bool {
	socketDir = strings.TrimSpace(socketDir)
	if socketDir == "" {
		return false
	}
	socketPath := filepath.Join(socketDir, "tmux-0", "default")
	return len(socketPath) >= 100
}
