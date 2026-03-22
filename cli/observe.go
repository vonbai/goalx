package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Observe captures live tmux pane output for all windows in a run.
func Observe(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx observe [NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil && runName == "" {
		// Fallback: find the only active run
		rc, err = findSingleActiveRun(projectRoot)
	}
	if err != nil {
		return err
	}

	if !SessionExists(rc.TmuxSession) {
		return fmt.Errorf("run '%s' is not active (no tmux session)", rc.Name)
	}

	fmt.Printf("## Run: %s — Observe\n\n", rc.Name)

	// Show run runtime summary if available
	statusPath := RunRuntimeStatePath(rc.RunDir)
	if data, err := os.ReadFile(statusPath); err == nil && len(data) > 0 {
		fmt.Println("### Status (from master)")
		fmt.Println(strings.TrimSpace(string(data)))
		fmt.Println()
	}

	// Master
	fmt.Println("### master")
	printPaneCapture(rc.TmuxSession, "master")
	fmt.Println()

	// Sessions
	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	for _, num := range sessionIndexes {
		windowName := sessionWindowName(rc.Config.Name, num)
		fmt.Printf("### %s\n", SessionName(num))
		printPaneCapture(rc.TmuxSession, windowName)
		fmt.Println()
	}

	// Check for dynamically added sessions (windows beyond the configured count)
	// by listing all tmux windows
	out, err := exec.Command("tmux", "list-windows", "-t", rc.TmuxSession, "-F", "#{window_name}").Output()
	if err == nil {
		configured := make(map[string]bool)
		configured["master"] = true
		for _, num := range sessionIndexes {
			configured[sessionWindowName(rc.Config.Name, num)] = true
		}
		for _, w := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if w != "" && !configured[w] {
				fmt.Printf("### %s (dynamic)\n", w)
				printPaneCapture(rc.TmuxSession, w)
				fmt.Println()
			}
		}
	}

	return nil
}

func printPaneCapture(tmuxSession, window string) {
	out, err := exec.Command(
		"tmux", "capture-pane",
		"-t", tmuxSession+":"+window,
		"-p", "-S", "-200",
	).Output()
	if err != nil {
		fmt.Println("(window not found)")
		return
	}

	// Filter empty lines and take last 20
	lines := strings.Split(string(out), "\n")
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) == 0 {
		fmt.Println("(no output)")
		return
	}
	start := 0
	if len(nonEmpty) > 20 {
		start = len(nonEmpty) - 20
	}
	for _, l := range nonEmpty[start:] {
		fmt.Println(l)
	}
}

// findSingleActiveRun finds the only active run for this project.
// Returns error if zero or multiple active runs.
func findSingleActiveRun(projectRoot string) (*RunContext, error) {
	runName, err := ResolveDefaultRunName(projectRoot)
	if err != nil {
		return nil, err
	}
	return ResolveRun(projectRoot, runName)
}
