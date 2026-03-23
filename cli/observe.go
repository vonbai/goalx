package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// Observe captures live tmux pane output for all windows in a run.
func Observe(projectRoot string, args []string) error {
	if len(args) == 1 && isHelpToken(args[0]) {
		fmt.Println("usage: goalx observe [NAME]")
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
		return fmt.Errorf("usage: goalx observe [NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil && runName == "" {
		rc, err = findSingleRunnableRun(projectRoot)
	}
	if err != nil {
		return err
	}

	fmt.Printf("## Run: %s — Observe\n\n", rc.Name)
	printStatusControlSummary(rc)

	// Show run runtime summary if available
	statusPath := RunRuntimeStatePath(rc.RunDir)
	if data, err := os.ReadFile(statusPath); err == nil && len(data) > 0 {
		fmt.Println("### Status (from master)")
		fmt.Println(strings.TrimSpace(string(data)))
		fmt.Println()
	}

	if !SessionExists(rc.TmuxSession) {
		fmt.Println("### transport")
		fmt.Println("transport degraded (no tmux session)")
		fmt.Println()

		fmt.Println("### master")
		printJournalExcerpt(filepath.Join(rc.RunDir, "master.jsonl"))
		fmt.Println()

		sessionIndexes, err := existingSessionIndexes(rc.RunDir)
		if err != nil {
			return err
		}
		for _, num := range sessionIndexes {
			fmt.Printf("### %s\n", SessionName(num))
			printJournalExcerpt(JournalPath(rc.RunDir, SessionName(num)))
			fmt.Println()
		}
		return nil
	}

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

func printJournalExcerpt(path string) {
	entries, err := goalx.LoadJournal(path)
	if err != nil || len(entries) == 0 {
		fmt.Println("(no journal output)")
		return
	}
	start := 0
	if len(entries) > 5 {
		start = len(entries) - 5
	}
	for _, entry := range entries[start:] {
		desc := strings.TrimSpace(entry.Desc)
		if desc == "" {
			desc = "(no description)"
		}
		if entry.Status != "" {
			fmt.Printf("[%d] %s: %s\n", entry.Round, entry.Status, desc)
			continue
		}
		fmt.Printf("[%d] %s\n", entry.Round, desc)
	}
}
