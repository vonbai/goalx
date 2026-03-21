package cli

import (
	"fmt"
	"time"
)

// HeartbeatCommand returns the shell command for the heartbeat tmux window.
// It runs a sleep loop that sends "Heartbeat: check now." to the master window.
func HeartbeatCommand(tmuxSession string, checkIntervalSeconds int) string {
	// Smart heartbeat: only send when master is idle (has prompt indicator).
	// After 3 consecutive skips, force-send to break potential deadlocks.
	return fmt.Sprintf(`SKIP=0; while sleep %d; do
  if tmux capture-pane -t %s:master -p -S -3 2>/dev/null | grep -qE '›|❯|shortcuts'; then
    tmux send-keys -t %s:master 'Heartbeat: execute check cycle now.' Enter
    SKIP=0
  else
    SKIP=$((SKIP+1))
    if [ $SKIP -ge 3 ]; then
      tmux send-keys -t %s:master 'Heartbeat: execute check cycle now.' Enter
      SKIP=0
    fi
  fi
done`, checkIntervalSeconds, tmuxSession, tmuxSession, tmuxSession)
}

func normalizeHeartbeatInterval(checkInterval time.Duration) (int, string) {
	checkSec := int(checkInterval.Seconds())
	if checkSec < 30 {
		return 300, fmt.Sprintf("⚠ check_interval %ds is below 30s minimum, using default 300s\n", checkSec)
	}
	return checkSec, ""
}
