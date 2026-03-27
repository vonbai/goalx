package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	refreshDisplayFacts(rc)

	fmt.Printf("Run: %s\n", rc.Name)
	printStatusControlSummary(rc)
	printRunAdvisories(rc)

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SESSION\tLAST_ROUND\tSTATUS\tLEASE\tSUMMARY")
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
			identity, err := RequireSessionIdentity(rc.RunDir, sName)
			if err != nil {
				return fmt.Errorf("load %s identity: %w", sName, err)
			}
			sessionList = append(sessionList, SessionRuntimeState{
				Name:         sName,
				State:        "pending",
				Mode:         identity.Mode,
				WorktreePath: resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, sName, sessionState),
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
			if projected := sessionLifecycleStateFromJournalStatus(last.Status); projected != "" && (status == "pending" || status == "active") {
				status = projected
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
		inboxState := readControlInboxState(ControlInboxPath(rc.RunDir, sName), SessionCursorPath(rc.RunDir, sName))
		if inboxState.Unread > 0 {
			queueSummary := fmt.Sprintf("unread=%d cursor=%d/%d", inboxState.Unread, inboxState.LastSeenID, inboxState.LastID)
			if transport := loadTransportTargetFacts(rc.RunDir, sName); hasTransportFacts(transport) {
				queueSummary += formatTransportQueueFacts(transport)
			}
			if summary == "no entries" {
				summary = queueSummary
			} else {
				summary += " | " + queueSummary
			}
		}
		if sess.DirtyFiles > 0 {
			if summary == "no entries" {
				summary = fmt.Sprintf("dirty worktree (%d files)", sess.DirtyFiles)
			} else {
				summary += fmt.Sprintf(" | dirty=%d", sess.DirtyFiles)
			}
		}
		if launch := sessionLaunchFacts(rc.RunDir, sName); launch != "" {
			if summary == "no entries" {
				summary = launch
			} else {
				summary += " | " + launch
			}
		}
		if capability := targetProviderCapabilitySummary(rc.RunDir, sName, rc.Config.Master.Engine); capability != "" {
			if summary == "no entries" {
				summary = capability
			} else {
				summary += " | " + capability
			}
		}
		if transport := transportTargetFactsSummary(rc.RunDir, sName); transport != "" {
			if summary == "no entries" {
				summary = transport
			} else {
				summary += " | " + transport
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", sName, lastRound, status, actorLeaseSummary(rc.RunDir, sName, "-"), summary)
	}

	// Master journal
	masterPath := filepath.Join(rc.RunDir, "master.jsonl")
	masterEntries, _ := goalx.LoadJournal(masterPath)
	masterSummary := goalx.Summary(masterEntries)
	if transport := transportTargetFactsSummary(rc.RunDir, "master"); transport != "" {
		if masterSummary == "no entries" {
			masterSummary = transport
		} else {
			masterSummary += " | " + transport
		}
	}
	if capability := targetProviderCapabilitySummary(rc.RunDir, "master", rc.Config.Master.Engine); capability != "" {
		if masterSummary == "no entries" {
			masterSummary = capability
		} else {
			masterSummary += " | " + capability
		}
	}
	fmt.Fprintf(w, "master\t-\t-\t%s\t%s\n", actorLeaseSummary(rc.RunDir, "master", "missing"), masterSummary)

	return w.Flush()
}

func printStatusControlSummary(rc *RunContext) {
	if rc == nil {
		return
	}
	unread := unreadControlInboxCount(MasterInboxPath(rc.RunDir), MasterCursorPath(rc.RunDir))
	masterLease := controlLeaseSummary(rc.RunDir, "master")
	sidecarLease := controlLeaseSummary(rc.RunDir, "sidecar")
	remindersDue, deliveriesFailed := controlQueueSummary(rc.RunDir)
	if activity, err := LoadActivitySnapshot(ActivityPath(rc.RunDir)); err == nil && activity != nil {
		if masterLease == "missing" {
			if actor, ok := activity.Actors["master"]; ok && actor.Lease != "" {
				masterLease = actor.Lease
			}
		}
		if sidecarLease == "missing" {
			if actor, ok := activity.Actors["sidecar"]; ok && actor.Lease != "" {
				sidecarLease = actor.Lease
			}
		}
	}
	runID := "-"
	epoch := "-"
	charter := "missing"
	runStatus := "unknown"
	if derived, err := loadDerivedRunState(rc.ProjectRoot, rc.RunDir); err == nil && derived != nil {
		if derived.Status != "" {
			runStatus = derived.Status
		}
		if derived.RunID != "" {
			runID = derived.RunID
		}
		if derived.Epoch > 0 {
			epoch = fmt.Sprintf("%d", derived.Epoch)
		}
		if derived.Charter != "" {
			charter = derived.Charter
		}
	}
	fmt.Printf("Control: run_id=%s epoch=%s charter=%s run_status=%s unread_inbox=%d master_lease=%s sidecar_lease=%s reminders_due=%d deliveries_failed=%d\n", runID, epoch, charter, runStatus, unread, masterLease, sidecarLease, remindersDue, deliveriesFailed)
	if activity, err := LoadActivitySnapshot(ActivityPath(rc.RunDir)); err == nil && activity != nil {
		if coverage := formatCoverageSummary(activity.Coverage); coverage != "" {
			fmt.Printf("Coverage: %s\n", coverage)
		}
	}
	fmt.Println()
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
			if item.Suppressed || item.ResolvedAt != "" {
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

func sessionLaunchFacts(runDir, sessionName string) string {
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, sessionName))
	if err != nil || identity == nil {
		return ""
	}

	parts := make([]string, 0, 4)
	if identity.Mode != "" {
		parts = append(parts, "mode="+identity.Mode)
	}
	if identity.Engine != "" || identity.Model != "" {
		engineModel := strings.Trim(identity.Engine+"/"+identity.Model, "/")
		if engineModel != "" {
			parts = append(parts, "engine="+engineModel)
		}
	}
	switch {
	case identity.RequestedEffort != "" && identity.EffectiveEffort != "":
		parts = append(parts, fmt.Sprintf("effort=%s/%s", identity.RequestedEffort, identity.EffectiveEffort))
	case identity.RequestedEffort != "":
		parts = append(parts, "effort="+string(identity.RequestedEffort))
	case identity.EffectiveEffort != "":
		parts = append(parts, "effort="+identity.EffectiveEffort)
	}
	switch {
	case identity.RouteRole != "" && identity.RouteProfile != "":
		parts = append(parts, "route="+identity.RouteRole+"/"+identity.RouteProfile)
	case identity.RouteRole != "":
		parts = append(parts, "route="+identity.RouteRole)
	case identity.RouteProfile != "":
		parts = append(parts, "route="+identity.RouteProfile)
	}
	return strings.Join(parts, " ")
}

func targetProviderCapabilitySummary(runDir, target, masterEngine string) string {
	if strings.TrimSpace(target) == "master" {
		return providerCapabilityDescriptor(masterEngine).summary()
	}
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, target))
	if err != nil || identity == nil {
		return ""
	}
	return providerCapabilityDescriptor(identity.Engine).summary()
}

func transportTargetFactsSummary(runDir, target string) string {
	facts := loadTransportTargetFacts(runDir, target)
	parts := make([]string, 0, 5)
	if facts.TransportState != "" {
		parts = append(parts, "transport="+facts.TransportState)
	}
	if facts.InputContainsWake {
		parts = append(parts, "input_wake=true")
	}
	if facts.QueuedMessageVisible {
		parts = append(parts, "queued=true")
	}
	if facts.ProviderDialogVisible {
		parts = append(parts, "dialog="+blankAsUnknown(facts.ProviderDialogKind))
		if facts.ProviderDialogHint != "" {
			parts = append(parts, "dialog_hint="+fmt.Sprintf("%q", facts.ProviderDialogHint))
		}
	}
	return strings.Join(parts, " ")
}

func formatTransportQueueFacts(facts TransportTargetFacts) string {
	parts := make([]string, 0, 5)
	if facts.LastSubmitAttemptAt != "" {
		parts = append(parts, " submit_at="+facts.LastSubmitAttemptAt)
	}
	if facts.TransportState != "" {
		parts = append(parts, " transport="+facts.TransportState)
	}
	if facts.LastTransportAcceptAt != "" {
		parts = append(parts, " accepted_at="+facts.LastTransportAcceptAt)
	}
	if facts.ProviderDialogVisible {
		parts = append(parts, " dialog="+blankAsUnknown(facts.ProviderDialogKind))
		if facts.ProviderDialogHint != "" {
			parts = append(parts, " dialog_hint="+fmt.Sprintf("%q", facts.ProviderDialogHint))
		}
	}
	return strings.Join(parts, "")
}

func hasTransportFacts(facts TransportTargetFacts) bool {
	return facts.Target != "" ||
		facts.Window != "" ||
		facts.PaneID != "" ||
		facts.Engine != "" ||
		facts.PromptVisible ||
		facts.WorkingVisible ||
		facts.QueuedMessageVisible ||
		facts.InputContainsWake ||
		facts.TransportState != "" ||
		facts.LastSampleAt != "" ||
		facts.LastOutputAt != "" ||
		facts.LastSubmitAttemptAt != "" ||
		facts.LastSubmitMode != "" ||
		facts.LastTransportAcceptAt != "" ||
		facts.LastTransportError != "" ||
		facts.ProviderDialogVisible ||
		facts.ProviderDialogKind != "" ||
		facts.ProviderDialogHint != ""
}

func actorLeaseSummary(runDir, holder, missing string) string {
	lease, err := LoadControlLease(ControlLeasePath(runDir, holder))
	if err != nil || lease == nil || lease.ExpiresAt == "" {
		return missing
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

func controlLeaseSummary(runDir, holder string) string {
	return actorLeaseSummary(runDir, holder, "missing")
}
