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
	appendAuditLog(runDir, "sidecar started pid=%d runID=%s epoch=%d", os.Getpid(), runID, epoch)
	defer func() {
		appendAuditLog(runDir, "sidecar exiting reason=%s", exitReason)
	}()
	reportError := func(err error) error {
		appendAuditLog(runDir, "sidecar error: %v", err)
		exitReason = err.Error()
		return err
	}
	if err := runSidecarTick(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid()); err != nil {
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
			if err := runSidecarTick(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid()); err != nil {
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
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	urgentUnread := hasUrgentUnread(runDir)
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
	if err := queueUnreadSessionWakeReminders(runDir, tmuxSession, runName); err != nil {
		return err
	}
	if err := queueMasterWakeReminder(runDir, tmuxSession, cfg.Master.Engine); err != nil {
		return err
	}
	if err := DeliverDueControlReminders(runDir, cfg.Master.Engine, interval, sendAgentNudge); err != nil {
		return err
	}
	if err := RefreshRunGuidance(projectRoot, runName, runDir); err != nil {
		appendAuditLog(runDir, "guidance refresh warning: %v", err)
	}
	return nil
}

func queueRefreshContextReminder(runDir, tmuxSession, engine string) error {
	if !SessionExists(tmuxSession) {
		return nil
	}
	_, err := QueueControlReminderWithEngine(runDir, "refresh-context", "identity-fence-changed", tmuxSession+":master", engine)
	return err
}

func queueUnreadSessionWakeReminders(runDir, tmuxSession, runName string) error {
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return err
	}
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
