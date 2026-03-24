package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type LaunchEnvSnapshot struct {
	Version    int               `json:"version"`
	CapturedAt string            `json:"captured_at,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
}

var launchEnvDenylist = map[string]struct{}{
	"CLAUDE_SESSION_ID":    {},
	"CODEX_SESSION_ID":     {},
	"CODEX_THREAD_ID":      {},
	"OLDPWD":               {},
	"PROMPT_COMMAND":       {},
	"PS1":                  {},
	"PWD":                  {},
	"SHLVL":                {},
	"TERM_PROGRAM":         {},
	"TERM_PROGRAM_VERSION": {},
	"TMUX":                 {},
	"TMUX_PANE":            {},
}

func ControlLaunchEnvPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "launch-env.json")
}

func buildEngineLaunchCommand(engineCmd, prompt string) string {
	return buildEngineLaunchCommandWithEnv(currentLaunchEnv(), engineCmd, prompt)
}

func buildEngineLaunchCommandWithEnv(launchEnv map[string]string, engineCmd, prompt string) string {
	parts := []string{buildLaunchEnvPrefix(launchEnv), buildEngineExecCommand(engineCmd, prompt)}
	return strings.Join(parts, " ")
}

func buildMasterLaunchCommand(goalxBin, runName, runDir, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	return buildMasterLaunchCommandWithEnv(currentLaunchEnv(), goalxBin, runName, runDir, runID, epoch, ttl, engineCmd, prompt)
}

func buildMasterLaunchCommandWithEnv(launchEnv map[string]string, goalxBin, runName, runDir, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	return buildLeaseWrappedLaunchCommandWithEnv(launchEnv, goalxBin, runName, runDir, "master", runID, epoch, ttl, engineCmd, prompt)
}

func buildLeaseWrappedLaunchCommand(goalxBin, runName, runDir, holder, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	return buildLeaseWrappedLaunchCommandWithEnv(currentLaunchEnv(), goalxBin, runName, runDir, holder, runID, epoch, ttl, engineCmd, prompt)
}

func buildLeaseWrappedLaunchCommandWithEnv(launchEnv map[string]string, goalxBin, runName, runDir, holder, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	ttlSeconds := int(ttl.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 30
	}
	script := fmt.Sprintf(
		"%s lease-loop --run %s --run-dir %s --holder %s --run-id %s --epoch %d --ttl-seconds %d --transport tmux --pid $$ >/dev/null 2>&1 & exec %s",
		shellQuote(goalxBin),
		shellQuote(runName),
		shellQuote(runDir),
		shellQuote(holder),
		shellQuote(runID),
		epoch,
		ttlSeconds,
		buildEngineExecCommand(engineCmd, prompt),
	)
	return buildLaunchEnvPrefix(launchEnv) + " /bin/bash -c " + shellQuote(script)
}

func buildEngineExecCommand(engineCmd, prompt string) string {
	return engineCmd + " " + shellQuote(prompt)
}

func buildLaunchEnvPrefix(env map[string]string) string {
	parts := []string{"env"}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+shellQuote(env[key]))
	}
	return strings.Join(parts, " ")
}

func currentLaunchEnv() map[string]string {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !shouldPropagateLaunchEnv(key) {
			continue
		}
		env[key] = value
	}
	return env
}

func shouldPropagateLaunchEnv(key string) bool {
	_, denied := launchEnvDenylist[key]
	return !denied
}

func CaptureCurrentLaunchEnvSnapshot() *LaunchEnvSnapshot {
	return &LaunchEnvSnapshot{
		Version:    1,
		CapturedAt: time.Now().UTC().Format(time.RFC3339),
		Env:        currentLaunchEnv(),
	}
}

func LoadLaunchEnvSnapshot(path string) (*LaunchEnvSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	snapshot := &LaunchEnvSnapshot{}
	if len(strings.TrimSpace(string(data))) == 0 {
		snapshot.Version = 1
		snapshot.Env = map[string]string{}
		return snapshot, nil
	}
	if err := json.Unmarshal(data, snapshot); err != nil {
		return nil, fmt.Errorf("parse launch env snapshot: %w", err)
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	if snapshot.Env == nil {
		snapshot.Env = map[string]string{}
	}
	return snapshot, nil
}

func SaveLaunchEnvSnapshot(path string, snapshot *LaunchEnvSnapshot) error {
	if snapshot == nil {
		return fmt.Errorf("launch env snapshot is nil")
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	if snapshot.CapturedAt == "" {
		snapshot.CapturedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if snapshot.Env == nil {
		snapshot.Env = map[string]string{}
	}
	return writeJSONFile(path, snapshot)
}

func RequireLaunchEnvSnapshot(runDir string) (*LaunchEnvSnapshot, error) {
	snapshot, err := LoadLaunchEnvSnapshot(ControlLaunchEnvPath(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("missing run launch env snapshot at %s", ControlLaunchEnvPath(runDir))
		}
		return nil, err
	}
	return snapshot, nil
}
