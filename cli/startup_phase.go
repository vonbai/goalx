package cli

import (
	"fmt"
	"strings"
	"time"
)

const runStartupGraceWindow = 20 * time.Second

type RunStartupState struct {
	Phase string
}

func (state RunStartupState) Launching() bool {
	return strings.TrimSpace(state.Phase) != ""
}

func LoadRunStartupState(runDir, tmuxSession string) (RunStartupState, error) {
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return RunStartupState{}, err
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return RunStartupState{}, err
	}
	return deriveRunStartupState(runDir, tmuxSession, controlState, runtimeState)
}

func deriveRunStartupState(runDir, tmuxSession string, controlState *ControlRunState, runtimeState *RunRuntimeState) (RunStartupState, error) {
	if !controlRunContinuityRunning(controlState, runtimeState) {
		return RunStartupState{}, nil
	}
	operations, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil || operations == nil {
		return RunStartupState{}, err
	}
	op, ok := operations.Targets[RunBootstrapOperationKey()]
	if !ok || strings.TrimSpace(op.Kind) != ControlOperationKindRunBootstrap {
		return RunStartupState{}, nil
	}
	switch strings.TrimSpace(op.State) {
	case ControlOperationStatePreparing, ControlOperationStateHandshaking, ControlOperationStateReconciling:
		return RunStartupState{Phase: "bootstrapping"}, nil
	case ControlOperationStateCommitted:
		if !runStartupWithinGrace(op, runtimeState) {
			return RunStartupState{}, nil
		}
		settled, err := runStartupTargetsSettled(runDir, tmuxSession)
		if err != nil {
			return RunStartupState{}, err
		}
		if !settled {
			return RunStartupState{Phase: "settling"}, nil
		}
	}
	return RunStartupState{}, nil
}

func runBootstrapStillLaunching(runDir string, controlState *ControlRunState, runtimeState *RunRuntimeState) bool {
	startup, err := deriveRunStartupState(runDir, "", controlState, runtimeState)
	return err == nil && startup.Phase == "bootstrapping"
}

func runStartupWithinGrace(op ControlOperationTarget, runtimeState *RunRuntimeState) bool {
	anchor, ok := runStartupAnchor(op, runtimeState)
	if !ok {
		return false
	}
	now := time.Now().UTC()
	return !anchor.After(now) && now.Sub(anchor) <= runStartupGraceWindow
}

func runStartupAnchor(op ControlOperationTarget, runtimeState *RunRuntimeState) (time.Time, bool) {
	for _, raw := range []string{op.CommittedAt, op.UpdatedAt} {
		if ts, ok := parseStartupTime(raw); ok {
			return ts, true
		}
	}
	if runtimeState != nil {
		for _, raw := range []string{runtimeState.StartedAt, runtimeState.UpdatedAt} {
			if ts, ok := parseStartupTime(raw); ok {
				return ts, true
			}
		}
	}
	return time.Time{}, false
}

func parseStartupTime(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func runStartupTargetsSettled(runDir, tmuxSession string) (bool, error) {
	if !SessionExistsInRun(runDir, tmuxSession) {
		return false, nil
	}
	presence, err := BuildTargetPresenceFacts(runDir, tmuxSession)
	if err != nil {
		return false, err
	}
	if targetPresenceMissing(presence["master"]) || targetPresenceMissing(presence["runtime-host"]) {
		return false, nil
	}
	for target, facts := range presence {
		if strings.HasPrefix(target, "session-") && targetPresenceMissing(facts) {
			return false, nil
		}
	}
	return true, nil
}

func startupLeaseSummary(current string, startup RunStartupState) string {
	if !startup.Launching() {
		return current
	}
	switch strings.TrimSpace(current) {
	case "", "missing", "expired":
		return startup.Phase
	default:
		return current
	}
}

func formatStartupSummary(startup RunStartupState) string {
	if !startup.Launching() {
		return ""
	}
	return fmt.Sprintf("Startup: %s", startup.Phase)
}

func startupTargetObserveLabel(target string, facts TargetPresenceFacts, startup RunStartupState) string {
	if startup.Launching() && targetPresenceMissing(facts) {
		return target + " " + startup.Phase
	}
	return targetPresenceObserveLabel(target, facts)
}

func startupTransportObserveLabel(target string, facts TransportTargetFacts, startup RunStartupState) string {
	if startup.Launching() {
		return target + " " + startup.Phase
	}
	return transportMissingLabel(target, facts)
}
