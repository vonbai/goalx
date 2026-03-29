package cli

import (
	"testing"
	"time"
)

func TestWaitForSessionLaunchReadyAllowsCodexBlankBootstrap(t *testing.T) {
	origCapture := captureAgentPane
	origTimeout := sessionLaunchHandshakeTimeout
	origPoll := sessionLaunchHandshakePollInterval
	defer func() {
		captureAgentPane = origCapture
		sessionLaunchHandshakeTimeout = origTimeout
		sessionLaunchHandshakePollInterval = origPoll
	}()

	sessionLaunchHandshakeTimeout = 40 * time.Millisecond
	sessionLaunchHandshakePollInterval = 10 * time.Millisecond

	var polls int
	captureAgentPane = func(target string) (string, error) {
		polls++
		if polls <= 5 {
			return "", nil
		}
		return "❯ ready\n", nil
	}

	if err := waitForSessionLaunchReady("gx-demo", "session-1", "session-1", "codex"); err != nil {
		t.Fatalf("waitForSessionLaunchReady: %v", err)
	}
	if polls < 6 {
		t.Fatalf("polls = %d, want at least 6", polls)
	}
}
