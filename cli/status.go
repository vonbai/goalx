package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

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

	fmt.Printf("Run: %s\n", rc.Name)
	printStatusControlSummary(rc)

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

func printStatusControlSummary(rc *RunContext) {
	if rc == nil {
		return
	}
	masterCursor, _ := LoadMasterCursorState(MasterCursorPath(rc.RunDir))
	unread := unreadMasterInboxCount(rc.RunDir, masterCursor)
	masterLease := controlLeaseSummary(rc.RunDir, "master")
	sidecarLease := controlLeaseSummary(rc.RunDir, "sidecar")
	runStatus := "unknown"
	if derived, err := loadDerivedRunState(rc.ProjectRoot, rc.RunDir); err == nil && derived != nil && derived.Status != "" {
		runStatus = derived.Status
	}
	remindersDue, deliveriesFailed := controlQueueSummary(rc.RunDir)
	fmt.Printf("Control: run_status=%s unread_inbox=%d master_lease=%s sidecar_lease=%s reminders_due=%d deliveries_failed=%d\n", runStatus, unread, masterLease, sidecarLease, remindersDue, deliveriesFailed)
	fmt.Println()
}

func unreadMasterInboxCount(runDir string, state *MasterCursorState) int {
	f, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		return 0
	}
	lastID := int64(0)
	for _, line := range splitNonEmptyLines(string(f)) {
		var msg MasterInboxMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID > lastID {
			lastID = msg.ID
		}
	}
	if state == nil || lastID <= state.LastSeenID {
		return 0
	}
	return int(lastID - state.LastSeenID)
}

func splitNonEmptyLines(s string) []string {
	lines := make([]string, 0)
	start := 0
	for i := 0; i <= len(s); i++ {
		if i < len(s) && s[i] != '\n' {
			continue
		}
		line := s[start:i]
		start = i + 1
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func controlQueueSummary(runDir string) (int, int) {
	remindersDue := 0
	deliveriesFailed := 0
	now := time.Now().UTC()

	if reminders, err := LoadControlReminders(ControlRemindersPath(runDir)); err == nil && reminders != nil {
		for _, item := range reminders.Items {
			if item.Suppressed || item.AckedAt != "" {
				continue
			}
			if item.CooldownUntil != "" {
				if cooldownUntil, err := time.Parse(time.RFC3339, item.CooldownUntil); err == nil && cooldownUntil.After(now) {
					continue
				}
			}
			remindersDue++
		}
	}
	if deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir)); err == nil && deliveries != nil {
		for _, item := range deliveries.Items {
			if item.Status == "failed" {
				deliveriesFailed++
			}
		}
	}
	return remindersDue, deliveriesFailed
}

func controlLeaseSummary(runDir, holder string) string {
	lease, err := LoadControlLease(ControlLeasePath(runDir, holder))
	if err != nil || lease == nil || lease.ExpiresAt == "" {
		return "missing"
	}
	expiresAt, err := time.Parse(time.RFC3339, lease.ExpiresAt)
	if err != nil {
		return "invalid"
	}
	if expiresAt.After(time.Now().UTC()) {
		return "healthy"
	}
	return "expired"
}
