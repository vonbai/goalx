package cli

import (
	"fmt"
	"strings"
)

// Pulse schedules a durable master wake reminder through the control plane.
func Pulse(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return fmt.Errorf("usage: goalx pulse [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if err := EnsureMasterControl(rc.RunDir); err != nil {
		return fmt.Errorf("ensure master control: %w", err)
	}
	if err := queueMasterWakeReminder(rc.RunDir, rc.TmuxSession, rc.Config.Master.Engine); err != nil {
		return fmt.Errorf("queue master wake reminder: %w", err)
	}
	return nil
}

func queueMasterWakeReminder(runDir, tmuxSession, engine string) error {
	if !SessionExists(tmuxSession) {
		return nil
	}
	_, err := QueueControlReminderWithEngine(runDir, "master-wake", "control-cycle", tmuxSession+":master", engine)
	return err
}

func queueSessionWakeReminder(runDir, tmuxSession, sessionName, windowName, engine string) error {
	if !SessionExists(tmuxSession) {
		return nil
	}
	if strings.TrimSpace(windowName) == "" {
		return nil
	}
	if !WindowExists(tmuxSession, windowName) {
		return nil
	}
	_, err := QueueControlReminderWithEngine(runDir, "session-wake:"+sessionName, "session-inbox-unread", tmuxSession+":"+windowName, engine)
	return err
}
