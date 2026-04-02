package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	freshnessStateFresh         = "fresh"
	freshnessStateStale         = "stale"
	freshnessStateUnknown       = "unknown"
	freshnessStateNotApplicable = "not_applicable"
)

type FreshnessState struct {
	Version   int                      `json:"version"`
	Cognition []CognitionFreshnessItem `json:"cognition,omitempty"`
	Evidence  []EvidenceFreshnessItem  `json:"evidence,omitempty"`
	UpdatedAt string                   `json:"updated_at,omitempty"`
}

type CognitionFreshnessItem struct {
	Scope    string `json:"scope"`
	Provider string `json:"provider"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
}

type EvidenceFreshnessItem struct {
	ScenarioID      string `json:"scenario_id"`
	LatestRevision  string `json:"latest_revision,omitempty"`
	CurrentRevision string `json:"current_revision,omitempty"`
	State           string `json:"state"`
	Reason          string `json:"reason,omitempty"`
}

func FreshnessStatePath(runDir string) string {
	return filepath.Join(runDir, "freshness-state.json")
}

func LoadFreshnessState(path string) (*FreshnessState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseFreshnessState(data)
	if err != nil {
		return nil, fmt.Errorf("parse freshness state: %w", err)
	}
	return state, nil
}

func SaveFreshnessState(path string, state *FreshnessState) error {
	if state == nil {
		return fmt.Errorf("freshness state is nil")
	}
	if err := validateFreshnessStateInput(state); err != nil {
		return err
	}
	normalizeFreshnessState(state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func parseFreshnessState(data []byte) (*FreshnessState, error) {
	var state FreshnessState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceFreshnessState, err)
	}
	if err := validateFreshnessStateInput(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceFreshnessState, err)
	}
	normalizeFreshnessState(&state)
	return &state, nil
}

func validateFreshnessStateInput(state *FreshnessState) error {
	if state == nil {
		return fmt.Errorf("freshness state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("freshness state version must be positive")
	}
	for _, item := range state.Cognition {
		if strings.TrimSpace(item.Scope) == "" {
			return fmt.Errorf("freshness cognition scope is required")
		}
		if strings.TrimSpace(item.Provider) == "" {
			return fmt.Errorf("freshness cognition provider is required")
		}
		if err := validateFreshnessValue(item.State); err != nil {
			return err
		}
	}
	for _, item := range state.Evidence {
		if strings.TrimSpace(item.ScenarioID) == "" {
			return fmt.Errorf("freshness evidence scenario_id is required")
		}
		if err := validateFreshnessValue(item.State); err != nil {
			return err
		}
	}
	return nil
}

func validateFreshnessValue(value string) error {
	switch strings.TrimSpace(value) {
	case freshnessStateFresh, freshnessStateStale, freshnessStateUnknown, freshnessStateNotApplicable:
		return nil
	default:
		return fmt.Errorf("freshness state %q is invalid", value)
	}
}

func normalizeFreshnessState(state *FreshnessState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	state.UpdatedAt = strings.TrimSpace(state.UpdatedAt)
	if state.Cognition == nil {
		state.Cognition = []CognitionFreshnessItem{}
	}
	for i := range state.Cognition {
		state.Cognition[i].Scope = strings.TrimSpace(state.Cognition[i].Scope)
		state.Cognition[i].Provider = strings.TrimSpace(state.Cognition[i].Provider)
		state.Cognition[i].State = strings.TrimSpace(state.Cognition[i].State)
		state.Cognition[i].Reason = strings.TrimSpace(state.Cognition[i].Reason)
	}
	if state.Evidence == nil {
		state.Evidence = []EvidenceFreshnessItem{}
	}
	for i := range state.Evidence {
		state.Evidence[i].ScenarioID = strings.TrimSpace(state.Evidence[i].ScenarioID)
		state.Evidence[i].LatestRevision = strings.TrimSpace(state.Evidence[i].LatestRevision)
		state.Evidence[i].CurrentRevision = strings.TrimSpace(state.Evidence[i].CurrentRevision)
		state.Evidence[i].State = strings.TrimSpace(state.Evidence[i].State)
		state.Evidence[i].Reason = strings.TrimSpace(state.Evidence[i].Reason)
	}
}
