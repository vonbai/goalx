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

const (
	acceptanceStatusPending = "pending"
	acceptanceStatusPassed  = "passed"
	acceptanceStatusFailed  = "failed"

	acceptanceChangeSame      = "same"
	acceptanceChangeExpanded  = "expanded"
	acceptanceChangeRewritten = "rewritten"
	acceptanceChangeNarrowed  = "narrowed"
)

type AcceptanceResult struct {
	Status       string `json:"status,omitempty"`
	CheckedAt    string `json:"checked_at,omitempty"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	EvidencePath string `json:"evidence_path,omitempty"`
}

type AcceptanceState struct {
	Version          int              `json:"version"`
	GoalVersion      int              `json:"goal_version,omitempty"`
	DefaultCommand   string           `json:"default_command,omitempty"`
	EffectiveCommand string           `json:"effective_command,omitempty"`
	ChangeKind       string           `json:"change_kind,omitempty"`
	ChangeReason     string           `json:"change_reason,omitempty"`
	UserApproved     bool             `json:"user_approved,omitempty"`
	LastResult       AcceptanceResult `json:"last_result,omitempty"`
	UpdatedAt        string           `json:"updated_at,omitempty"`
}

func AcceptanceNotesPath(runDir string) string {
	return filepath.Join(runDir, "acceptance.md")
}

func AcceptanceStatePath(runDir string) string {
	return filepath.Join(runDir, "acceptance.json")
}

func AcceptanceEvidencePath(runDir string) string {
	return filepath.Join(runDir, "acceptance-last.txt")
}

func NewAcceptanceState(cfg *goalx.Config, goalVersion int) *AcceptanceState {
	cmd, _ := goalx.ResolveAcceptanceCommandSource(cfg)
	return &AcceptanceState{
		Version:          1,
		GoalVersion:      goalVersion,
		DefaultCommand:   cmd,
		EffectiveCommand: cmd,
		ChangeKind:       acceptanceChangeSame,
		LastResult: AcceptanceResult{
			Status: acceptanceStatusPending,
		},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func LoadAcceptanceState(path string) (*AcceptanceState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state AcceptanceState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	normalizeAcceptanceState(&state)
	return &state, nil
}

func SaveAcceptanceState(path string, state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	normalizeAcceptanceState(state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EnsureAcceptanceState(runDir string, cfg *goalx.Config, goalVersion int) (*AcceptanceState, error) {
	path := AcceptanceStatePath(runDir)
	state, err := LoadAcceptanceState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = NewAcceptanceState(cfg, goalVersion)
		if err := SaveAcceptanceState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}

	defaultCommand, _ := goalx.ResolveAcceptanceCommandSource(cfg)
	if strings.TrimSpace(state.DefaultCommand) == "" {
		state.DefaultCommand = defaultCommand
	}
	if strings.TrimSpace(state.EffectiveCommand) == "" {
		state.EffectiveCommand = state.DefaultCommand
	}
	if state.GoalVersion <= 0 {
		state.GoalVersion = goalVersion
	}
	normalizeAcceptanceState(state)
	if err := SaveAcceptanceState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}

func ValidateAcceptanceStateForVerification(state *AcceptanceState, goal *GoalState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	if strings.TrimSpace(state.EffectiveCommand) == "" {
		return fmt.Errorf("no acceptance command configured")
	}
	if goal != nil && goal.Version > 0 && state.GoalVersion != goal.Version {
		return fmt.Errorf("acceptance goal_version=%d but goal.json version is %d", state.GoalVersion, goal.Version)
	}

	if strings.TrimSpace(state.DefaultCommand) == strings.TrimSpace(state.EffectiveCommand) {
		if state.ChangeKind != acceptanceChangeSame {
			return fmt.Errorf("acceptance change_kind must be %q when effective_command matches default_command", acceptanceChangeSame)
		}
		return nil
	}

	switch state.ChangeKind {
	case acceptanceChangeExpanded, acceptanceChangeRewritten, acceptanceChangeNarrowed:
	default:
		return fmt.Errorf("acceptance command differs from default_command but change_kind is missing or invalid")
	}
	if strings.TrimSpace(state.ChangeReason) == "" {
		return fmt.Errorf("acceptance command differs from default_command but change_reason is empty")
	}
	if state.ChangeKind == acceptanceChangeNarrowed && !state.UserApproved {
		return fmt.Errorf("narrowed acceptance gate requires explicit user approval")
	}
	return nil
}

func acceptanceStatus(state *AcceptanceState) string {
	if state == nil {
		return ""
	}
	if strings.TrimSpace(state.LastResult.Status) == "" {
		return acceptanceStatusPending
	}
	return state.LastResult.Status
}

func updateStatusWithAcceptance(statusPath string, state *AcceptanceState, summary GoalSummary, completion *CompletionState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}

	payload := map[string]any{}
	if data, err := os.ReadFile(statusPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &payload); err != nil {
			return fmt.Errorf("parse status %s: %w", statusPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	payload["acceptance_status"] = acceptanceStatus(state)
	payload["acceptance_checked_at"] = state.LastResult.CheckedAt
	payload["acceptance_evidence_path"] = state.LastResult.EvidencePath
	payload["acceptance_default_command"] = state.DefaultCommand
	payload["acceptance_effective_command"] = state.EffectiveCommand
	payload["acceptance_change_kind"] = state.ChangeKind
	if state.ChangeReason != "" {
		payload["acceptance_change_reason"] = state.ChangeReason
	}
	payload["goal_version"] = summary.Version
	payload["required_total"] = summary.RequiredTotal
	payload["required_satisfied"] = summary.RequiredSatisfied
	payload["required_remaining"] = summary.RequiredRemaining
	payload["optional_open"] = summary.OptionalOpen
	if completion != nil {
		payload["goal_satisfied"] = completion.GoalSatisfied
		payload["completion_mode"] = completion.CompletionMode
		payload["code_changed"] = completion.CodeChanged
		payload["base_revision"] = completion.BaseRevision
		payload["head_revision"] = completion.HeadRevision
		payload["changed_files_count"] = len(completion.ChangedFiles)
		if completion.KeptSession != "" {
			payload["kept_session"] = completion.KeptSession
		}
		if completion.KeptBranch != "" {
			payload["kept_branch"] = completion.KeptBranch
		}
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(statusPath, data, 0o644)
}

func updateRunVerificationState(projectRoot, runDir string, cfg *goalx.Config, state *AcceptanceState, summary GoalSummary, completion *CompletionState) error {
	runtimeState, err := EnsureRuntimeState(runDir, cfg)
	if err != nil {
		return err
	}
	runtimeState.AcceptanceMet = state != nil && acceptanceStatus(state) == acceptanceStatusPassed
	if state != nil {
		runtimeState.AcceptanceStatus = acceptanceStatus(state)
		runtimeState.AcceptanceCheckedAt = state.LastResult.CheckedAt
		runtimeState.AcceptanceEvidencePath = state.LastResult.EvidencePath
	}
	runtimeState.GoalVersion = summary.Version
	runtimeState.RequiredTotal = summary.RequiredTotal
	runtimeState.RequiredSatisfied = summary.RequiredSatisfied
	runtimeState.RequiredRemaining = summary.RequiredRemaining
	runtimeState.OptionalOpen = summary.OptionalOpen
	if completion != nil {
		runtimeState.GoalSatisfied = completion.GoalSatisfied
		runtimeState.CompletionMode = completion.CompletionMode
		runtimeState.CodeChanged = completion.CodeChanged
		runtimeState.BaseRevision = completion.BaseRevision
		runtimeState.HeadRevision = completion.HeadRevision
	}
	runtimeState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), runtimeState); err != nil {
		return err
	}
	return syncProjectStatusCache(projectRoot, runtimeState)
}

func normalizeAcceptanceState(state *AcceptanceState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	if strings.TrimSpace(state.EffectiveCommand) == "" {
		state.EffectiveCommand = strings.TrimSpace(state.DefaultCommand)
	}
	if strings.TrimSpace(state.DefaultCommand) == "" {
		state.DefaultCommand = strings.TrimSpace(state.EffectiveCommand)
	}
	if strings.TrimSpace(state.ChangeKind) == "" {
		if strings.TrimSpace(state.EffectiveCommand) == strings.TrimSpace(state.DefaultCommand) {
			state.ChangeKind = acceptanceChangeSame
		}
	}
	if strings.TrimSpace(state.LastResult.Status) == "" {
		state.LastResult.Status = acceptanceStatusPending
	}
}
