package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type CoordinationState struct {
	Version       int                                 `json:"version"`
	PlanSummary   []string                            `json:"plan_summary,omitempty"`
	Required      map[string]CoordinationRequiredItem `json:"required,omitempty"`
	Sessions      map[string]CoordinationSession      `json:"sessions,omitempty"`
	Decision      *CoordinationDecision               `json:"decision,omitempty"`
	OpenQuestions []string                            `json:"open_questions,omitempty"`
	UpdatedAt     string                              `json:"updated_at,omitempty"`
}

type CoordinationRequiredItem struct {
	Owner          string                       `json:"owner,omitempty"`
	ExecutionState string                       `json:"execution_state,omitempty"`
	BlockedBy      string                       `json:"blocked_by,omitempty"`
	Surfaces       CoordinationRequiredSurfaces `json:"surfaces"`
	UpdatedAt      string                       `json:"updated_at,omitempty"`
}

type CoordinationRequiredSurfaces struct {
	Repo           string `json:"repo,omitempty"`
	Runtime        string `json:"runtime,omitempty"`
	RunArtifacts   string `json:"run_artifacts,omitempty"`
	WebResearch    string `json:"web_research,omitempty"`
	ExternalSystem string `json:"external_system,omitempty"`
}

type CoordinationSession struct {
	State              string                    `json:"state,omitempty"`
	Scope              string                    `json:"scope,omitempty"`
	DispatchableSlices []goalx.DispatchableSlice `json:"dispatchable_slices,omitempty"`
	LastRound          int                       `json:"last_round,omitempty"`
	UpdatedAt          string                    `json:"updated_at,omitempty"`
}

type CoordinationDecision struct {
	RootCause        string `json:"root_cause,omitempty"`
	LocalPath        string `json:"local_path,omitempty"`
	CompatiblePath   string `json:"compatible_path,omitempty"`
	ArchitecturePath string `json:"architecture_path,omitempty"`
	ChosenPath       string `json:"chosen_path,omitempty"`
	ChosenPathReason string `json:"chosen_path_reason,omitempty"`
}

const (
	coordinationRequiredExecutionStateActive  = "active"
	coordinationRequiredExecutionStateProbing = "probing"
	coordinationRequiredExecutionStateWaiting = "waiting"
	coordinationRequiredExecutionStateBlocked = "blocked"

	coordinationRequiredSurfacePending       = "pending"
	coordinationRequiredSurfaceActive        = "active"
	coordinationRequiredSurfaceAvailable     = "available"
	coordinationRequiredSurfaceExhausted     = "exhausted"
	coordinationRequiredSurfaceUnreachable   = "unreachable"
	coordinationRequiredSurfaceNotApplicable = "not_applicable"
)

func CoordinationPath(runDir string) string {
	return filepath.Join(runDir, "coordination.json")
}

func LoadCoordinationState(path string) (*CoordinationState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseCoordinationState(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return state, nil
}

func SaveCoordinationState(path string, state *CoordinationState) error {
	if err := validateCoordinationState(state); err != nil {
		return err
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func EnsureCoordinationState(runDir, objective string) (*CoordinationState, error) {
	path := CoordinationPath(runDir)
	state, err := LoadCoordinationState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = &CoordinationState{
			Version:   1,
			Required:  map[string]CoordinationRequiredItem{},
			Sessions:  map[string]CoordinationSession{},
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := SaveCoordinationState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	return state, nil
}

func parseCoordinationState(data []byte) (*CoordinationState, error) {
	var state CoordinationState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCoordination, err)
	}
	if err := validateCoordinationState(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCoordination, err)
	}
	return &state, nil
}

// normalizeCoordinationState ensures structural consistency without
// truncating or modifying master-written content.
func normalizeCoordinationState(state *CoordinationState) {
	if state == nil {
		return
	}
	if state.Required == nil {
		state.Required = map[string]CoordinationRequiredItem{}
	}
	if state.Sessions == nil {
		state.Sessions = map[string]CoordinationSession{}
	}
}

func validateCoordinationState(state *CoordinationState) error {
	if state == nil {
		return fmt.Errorf("coordination state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("coordination state version must be positive")
	}
	normalizeCoordinationState(state)
	for reqID, item := range state.Required {
		if err := validateCoordinationRequiredItem(reqID, item); err != nil {
			return err
		}
	}
	return nil
}

func validateCoordinationRequiredItem(reqID string, item CoordinationRequiredItem) error {
	if strings.TrimSpace(reqID) == "" {
		return fmt.Errorf("coordination required item id must be non-empty")
	}
	if strings.TrimSpace(item.Owner) == "" {
		return fmt.Errorf("coordination required item %q missing owner", reqID)
	}
	switch item.ExecutionState {
	case coordinationRequiredExecutionStateActive, coordinationRequiredExecutionStateProbing, coordinationRequiredExecutionStateWaiting, coordinationRequiredExecutionStateBlocked:
	default:
		return fmt.Errorf("coordination required item %q has invalid execution_state %q", reqID, item.ExecutionState)
	}
	if strings.TrimSpace(item.BlockedBy) != "" && item.ExecutionState != coordinationRequiredExecutionStateWaiting && item.ExecutionState != coordinationRequiredExecutionStateBlocked {
		return fmt.Errorf("coordination required item %q blocked_by requires waiting or blocked execution_state", reqID)
	}
	if err := validateCoordinationRequiredSurface("repo", reqID, item.Surfaces.Repo); err != nil {
		return err
	}
	if err := validateCoordinationRequiredSurface("runtime", reqID, item.Surfaces.Runtime); err != nil {
		return err
	}
	if err := validateCoordinationRequiredSurface("run_artifacts", reqID, item.Surfaces.RunArtifacts); err != nil {
		return err
	}
	if err := validateCoordinationRequiredSurface("web_research", reqID, item.Surfaces.WebResearch); err != nil {
		return err
	}
	if err := validateCoordinationRequiredSurface("external_system", reqID, item.Surfaces.ExternalSystem); err != nil {
		return err
	}
	return nil
}

func validateCoordinationRequiredSurface(name, reqID, value string) error {
	switch value {
	case coordinationRequiredSurfacePending,
		coordinationRequiredSurfaceActive,
		coordinationRequiredSurfaceAvailable,
		coordinationRequiredSurfaceExhausted,
		coordinationRequiredSurfaceUnreachable,
		coordinationRequiredSurfaceNotApplicable:
		return nil
	default:
		return fmt.Errorf("coordination required item %q has invalid %s surface state %q", reqID, name, value)
	}
}

func coordinationRequiredSurfacesExhausted(surfaces CoordinationRequiredSurfaces) bool {
	values := []string{
		surfaces.Repo,
		surfaces.Runtime,
		surfaces.RunArtifacts,
		surfaces.WebResearch,
		surfaces.ExternalSystem,
	}
	for _, value := range values {
		switch value {
		case coordinationRequiredSurfaceExhausted, coordinationRequiredSurfaceUnreachable, coordinationRequiredSurfaceNotApplicable:
		default:
			return false
		}
	}
	return true
}
