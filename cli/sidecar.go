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
	if err := runSidecarTick(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid()); err != nil {
		if errors.Is(err, errSidecarStale) {
			shouldExpire = false
			return nil
		}
		return err
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
			return nil
		case <-ticker.C:
			if err := runSidecarTick(projectRoot, runName, runDir, runID, epoch, interval, os.Getpid()); err != nil {
				if errors.Is(err, errSidecarStale) {
					shouldExpire = false
					return nil
				}
				return err
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
	ttl := interval * 2
	if ttl < time.Second {
		ttl = time.Second
	}
	if err := RenewControlLease(runDir, "sidecar", runID, epoch, ttl, "process", pid); err != nil {
		return err
	}
	if err := Pulse(projectRoot, []string{"--run", runName}); err != nil {
		return err
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return err
	}
	return DeliverDueControlReminders(runDir, cfg.Master.Engine, sendAgentNudge)
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
