package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const leaseLoopUsage = "usage: goalx lease-loop (--run RUN | --run-dir PATH) --holder HOLDER --run-id RUN_ID --epoch N --ttl-seconds N --transport NAME --pid PID"

var errLeaseLoopStale = errors.New("lease loop is stale")
var errLeaseTargetExited = errors.New("lease target exited")

func LeaseLoop(projectRoot string, args []string) error {
	runName, runDir, holder, runID, epoch, ttl, transport, pid, err := parseLeaseLoopArgs(args)
	if err != nil {
		return err
	}
	if runDir == "" {
		rc, err := ResolveRun(projectRoot, runName)
		if err != nil {
			return err
		}
		runDir = rc.RunDir
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runLeaseLoop(ctx, runDir, holder, runID, epoch, ttl, transport, pid)
}

func parseLeaseLoopArgs(args []string) (string, string, string, string, int, time.Duration, string, int, error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", "", "", "", 0, 0, "", 0, err
	}

	var runDir, holder, runID, transport string
	var epoch, ttlSeconds, pid int
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--help", "-h":
			return "", "", "", "", 0, 0, "", 0, fmt.Errorf(leaseLoopUsage)
		case "--run-dir":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --run-dir")
			}
			i++
			runDir = rest[i]
		case "--holder":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --holder")
			}
			i++
			holder = rest[i]
		case "--run-id":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --run-id")
			}
			i++
			runID = rest[i]
		case "--epoch":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --epoch")
			}
			i++
			epoch, err = strconv.Atoi(rest[i])
			if err != nil || epoch <= 0 {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("invalid --epoch %q", rest[i])
			}
		case "--ttl-seconds":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --ttl-seconds")
			}
			i++
			ttlSeconds, err = strconv.Atoi(rest[i])
			if err != nil || ttlSeconds <= 0 {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("invalid --ttl-seconds %q", rest[i])
			}
		case "--transport":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --transport")
			}
			i++
			transport = rest[i]
		case "--pid":
			if i+1 >= len(rest) {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("missing value for --pid")
			}
			i++
			pid, err = strconv.Atoi(rest[i])
			if err != nil || pid <= 0 {
				return "", "", "", "", 0, 0, "", 0, fmt.Errorf("invalid --pid %q", rest[i])
			}
		default:
			return "", "", "", "", 0, 0, "", 0, fmt.Errorf(leaseLoopUsage)
		}
	}

	if (runName == "" && runDir == "") || holder == "" || runID == "" || epoch <= 0 || ttlSeconds <= 0 || transport == "" || pid <= 0 {
		return "", "", "", "", 0, 0, "", 0, fmt.Errorf(leaseLoopUsage)
	}
	return runName, runDir, holder, runID, epoch, time.Duration(ttlSeconds) * time.Second, transport, pid, nil
}

func runLeaseLoop(ctx context.Context, runDir, holder, runID string, epoch int, ttl time.Duration, transport string, pid int) error {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	if pid <= 0 {
		return fmt.Errorf("invalid lease pid %d", pid)
	}

	shouldExpire := true
	exitReason := "completed"
	defer func() {
		appendAuditLog(runDir, "lease-loop exiting holder=%s reason=%s", holder, exitReason)
	}()
	if err := renewLeaseLoopOnce(runDir, holder, runID, epoch, ttl, transport, pid); err != nil {
		switch {
		case errors.Is(err, errLeaseLoopStale):
			shouldExpire = false
			exitReason = errLeaseLoopStale.Error()
			return nil
		case errors.Is(err, errLeaseTargetExited):
			exitReason = errLeaseTargetExited.Error()
			return nil
		default:
			exitReason = err.Error()
			return err
		}
	}

	interval := ttl / 2
	if interval < time.Second {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer func() {
		if shouldExpire {
			_ = ExpireControlLease(runDir, holder)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			exitReason = ctx.Err().Error()
			return nil
		case <-ticker.C:
			if err := renewLeaseLoopOnce(runDir, holder, runID, epoch, ttl, transport, pid); err != nil {
				switch {
				case errors.Is(err, errLeaseLoopStale):
					shouldExpire = false
					exitReason = errLeaseLoopStale.Error()
					return nil
				case errors.Is(err, errLeaseTargetExited):
					exitReason = errLeaseTargetExited.Error()
					return nil
				default:
					exitReason = err.Error()
					return err
				}
			}
		}
	}
}

func renewLeaseLoopOnce(runDir, holder, runID string, epoch int, ttl time.Duration, transport string, pid int) error {
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return errLeaseLoopStale
		}
		return err
	}
	if meta == nil || meta.RunID != runID || meta.Epoch != epoch {
		return errLeaseLoopStale
	}
	if _, err := os.Stat(RunSpecPath(runDir)); err != nil {
		if os.IsNotExist(err) {
			return errLeaseLoopStale
		}
		return err
	}
	if !processAlive(pid) {
		return errLeaseTargetExited
	}
	return RenewControlLease(runDir, holder, runID, epoch, ttl, transport, pid)
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
