package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type cleanupStack struct {
	steps []func() error
}

func (s *cleanupStack) Add(step func() error) {
	if step == nil {
		return
	}
	s.steps = append(s.steps, step)
}

func (s *cleanupStack) Commit() {
	s.steps = nil
}

func (s *cleanupStack) Run() error {
	if s == nil || len(s.steps) == 0 {
		return nil
	}
	var errs []error
	for i := len(s.steps) - 1; i >= 0; i-- {
		if err := s.steps[i](); err != nil && !os.IsNotExist(err) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func cleanupSessionIdentitySurface(runDir, sessionName string) error {
	path := SessionIdentityPath(runDir, sessionName)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
		return nil
	}
	return nil
}

func cleanupSessionControlSurface(runDir, sessionName string) error {
	return errors.Join(
		removeIfExists(ControlInboxPath(runDir, sessionName)),
		removeIfExists(SessionCursorPath(runDir, sessionName)),
		removeIfExists(ControlLeasePath(runDir, sessionName)),
		removeIfExists(PanePIDPath(runDir, sessionName)),
	)
}

func cleanupSessionProgram(runDir string, idx int) error {
	return removeIfExists(filepath.Join(runDir, sessionNameToProgramFile(idx)))
}

func cleanupSessionJournal(runDir, sessionName string) error {
	return removeIfExists(JournalPath(runDir, sessionName))
}

func cleanupSessionRuntimeEntry(runDir, sessionName string) error {
	if _, err := os.Stat(SessionsRuntimeStatePath(runDir)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return RemoveSessionRuntimeState(runDir, sessionName)
}

func cleanupSessionWorktreeBoundary(repoRoot, worktreePath, branch string) error {
	if worktreePath == "" && branch == "" {
		return nil
	}
	var errs []error
	if info, err := os.Stat(worktreePath); err == nil && info.IsDir() {
		if err := RemoveWorktree(repoRoot, worktreePath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove worktree %s: %w", worktreePath, err))
		}
	}
	if branch != "" {
		exists, err := branchExists(repoRoot, branch)
		if err != nil {
			errs = append(errs, err)
		} else if exists {
			inUse, inUseErr := branchCheckedOutInAnyWorktree(repoRoot, branch)
			if inUseErr != nil {
				errs = append(errs, inUseErr)
			} else if !inUse {
				if err := DeleteBranch(repoRoot, branch); err != nil {
					errs = append(errs, fmt.Errorf("delete branch %s: %w", branch, err))
				}
			}
		}
	}
	return errors.Join(errs...)
}

func cleanupSessionWindow(runDir, tmuxSession, windowName string) error {
	if tmuxSession == "" || windowName == "" {
		return nil
	}
	if !SessionExistsInRun(runDir, tmuxSession) || !WindowExistsInRun(runDir, tmuxSession, windowName) {
		return nil
	}
	return KillWindowInRun(runDir, tmuxSession, windowName)
}

func removeIfExists(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func restoreParkedSession(projectRoot, runName, runDir, sessionName, scope string) error {
	err := Resume(projectRoot, []string{"--run", runName, sessionName})
	if err == nil {
		return nil
	}
	if !strings.Contains(err.Error(), "already active") {
		return err
	}

	sessionState, stateErr := EnsureSessionsRuntimeState(runDir)
	if stateErr != nil {
		return stateErr
	}
	current := sessionState.Sessions[sessionName]
	identity, identityErr := RequireSessionIdentity(runDir, sessionName)
	if identityErr != nil {
		return identityErr
	}
	return UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         sessionName,
		State:        "active",
		Mode:         identity.Mode,
		Branch:       resolvedSessionBranch(runDir, runName, sessionName, sessionState),
		WorktreePath: resolvedSessionWorktreePath(runDir, runName, sessionName, sessionState),
		OwnerScope:   scopeOrFallback(scope, current.OwnerScope, sessionName),
	})
}

func scopeOrFallback(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
