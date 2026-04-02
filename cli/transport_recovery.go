package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	missingTargetRelaunchMinBackoff = 15 * time.Second
	missingTargetRelaunchMaxBackoff = 10 * time.Minute
)

type TransportRecoveryState struct {
	Version   int                                `json:"version"`
	Targets   map[string]TransportRecoveryTarget `json:"targets,omitempty"`
	UpdatedAt string                             `json:"updated_at,omitempty"`
}

type TransportRecoveryTarget struct {
	Target                           string `json:"target,omitempty"`
	LastWakeSubmitAt                 string `json:"last_wake_submit_at,omitempty"`
	LastEnterRepairAt                string `json:"last_enter_repair_at,omitempty"`
	LastInterruptEscalationAt        string `json:"last_interrupt_escalation_at,omitempty"`
	LastInterruptReason              string `json:"last_interrupt_reason,omitempty"`
	LastInterruptResultingState      string `json:"last_interrupt_resulting_state,omitempty"`
	UrgentEscalationAttempts         int    `json:"urgent_escalation_attempts,omitempty"`
	CurrentAttentionState            string `json:"current_attention_state,omitempty"`
	CurrentAttentionFirstSeenAt      string `json:"current_attention_first_seen_at,omitempty"`
	CurrentAttentionLastSeenAt       string `json:"current_attention_last_seen_at,omitempty"`
	CurrentAttentionLastAlertAt      string `json:"current_attention_last_alert_at,omitempty"`
	CurrentAttentionLastAlertReason  string `json:"current_attention_last_alert_reason,omitempty"`
	CurrentMissingState              string `json:"current_missing_state,omitempty"`
	CurrentMissingFirstSeenAt        string `json:"current_missing_first_seen_at,omitempty"`
	CurrentMissingLastSeenAt         string `json:"current_missing_last_seen_at,omitempty"`
	CurrentMissingLastAlertAt        string `json:"current_missing_last_alert_at,omitempty"`
	CurrentMissingLastAlertReason    string `json:"current_missing_last_alert_reason,omitempty"`
	CurrentMissingLastRelaunchAt     string `json:"current_missing_last_relaunch_at,omitempty"`
	CurrentMissingLastRelaunchResult string `json:"current_missing_last_relaunch_result,omitempty"`
	CurrentMissingLastRelaunchError  string `json:"current_missing_last_relaunch_error,omitempty"`
	CurrentMissingRelaunchAttempts   int    `json:"current_missing_relaunch_attempts,omitempty"`
}

func TransportRecoveryPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "transport-recovery.json")
}

func LoadTransportRecovery(path string) (*TransportRecoveryState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &TransportRecoveryState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		state.Targets = map[string]TransportRecoveryTarget{}
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse transport recovery: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Targets == nil {
		state.Targets = map[string]TransportRecoveryTarget{}
	}
	return state, nil
}

func SaveTransportRecovery(path string, state *TransportRecoveryState) error {
	if state == nil {
		return fmt.Errorf("transport recovery is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Targets == nil {
		state.Targets = map[string]TransportRecoveryTarget{}
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(path, state)
}

func ensureTransportRecovery(runDir string) error {
	if _, err := LoadTransportRecovery(TransportRecoveryPath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return SaveTransportRecovery(TransportRecoveryPath(runDir), &TransportRecoveryState{
			Version: 1,
			Targets: map[string]TransportRecoveryTarget{},
		})
	}
	return nil
}

func loadTransportRecoveryTarget(runDir, target string) TransportRecoveryTarget {
	state, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil || state == nil || state.Targets == nil {
		return TransportRecoveryTarget{}
	}
	return state.Targets[target]
}

func updateTransportRecoveryTarget(runDir, target string, mutate func(*TransportRecoveryTarget)) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	state, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil {
		return err
	}
	entry := state.Targets[target]
	entry.Target = target
	mutate(&entry)
	if strings.TrimSpace(entry.Target) == "" {
		delete(state.Targets, target)
	} else {
		state.Targets[target] = entry
	}
	state.UpdatedAt = ""
	return SaveTransportRecovery(TransportRecoveryPath(runDir), state)
}

func recordWakeSubmit(runDir, target string, outcome TransportDeliveryOutcome) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		switch strings.TrimSpace(outcome.SubmitMode) {
		case "enter_only_repair":
			entry.LastEnterRepairAt = now
		case "payload_enter", "payload_then_enter":
			entry.LastWakeSubmitAt = now
		}
	})
}

func recordInterruptEscalation(runDir, target, reason string, outcome TransportDeliveryOutcome) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		entry.LastInterruptEscalationAt = now
		entry.LastInterruptReason = strings.TrimSpace(reason)
		entry.LastInterruptResultingState = strings.TrimSpace(outcome.TransportState)
		entry.UrgentEscalationAttempts++
	})
}

func resetUrgentEscalationAttempts(runDir, target string) error {
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		entry.UrgentEscalationAttempts = 0
	})
}

func recordTargetPresenceObservation(runDir string, facts TargetPresenceFacts) error {
	target := strings.TrimSpace(facts.Target)
	if target == "" {
		return nil
	}
	now := presenceObservationTime(facts)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		if targetPresenceMissing(facts) {
			state := strings.TrimSpace(facts.State)
			if entry.CurrentMissingState != state {
				entry.CurrentMissingState = state
				entry.CurrentMissingFirstSeenAt = now
				entry.CurrentMissingLastAlertAt = ""
				entry.CurrentMissingLastAlertReason = ""
				entry.CurrentMissingLastRelaunchAt = ""
				entry.CurrentMissingLastRelaunchResult = ""
				entry.CurrentMissingLastRelaunchError = ""
				entry.CurrentMissingRelaunchAttempts = 0
			}
			entry.CurrentMissingLastSeenAt = now
			return
		}
		entry.CurrentMissingState = ""
		entry.CurrentMissingFirstSeenAt = ""
		entry.CurrentMissingLastSeenAt = ""
		entry.CurrentMissingLastAlertAt = ""
		entry.CurrentMissingLastAlertReason = ""
		entry.CurrentMissingLastRelaunchAt = ""
		entry.CurrentMissingLastRelaunchResult = ""
		entry.CurrentMissingLastRelaunchError = ""
		entry.CurrentMissingRelaunchAttempts = 0
	})
}

func recordMissingTargetAlert(runDir, target, missingState, reason string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		if entry.CurrentMissingState == "" {
			entry.CurrentMissingState = strings.TrimSpace(missingState)
		}
		entry.CurrentMissingLastSeenAt = now
		entry.CurrentMissingLastAlertAt = now
		entry.CurrentMissingLastAlertReason = strings.TrimSpace(reason)
	})
}

func recordMissingTargetRelaunchAttempt(runDir, target, missingState string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		if entry.CurrentMissingState == "" {
			entry.CurrentMissingState = strings.TrimSpace(missingState)
		}
		entry.CurrentMissingLastSeenAt = now
		entry.CurrentMissingLastRelaunchAt = now
		entry.CurrentMissingRelaunchAttempts++
	})
}

func recordMissingTargetRelaunchResult(runDir, target, missingState, result string, relaunchErr error) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		if entry.CurrentMissingState == "" {
			entry.CurrentMissingState = strings.TrimSpace(missingState)
		}
		entry.CurrentMissingLastSeenAt = now
		entry.CurrentMissingLastRelaunchAt = now
		entry.CurrentMissingLastRelaunchResult = strings.TrimSpace(result)
		if relaunchErr != nil {
			entry.CurrentMissingLastRelaunchError = relaunchErr.Error()
		} else {
			entry.CurrentMissingLastRelaunchError = ""
		}
	})
}

func missingTargetRelaunchBackoff(interval time.Duration, attempts int) time.Duration {
	if interval <= 0 {
		interval = missingTargetRelaunchMinBackoff
	}
	if interval < missingTargetRelaunchMinBackoff {
		interval = missingTargetRelaunchMinBackoff
	}
	if attempts <= 1 {
		return interval
	}
	backoff := interval
	for i := 1; i < attempts; i++ {
		backoff *= 2
		if backoff >= missingTargetRelaunchMaxBackoff {
			return missingTargetRelaunchMaxBackoff
		}
	}
	if backoff > missingTargetRelaunchMaxBackoff {
		return missingTargetRelaunchMaxBackoff
	}
	return backoff
}

func missingTargetRelaunchReady(entry TransportRecoveryTarget, now time.Time, interval time.Duration) bool {
	if strings.TrimSpace(entry.CurrentMissingLastRelaunchAt) == "" {
		return true
	}
	lastAttemptAt, err := time.Parse(time.RFC3339, entry.CurrentMissingLastRelaunchAt)
	if err != nil {
		return true
	}
	backoff := missingTargetRelaunchBackoff(interval, entry.CurrentMissingRelaunchAttempts)
	return !now.Before(lastAttemptAt.Add(backoff))
}

func recordTargetAttentionObservation(runDir string, facts TargetAttentionFacts) error {
	target := strings.TrimSpace(facts.Target)
	if target == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		state := strings.TrimSpace(facts.AttentionState)
		if state == "" || state == TargetAttentionHealthy || state == TargetAttentionNeedsAttention {
			entry.CurrentAttentionState = ""
			entry.CurrentAttentionFirstSeenAt = ""
			entry.CurrentAttentionLastSeenAt = ""
			entry.CurrentAttentionLastAlertAt = ""
			entry.CurrentAttentionLastAlertReason = ""
			return
		}
		if entry.CurrentAttentionState != state {
			entry.CurrentAttentionState = state
			entry.CurrentAttentionFirstSeenAt = now
			entry.CurrentAttentionLastAlertAt = ""
			entry.CurrentAttentionLastAlertReason = ""
		}
		entry.CurrentAttentionLastSeenAt = now
	})
}

func recordTargetAttentionAlert(runDir, target, state, reason string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	return updateTransportRecoveryTarget(runDir, target, func(entry *TransportRecoveryTarget) {
		if entry.CurrentAttentionState == "" {
			entry.CurrentAttentionState = strings.TrimSpace(state)
			entry.CurrentAttentionFirstSeenAt = now
		}
		entry.CurrentAttentionLastSeenAt = now
		entry.CurrentAttentionLastAlertAt = now
		entry.CurrentAttentionLastAlertReason = strings.TrimSpace(reason)
	})
}

func presenceObservationTime(facts TargetPresenceFacts) string {
	if ts := strings.TrimSpace(facts.CheckedAt); ts != "" {
		return ts
	}
	return time.Now().UTC().Format(time.RFC3339)
}
