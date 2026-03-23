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

	acceptanceScopeBaseline     = "baseline"
	acceptanceScopeGoalSpecific = "goal_specific"
	acceptanceScopeNarrowed     = "narrowed"
	acceptanceScopeExpanded     = "expanded"
	acceptanceScopeRewritten    = "rewritten"
)

type AcceptanceState struct {
	Version         int    `json:"version"`
	BaselineCommand string `json:"baseline_command,omitempty"`
	BaselineSource  string `json:"baseline_source,omitempty"`
	Command         string `json:"command,omitempty"`
	CommandSource   string `json:"command_source,omitempty"`
	ScopeType       string `json:"scope_type,omitempty"`
	ScopeReason     string `json:"scope_reason,omitempty"`
	ContractVersion int    `json:"contract_version,omitempty"`
	Status          string `json:"status"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	CheckedAt       string `json:"checked_at,omitempty"`
	LastExitCode    *int   `json:"last_exit_code,omitempty"`
	EvidencePath    string `json:"evidence_path,omitempty"`
}

func AcceptanceChecklistPath(runDir string) string {
	return filepath.Join(runDir, "acceptance.md")
}

func AcceptanceStatePath(runDir string) string {
	return filepath.Join(runDir, "acceptance.json")
}

func AcceptanceEvidencePath(runDir string) string {
	return filepath.Join(runDir, "acceptance-last.txt")
}

func NewAcceptanceState(cfg *goalx.Config) *AcceptanceState {
	cmd, source := goalx.ResolveAcceptanceCommandSource(cfg)
	return &AcceptanceState{
		Version:         1,
		BaselineCommand: cmd,
		BaselineSource:  source,
		Command:         cmd,
		CommandSource:   source,
		ScopeType:       acceptanceScopeBaseline,
		Status:          acceptanceStatusPending,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
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
	return &state, nil
}

func SaveAcceptanceState(path string, state *AcceptanceState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	if strings.TrimSpace(state.ScopeType) == "" {
		state.ScopeType = acceptanceScopeBaseline
	}
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

func EnsureAcceptanceState(runDir string, cfg *goalx.Config) (*AcceptanceState, error) {
	path := AcceptanceStatePath(runDir)
	state, err := LoadAcceptanceState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = NewAcceptanceState(cfg)
		if err := SaveAcceptanceState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}

	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Status == "" {
		state.Status = acceptanceStatusPending
	}
	if state.BaselineCommand == "" {
		state.BaselineCommand, state.BaselineSource = goalx.ResolveAcceptanceCommandSource(cfg)
		if state.BaselineCommand == "" {
			state.BaselineCommand = state.Command
			state.BaselineSource = state.CommandSource
		}
	}
	if state.Command == "" {
		state.Command, state.CommandSource = goalx.ResolveAcceptanceCommandSource(cfg)
	}
	if state.CommandSource == "" {
		state.CommandSource = state.BaselineSource
	}
	if state.ScopeType == "" {
		if strings.TrimSpace(state.Command) == strings.TrimSpace(state.BaselineCommand) {
			state.ScopeType = acceptanceScopeBaseline
		}
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := SaveAcceptanceState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}

func ValidateAcceptanceStateForVerification(state *AcceptanceState, contract *GoalContractState) error {
	if state == nil {
		return fmt.Errorf("acceptance state is nil")
	}
	command := strings.TrimSpace(state.Command)
	if command == "" {
		return fmt.Errorf("no acceptance command configured")
	}
	baseline := strings.TrimSpace(state.BaselineCommand)
	if baseline == "" {
		baseline = command
	}
	if command == baseline {
		return nil
	}
	scopeType := strings.TrimSpace(state.ScopeType)
	switch scopeType {
	case acceptanceScopeGoalSpecific, acceptanceScopeNarrowed, acceptanceScopeExpanded, acceptanceScopeRewritten:
	default:
		return fmt.Errorf("acceptance command differs from baseline but scope_type is missing or invalid")
	}
	if strings.TrimSpace(state.ScopeReason) == "" {
		return fmt.Errorf("acceptance command differs from baseline but scope_reason is empty")
	}
	if contract != nil && contract.Version > 0 && state.ContractVersion > 0 && state.ContractVersion != contract.Version {
		return fmt.Errorf("acceptance scope targets contract version %d but current goal contract is version %d", state.ContractVersion, contract.Version)
	}
	if contract != nil && contract.Version > 0 && state.ContractVersion == 0 {
		return fmt.Errorf("acceptance command differs from baseline but contract_version is missing")
	}
	return nil
}

func updateStatusWithAcceptance(statusPath string, state *AcceptanceState, contractSummary GoalContractSummary, completion *CompletionState) error {
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

	payload["acceptance_status"] = state.Status
	payload["acceptance_baseline_command"] = state.BaselineCommand
	payload["acceptance_baseline_source"] = state.BaselineSource
	payload["acceptance_command"] = state.Command
	payload["acceptance_command_source"] = state.CommandSource
	payload["acceptance_scope_type"] = state.ScopeType
	payload["acceptance_scope_reason"] = state.ScopeReason
	payload["acceptance_contract_version"] = state.ContractVersion
	payload["acceptance_checked_at"] = state.CheckedAt
	if state.LastExitCode != nil {
		payload["acceptance_exit_code"] = *state.LastExitCode
	}
	if state.EvidencePath != "" {
		payload["acceptance_evidence_path"] = state.EvidencePath
	}
	payload["goal_contract_status"] = contractSummary.Status
	payload["goal_required_total"] = contractSummary.RequiredTotal
	payload["goal_required_done"] = contractSummary.RequiredDone
	payload["goal_required_remaining"] = contractSummary.RequiredRemaining
	payload["goal_enhancement_open"] = contractSummary.EnhancementOpen
	if completion != nil {
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

func updateRunVerificationState(projectRoot, runDir string, cfg *goalx.Config, state *AcceptanceState, contractSummary GoalContractSummary, completion *CompletionState) error {
	runtimeState, err := EnsureRuntimeState(runDir, cfg)
	if err != nil {
		return err
	}
	runtimeState.AcceptanceMet = state != nil && state.Status == acceptanceStatusPassed
	if state != nil {
		runtimeState.AcceptanceStatus = state.Status
		runtimeState.AcceptanceCheckedAt = state.CheckedAt
		runtimeState.AcceptanceEvidencePath = state.EvidencePath
	}
	runtimeState.GoalContractStatus = contractSummary.Status
	runtimeState.GoalRequiredTotal = contractSummary.RequiredTotal
	runtimeState.GoalRequiredDone = contractSummary.RequiredDone
	runtimeState.GoalRequiredRemain = contractSummary.RequiredRemaining
	runtimeState.GoalEnhancementOpen = contractSummary.EnhancementOpen
	if completion != nil {
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
