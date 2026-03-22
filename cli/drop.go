package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

// Drop cleans up a run so the same name can be reused safely.
func Drop(projectRoot string, args []string) error {
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
	if unsaved, err := hasUnsavedRunArtifacts(projectRoot, rc); err != nil {
		return err
	} else if unsaved {
		return fmt.Errorf("run '%s' has unsaved artifacts; run `goalx save %s` before drop", rc.Name, rc.Name)
	}

	// Kill tmux session if still active
	if SessionExists(rc.TmuxSession) {
		if err := KillSession(rc.TmuxSession); err != nil {
			return fmt.Errorf("kill tmux session: %w", err)
		}
		fmt.Printf("Stopped tmux session %s\n", rc.TmuxSession)
	}

	// Remove all session worktrees discovered in the run directory.
	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	for _, num := range sessionIndexes {
		wtPath := WorktreePath(rc.RunDir, rc.Config.Name, num)
		branch := fmt.Sprintf("goalx/%s/%d", rc.Config.Name, num)
		if err := RemoveWorktree(rc.ProjectRoot, wtPath); err != nil {
			fmt.Printf("Warning: remove worktree %s: %v\n", wtPath, err)
		} else {
			fmt.Printf("Removed worktree: %s\n", wtPath)
		}
		if err := DeleteBranch(rc.ProjectRoot, branch); err != nil {
			fmt.Printf("Warning: delete branch %s: %v\n", branch, err)
		} else {
			fmt.Printf("Deleted branch: %s\n", branch)
		}
	}

	if err := os.RemoveAll(rc.RunDir); err != nil {
		return fmt.Errorf("remove run dir %s: %w", rc.RunDir, err)
	}

	fmt.Printf("Run '%s' dropped. Removed run data at %s\n", rc.Name, rc.RunDir)
	return nil
}

func hasUnsavedRunArtifacts(projectRoot string, rc *RunContext) (bool, error) {
	saveDir := filepath.Join(projectRoot, ".goalx", "runs", rc.Name)
	if _, err := os.Stat(saveDir); err == nil {
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
		filepath.Join(rc.RunDir, "selection.json"),
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Size() > 0 {
			return true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return false, err
		}
	}
	return false, nil
}
