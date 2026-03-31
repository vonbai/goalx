package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// SessionName returns the canonical session name for a 1-based index.
func SessionName(num int) string {
	return fmt.Sprintf("session-%d", num)
}

// JournalPath returns the journal file path for a session.
func JournalPath(runDir, sessionName string) string {
	return filepath.Join(runDir, "journals", sessionName+".jsonl")
}

func legacyWorktreesDir(runDir string) string {
	return filepath.Join(runDir, "worktrees")
}

func configuredWorktreesDir(runDir string) string {
	root, _, ok := loadConfiguredWorktreeRoot(runDir)
	if !ok {
		return legacyWorktreesDir(runDir)
	}
	return root
}

func loadConfiguredWorktreeRoot(runDir string) (root string, configured bool, ok bool) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil || cfg == nil {
		return "", false, false
	}
	raw := strings.TrimSpace(cfg.WorktreeRoot)
	if raw == "" {
		return legacyWorktreesDir(runDir), false, true
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil || meta == nil || strings.TrimSpace(meta.ProjectRoot) == "" {
		return "", true, false
	}
	return resolveConfiguredWorktreeRoot(strings.TrimSpace(meta.ProjectRoot), raw), true, true
}

func resolveConfiguredWorktreeRoot(projectRoot, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	return filepath.Clean(filepath.Join(projectRoot, raw))
}

func configuredRunWorktreePath(root, runName string) string {
	return filepath.Join(root, runName)
}

func legacyConfiguredRunWorktreePath(root, runName string) string {
	return filepath.Join(root, runName+"-root")
}

func isConfiguredRunWorktreeName(runName, name string) bool {
	return name == runName || name == runName+"-root"
}

func runWorktreePathForConfig(projectRoot, runDir string, cfg *goalx.Config) string {
	root := legacyWorktreesDir(runDir)
	if cfg != nil && strings.TrimSpace(cfg.WorktreeRoot) != "" {
		root = resolveConfiguredWorktreeRoot(projectRoot, cfg.WorktreeRoot)
		return configuredRunWorktreePath(root, cfg.Name)
	}
	return filepath.Join(root, "root")
}

func sessionWorktreePathForConfig(projectRoot, runDir string, cfg *goalx.Config, num int) string {
	name := ""
	if cfg != nil {
		name = cfg.Name
	}
	root := legacyWorktreesDir(runDir)
	if cfg != nil && strings.TrimSpace(cfg.WorktreeRoot) != "" {
		root = resolveConfiguredWorktreeRoot(projectRoot, cfg.WorktreeRoot)
	}
	return filepath.Join(root, name+"-"+strconv.Itoa(num))
}

// RunWorktreePath returns the run-scoped root worktree path.
func RunWorktreePath(runDir string) string {
	root, configured, ok := loadConfiguredWorktreeRoot(runDir)
	if !ok {
		return filepath.Join(legacyWorktreesDir(runDir), "root")
	}
	if configured {
		cfg, err := LoadRunSpec(runDir)
		if err == nil && cfg != nil {
			current := configuredRunWorktreePath(root, cfg.Name)
			if _, err := os.Stat(current); err == nil {
				return current
			}
			legacy := legacyConfiguredRunWorktreePath(root, cfg.Name)
			if _, err := os.Stat(legacy); err == nil {
				return legacy
			}
			return current
		}
	}
	return filepath.Join(root, "root")
}

// ReportsDir returns the run-scoped reports directory.
func ReportsDir(runDir string) string {
	return filepath.Join(runDir, "reports")
}

// SummaryPath returns the canonical run-level result surface.
func SummaryPath(runDir string) string {
	return filepath.Join(runDir, "summary.md")
}

// ExperimentsLogPath returns the canonical experiment ledger surface.
func ExperimentsLogPath(runDir string) string {
	return filepath.Join(runDir, "experiments.jsonl")
}

// WorktreePath returns the worktree directory for a session.
func WorktreePath(runDir, cfgName string, num int) string {
	return filepath.Join(configuredWorktreesDir(runDir), cfgName+"-"+strconv.Itoa(num))
}

func sessionStateWorktreePath(state *SessionsRuntimeState, sessionName string) (string, bool) {
	if state == nil {
		return "", false
	}
	session, ok := state.Sessions[sessionName]
	if !ok {
		return "", false
	}
	return session.WorktreePath, true
}

func resolvedSessionWorktreePath(runDir, runName, sessionName string, state *SessionsRuntimeState) string {
	if worktreePath, ok := sessionStateWorktreePath(state, sessionName); ok {
		return worktreePath
	}
	idx, err := parseSessionIndex(sessionName)
	if err != nil {
		return ""
	}
	legacyPath := WorktreePath(runDir, runName, idx)
	if info, err := os.Stat(legacyPath); err == nil && info.IsDir() {
		return legacyPath
	}
	if configured, isConfigured, ok := loadConfiguredWorktreeRoot(runDir); ok && isConfigured {
		legacyPath = filepath.Join(legacyWorktreesDir(runDir), runName+"-"+strconv.Itoa(idx))
		if info, err := os.Stat(legacyPath); err == nil && info.IsDir() {
			return legacyPath
		}
		if info, err := os.Stat(filepath.Join(configured, runName+"-"+strconv.Itoa(idx))); err == nil && info.IsDir() {
			return filepath.Join(configured, runName+"-"+strconv.Itoa(idx))
		}
	}
	return ""
}

func sessionWorkdir(runDir, runName, sessionName string, state *SessionsRuntimeState) string {
	if worktreePath := resolvedSessionWorktreePath(runDir, runName, sessionName, state); worktreePath != "" {
		return worktreePath
	}
	return RunWorktreePath(runDir)
}

func resolvedSessionBranch(runDir, runName, sessionName string, state *SessionsRuntimeState) string {
	if state != nil {
		if session, ok := state.Sessions[sessionName]; ok && session.Branch != "" {
			return session.Branch
		}
	}
	if resolvedSessionWorktreePath(runDir, runName, sessionName, state) == "" {
		return ""
	}
	idx, err := parseSessionIndex(sessionName)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("goalx/%s/%d", runName, idx)
}
