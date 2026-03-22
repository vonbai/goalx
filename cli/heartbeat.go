package cli

import (
	"fmt"
	"time"
)

// HeartbeatCommand returns the shell command for the heartbeat tmux window.
// It routes ticks through the goalx pulse command so the durable heartbeat
// state and the tmux nudge stay in sync.
func HeartbeatCommand(goalxBin, runName string, checkIntervalSeconds int) string {
	return fmt.Sprintf(`while sleep %d; do
  %s pulse --run %s
done`, checkIntervalSeconds, shellQuote(goalxBin), shellQuote(runName))
}

func normalizeHeartbeatInterval(checkInterval time.Duration) (int, string) {
	checkSec := int(checkInterval.Seconds())
	if checkSec < 30 {
		return 300, fmt.Sprintf("⚠ check_interval %ds is below 30s minimum, using default 300s\n", checkSec)
	}
	return checkSec, ""
}
