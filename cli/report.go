package cli

import (
	"fmt"
	"path/filepath"

	goalx "github.com/vonbai/goalx"
)

// Report prints a formatted report of run progress from journals.
func Report(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx report [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	fmt.Printf("=== Report: %s ===\n", rc.Name)
	fmt.Printf("Mode: %s\n", rc.Config.Mode)
	fmt.Printf("Objective: %s\n\n", rc.Config.Objective)

	// Per-session progress
	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	for _, num := range sessionIndexes {
		sName := SessionName(num)
		jPath := JournalPath(rc.RunDir, sName)
		entries, _ := goalx.LoadJournal(jPath)

		fmt.Printf("--- %s ---\n", sName)
		if len(entries) == 0 {
			fmt.Println("  No journal entries.")
		} else {
			for _, e := range entries {
				if e.Round > 0 {
					fmt.Printf("  Round %d: %s [%s]\n", e.Round, e.Desc, e.Status)
				} else if e.Desc != "" {
					fmt.Printf("  %s\n", e.Desc)
				}
			}
		}
		fmt.Println()
	}

	// Master summary
	masterPath := filepath.Join(rc.RunDir, "master.jsonl")
	masterEntries, _ := goalx.LoadJournal(masterPath)
	fmt.Println("--- master ---")
	if len(masterEntries) == 0 {
		fmt.Println("  No master entries.")
	} else {
		for _, e := range masterEntries {
			if e.Action != "" {
				fmt.Printf("  [%s] %s: %s\n", e.Action, e.Session, e.Finding)
			}
		}
	}

	return nil
}
