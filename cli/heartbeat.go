package cli

import (
	"fmt"
	"time"
)

// HeartbeatCommand returns the shell command for the heartbeat tmux window.
// It runs a sleep loop that sends "Heartbeat: check now." to the master window.
func HeartbeatCommand(tmuxSession string, checkIntervalSeconds int) string {
	return fmt.Sprintf(
		`while sleep %d; do tmux send-keys -t %s:master 'Heartbeat: execute check cycle now.' Enter; done`,
		checkIntervalSeconds, tmuxSession,
	)
}

func normalizeHeartbeatInterval(checkInterval time.Duration) (int, string) {
	checkSec := int(checkInterval.Seconds())
	if checkSec < 30 {
		return 300, fmt.Sprintf("⚠ check_interval %ds is below 30s minimum, using default 300s\n", checkSec)
	}
	return checkSec, ""
}
