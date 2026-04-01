package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	goalx "github.com/vonbai/goalx"
)

const runtimeHostUsage = "usage: goalx runtime-host --run RUN [--interval SECONDS]"

var errRuntimeHostStale = errors.New("runtime host run is stale")
var errRuntimeHostCompleted = errors.New("runtime host run completed")

func RuntimeHostCommand(projectRoot string, args []string) error {
	runName, interval, err := parseRuntimeHostArgs(args)
	if err != nil {
		return err
	}
	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	meta, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective)
	if err != nil {
		return err
	}
	if interval <= 0 {
		checkSec, _ := normalizeRuntimeHostInterval(rc.Config.Master.CheckInterval)
		interval = time.Duration(checkSec) * time.Second
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runRuntimeHostLoop(ctx, rc.ProjectRoot, rc.Name, rc.RunDir, meta.RunID, meta.Epoch, interval)
}

func parseRuntimeHostArgs(args []string) (string, time.Duration, error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", 0, err
	}
	var interval time.Duration
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--help", "-h":
			return "", 0, fmt.Errorf(runtimeHostUsage)
		case "--interval":
			if i+1 >= len(rest) {
				return "", 0, fmt.Errorf("missing value for --interval")
			}
			seconds, err := strconv.Atoi(rest[i+1])
			if err != nil || seconds <= 0 {
				return "", 0, fmt.Errorf("invalid --interval %q", rest[i+1])
			}
			interval = time.Duration(seconds) * time.Second
			i++
		default:
			return "", 0, fmt.Errorf(runtimeHostUsage)
		}
	}
	if runName == "" {
		return "", 0, fmt.Errorf(runtimeHostUsage)
	}
	return runName, interval, nil
}

func runRuntimeHostLoop(ctx context.Context, projectRoot, runName, runDir, runID string, epoch int, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	shouldExpire := true
	exitReason := "completed"
	var watcher *TmuxControlWatcher
	defer func() {
		if shouldExpire {
			_ = ExpireControlLease(runDir, "runtime-host")
		}
	}()
	appendAuditLog(runDir, "runtime-host started pid=%d runID=%s epoch=%d", os.Getpid(), runID, epoch)
	defer func() {
		appendAuditLog(runDir, "runtime-host exiting reason=%s", exitReason)
	}()
	defer func() {
		if watcher != nil {
			_ = watcher.Close()
		}
	}()
	reportError := func(err error) error {
		appendAuditLog(runDir, "runtime-host error: %v", err)
		exitReason = err.Error()
		return err
	}
	watcher = ensureRuntimeHostTransportWatcher(projectRoot, runName, runDir, watcher)
	if err := runRuntimeHostTickWithWatcher(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid(), watcher); err != nil {
		if errors.Is(err, errRuntimeHostStale) {
			shouldExpire = false
			exitReason = errRuntimeHostStale.Error()
			return nil
		}
		if errors.Is(err, errRuntimeHostCompleted) {
			exitReason = errRuntimeHostCompleted.Error()
			return nil
		}
		return reportError(err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			exitReason = ctx.Err().Error()
			return nil
		case <-ticker.C:
			watcher = ensureRuntimeHostTransportWatcher(projectRoot, runName, runDir, watcher)
			if err := runRuntimeHostTickWithWatcher(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid(), watcher); err != nil {
				if errors.Is(err, errRuntimeHostStale) {
					shouldExpire = false
					exitReason = errRuntimeHostStale.Error()
					return nil
				}
				if errors.Is(err, errRuntimeHostCompleted) {
					exitReason = errRuntimeHostCompleted.Error()
					return nil
				}
				return reportError(err)
			}
		}
	}
}

func runRuntimeHostTick(projectRoot, runName, runDir, runID string, epoch int, interval time.Duration, pid int) error {
	return runRuntimeHostTickWithWatcher(projectRoot, runName, runDir, runID, epoch, interval, pid, nil)
}

func runRuntimeHostTickWithWatcher(projectRoot, runName, runDir, runID string, epoch int, interval time.Duration, pid int, watcher *TmuxControlWatcher) error {
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return errRuntimeHostStale
		}
		return err
	}
	if meta == nil || meta.RunID != runID || meta.Epoch != epoch {
		return errRuntimeHostStale
	}
	if _, err := os.Stat(RunSpecPath(runDir)); err != nil {
		if os.IsNotExist(err) {
			return errRuntimeHostStale
		}
		return err
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return err
	}
	tmuxSession := resolveRunTmuxSession(projectRoot, runDir, runName)
	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return err
	}
	ttl := interval * 2
	if ttl < time.Second {
		ttl = time.Second
	}
	if err := RenewControlLease(runDir, "runtime-host", runID, epoch, ttl, "process", pid); err != nil {
		return err
	}
	if err := ApplyPendingControlOps(runDir); err != nil {
		return err
	}
	_, changed, err := RefreshIdentityFence(runDir, meta)
	if err != nil {
		return err
	}
	if changed {
		if err := queueRefreshContextReminder(runDir, tmuxSession, cfg.Master.Engine); err != nil {
			return err
		}
	}
	if liveness, err := ScanLiveness(runDir); err == nil {
		if err := SaveLivenessState(runDir, liveness); err != nil {
			return err
		}
	}
	if err := RefreshWorktreeSnapshot(runDir); err != nil {
		return err
	}
	presence, err := BuildTargetPresenceFacts(runDir, tmuxSession)
	if err != nil {
		return err
	}
	closeoutFacts, err := BuildRunCloseoutFacts(runDir)
	if err != nil {
		return err
	}
	switch closeoutFacts.MaintenanceAction(presence["master"]) {
	case RunCloseoutMaintenanceActionRecoverMaster:
		if !presence["master"].SessionExists {
			if err := relaunchMaster(projectRoot, runDir, tmuxSession, cfg); err != nil {
				return err
			}
		} else {
			if err := relaunchMissingMasterWindow(projectRoot, runDir, tmuxSession, cfg); err != nil {
				return err
			}
		}
		return nil
	case RunCloseoutMaintenanceActionFinalize:
		if err := finalizeCompletedRunFromRuntimeHost(projectRoot, runName, runDir, tmuxSession); err != nil {
			return err
		}
		return errRuntimeHostCompleted
	}
	presence, err = reconcileTargetPresenceRecovery(projectRoot, runDir, tmuxSession, cfg, runState, closeoutFacts.Complete, presence)
	if err != nil {
		return err
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	if err := runRuntimeHostMaintenanceCycle(projectRoot, runName, runDir, tmuxSession, cfg, interval, presence, watcher, controlState); err != nil {
		return err
	}
	return nil
}

func refreshActivityFacts(runDir, projectRoot, runName string) error {
	snapshot, err := BuildActivitySnapshot(projectRoot, runName, runDir)
	if err != nil {
		return err
	}
	return SaveActivitySnapshot(runDir, snapshot)
}

func refreshTransportFactsForRuntimeHost(runDir, tmuxSession, masterEngine string, watcher *TmuxControlWatcher, controlState *ControlRunState) error {
	if controlState == nil {
		return fmt.Errorf("control run state is nil")
	}
	var facts *TransportFacts
	var err error
	if watcher != nil && watcher.Alive() {
		if err := watcher.writeSnapshot(); err != nil {
			return err
		}
		facts, err = LoadTransportFacts(TransportFactsPath(runDir))
	} else {
		facts, err = BuildTransportFacts(runDir, tmuxSession, masterEngine)
		if err == nil {
			err = SaveTransportFacts(runDir, facts)
		}
	}
	if err != nil {
		return err
	}
	if err := SaveTransportFacts(runDir, facts); err != nil {
		return err
	}
	return reconcileProviderDialogAlerts(runDir, tmuxSession, masterEngine, controlState, facts)
}

func runRuntimeHostMaintenanceCycle(projectRoot, runName, runDir, tmuxSession string, cfg *goalx.Config, interval time.Duration, presence map[string]TargetPresenceFacts, watcher *TmuxControlWatcher, controlState *ControlRunState) error {
	nudgeDeliver := func(target, engine string) (TransportDeliveryOutcome, error) {
		return sendAgentNudgeDetailedInRunFunc(runDir, target, engine)
	}
	if err := refreshRuntimeHostTransportFacts(runDir, tmuxSession, cfg.Master.Engine, watcher, controlState, "pre-queue"); err != nil {
		return err
	}
	if err := processUrgentTransportTargets(runDir, runName, tmuxSession, cfg, presence); err != nil {
		return err
	}
	if err := queueUnreadSessionWakeReminders(runDir, tmuxSession, runName, interval); err != nil {
		return err
	}
	if err := queueMasterWakeReminder(runDir, tmuxSession, cfg.Master.Engine); err != nil {
		return err
	}
	if err := DeliverDueControlReminders(runDir, cfg.Master.Engine, interval, nudgeDeliver); err != nil {
		return err
	}
	if err := refreshRuntimeHostTransportFacts(runDir, tmuxSession, cfg.Master.Engine, watcher, controlState, "snapshot"); err != nil {
		return err
	}
	if err := refreshActivityFacts(runDir, projectRoot, runName); err != nil {
		return err
	}
	if err := RefreshEvolveFacts(runDir); err != nil {
		return err
	}
	if _, err := processMasterAlerts(runDir, tmuxSession, cfg.Master.Engine, presence); err != nil {
		return err
	}
	if _, err := processTargetAttentionAlerts(runDir, tmuxSession, cfg.Master.Engine, presence); err != nil {
		return err
	}
	if err := RefreshRunMemorySeeds(runDir); err != nil {
		return err
	}
	if err := AppendExtractedMemoryProposals(runDir, time.Now().UTC()); err != nil {
		return err
	}
	if err := PromoteMemoryProposals(); err != nil {
		return err
	}
	successContextChanged, err := RefreshRunGuidance(projectRoot, runName, runDir)
	if err != nil {
		appendAuditLog(runDir, "guidance refresh warning: %v", err)
	} else if successContextChanged {
		if err := queueRefreshContextReminder(runDir, tmuxSession, cfg.Master.Engine); err != nil {
			return err
		}
	}
	return nil
}

func refreshRuntimeHostTransportFacts(runDir, tmuxSession, masterEngine string, watcher *TmuxControlWatcher, controlState *ControlRunState, warningPhase string) error {
	if watcher != nil && watcher.Alive() {
		if err := refreshTransportFactsForRuntimeHost(runDir, tmuxSession, masterEngine, watcher, controlState); err != nil {
			appendAuditLog(runDir, "transport watcher %s warning: %v", warningPhase, err)
		}
		return nil
	}
	return refreshTransportFactsForRuntimeHost(runDir, tmuxSession, masterEngine, nil, controlState)
}

func reconcileProviderDialogAlerts(runDir, tmuxSession, masterEngine string, controlState *ControlRunState, facts *TransportFacts) error {
	if controlState == nil {
		return fmt.Errorf("control run state is nil")
	}
	current := map[string]string{}
	if facts != nil {
		for target, targetFacts := range facts.Targets {
			if !targetFacts.ProviderDialogVisible {
				continue
			}
			current[target] = providerDialogAlertFingerprint(targetFacts)
			if controlState.ProviderDialogAlerts[target] == current[target] {
				continue
			}
			body := fmt.Sprintf("Provider dialog visible in unattended GoalX run; target=%s engine=%s kind=%s hint=%s", blankAsUnknown(target), blankAsUnknown(targetFacts.Engine), blankAsUnknown(targetFacts.ProviderDialogKind), blankAsUnknown(targetFacts.ProviderDialogHint))
			if _, err := appendControlInboxMessage(runDir, "master", "provider-dialog-visible", "goalx runtime-host", body, true); err != nil {
				return err
			}
			dedupeKey := fmt.Sprintf("provider-dialog:%s:%s", target, current[target])
			if _, err := deliverControlNudge(runDir, dedupeKey, dedupeKey, tmuxSession+":master", masterEngine, false, func(target, engine string) (TransportDeliveryOutcome, error) {
				return sendAgentNudgeDetailedInRunFunc(runDir, target, engine)
			}); err != nil {
				return err
			}
		}
	}
	if mapsEqual(controlState.ProviderDialogAlerts, current) {
		return nil
	}
	return submitAndApplyControlOp(runDir, controlOpRunStateProviderDialogAlerts, controlRunStateProviderDialogAlertsBody{
		Alerts: current,
	})
}

func providerDialogAlertFingerprint(facts TransportTargetFacts) string {
	return strings.Join([]string{
		strings.TrimSpace(facts.Engine),
		strings.TrimSpace(facts.ProviderDialogKind),
		strings.TrimSpace(facts.ProviderDialogHint),
	}, "|")
}

func mapsEqual(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func ensureRuntimeHostTransportWatcher(projectRoot, runName, runDir string, watcher *TmuxControlWatcher) *TmuxControlWatcher {
	if watcher != nil && watcher.Alive() {
		return watcher
	}
	tmuxSession := resolveRunTmuxSession(projectRoot, runDir, runName)
	if !SessionExistsInRun(runDir, tmuxSession) {
		return nil
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		appendAuditLog(runDir, "transport watcher start warning: %v", err)
		return nil
	}
	next, err := startTmuxControlWatcher(runDir, tmuxSession, cfg.Master.Engine)
	if err != nil {
		appendAuditLog(runDir, "transport watcher start warning: %v", err)
		return nil
	}
	return next
}

func queueRefreshContextReminder(runDir, tmuxSession, engine string) error {
	if !SessionExistsInRun(runDir, tmuxSession) {
		return nil
	}
	_, err := QueueControlReminderWithEngine(runDir, "refresh-context", "identity-fence-changed", tmuxSession+":master", engine)
	return err
}

func queueUnreadSessionWakeReminders(runDir, tmuxSession, runName string, interval time.Duration) error {
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	transportFacts, _ := LoadTransportFacts(TransportFactsPath(runDir))
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		inboxState := readControlInboxState(ControlInboxPath(runDir, sessionName), SessionCursorPath(runDir, sessionName))
		dedupeKey := "session-wake:" + sessionName
		if inboxState.Unread == 0 {
			if err := SuppressControlReminder(runDir, dedupeKey); err != nil {
				return err
			}
			continue
		}
		transport := latestSessionTransportFacts(transportFacts, sessionName)
		if transportAcceptedRecently(transport, interval, now) {
			continue
		}
		if !transportNeedsRepair(transport) {
			if delivery, ok := latestSessionInboxDelivery(runDir, sessionName); ok && deliveryAcceptedWithin(delivery, interval, now) {
				continue
			}
		}
		windowName, err := resolveWindowName(runName, sessionName)
		if err != nil {
			return err
		}
		identity, err := RequireSessionIdentity(runDir, sessionName)
		if err != nil {
			return err
		}
		if err := queueSessionWakeReminder(runDir, tmuxSession, sessionName, windowName, identity.Engine); err != nil {
			return err
		}
	}
	return nil
}

func processTargetAttentionAlerts(runDir, tmuxSession, masterEngine string, presence map[string]TargetPresenceFacts) (bool, error) {
	attention, err := loadTargetAttentionFacts(runDir)
	if err != nil {
		return false, err
	}
	if len(attention) == 0 {
		return false, nil
	}
	masterPresence := presence["master"]
	masterAvailable := !targetPresenceMissing(masterPresence) && targetPresenceAvailableForTransport(masterPresence)
	alerted := false
	for target, facts := range attention {
		if target == "master" {
			continue
		}
		if err := recordTargetAttentionObservation(runDir, facts); err != nil {
			return false, err
		}
		if !targetAttentionEscalates(facts) || !masterAvailable {
			continue
		}
		recovery := loadTransportRecoveryTarget(runDir, target)
		state := strings.TrimSpace(facts.AttentionState)
		if recovery.CurrentAttentionState == state && recovery.CurrentAttentionLastAlertAt != "" {
			continue
		}
		reason := formatBlockedTargetReason(facts)
		appendAuditLog(runDir, "target_attention_alert target=%s state=%s reason=%s", target, state, reason)
		body := fmt.Sprintf("Target attention needed in active GoalX run; target=%s state=%s %s", target, state, reason)
		if _, err := appendControlInboxMessage(runDir, "master", "target-attention", "goalx runtime-host", body, true); err != nil {
			return false, err
		}
		if err := recordTargetAttentionAlert(runDir, target, state, reason); err != nil {
			return false, err
		}
		dedupeKey := fmt.Sprintf("master-attention:%s:%s", target, state)
		if _, err := deliverControlNudge(runDir, dedupeKey, dedupeKey, tmuxSession+":master", masterEngine, false, func(target, engine string) (TransportDeliveryOutcome, error) {
			return sendAgentNudgeDetailedInRunFunc(runDir, target, engine)
		}); err != nil {
			return false, err
		}
		alerted = true
	}
	return alerted, nil
}

func formatBlockedTargetReason(facts TargetAttentionFacts) string {
	parts := []string{}
	if facts.Unread > 0 {
		parts = append(parts, fmt.Sprintf("unread=%d", facts.Unread))
	}
	if facts.CursorLag > 0 {
		parts = append(parts, fmt.Sprintf("cursor_lag=%d", facts.CursorLag))
	}
	if facts.JournalStaleMinutes > 0 {
		parts = append(parts, fmt.Sprintf("journal_stale=%dm", facts.JournalStaleMinutes))
	}
	if facts.OutputStaleMinutes > 0 {
		parts = append(parts, fmt.Sprintf("output_stale=%dm", facts.OutputStaleMinutes))
	}
	if facts.WorktreeStaleMinutes > 0 {
		parts = append(parts, fmt.Sprintf("worktree_stale=%dm", facts.WorktreeStaleMinutes))
	}
	if strings.TrimSpace(facts.TransportState) != "" {
		parts = append(parts, "transport="+facts.TransportState)
	}
	if strings.TrimSpace(facts.RuntimeState) != "" {
		parts = append(parts, "runtime="+facts.RuntimeState)
	}
	return strings.Join(parts, " ")
}

type masterAlert struct {
	Key         string
	Fingerprint string
	Body        string
}

func processMasterAlerts(runDir, tmuxSession, masterEngine string, presence map[string]TargetPresenceFacts) (bool, error) {
	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		return false, err
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return false, err
	}
	alerts, current, err := buildMasterAlerts(runDir, activity)
	if err != nil {
		return false, err
	}
	if len(current) == 0 {
		if len(controlState.MasterAlerts) == 0 {
			return false, nil
		}
		if err := submitAndApplyControlOp(runDir, controlOpRunStateMasterAlerts, controlRunStateMasterAlertsBody{}); err != nil {
			return false, err
		}
		return false, nil
	}
	masterPresence := presence["master"]
	masterAvailable := !targetPresenceMissing(masterPresence) && targetPresenceAvailableForTransport(masterPresence)
	if !masterAvailable {
		return false, nil
	}
	alerted := false
	for _, alert := range alerts {
		if controlState.MasterAlerts[alert.Key] == alert.Fingerprint {
			continue
		}
		appendAuditLog(runDir, "master_alert key=%s fingerprint=%s", alert.Key, alert.Fingerprint)
		if _, err := appendControlInboxMessage(runDir, "master", "master-alert", "goalx runtime-host", alert.Body, true); err != nil {
			return false, err
		}
		dedupeKey := "master-alert:" + alert.Key + ":" + alert.Fingerprint
		if _, err := deliverControlNudge(runDir, dedupeKey, dedupeKey, tmuxSession+":master", masterEngine, false, func(target, engine string) (TransportDeliveryOutcome, error) {
			return sendAgentNudgeDetailedInRunFunc(runDir, target, engine)
		}); err != nil {
			return false, err
		}
		alerted = true
	}
	if mapsEqual(controlState.MasterAlerts, current) {
		return alerted, nil
	}
	if err := submitAndApplyControlOp(runDir, controlOpRunStateMasterAlerts, controlRunStateMasterAlertsBody{
		Alerts: current,
	}); err != nil {
		return false, err
	}
	return alerted, nil
}

func buildMasterAlerts(runDir string, activity *ActivitySnapshot) ([]masterAlert, map[string]string, error) {
	alerts := make([]masterAlert, 0, 4)
	current := map[string]string{}
	if activity != nil && activity.Lifecycle.RunActive && activity.Budget.MaxDurationSeconds > 0 && activity.Budget.Exhausted {
		summary := formatBudgetSummary(activity.Budget)
		key := "budget_exhausted"
		fingerprint := strings.Join([]string{
			"budget_exhausted",
			fmt.Sprintf("max_duration=%d", activity.Budget.MaxDurationSeconds),
			"started_at=" + strings.TrimSpace(activity.Budget.StartedAt),
			"deadline_at=" + strings.TrimSpace(activity.Budget.DeadlineAt),
		}, "|")
		body := "Budget exhausted in active GoalX run"
		if summary != "" {
			body += "; " + summary
		}
		alerts = append(alerts, masterAlert{
			Key:         key,
			Fingerprint: fingerprint,
			Body:        body,
		})
		current[key] = fingerprint
	}
	if activity == nil || !activity.Coverage.RequiredPresent {
		nonFrontierAlerts, nonFrontierCurrent, err := buildNonFrontierMasterAlerts(runDir)
		if err != nil {
			return nil, nil, err
		}
		alerts = append(alerts, nonFrontierAlerts...)
		for key, fingerprint := range nonFrontierCurrent {
			current[key] = fingerprint
		}
		if len(alerts) == 0 {
			return nil, nil, nil
		}
		sort.Slice(alerts, func(i, j int) bool {
			return alerts[i].Key < alerts[j].Key
		})
		return alerts, current, nil
	}
	coord, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		return nil, nil, err
	}
	coverage := activity.Coverage

	for _, id := range coverage.UnmappedRequiredIDs {
		key := "unmapped_required:" + id
		fingerprint := key
		alerts = append(alerts, masterAlert{
			Key:         key,
			Fingerprint: fingerprint,
			Body:        fmt.Sprintf("Required frontier gap in active GoalX run; required=%s fact=unmapped_required", id),
		})
		current[key] = fingerprint
	}
	for _, id := range coverage.MasterOrphanedRequiredIDs {
		required, ok := coordinationRequiredByID(coord, id)
		if !ok {
			continue
		}
		reusable := append([]string{}, coverage.IdleReusableSessions...)
		reusable = append(reusable, coverage.ParkedReusableSessions...)
		sort.Strings(reusable)
		fingerprint := strings.Join([]string{
			"master_orphaned",
			id,
			"owner=" + strings.TrimSpace(required.Owner),
			"execution_state=" + strings.TrimSpace(required.ExecutionState),
			"reusable_sessions=" + strings.Join(reusable, ","),
		}, "|")
		key := "master_orphaned:" + id
		body := fmt.Sprintf("Required frontier gap in active GoalX run; required=%s fact=master_orphaned owner=%s execution_state=%s", id, blankAsUnknown(required.Owner), blankAsUnknown(required.ExecutionState))
		if len(reusable) > 0 {
			body += " reusable_sessions=" + strings.Join(reusable, ",")
		}
		alerts = append(alerts, masterAlert{
			Key:         key,
			Fingerprint: fingerprint,
			Body:        body,
		})
		current[key] = fingerprint
	}
	for _, id := range coverage.PrematureBlockedRequiredIDs {
		required, ok := coordinationRequiredByID(coord, id)
		if !ok {
			continue
		}
		nonTerminal := requiredFrontierNonTerminalSurfaces(required.Surfaces)
		fingerprint := strings.Join([]string{
			"premature_blocked",
			id,
			"owner=" + strings.TrimSpace(required.Owner),
			"execution_state=" + strings.TrimSpace(required.ExecutionState),
			"blocked_by=" + strings.TrimSpace(required.BlockedBy),
			"non_terminal_surfaces=" + strings.Join(nonTerminal, ","),
		}, "|")
		key := "premature_blocked:" + id
		body := fmt.Sprintf("Required frontier gap in active GoalX run; required=%s fact=premature_blocked owner=%s execution_state=%s", id, blankAsUnknown(required.Owner), blankAsUnknown(required.ExecutionState))
		if blockedBy := strings.TrimSpace(required.BlockedBy); blockedBy != "" {
			body += " blocked_by=" + blockedBy
		}
		if len(nonTerminal) > 0 {
			body += " non_terminal_surfaces=" + strings.Join(nonTerminal, ",")
		}
		alerts = append(alerts, masterAlert{
			Key:         key,
			Fingerprint: fingerprint,
			Body:        body,
		})
		current[key] = fingerprint
	}
	controlGapFacts, err := BuildControlGapFacts(runDir)
	if err != nil {
		return nil, nil, err
	}
	if controlGapFacts != nil {
		if controlGapFacts.StatusDrift {
			key := "control_gap:status_drift"
			fingerprint := strings.Join([]string{
				"status_drift",
				"status_updated_at=" + strings.TrimSpace(controlGapFacts.StatusUpdatedAt),
			}, "|")
			body := "Control gap in active GoalX run; fact=status_drift"
			if controlGapFacts.StatusUpdatedAt != "" {
				body += " status_updated_at=" + controlGapFacts.StatusUpdatedAt
			}
			alerts = append(alerts, masterAlert{
				Key:         key,
				Fingerprint: fingerprint,
				Body:        body,
			})
			current[key] = fingerprint
		}
		if controlGapFacts.CoordinationStale {
			key := "control_gap:coordination_stale"
			fingerprint := strings.Join([]string{
				"coordination_stale",
				"coordination_updated_at=" + strings.TrimSpace(controlGapFacts.CoordinationUpdatedAt),
				"latest_control_change_at=" + strings.TrimSpace(controlGapFacts.LatestControlChangeAt),
			}, "|")
			body := "Control gap in active GoalX run; fact=coordination_stale"
			if controlGapFacts.CoordinationUpdatedAt != "" {
				body += " coordination_updated_at=" + controlGapFacts.CoordinationUpdatedAt
			}
			if controlGapFacts.LatestControlChangeAt != "" {
				body += " latest_control_change_at=" + controlGapFacts.LatestControlChangeAt
			}
			alerts = append(alerts, masterAlert{
				Key:         key,
				Fingerprint: fingerprint,
				Body:        body,
			})
			current[key] = fingerprint
		}
		if controlGapFacts.SerializedRequiredFrontier {
			key := "control_gap:serialized_required_frontier"
			fingerprint := strings.Join([]string{
				"serialized_required_frontier",
				"active_required_owners=" + strings.Join(controlGapFacts.ActiveRequiredOwners, ","),
				"reusable_sessions=" + strings.Join(controlGapFacts.ReusableSessions, ","),
				fmt.Sprintf("open_required_count=%d", controlGapFacts.OpenRequiredCount),
			}, "|")
			body := "Control gap in active GoalX run; fact=serialized_required_frontier"
			body += " active_required_owners=" + strings.Join(controlGapFacts.ActiveRequiredOwners, ",")
			body += fmt.Sprintf(" open_required_count=%d", controlGapFacts.OpenRequiredCount)
			if len(controlGapFacts.ReusableSessions) > 0 {
				body += " reusable_sessions=" + strings.Join(controlGapFacts.ReusableSessions, ",")
			}
			alerts = append(alerts, masterAlert{
				Key:         key,
				Fingerprint: fingerprint,
				Body:        body,
			})
			current[key] = fingerprint
		}
	}
	nonFrontierAlerts, nonFrontierCurrent, err := buildNonFrontierMasterAlerts(runDir)
	if err != nil {
		return nil, nil, err
	}
	alerts = append(alerts, nonFrontierAlerts...)
	for key, fingerprint := range nonFrontierCurrent {
		current[key] = fingerprint
	}
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Key < alerts[j].Key
	})
	if len(alerts) == 0 {
		return nil, nil, nil
	}
	return alerts, current, nil
}

func buildNonFrontierMasterAlerts(runDir string) ([]masterAlert, map[string]string, error) {
	alerts := make([]masterAlert, 0, 2)
	current := map[string]string{}

	qualityDebt, err := BuildQualityDebt(runDir)
	if err != nil {
		return nil, nil, err
	}
	acceptance, err := LoadAcceptanceState(AcceptanceStatePath(runDir))
	if err != nil {
		return nil, nil, err
	}
	if qualityDebt != nil && !qualityDebt.Zero() && hasBuilderEvidence(runDir, acceptance) {
		parts := qualityDebtAlertParts(qualityDebt)
		if len(parts) > 0 {
			key := "quality_debt:open"
			fingerprint := strings.Join(append([]string{"quality_debt"}, parts...), "|")
			body := "Quality debt in active GoalX run; fact=quality_debt " + strings.Join(parts, " ")
			alerts = append(alerts, masterAlert{
				Key:         key,
				Fingerprint: fingerprint,
				Body:        body,
			})
			current[key] = fingerprint
		}
	}

	status, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		return nil, nil, err
	}
	evolveFacts, err := LoadCurrentEvolveFacts(runDir)
	if err != nil {
		return nil, nil, err
	}
	if evolveFacts != nil && strings.TrimSpace(evolveFacts.ManagementGap) != "" {
		parts := []string{
			"gap=" + strings.TrimSpace(evolveFacts.ManagementGap),
			"frontier_state=" + blankAsUnknown(evolveFacts.FrontierState),
			fmt.Sprintf("open_candidate_count=%d", evolveFacts.OpenCandidateCount),
			fmt.Sprintf("active_sessions=%d", evolveFacts.ActiveSessionCount),
		}
		if evolveFacts.LastManagementEventAt != "" {
			parts = append(parts, "last_management_event_at="+evolveFacts.LastManagementEventAt)
		}
		if status != nil && strings.TrimSpace(status.Phase) != "" {
			parts = append(parts, "phase="+strings.TrimSpace(status.Phase))
		}
		key := "evolve_management_gap:" + strings.TrimSpace(evolveFacts.ManagementGap)
		fingerprint := strings.Join(append([]string{"evolve_management_gap"}, parts...), "|")
		body := "Evolve management gap in active GoalX run; fact=evolve_management_gap " + strings.Join(parts, " ")
		alerts = append(alerts, masterAlert{
			Key:         key,
			Fingerprint: fingerprint,
			Body:        body,
		})
		current[key] = fingerprint
	}

	if len(alerts) == 0 {
		return nil, nil, nil
	}
	return alerts, current, nil
}

func qualityDebtAlertParts(debt *QualityDebt) []string {
	if debt == nil || debt.Zero() {
		return nil
	}
	parts := make([]string, 0, 6)
	if len(debt.SuccessDimensionUnowned) > 0 {
		parts = append(parts, "success_dimension_unowned="+strings.Join(debt.SuccessDimensionUnowned, ","))
	}
	if len(debt.ProofPlanGap) > 0 {
		parts = append(parts, "proof_plan_gap="+strings.Join(debt.ProofPlanGap, ","))
	}
	if debt.CriticGateMissing {
		parts = append(parts, "critic_gate_missing")
	}
	if debt.FinisherGateMissing {
		parts = append(parts, "finisher_gate_missing")
	}
	if debt.OnlyCorrectnessEvidence {
		parts = append(parts, "only_correctness_evidence_present")
	}
	if debt.DomainPackMissing {
		parts = append(parts, "domain_pack_missing_for_nontrivial_run")
	}
	return parts
}

func coordinationRequiredByID(coord *CoordinationState, id string) (CoordinationRequiredItem, bool) {
	if coord == nil || coord.Required == nil {
		return CoordinationRequiredItem{}, false
	}
	required, ok := coord.Required[strings.TrimSpace(id)]
	return required, ok
}

func requiredFrontierNonTerminalSurfaces(surfaces CoordinationRequiredSurfaces) []string {
	parts := []struct {
		name  string
		value string
	}{
		{name: "repo", value: surfaces.Repo},
		{name: "runtime", value: surfaces.Runtime},
		{name: "run_artifacts", value: surfaces.RunArtifacts},
		{name: "web_research", value: surfaces.WebResearch},
		{name: "external_system", value: surfaces.ExternalSystem},
	}
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(part.value) {
		case coordinationRequiredSurfaceExhausted, coordinationRequiredSurfaceUnreachable, coordinationRequiredSurfaceNotApplicable:
			continue
		default:
			names = append(names, part.name)
		}
	}
	return names
}

func processUrgentTransportTargets(runDir, runName, tmuxSession string, cfg *goalx.Config, presence map[string]TargetPresenceFacts) error {
	type urgentTarget struct {
		name   string
		tmux   string
		engine string
	}

	targets := []urgentTarget{{
		name:   "master",
		tmux:   tmuxSession + ":master",
		engine: cfg.Master.Engine,
	}}
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		name := SessionName(idx)
		windowName, err := resolveWindowName(runName, name)
		if err != nil {
			return err
		}
		identity, err := RequireSessionIdentity(runDir, name)
		if err != nil {
			continue
		}
		targets = append(targets, urgentTarget{
			name:   name,
			tmux:   tmuxSession + ":" + windowName,
			engine: identity.Engine,
		})
	}

	for _, target := range targets {
		if !hasUrgentUnreadTarget(runDir, target.name) {
			if err := resetUrgentEscalationAttempts(runDir, target.name); err != nil {
				return err
			}
			continue
		}
		if presence != nil {
			if targetFacts, ok := presence[target.name]; ok {
				if targetPresenceMissing(targetFacts) || !targetPresenceAvailableForTransport(targetFacts) {
					continue
				}
			}
		}
		facts := loadTransportTargetFacts(runDir, target.name)
		switch normalizeTUITransportState(facts.TransportState) {
		case TUIStateProviderDialog, TUIStateBlank:
			continue
		case TUIStateInterrupted:
			outcome, err := escalateInterruptTransportInRunFunc(runDir, target.tmux, target.engine, "urgent_unread")
			if err != nil {
				return err
			}
			if err := recordInterruptEscalation(runDir, target.name, "urgent_unread", outcome); err != nil {
				return err
			}
		default:
			outcome, err := sendAgentNudgeDetailedInRunFunc(runDir, target.tmux, target.engine)
			if err != nil {
				return err
			}
			if err := recordWakeSubmit(runDir, target.name, outcome); err != nil {
				return err
			}
		}
	}
	return nil
}

func reconcileTargetPresenceRecovery(projectRoot, runDir, tmuxSession string, cfg *goalx.Config, runState *RunRuntimeState, closeoutComplete bool, presence map[string]TargetPresenceFacts) (map[string]TargetPresenceFacts, error) {
	if runState == nil || !runState.Active || closeoutComplete {
		return presence, nil
	}
	if presence == nil {
		return nil, fmt.Errorf("target presence is nil")
	}
	for _, facts := range presence {
		if err := recordTargetPresenceObservation(runDir, facts); err != nil {
			return nil, err
		}
	}
	masterFacts := presence["master"]
	if targetPresenceMissing(masterFacts) {
		recovery := loadTransportRecoveryTarget(runDir, "master")
		if recovery.CurrentMissingLastRelaunchAt == "" || recovery.CurrentMissingLastRelaunchResult != "success" {
			if err := recordMissingTargetRelaunchAttempt(runDir, "master", masterFacts.State); err != nil {
				return nil, err
			}
			var relaunchErr error
			if masterFacts.SessionExists {
				relaunchErr = relaunchMissingMasterWindow(projectRoot, runDir, tmuxSession, cfg)
			} else {
				appendAuditLog(runDir, "target_relaunch_attempt target=master cause=%s session_exists=%t window_exists=%t", blankAsUnknown(masterFacts.State), masterFacts.SessionExists, masterFacts.WindowExists)
				relaunchErr = relaunchMaster(projectRoot, runDir, tmuxSession, cfg)
			}
			if relaunchErr != nil {
				if !masterFacts.SessionExists {
					appendAuditLog(runDir, "target_relaunch_result target=master result=failure cause=%s err=%v", blankAsUnknown(masterFacts.State), relaunchErr)
				}
				if recordErr := recordMissingTargetRelaunchResult(runDir, "master", masterFacts.State, "failure", relaunchErr); recordErr != nil {
					return nil, recordErr
				}
				return nil, relaunchErr
			}
			if !masterFacts.SessionExists {
				appendAuditLog(runDir, "target_relaunch_result target=master result=success cause=%s", blankAsUnknown(masterFacts.State))
			}
			if err := recordMissingTargetRelaunchResult(runDir, "master", masterFacts.State, "success", nil); err != nil {
				return nil, err
			}
		}
	}
	masterFacts = presence["master"]
	masterAvailable := targetPresenceAvailableForTransport(masterFacts)
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		sessionFacts, ok := presence[sessionName]
		if !ok {
			continue
		}
		if !targetPresenceMissing(sessionFacts) {
			continue
		}
		if err := recordTargetPresenceObservation(runDir, sessionFacts); err != nil {
			return nil, err
		}
		if !masterAvailable {
			continue
		}
		recovery := loadTransportRecoveryTarget(runDir, sessionName)
		if recovery.CurrentMissingState == sessionFacts.State && recovery.CurrentMissingLastAlertAt != "" {
			continue
		}
		reason := fmt.Sprintf("target missing: %s state=%s first_seen=%s", sessionName, sessionFacts.State, blankAsUnknown(recovery.CurrentMissingFirstSeenAt))
		appendAuditLog(runDir, "target_missing_alert target=%s cause=%s first_seen=%s", sessionName, sessionFacts.State, blankAsUnknown(recovery.CurrentMissingFirstSeenAt))
		body := fmt.Sprintf("Target missing in active GoalX run; target=%s state=%s first_seen=%s action=do_not_respawn_worker", sessionName, sessionFacts.State, blankAsUnknown(recovery.CurrentMissingFirstSeenAt))
		if _, err := appendControlInboxMessage(runDir, "master", "target-missing", "goalx runtime-host", body, true); err != nil {
			return nil, err
		}
		if err := recordMissingTargetAlert(runDir, sessionName, sessionFacts.State, reason); err != nil {
			return nil, err
		}
	}
	return presence, nil
}

func transportNeedsRepair(facts TransportTargetFacts) bool {
	return facts.TransportState == string(TUIStateBufferedInput) || facts.InputContainsWake
}

func transportAcceptedRecently(facts TransportTargetFacts, window time.Duration, now time.Time) bool {
	if !isAcceptedTUITransportState(facts.TransportState) {
		return false
	}
	for _, ts := range []string{facts.LastTransportAcceptAt, facts.LastSubmitAttemptAt, facts.LastSampleAt} {
		if deliveryTimestampWithin(ts, window, now) {
			return true
		}
	}
	return false
}

func defaultStopRunRuntimeHost(runDir string) error {
	meta, _ := LoadRunMetadata(RunMetadataPath(runDir))
	lease, err := LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if meta != nil && meta.RunID != "" && lease.RunID != "" && lease.RunID != meta.RunID {
		return nil
	}
	if lease.PID > 0 {
		proc, err := os.FindProcess(lease.PID)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				current, loadErr := LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
				if loadErr == nil && current.PID == 0 {
					return nil
				}
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					break
				}
				time.Sleep(50 * time.Millisecond)
			}
			_ = proc.Signal(syscall.SIGKILL)
		}
	}
	return ExpireControlLease(runDir, "runtime-host")
}
