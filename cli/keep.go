package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	goalx "github.com/vonbai/goalx"
)

// Keep merges or preserves a specific session from a run.
func Keep(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: goalx keep [--run NAME] <session-name>")
	}
	sessionName := rest[0]

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	// Parse session index from name
	idx, err := parseSessionIndex(sessionName)
	if err != nil {
		return err
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session %q out of range for run %q", sessionName, rc.Name)
	}

	branch := fmt.Sprintf("goalx/%s/%d", rc.Config.Name, idx)

	if rc.Config.Mode == goalx.ModeDevelop {
		fmt.Printf("Merging branch %s into current branch...\n", branch)
		if err := MergeWorktree(rc.ProjectRoot, branch); err != nil {
			return fmt.Errorf("merge %s: %w", branch, err)
		}
		fmt.Printf("Merged %s successfully.\n", branch)
	} else {
		wtPath := WorktreePath(rc.RunDir, rc.Config.Name, idx)
		fmt.Printf("Research session preserved at: %s\n", wtPath)
		fmt.Printf("Branch: %s\n", branch)
	}

	// Write selection.json
	selection := map[string]string{
		"kept":   sessionName,
		"branch": branch,
	}
	data, _ := json.MarshalIndent(selection, "", "  ")
	selPath := filepath.Join(rc.RunDir, "selection.json")
	if err := os.WriteFile(selPath, data, 0644); err != nil {
		return fmt.Errorf("write selection.json: %w", err)
	}
	fmt.Printf("Selection recorded: %s\n", selPath)
	return nil
}

func parseSessionIndex(name string) (int, error) {
	// Expect "session-N"
	if len(name) > 8 && name[:8] == "session-" {
		n, err := strconv.Atoi(name[8:])
		if err == nil && n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("invalid session name %q (expected session-N)", name)
}
