package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	goalx "github.com/vonbai/goalx"
)

// Status shows the current progress for each session in a run.
func Status(projectRoot string, args []string) error {
	if len(args) == 1 && isHelpToken(args[0]) {
		fmt.Println("usage: goalx status [NAME] [session-N]")
		return nil
	}
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
	coord, _ := LoadCoordinationState(CoordinationPath(rc.RunDir))
	sessionState, _ := EnsureSessionsRuntimeState(rc.RunDir)

	sessionList := sortedSessionStates(sessionState)
	if len(sessionList) == 0 {
		indexes, err := existingSessionIndexes(rc.RunDir)
		if err != nil {
			return err
		}
		for _, num := range indexes {
			sName := SessionName(num)
			sessionList = append(sessionList, SessionRuntimeState{
				Name:         sName,
				State:        "pending",
				Mode:         string(goalx.EffectiveSessionConfig(rc.Config, num-1).Mode),
				WorktreePath: WorktreePath(rc.RunDir, rc.Config.Name, num),
			})
		}
	}
	for _, sess := range sessionList {
		sName := sess.Name
		if sessionFilter != "" && sName != sessionFilter {
			continue
		}
		jPath := JournalPath(rc.RunDir, sName)
		entries, _ := goalx.LoadJournal(jPath)

		lastRound := "-"
		status := sess.State
		if status == "" {
			status = "pending"
		}
		if len(entries) > 0 {
			last := entries[len(entries)-1]
			if last.Round > 0 {
				lastRound = fmt.Sprintf("%d", last.Round)
			}
			if status == "pending" && last.Status != "" {
				status = last.Status
			}
		}

		summary := goalx.Summary(entries)
		if sess.LastRound > 0 {
			lastRound = fmt.Sprintf("%d", sess.LastRound)
		}
		if coord != nil {
			if sess, ok := coord.Sessions[sName]; ok {
				if sess.LastRound > 0 {
					lastRound = fmt.Sprintf("%d", sess.LastRound)
				}
				if status == "pending" && sess.State != "" {
					status = sess.State
				}
				switch sess.State {
				case "parked":
					if sess.Scope != "" {
						summary = "parked: " + sess.Scope
					} else {
						summary = "parked"
					}
				case "blocked":
					if sess.BlockedBy != "" {
						summary = "blocked: " + sess.BlockedBy
					}
				case "active":
					if summary == "no entries" && sess.Scope != "" {
						summary = "active: " + sess.Scope
					}
				}
			}
		}
		guidancePending := sess.GuidancePending
		if !guidancePending {
			if guidanceState, err := LoadSessionGuidanceState(SessionGuidanceStatePath(rc.RunDir, sName)); err == nil && guidanceState != nil {
				guidancePending = guidanceState.Pending
			}
		}
		if guidancePending {
			if status == "idle" || status == "pending" {
				status = "guidance-pending"
			}
			if summary == "no entries" {
				summary = "guidance pending"
			} else if summary != "guidance pending" {
				summary += " | guidance pending"
			}
		}
		if sess.DirtyFiles > 0 {
			if summary == "no entries" {
				summary = fmt.Sprintf("dirty worktree (%d files)", sess.DirtyFiles)
			} else {
				summary += fmt.Sprintf(" | dirty=%d", sess.DirtyFiles)
			}
		}
		if sess.LastTestSummary != "" {
			summary += " | " + sess.LastTestSummary
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", sName, lastRound, status, summary)
	}

	// Master journal
	masterPath := filepath.Join(rc.RunDir, "master.jsonl")
	masterEntries, _ := goalx.LoadJournal(masterPath)
	masterSummary := goalx.Summary(masterEntries)
	fmt.Fprintf(w, "master\t-\t-\t%s\n", masterSummary)

	return w.Flush()
}
