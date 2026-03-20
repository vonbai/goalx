package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ar "github.com/vonbai/autoresearch"
)

// Next detects the current pipeline state and suggests the next action.
func Next(projectRoot string, _ []string) error {
	home, _ := os.UserHomeDir()
	runsDir := filepath.Join(home, ".autoresearch", "runs", ar.ProjectID(projectRoot))
	savesDir := filepath.Join(projectRoot, ".goalx", "runs")

	// Check for active runs
	activeRun := findActiveRun(projectRoot, runsDir)
	if activeRun != "" {
		fmt.Printf("Active run: %s\n", activeRun)
		fmt.Printf("  → goalx attach %s\n", activeRun)
		return nil
	}

	// Check for completed (not yet saved) runs
	completedRun := findCompletedRun(projectRoot, runsDir)
	if completedRun != "" {
		fmt.Printf("Completed run: %s (not yet saved)\n", completedRun)
		fmt.Printf("  → goalx save %s    # save artifacts to .goalx/runs/\n", completedRun)
		fmt.Printf("  → goalx review %s  # inspect results\n", completedRun)
		fmt.Printf("  → goalx drop %s    # clean up worktrees\n", completedRun)
		return nil
	}

	// Check saved runs in .goalx/runs/
	hasSaves := false
	hasDebate := false
	hasResearch := false
	latestName := ""

	if entries, err := os.ReadDir(savesDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			hasSaves = true
			dir := filepath.Join(savesDir, e.Name())
			cfg, err := ar.LoadYAML[ar.Config](filepath.Join(dir, "goalx.yaml"))
			if err != nil {
				continue
			}
			latestName = e.Name()
			if cfg.Mode == ar.ModeResearch {
				hasResearch = true
				// Check if it looks like a debate (has "debate" or "consensus" in name/objective)
				if e.Name() == "debate" || containsAny(cfg.Objective, "辩论", "debate", "共识", "consensus") {
					hasDebate = true
				}
			}
		}
	}

	if hasDebate {
		fmt.Printf("Debate completed: %s\n", latestName)
		fmt.Println("  → goalx implement   # generate develop config from consensus")
		fmt.Println("  → goalx start       # start implementation")
		return nil
	}

	if hasResearch {
		fmt.Printf("Research completed: %s\n", latestName)
		fmt.Println("  → goalx debate      # generate debate config from research")
		fmt.Println("  → goalx start       # start debate round")
		fmt.Println()
		fmt.Println("  Or skip debate:")
		fmt.Println("  → goalx implement   # generate develop config directly")
		return nil
	}

	if hasSaves {
		fmt.Println("Saved runs exist but no clear next step detected.")
		fmt.Println("  → goalx list        # see all runs")
		fmt.Println("  → goalx init \"...\"  # start a new research")
		return nil
	}

	// Nothing exists
	fmt.Println("No runs or saved results found.")
	fmt.Println()
	fmt.Println("Quickstart:")
	fmt.Println("  goalx init \"your objective\" --research --parallel 2")
	fmt.Println("  goalx start")
	return nil
}

func findActiveRun(projectRoot, runsDir string) string {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tmuxSess := ar.TmuxSessionName(projectRoot, e.Name())
		if SessionExists(tmuxSess) {
			return e.Name()
		}
	}
	return ""
}

func findCompletedRun(projectRoot, runsDir string) string {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tmuxSess := ar.TmuxSessionName(projectRoot, e.Name())
		if !SessionExists(tmuxSess) {
			return e.Name()
		}
	}
	return ""
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
