package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"

	goalx "github.com/vonbai/goalx"
)

// Review shows a comparative summary of all sessions in a run.
func Review(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx review [--run NAME]") {
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
		return fmt.Errorf("usage: goalx review [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	manifest, err := EnsureRunArtifacts(rc.RunDir, rc.Config)
	if err != nil {
		return err
	}

	fmt.Printf("=== Review: %s (%s) ===\n", rc.Name, rc.Config.Mode)
	fmt.Printf("Objective: %s\n\n", rc.Config.Objective)

	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load session runtime state: %w", err)
	}
	for _, num := range sessionIndexes {
		sName := SessionName(num)
		workdir := sessionWorkdir(rc.RunDir, rc.Config.Name, sName, sessionState)

		fmt.Printf("--- %s ---\n", sName)

		// Git log summary
		out, err := exec.Command("git", "-C", workdir, "log", "--oneline", "-5").Output()
		if err == nil && len(out) > 0 {
			fmt.Printf("Recent commits:\n%s\n", string(out))
		}

		// Journal summary
		jPath := JournalPath(rc.RunDir, sName)
		entries, _ := goalx.LoadJournal(jPath)
		fmt.Printf("Journal: %s\n", goalx.Summary(entries))

		// Mode-specific output
		identity, err := RequireSessionIdentity(rc.RunDir, sName)
		if err != nil {
			return fmt.Errorf("load %s identity: %w", sName, err)
		}
		if goalx.Mode(identity.Mode) == goalx.ModeResearch {
			reportPath := ""
			if artifact := FindSessionArtifact(manifest, sName, "report"); artifact != nil {
				reportPath = artifact.Path
			}
			if reportPath == "" {
				reportPath = findSessionReport(workdir, identity.Target.Files)
			}
			if reportPath != "" {
				printFirstLines(reportPath, 20)
			}
		} else {
			out, err := exec.Command("git", "-C", workdir, "diff", "--stat", "HEAD~5").Output()
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
