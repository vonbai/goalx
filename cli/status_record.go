package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	runStatusPhaseWorking  = "working"
	runStatusPhaseReview   = "review"
	runStatusPhaseComplete = "complete"
)

type RunStatusRecord struct {
	Version           int      `json:"version"`
	Phase             string   `json:"phase"`
	RequiredRemaining *int     `json:"required_remaining"`
	OpenRequiredIDs   []string `json:"open_required_ids,omitempty"`
	ActiveSessions    []string `json:"active_sessions,omitempty"`
	KeepSession       string   `json:"keep_session,omitempty"`
	LastVerifiedAt    string   `json:"last_verified_at,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
}

func LoadRunStatusRecord(path string) (*RunStatusRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	record, err := parseRunStatusRecord(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return record, nil
}

func SaveRunStatusRecord(path string, record *RunStatusRecord) error {
	if err := validateRunStatusRecord(record); err != nil {
		return err
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseRunStatusRecord(data []byte) (*RunStatusRecord, error) {
	var record RunStatusRecord
	if err := decodeStrictJSON(data, &record); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceStatus, err)
	}
	if err := validateRunStatusRecord(&record); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceStatus, err)
	}
	return &record, nil
}

func validateRunStatusRecord(record *RunStatusRecord) error {
	if record == nil {
		return fmt.Errorf("run status record is nil")
	}
	if record.Version <= 0 {
		return fmt.Errorf("run status record version must be positive")
	}
	switch strings.TrimSpace(record.Phase) {
	case runStatusPhaseWorking, runStatusPhaseReview, runStatusPhaseComplete:
	default:
		return fmt.Errorf("invalid run status phase %q", record.Phase)
	}
	if record.RequiredRemaining == nil {
		return fmt.Errorf("run status record missing required_remaining")
	}
	if *record.RequiredRemaining < 0 {
		return fmt.Errorf("run status record required_remaining must be non-negative")
	}
	return nil
}
