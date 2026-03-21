package cli

import (
	"fmt"
	"os"
	"os/exec"
)

// Diff shows the code diff between two sessions, or a single session vs main.
func Diff(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) < 1 || len(rest) > 2 {
		return fmt.Errorf("usage: goalx diff [--run NAME] <session-a> [session-b]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	idxA, err := parseSessionIndex(rest[0])
	if err != nil {
		return err
	}
	ok, err := hasSessionIndex(rc.RunDir, idxA)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session %q out of range for run %q", rest[0], rc.Name)
	}
	branchA := fmt.Sprintf("goalx/%s/%d", rc.Config.Name, idxA)

	if len(rest) == 2 {
		// Diff session A vs session B
		idxB, err := parseSessionIndex(rest[1])
		if err != nil {
			return err
		}
		ok, err := hasSessionIndex(rc.RunDir, idxB)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("session %q out of range for run %q", rest[1], rc.Name)
		}
		branchB := fmt.Sprintf("goalx/%s/%d", rc.Config.Name, idxB)
		fmt.Printf("=== diff %s vs %s ===\n", rest[0], rest[1])
		cmd := exec.Command("git", "-C", rc.ProjectRoot, "diff", branchA+".."+branchB, "--stat")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	// Diff session A vs current branch in the main project worktree.
	fmt.Printf("=== diff %s vs current branch ===\n", rest[0])
	cmd := exec.Command("git", "-C", rc.ProjectRoot, "diff", "HEAD.."+branchA, "--stat")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
