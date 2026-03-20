package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	ar "github.com/vonbai/autoresearch"
)

// Status shows the current progress for each session in a run.
func Status(projectRoot string, args []string) error {
	runName, sessionFilter, err := parseStatusArgs(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tLAST_ROUND\tSTATUS\tSUMMARY")

	// Session journals
	sessions := ar.ExpandSessions(rc.Config)
	for i := range sessions {
		sName := SessionName(i + 1)
		if sessionFilter != "" && sName != sessionFilter {
			continue
		}
		jPath := JournalPath(rc.RunDir, sName)
		entries, _ := ar.LoadJournal(jPath)

		lastRound := "-"
		status := "pending"
		if len(entries) > 0 {
			last := entries[len(entries)-1]
			if last.Round > 0 {
				lastRound = fmt.Sprintf("%d", last.Round)
			}
			if last.Status != "" {
				status = last.Status
			}
		}

		summary := ar.Summary(entries)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sName, lastRound, status, summary)
	}

	// Master journal
	masterPath := filepath.Join(rc.RunDir, "master.jsonl")
	masterEntries, _ := ar.LoadJournal(masterPath)
	masterSummary := ar.Summary(masterEntries)
	fmt.Fprintf(w, "master\t-\t-\t%s\n", masterSummary)

	return w.Flush()
}
