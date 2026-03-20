package cli

import (
	"fmt"
	"os"
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

	// Kill tmux session if still active
	if SessionExists(rc.TmuxSession) {
		if err := KillSession(rc.TmuxSession); err != nil {
			return fmt.Errorf("kill tmux session: %w", err)
		}
		fmt.Printf("Stopped tmux session %s\n", rc.TmuxSession)
	}

	// Remove all worktrees
	count := sessionCount(rc.Config)
	for num := 1; num <= count; num++ {
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
