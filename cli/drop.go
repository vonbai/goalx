package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Drop cleans up a run so the same name can be reused safely.
func Drop(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx drop [--run NAME]") {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx drop [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if unsaved, err := hasUnsavedRunArtifacts(rc.ProjectRoot, rc); err != nil {
		return err
	} else if unsaved {
		return fmt.Errorf("run '%s' has unsaved artifacts; run `goalx save %s` before drop", rc.Name, rc.Name)
	}
	if err := runtimeSupervisor.Stop(rc.RunDir); err != nil {
		return err
	}
	killRunPaneProcessTrees(rc.RunDir, rc.TmuxSession)
	_ = FinalizeControlRun(rc.RunDir, "dropped")

	// Kill tmux session if still active
	if SessionExistsInRun(rc.RunDir, rc.TmuxSession) {
		if err := KillSessionIfExistsInRun(rc.RunDir, rc.TmuxSession); err != nil {
			return fmt.Errorf("kill tmux session: %w", err)
		}
		fmt.Printf("Stopped tmux session %s\n", rc.TmuxSession)
	}

	if sourceRootAvailable(rc.ProjectRoot) {
		// Remove all session worktrees discovered in the run directory.
		sessionIndexes, err := existingSessionIndexes(rc.RunDir)
		if err != nil {
			return err
		}
		sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
		if err != nil {
			return fmt.Errorf("load session runtime state: %w", err)
		}
		removedWorktrees := map[string]bool{}
		removedBranches := map[string]bool{}
		removeSessionBranch := func(branch string) {
			if branch == "" || removedBranches[branch] {
				return
			}
			if err := DeleteBranch(rc.ProjectRoot, branch); err != nil {
				fmt.Printf("Warning: delete branch %s: %v\n", branch, err)
				return
			}
			removedBranches[branch] = true
			fmt.Printf("Deleted branch: %s\n", branch)
		}
		for _, num := range sessionIndexes {
			sessionName := SessionName(num)
			wtPath := resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, sessionName, sessionState)
			branch := resolvedSessionBranch(rc.RunDir, rc.Config.Name, sessionName, sessionState)
			if wtPath != "" {
				if err := RemoveWorktree(rc.ProjectRoot, wtPath); err != nil {
					fmt.Printf("Warning: remove worktree %s: %v\n", wtPath, err)
				} else {
					removedWorktrees[wtPath] = true
					fmt.Printf("Removed worktree: %s\n", wtPath)
				}
			}
			removeSessionBranch(branch)
		}

		worktreesDir := configuredWorktreesDir(rc.RunDir)
		if entries, err := os.ReadDir(worktreesDir); err == nil {
			prefix := rc.Config.Name + "-"
			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}
				name := entry.Name()
				if name == "root" || name == rc.Config.Name+"-root" {
					continue
				}
				if filepath.Clean(worktreesDir) != filepath.Clean(legacyWorktreesDir(rc.RunDir)) && !strings.HasPrefix(name, prefix) {
					continue
				}
				wtPath := filepath.Join(worktreesDir, name)
				if removedWorktrees[wtPath] {
					continue
				}
				if err := RemoveWorktree(rc.ProjectRoot, wtPath); err != nil {
					fmt.Printf("Warning: remove worktree %s: %v\n", wtPath, err)
				} else {
					removedWorktrees[wtPath] = true
					fmt.Printf("Removed worktree: %s\n", wtPath)
				}
				if i := strings.LastIndex(name, "-"); i >= 0 {
					if idx, err := parseSessionIndex("session-" + name[i+1:]); err == nil {
						removeSessionBranch(fmt.Sprintf("goalx/%s/%d", rc.Config.Name, idx))
					}
				}
			}
		}
		runWT := RunWorktreePath(rc.RunDir)
		if !removedWorktrees[runWT] {
			if info, err := os.Stat(runWT); err == nil && info.IsDir() {
				if err := RemoveWorktree(rc.ProjectRoot, runWT); err != nil {
					fmt.Printf("Warning: remove run worktree %s: %v\n", runWT, err)
				} else {
					removedWorktrees[runWT] = true
					fmt.Printf("Removed worktree: %s\n", runWT)
				}
			}
		}
		removeSessionBranch(fmt.Sprintf("goalx/%s/root", rc.Config.Name))
	}

	if err := os.RemoveAll(rc.RunDir); err != nil {
		return fmt.Errorf("remove run dir %s: %w", rc.RunDir, err)
	}
	if err := RemoveRunRegistration(rc.ProjectRoot, rc.Name); err != nil {
		return fmt.Errorf("remove run registry entry: %w", err)
	}

	fmt.Printf("Run '%s' dropped. Removed run data at %s\n", rc.Name, rc.RunDir)
	return nil
}

func sourceRootAvailable(projectRoot string) bool {
	info, err := os.Stat(projectRoot)
	return err == nil && info.IsDir()
}

func hasUnsavedRunArtifacts(projectRoot string, rc *RunContext) (bool, error) {
	if _, err := ResolveSavedRunLocation(rc.ProjectRoot, rc.Name); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	manifest, err := EnsureRunArtifacts(rc.RunDir, rc.Config)
	if err != nil {
		return false, err
	}
	for _, session := range manifest.Sessions {
		if len(session.Artifacts) > 0 {
			return true, nil
		}
	}

	for _, path := range []string{
		filepath.Join(rc.RunDir, "summary.md"),
		IntegrationStatePath(rc.RunDir),
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
			return true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}
