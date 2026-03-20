package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	ar "github.com/vonbai/autoresearch"
)

// Review shows a comparative summary of all sessions in a run.
func Review(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx review [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	fmt.Printf("=== Review: %s (%s) ===\n", rc.Name, rc.Config.Mode)
	fmt.Printf("Objective: %s\n\n", rc.Config.Objective)

	sessions := ar.ExpandSessions(rc.Config)
	for i := range sessions {
		num := i + 1
		sName := SessionName(num)
		wtPath := WorktreePath(rc.RunDir, rc.Config.Name, num)

		fmt.Printf("--- %s ---\n", sName)

		// Git log summary
		out, err := exec.Command("git", "-C", wtPath, "log", "--oneline", "-5").Output()
		if err == nil && len(out) > 0 {
			fmt.Printf("Recent commits:\n%s\n", string(out))
		}

		// Journal summary
		jPath := JournalPath(rc.RunDir, sName)
		entries, _ := ar.LoadJournal(jPath)
		fmt.Printf("Journal: %s\n", ar.Summary(entries))

		// Mode-specific output
		if rc.Config.Mode == ar.ModeResearch {
			reportPath := filepath.Join(wtPath, "report.md")
			printFirstLines(reportPath, 20)
		} else {
			out, err := exec.Command("git", "-C", wtPath, "diff", "--stat", "HEAD~5").Output()
			if err == nil && len(out) > 0 {
				fmt.Printf("Diff stat:\n%s", string(out))
			}
		}
		fmt.Println()
	}
	return nil
}

func printFirstLines(path string, n int) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < n && scanner.Scan(); i++ {
		fmt.Println(scanner.Text())
	}
}
