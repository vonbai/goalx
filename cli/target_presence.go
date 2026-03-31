package cli

import (
	"strings"
	"time"
)

const (
	TargetPresencePresent        = "present"
	TargetPresenceParked         = "parked"
	TargetPresenceUnknown        = "unknown"
	TargetPresenceSessionMissing = "session_missing"
	TargetPresenceWindowMissing  = "window_missing"
	TargetPresencePaneMissing    = "pane_missing"
	TargetPresenceLeaseExpired   = "lease_expired"
	TargetPresenceProcessMissing = "process_missing"
)

type TargetPresenceFacts struct {
	Target          string `json:"target,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Window          string `json:"window,omitempty"`
	SessionExpected bool   `json:"session_expected,omitempty"`
	SessionExists   bool   `json:"session_exists,omitempty"`
	WindowExpected  bool   `json:"window_expected,omitempty"`
	WindowExists    bool   `json:"window_exists,omitempty"`
	PaneID          string `json:"pane_id,omitempty"`
	PaneExists      bool   `json:"pane_exists,omitempty"`
	LeasePresent    bool   `json:"lease_present,omitempty"`
	LeaseHealthy    bool   `json:"lease_healthy,omitempty"`
	ProcessPID      int    `json:"process_pid,omitempty"`
	ProcessPIDAlive bool   `json:"process_pid_alive,omitempty"`
	State           string `json:"state,omitempty"`
	CheckedAt       string `json:"checked_at,omitempty"`
}

func BuildTargetPresenceFacts(runDir, tmuxSession string) (map[string]TargetPresenceFacts, error) {
	checkedAt := time.Now().UTC().Format(time.RFC3339)
	sessionExists := SessionExistsInRun(runDir, tmuxSession)
	windowsByName := map[string]struct{}{}
	if sessionExists {
		windowsByName, _ = tmuxWindowsByNameInRun(runDir, tmuxSession)
	}
	panesByWindow, err := tmuxPanesByWindow(runDir, tmuxSession)
	if err != nil {
		return nil, err
	}
	parkedTargets, err := loadParkedSessionTargets(runDir)
	if err != nil {
		return nil, err
	}

	targets := map[string]TargetPresenceFacts{
		"master": buildTmuxTargetPresence("master", "master", "master", checkedAt, sessionExists, true, windowsByName, panesByWindow),
	}

	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		name := SessionName(idx)
		targets[name] = buildTmuxTargetPresence(name, "session", name, checkedAt, sessionExists, !parkedTargets[name], windowsByName, panesByWindow)
	}
	targets["runtime-host"] = buildRuntimeHostTargetPresence(runDir, checkedAt)
	return targets, nil
}

func LoadTargetPresenceFact(runDir, tmuxSession, target string) (TargetPresenceFacts, error) {
	targets, err := BuildTargetPresenceFacts(runDir, tmuxSession)
	if err != nil {
		return TargetPresenceFacts{}, err
	}
	return targets[target], nil
}

func targetPresenceAvailableForTransport(facts TargetPresenceFacts) bool {
	state := strings.TrimSpace(facts.State)
	return state == "" || state == TargetPresencePresent || state == TargetPresenceUnknown
}

func targetPresenceMissing(facts TargetPresenceFacts) bool {
	state := strings.TrimSpace(facts.State)
	return state != "" && state != TargetPresencePresent && state != TargetPresenceUnknown && state != TargetPresenceParked
}

func targetPresenceMissingLabel(target string, facts TargetPresenceFacts) string {
	switch strings.TrimSpace(facts.State) {
	case TargetPresenceSessionMissing:
		return target + " session missing"
	case TargetPresenceWindowMissing:
		return target + " window missing"
	case TargetPresencePaneMissing:
		return target + " pane missing"
	default:
		return ""
	}
}

func targetPresenceObserveLabel(target string, facts TargetPresenceFacts) string {
	if label := targetPresenceMissingLabel(target, facts); label != "" {
		return label
	}
	if strings.TrimSpace(facts.State) == TargetPresenceParked {
		return target + " parked"
	}
	return ""
}

func buildTmuxTargetPresence(target, kind, window, checkedAt string, sessionExists, windowExpected bool, windowsByName map[string]struct{}, panesByWindow map[string]tmuxPaneRef) TargetPresenceFacts {
	facts := TargetPresenceFacts{
		Target:          target,
		Kind:            kind,
		Window:          window,
		SessionExpected: true,
		SessionExists:   sessionExists,
		WindowExpected:  windowExpected,
		CheckedAt:       checkedAt,
	}
	if !windowExpected {
		if sessionExists {
			if _, ok := windowsByName[window]; ok {
				facts.WindowExists = true
			}
			if pane, ok := panesByWindow[window]; ok {
				facts.PaneExists = true
				facts.PaneID = strings.TrimSpace(pane.PaneID)
			}
		}
		facts.State = TargetPresenceParked
		return facts
	}
	if !sessionExists {
		facts.State = TargetPresenceSessionMissing
		return facts
	}
	if len(windowsByName) == 0 && len(panesByWindow) == 0 {
		facts.State = TargetPresenceUnknown
		return facts
	}
	if _, ok := windowsByName[window]; !ok {
		facts.State = TargetPresenceWindowMissing
		return facts
	}
	facts.WindowExists = true
	pane, ok := panesByWindow[window]
	if !ok {
		facts.State = TargetPresencePaneMissing
		return facts
	}
	facts.PaneExists = true
	facts.PaneID = strings.TrimSpace(pane.PaneID)
	facts.State = TargetPresencePresent
	return facts
}

func loadParkedSessionTargets(runDir string) (map[string]bool, error) {
	parked := map[string]bool{}
	runtimeState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	if runtimeState != nil {
		for name, session := range runtimeState.Sessions {
			if strings.TrimSpace(session.State) == "parked" {
				parked[name] = true
			}
		}
	}
	coordination, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		return nil, err
	}
	if coordination != nil {
		for name, session := range coordination.Sessions {
			if strings.TrimSpace(session.State) == "parked" {
				parked[name] = true
			}
		}
	}
	return parked, nil
}

func buildLeaseTargetPresence(runDir, target, checkedAt string) TargetPresenceFacts {
	facts := TargetPresenceFacts{
		Target:    target,
		Kind:      target,
		CheckedAt: checkedAt,
	}
	lease, err := LoadControlLease(ControlLeasePath(runDir, target))
	if err != nil || lease == nil {
		facts.State = TargetPresenceLeaseExpired
		return facts
	}
	facts.LeasePresent = true
	facts.ProcessPID = lease.PID
	facts.ProcessPIDAlive = processAlive(lease.PID)
	facts.LeaseHealthy = controlLeaseHealthyAt(lease, time.Now().UTC())
	switch {
	case !facts.LeaseHealthy:
		facts.State = TargetPresenceLeaseExpired
	case lease.PID > 0 && !facts.ProcessPIDAlive:
		facts.State = TargetPresenceProcessMissing
	default:
		facts.State = TargetPresencePresent
	}
	return facts
}

func buildRuntimeHostTargetPresence(runDir, checkedAt string) TargetPresenceFacts {
	if host, err := LoadRunHostState(RunHostStatePath(runDir)); err == nil && host != nil {
		facts := TargetPresenceFacts{
			Target:          "runtime-host",
			Kind:            "runtime_host",
			CheckedAt:       checkedAt,
			LeasePresent:    true,
			LeaseHealthy:    host.Running,
			ProcessPID:      host.PID,
			ProcessPIDAlive: processAlive(host.PID),
		}
		switch {
		case !host.Running:
			facts.State = TargetPresenceLeaseExpired
		case host.PID > 0 && !facts.ProcessPIDAlive:
			facts.State = TargetPresenceProcessMissing
		default:
			facts.State = TargetPresencePresent
		}
		return facts
	}
	return buildLeaseTargetPresence(runDir, "runtime-host", checkedAt)
}
