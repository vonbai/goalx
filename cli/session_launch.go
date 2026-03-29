package cli

import (
	"fmt"
	"strings"
	"time"
)

var (
	sessionLaunchHandshakeTimeout      = 2 * time.Second
	sessionLaunchHandshakePollInterval = 100 * time.Millisecond
)

func waitForSessionLaunchReady(tmuxSession, sessionName, windowName, engine string) error {
	target := tmuxSession + ":" + windowName
	deadline := time.Now().Add(sessionLaunchHandshakeTimeoutForEngine(engine))
	var lastErr error
	lastSummary := "blank"
	for {
		summary, ready, err := inspectSessionLaunchTarget(target, engine)
		if err == nil && ready {
			return nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = nil
			lastSummary = summary
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(sessionLaunchHandshakePollInterval)
	}
	if lastErr != nil {
		return fmt.Errorf("launch handshake failed for %s: %w", sessionName, lastErr)
	}
	return fmt.Errorf("launch handshake failed for %s: pane not ready (%s)", sessionName, lastSummary)
}

func sessionLaunchHandshakeTimeoutForEngine(engine string) time.Duration {
	timeout := sessionLaunchHandshakeTimeout
	if strings.TrimSpace(engine) == "codex" {
		return timeout * 4
	}
	return timeout
}

func inspectSessionLaunchTarget(target, engine string) (summary string, ready bool, err error) {
	capture := captureAgentPane
	if capture == nil {
		capture = CapturePaneTargetOutput
	}
	out, err := capture(target)
	if err != nil {
		return "", false, err
	}
	recent := tailRecentNonEmptyLines(out, 8)
	if visible, kind, hint := targetProviderDialogVisible(recent); visible {
		detail := strings.TrimSpace(kind)
		if strings.TrimSpace(hint) != "" {
			if detail != "" {
				detail += ": "
			}
			detail += strings.TrimSpace(hint)
		}
		if detail == "" {
			detail = "provider dialog visible"
		}
		return "", false, fmt.Errorf("provider dialog visible (%s)", detail)
	}
	if strings.TrimSpace(out) == "" {
		return "blank", false, nil
	}
	facts := TransportTargetFacts{
		Engine:               strings.TrimSpace(engine),
		PromptVisible:        targetPromptVisible(recent),
		WorkingVisible:       targetWorkingVisible(engine, recent),
		QueuedMessageVisible: targetQueuedMessageVisible(engine, recent),
		InputContainsWake:    targetWakeBuffered(recent),
	}
	state := normalizeTUITransportState(classifyTransportState(engine, facts, recent))
	if state == "" {
		state = TUIStateUnknown
	}
	if state == TUIStateUnknown {
		return "output visible", true, nil
	}
	return string(state), true, nil
}
