package cli

import (
	"fmt"
	"time"
)

const waitUsage = `usage: goalx wait [--run NAME] [master|session-N] [--timeout DURATION]`

var waitPollInterval = time.Second

// Wait blocks until a target inbox has unread entries, a timeout expires, or the run stops.
func Wait(projectRoot string, args []string) error {
	if printUsageIfHelp(args, waitUsage) {
		return nil
	}

	runName, target, timeout, err := parseWaitArgs(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	targetName, inboxPath, cursorPath, err := resolveWaitTarget(rc, target)
	if err != nil {
		return err
	}

	event, err := waitForInboxEvent(rc.RunDir, rc.Name, targetName, inboxPath, cursorPath, timeout)
	if event != "" {
		fmt.Println(event)
	}
	return err
}

func parseWaitArgs(args []string) (runName, target string, timeout time.Duration, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", "", 0, err
	}

	positional := make([]string, 0, len(rest))
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--timeout":
			if i+1 >= len(rest) {
				return "", "", 0, fmt.Errorf("missing value for --timeout")
			}
			i++
			timeout, err = time.ParseDuration(rest[i])
			if err != nil {
				return "", "", 0, fmt.Errorf("parse --timeout: %w", err)
			}
		default:
			positional = append(positional, rest[i])
		}
	}

	if runName == "" && len(positional) > 0 && !isWaitTarget(positional[0]) {
		runName = positional[0]
		positional = positional[1:]
	}
	if len(positional) > 1 {
		return "", "", 0, fmt.Errorf(waitUsage)
	}
	if len(positional) == 1 {
		target = positional[0]
	}
	if target == "" {
		target = "master"
	}
	if !isWaitTarget(target) {
		return "", "", 0, fmt.Errorf("invalid wait target %q (expected master or session-N)", target)
	}
	return runName, target, timeout, nil
}

func isWaitTarget(arg string) bool {
	if arg == "master" {
		return true
	}
	_, err := parseSessionIndex(arg)
	return err == nil
}

func resolveWaitTarget(rc *RunContext, target string) (targetName, inboxPath, cursorPath string, err error) {
	if target == "" || target == "master" {
		if err := EnsureMasterControl(rc.RunDir); err != nil {
			return "", "", "", err
		}
		return "master", MasterInboxPath(rc.RunDir), MasterCursorPath(rc.RunDir), nil
	}

	idx, err := parseSessionIndex(target)
	if err != nil {
		return "", "", "", err
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		return "", "", "", err
	}
	if !ok {
		return "", "", "", fmt.Errorf("session %q out of range for run %q", target, rc.Name)
	}
	if err := EnsureSessionControl(rc.RunDir, target); err != nil {
		return "", "", "", err
	}
	return target, ControlInboxPath(rc.RunDir, target), SessionCursorPath(rc.RunDir, target), nil
}

func waitForInboxEvent(runDir, runName, targetName, inboxPath, cursorPath string, timeout time.Duration) (string, error) {
	if unread := unreadControlInboxCount(inboxPath, cursorPath); unread > 0 {
		return fmt.Sprintf("wait: inbox pending for %s (%d unread)", targetName, unread), nil
	}
	if stopped, lifecycle := waitRunStopped(runDir); stopped {
		return "", fmt.Errorf("run %q is stopped (%s)", runName, lifecycle)
	}

	poll := waitPollInterval
	if poll <= 0 {
		poll = time.Second
	}
	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	var timeoutC <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutC = timer.C
	}

	for {
		select {
		case <-ticker.C:
			if unread := unreadControlInboxCount(inboxPath, cursorPath); unread > 0 {
				return fmt.Sprintf("wait: inbox pending for %s (%d unread)", targetName, unread), nil
			}
			if stopped, lifecycle := waitRunStopped(runDir); stopped {
				return "", fmt.Errorf("run %q is stopped (%s)", runName, lifecycle)
			}
		case <-timeoutC:
			return fmt.Sprintf("wait: timed out waiting for %s after %s", targetName, timeout), nil
		}
	}
}

func waitRunStopped(runDir string) (bool, string) {
	if state, err := LoadControlRunState(ControlRunStatePath(runDir)); err == nil && state != nil {
		switch state.LifecycleState {
		case "", "active":
		default:
			return true, state.LifecycleState
		}
	}
	if state, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir)); err == nil && state != nil && !state.Active {
		lifecycle := "inactive"
		if state.Phase != "" {
			lifecycle = state.Phase
		}
		return true, lifecycle
	}
	return false, ""
}
