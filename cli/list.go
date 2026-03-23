package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
)

// List scans all runs for the current project and prints a table.
func List(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx list") {
		return nil
	}
	states, err := listDerivedRunStates(projectRoot)
	if err != nil {
		return err
	}
	if len(states) == 0 {
		fmt.Println("No runs found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tMODE\tSTATUS\tSELECTOR\tSESSIONS\tCREATED")

	for _, state := range states {
		sessions := 0
		if indexes, err := existingSessionIndexes(state.RunDir); err == nil {
			sessions = len(indexes)
		}

		info, _ := os.Stat(state.RunDir)
		created := ""
		if info != nil {
			created = info.ModTime().Format("2006-01-02 15:04")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", state.Name, state.Mode, state.Status, state.Selector, sessions, created)
	}
	return w.Flush()
}
