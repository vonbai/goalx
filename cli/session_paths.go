package cli

import (
	"fmt"
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

// WorktreePath returns the worktree directory for a session.
func WorktreePath(runDir, cfgName string, num int) string {
	return filepath.Join(runDir, "worktrees", cfgName+"-"+strconv.Itoa(num))
}

// GuidancePath returns the guidance file path for a session.
func GuidancePath(runDir, sessionName string) string {
	return filepath.Join(runDir, "guidance", sessionName+".md")
}
