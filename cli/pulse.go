package cli

import "fmt"

// Pulse records a durable heartbeat tick, then nudges the master to read control files.
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
	if _, err := RecordHeartbeatTick(rc.RunDir); err != nil {
		return fmt.Errorf("record heartbeat tick: %w", err)
	}
	if !SessionExists(rc.TmuxSession) {
		return nil
	}
	if err := sendAgentNudge(rc.TmuxSession+":master", rc.Config.Master.Engine); err != nil {
		return fmt.Errorf("nudge master: %w", err)
	}
	return nil
}
