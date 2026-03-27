package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	goalx "github.com/vonbai/goalx"
)

const sidecarUsage = "usage: goalx sidecar --run RUN [--interval SECONDS]"

var launchRunSidecar = defaultLaunchRunSidecar
var stopRunSidecar = defaultStopRunSidecar

var errSidecarStale = errors.New("sidecar run is stale")
var errSidecarCompleted = errors.New("sidecar run completed")

func Sidecar(projectRoot string, args []string) error {
	runName, interval, err := parseSidecarArgs(args)
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
		checkSec, _ := normalizeSidecarInterval(rc.Config.Master.CheckInterval)
		interval = time.Duration(checkSec) * time.Second
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runSidecarLoop(ctx, rc.ProjectRoot, rc.Name, rc.RunDir, meta.RunID, meta.Epoch, interval)
}

func parseSidecarArgs(args []string) (string, time.Duration, error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", 0, err
	}
	var interval time.Duration
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--help", "-h":
			return "", 0, fmt.Errorf(sidecarUsage)
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
			return "", 0, fmt.Errorf(sidecarUsage)
		}
	}
	if runName == "" {
		return "", 0, fmt.Errorf(sidecarUsage)
	}
	return runName, interval, nil
}

func runSidecarLoop(ctx context.Context, projectRoot, runName, runDir, runID string, epoch int, interval time.Duration) error {
	if interval <= 0 {
		interval = 5 * time.Minute
	}
	shouldExpire := true
	exitReason := "completed"
	var watcher *TmuxControlWatcher
	appendAuditLog(runDir, "sidecar started pid=%d runID=%s epoch=%d", os.Getpid(), runID, epoch)
	defer func() {
		appendAuditLog(runDir, "sidecar exiting reason=%s", exitReason)
	}()
	defer func() {
		if watcher != nil {
			_ = watcher.Close()
		}
	}()
	reportError := func(err error) error {
		appendAuditLog(runDir, "sidecar error: %v", err)
		exitReason = err.Error()
		return err
	}
	watcher = ensureSidecarTransportWatcher(projectRoot, runName, runDir, watcher)
	if err := runSidecarTickWithWatcher(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid(), watcher); err != nil {
		if errors.Is(err, errSidecarStale) {
			shouldExpire = false
			exitReason = errSidecarStale.Error()
			return nil
		}
		if errors.Is(err, errSidecarCompleted) {
			exitReason = errSidecarCompleted.Error()
			return nil
		}
		return reportError(err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer func() {
		if shouldExpire {
			_ = ExpireControlLease(runDir, "sidecar")
		}
	}()

	for {
		select {
		case <-ctx.Done():
			exitReason = ctx.Err().Error()
			return nil
		case <-ticker.C:
			watcher = ensureSidecarTransportWatcher(projectRoot, runName, runDir, watcher)
			if err := runSidecarTickWithWatcher(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid(), watcher); err != nil {
				if errors.Is(err, errSidecarStale) {
					shouldExpire = false
					exitReason = errSidecarStale.Error()
					return nil
				}
				if errors.Is(err, errSidecarCompleted) {
					exitReason = errSidecarCompleted.Error()
					return nil
				}
				return reportError(err)
			}
		}
	}
}

func runSidecarTick(projectRoot, runName, runDir, runID string, epoch int, interval time.Duration, pid int) error {
	return runSidecarTickWithWatcher(projectRoot, runName, runDir, runID, epoch, interval, pid, nil)
}

func runSidecarTickWithWatcher(projectRoot, runName, runDir, runID string, epoch int, interval time.Duration, pid int, watcher *TmuxControlWatcher) error {
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return errSidecarStale
		}
		return err
	}
	if meta == nil || meta.RunID != runID || meta.Epoch != epoch {
		return errSidecarStale
	}
	if _, err := os.Stat(RunSpecPath(runDir)); err != nil {
		if os.IsNotExist(err) {
			return errSidecarStale
		}
		return err
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return err
	}
	tmuxSession := goalx.TmuxSessionName(projectRoot, runName)
	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return err
	}
	if runState != nil && runState.Phase == "complete" && !SessionExists(tmuxSession) {
		if unreadControlInboxCount(MasterInboxPath(runDir), MasterCursorPath(runDir)) > 0 {
			if err := relaunchMaster(projectRoot, runDir, tmuxSession, cfg); err != nil {
				return err
			}
			return nil
		}
		if err := finalizeCompletedRunFromSidecar(projectRoot, runName, runDir); err != nil {
			return err
		}
		return errSidecarCompleted
	}
	ttl := interval * 2
	if ttl < time.Second {
		ttl = time.Second
	}
	if err := RenewControlLease(runDir, "sidecar", runID, epoch, ttl, "process", pid); err != nil {
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
	if snapshot, err := SnapshotWorktrees(runDir); err == nil {
		if err := SaveWorktreeSnapshot(runDir, snapshot); err != nil {
			return err
		}
	}
	if watcher != nil && watcher.Alive() {
		controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
		if err != nil {
			return err
		}
		if err := refreshTransportFactsForSidecar(runDir, tmuxSession, cfg.Master.Engine, watcher, controlState); err != nil {
			appendAuditLog(runDir, "transport watcher pre-queue snapshot warning: %v", err)
		}
		urgentUnread := hasUrgentUnread(runDir) || masterNeedsRecovery(runDir)
		urgentTicks := controlState.UrgentUnreadTicks
		if urgentUnread {
			urgentTicks++
			tmuxTarget := tmuxSession + ":master"
			if urgentTicks == 1 {
				if err := SendEscape(tmuxTarget); err != nil {
					return err
				}
				time.Sleep(500 * time.Millisecond)
				if err := sendAgentNudge(tmuxTarget, cfg.Master.Engine); err != nil {
					return err
				}
			} else if urgentTicks >= 3 {
				if err := relaunchMaster(projectRoot, runDir, tmuxSession, cfg); err != nil {
					return err
				}
				urgentTicks = 0
			}
		} else {
			urgentTicks = 0
		}
		if urgentTicks != controlState.UrgentUnreadTicks {
			controlState.UrgentUnreadTicks = urgentTicks
			controlState.UpdatedAt = ""
			if err := SaveControlRunState(ControlRunStatePath(runDir), controlState); err != nil {
				return err
			}
		}
		if err := queueUnreadSessionWakeReminders(runDir, tmuxSession, runName, interval); err != nil {
			return err
		}
		if err := queueMasterWakeReminder(runDir, tmuxSession, cfg.Master.Engine); err != nil {
			return err
		}
		if err := DeliverDueControlReminders(runDir, cfg.Master.Engine, interval, sendAgentNudgeDetailed); err != nil {
			return err
		}
		if err := refreshTransportFactsForSidecar(runDir, tmuxSession, cfg.Master.Engine, watcher, controlState); err != nil {
			appendAuditLog(runDir, "transport watcher snapshot warning: %v", err)
		}
		if err := RefreshRunMemorySeeds(runDir); err != nil {
			return err
		}
		if err := AppendExtractedMemoryProposals(runDir, time.Now().UTC()); err != nil {
			return err
		}
		if err := RefreshRunGuidance(projectRoot, runName, runDir); err != nil {
			appendAuditLog(runDir, "guidance refresh warning: %v", err)
		}
		if err := refreshActivityFacts(runDir, projectRoot, runName); err != nil {
			appendAuditLog(runDir, "activity snapshot warning: %v", err)
		}
		return nil
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	if err := refreshTransportFactsForSidecar(runDir, tmuxSession, cfg.Master.Engine, nil, controlState); err != nil {
		return err
	}
	urgentUnread := hasUrgentUnread(runDir) || masterNeedsRecovery(runDir)
	urgentTicks := controlState.UrgentUnreadTicks
	if urgentUnread {
		urgentTicks++
		tmuxTarget := tmuxSession + ":master"
		if urgentTicks == 1 {
			if err := SendEscape(tmuxTarget); err != nil {
				return err
			}
			time.Sleep(500 * time.Millisecond)
			if err := sendAgentNudge(tmuxTarget, cfg.Master.Engine); err != nil {
				return err
			}
		} else if urgentTicks >= 3 {
			if err := relaunchMaster(projectRoot, runDir, tmuxSession, cfg); err != nil {
				return err
			}
			urgentTicks = 0
		}
	} else {
		urgentTicks = 0
	}
	if urgentTicks != controlState.UrgentUnreadTicks {
		controlState.UrgentUnreadTicks = urgentTicks
		controlState.UpdatedAt = ""
		if err := SaveControlRunState(ControlRunStatePath(runDir), controlState); err != nil {
			return err
		}
	}
	if err := queueUnreadSessionWakeReminders(runDir, tmuxSession, runName, interval); err != nil {
		return err
	}
	if err := queueMasterWakeReminder(runDir, tmuxSession, cfg.Master.Engine); err != nil {
		return err
	}
	if err := DeliverDueControlReminders(runDir, cfg.Master.Engine, interval, sendAgentNudgeDetailed); err != nil {
		return err
	}
	if err := refreshTransportFactsForSidecar(runDir, tmuxSession, cfg.Master.Engine, nil, controlState); err != nil {
		return err
	}
	if err := RefreshRunMemorySeeds(runDir); err != nil {
		return err
	}
	if err := AppendExtractedMemoryProposals(runDir, time.Now().UTC()); err != nil {
		return err
	}
	if err := RefreshRunGuidance(projectRoot, runName, runDir); err != nil {
		appendAuditLog(runDir, "guidance refresh warning: %v", err)
	}
	if err := refreshActivityFacts(runDir, projectRoot, runName); err != nil {
		appendAuditLog(runDir, "activity snapshot warning: %v", err)
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

func refreshTransportFactsForSidecar(runDir, tmuxSession, masterEngine string, watcher *TmuxControlWatcher, controlState *ControlRunState) error {
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
	return reconcileProviderDialogAlerts(runDir, controlState, facts)
}

func reconcileProviderDialogAlerts(runDir string, controlState *ControlRunState, facts *TransportFacts) error {
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
			if _, err := appendControlInboxMessage(runDir, "master", "provider-dialog-visible", "goalx sidecar", body, true); err != nil {
				return err
			}
		}
	}
	if mapsEqual(controlState.ProviderDialogAlerts, current) {
		return nil
	}
	if len(current) == 0 {
		controlState.ProviderDialogAlerts = nil
	} else {
		controlState.ProviderDialogAlerts = current
	}
	controlState.UpdatedAt = ""
	return SaveControlRunState(ControlRunStatePath(runDir), controlState)
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

func ensureSidecarTransportWatcher(projectRoot, runName, runDir string, watcher *TmuxControlWatcher) *TmuxControlWatcher {
	if watcher != nil && watcher.Alive() {
		return watcher
	}
	tmuxSession := goalx.TmuxSessionName(projectRoot, runName)
	if !SessionExists(tmuxSession) {
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
	if !SessionExists(tmuxSession) {
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

func transportNeedsRepair(facts TransportTargetFacts) bool {
	return facts.TransportState == "buffered" || facts.InputContainsWake
}

func transportAcceptedRecently(facts TransportTargetFacts, window time.Duration, now time.Time) bool {
	if strings.TrimSpace(facts.TransportState) != "sent" {
		return false
	}
	for _, ts := range []string{facts.LastTransportAcceptAt, facts.LastSubmitAttemptAt, facts.LastSampleAt} {
		if deliveryTimestampWithin(ts, window, now) {
			return true
		}
	}
	return false
}

func masterNeedsRecovery(runDir string) bool {
	return loadTransportTargetFacts(runDir, "master").ProviderDialogVisible
}

func defaultLaunchRunSidecar(projectRoot, runName string, interval time.Duration) error {
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	seconds := int(interval.Seconds())
	if seconds <= 0 {
		seconds = 300
	}

	runDir := goalx.RunDir(projectRoot, runName)
	logFile, err := os.OpenFile(filepath.Join(runDir, "sidecar.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open sidecar log: %w", err)
	}
	defer logFile.Close()

	cmd := exec.Command(goalxBin, "sidecar", "--run", runName, "--interval", strconv.Itoa(seconds))
	cmd.Dir = projectRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start sidecar: %w", err)
	}
	meta, _ := LoadRunMetadata(RunMetadataPath(runDir))
	epoch := 1
	if meta != nil && meta.Epoch > 0 {
		epoch = meta.Epoch
	}
	runID := ""
	if meta != nil {
		runID = meta.RunID
	}
	if err := RenewControlLease(runDir, "sidecar", runID, epoch, interval*2, "process", cmd.Process.Pid); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func defaultStopRunSidecar(runDir string) error {
	meta, _ := LoadRunMetadata(RunMetadataPath(runDir))
	lease, err := LoadControlLease(ControlLeasePath(runDir, "sidecar"))
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
				current, loadErr := LoadControlLease(ControlLeasePath(runDir, "sidecar"))
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
	return ExpireControlLease(runDir, "sidecar")
}
