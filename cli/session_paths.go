package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// SessionName returns the canonical session name for a 1-based index.
func SessionName(num int) string {
	return fmt.Sprintf("session-%d", num)
}

// JournalPath returns the journal file path for a session.
func JournalPath(runDir, sessionName string) string {
	return filepath.Join(runDir, "journals", sessionName+".jsonl")
}

// RunWorktreePath returns the run-scoped root worktree path.
func RunWorktreePath(runDir string) string {
	return filepath.Join(runDir, "worktrees", "root")
}

// ReportsDir returns the run-scoped reports directory.
func ReportsDir(runDir string) string {
	return filepath.Join(runDir, "reports")
}

// WorktreePath returns the worktree directory for a session.
func WorktreePath(runDir, cfgName string, num int) string {
	return filepath.Join(runDir, "worktrees", cfgName+"-"+strconv.Itoa(num))
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
