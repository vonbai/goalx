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
	goalContractKindUserRequired    = "user_required"
	goalContractKindGoalNecessary   = "goal_necessary"
	goalContractKindGoalEnhancement = "goal_enhancement"

	goalContractStatusQueued     = "queued"
	goalContractStatusDelegated  = "delegated"
	goalContractStatusInProgress = "in_progress"
	goalContractStatusBlocked    = "blocked"
	goalContractStatusDone       = "done"
	goalContractStatusWaived     = "waived"

	goalContractSummaryPending   = "pending"
	goalContractSummarySatisfied = "satisfied"

	goalSatisfactionPreexisting = "preexisting"
	goalSatisfactionRunChange   = "run_change"
	goalSatisfactionMixed       = "mixed"

	goalContractExecutionStateActive          = "active"
	goalContractExecutionStateWaitingExternal = "waiting_external"
	goalContractExecutionStateDispatchable    = "dispatchable"
)

type GoalContractState struct {
	Version   int                `json:"version"`
	Objective string             `json:"objective,omitempty"`
	UpdatedAt string             `json:"updated_at,omitempty"`
	Items     []GoalContractItem `json:"items"`
}

type GoalContractItem struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	Source            string   `json:"source,omitempty"`
	Requirement       string   `json:"requirement"`
	Status            string   `json:"status"`
	ExecutionState    string   `json:"execution_state,omitempty"`
	SatisfactionBasis string   `json:"satisfaction_basis,omitempty"`
	Owner             string   `json:"owner,omitempty"`
	Notes             string   `json:"notes,omitempty"`
	Evidence          []string `json:"evidence,omitempty"`
	UserApproved      bool     `json:"user_approved,omitempty"`
}

type GoalContractSummary struct {
	Status            string
	Total             int
	RequiredTotal     int
	RequiredDone      int
	RequiredRemaining int
	EnhancementOpen   int
}

type GoalDispatchSummary struct {
	Total             int
	RequiredTotal     int
	RequiredDone      int
	RequiredRemaining int
	Blocked           int
	WaitingExternal   int
	Dispatchable      int
}

func GoalContractPath(runDir string) string {
	return filepath.Join(runDir, "goal-contract.json")
}

func NewGoalContractState(objective string) *GoalContractState {
	return &GoalContractState{
		Version:   1,
		Objective: objective,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Items:     []GoalContractItem{},
	}
}

func LoadGoalContractState(path string) (*GoalContractState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state GoalContractState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &state, nil
}

func SaveGoalContractState(path string, state *GoalContractState) error {
	if state == nil {
		return fmt.Errorf("goal contract state is nil")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EnsureGoalContractState(runDir, objective string) (*GoalContractState, error) {
	path := GoalContractPath(runDir)
	state, err := LoadGoalContractState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = NewGoalContractState(objective)
		if err := SaveGoalContractState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}

	changed := false
	if state.Version <= 0 {
		state.Version = 1
		changed = true
	}
	if state.Objective == "" {
		state.Objective = objective
		changed = true
	}
	if state.Items == nil {
		state.Items = []GoalContractItem{}
		changed = true
	}
	for i := range state.Items {
		if strings.TrimSpace(state.Items[i].Kind) == "" {
			state.Items[i].Kind = goalContractKindUserRequired
			changed = true
		}
		if strings.TrimSpace(state.Items[i].Status) == "" {
			state.Items[i].Status = goalContractStatusQueued
			changed = true
		}
	}
	if state.UpdatedAt == "" || changed {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		changed = true
	}
	if changed {
		if err := SaveGoalContractState(path, state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func SummarizeGoalContract(state *GoalContractState) GoalContractSummary {
	summary := GoalContractSummary{Status: goalContractSummaryPending}
	if state == nil {
		return summary
	}
	summary.Total = len(state.Items)
	for _, item := range state.Items {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			kind = goalContractKindUserRequired
		}
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = goalContractStatusQueued
		}
		if isRequiredGoalContractKind(kind) {
			summary.RequiredTotal++
			if status == goalContractStatusDone || (status == goalContractStatusWaived && item.UserApproved) {
				summary.RequiredDone++
			} else {
				summary.RequiredRemaining++
			}
			continue
		}
		if kind == goalContractKindGoalEnhancement && status != goalContractStatusDone && status != goalContractStatusWaived {
			summary.EnhancementOpen++
		}
	}
	if summary.RequiredTotal > 0 && summary.RequiredRemaining == 0 {
		summary.Status = goalContractSummarySatisfied
	}
	return summary
}

func SummarizeGoalDispatch(state *GoalContractState) GoalDispatchSummary {
	summary := GoalDispatchSummary{}
	if state == nil {
		return summary
	}
	summary.Total = len(state.Items)
	for _, item := range state.Items {
		kind := strings.TrimSpace(item.Kind)
		if kind == "" {
			kind = goalContractKindUserRequired
		}
		if !isRequiredGoalContractKind(kind) {
			continue
		}
		summary.RequiredTotal++
		status := strings.TrimSpace(item.Status)
		if status == "" {
			status = goalContractStatusQueued
		}
		if status == goalContractStatusDone || (status == goalContractStatusWaived && item.UserApproved) {
			summary.RequiredDone++
			continue
		}
		summary.RequiredRemaining++
		if status == goalContractStatusBlocked {
			summary.Blocked++
			continue
		}
		switch normalizeGoalExecutionState(item.ExecutionState) {
		case goalContractExecutionStateWaitingExternal:
			summary.WaitingExternal++
		case goalContractExecutionStateDispatchable, goalContractExecutionStateActive:
			summary.Dispatchable++
		default:
			summary.Dispatchable++
		}
	}
	return summary
}

func (s GoalDispatchSummary) HasDispatchableWork() bool {
	return s.Dispatchable > 0
}

func normalizeGoalExecutionState(state string) string {
	switch strings.TrimSpace(state) {
	case goalContractExecutionStateActive:
		return goalContractExecutionStateActive
	case goalContractExecutionStateWaitingExternal:
		return goalContractExecutionStateWaitingExternal
	case goalContractExecutionStateDispatchable:
		return goalContractExecutionStateDispatchable
	default:
		return goalContractExecutionStateDispatchable
	}
}

func ValidateGoalContractForCompletion(state *GoalContractState) (GoalContractSummary, error) {
	summary := SummarizeGoalContract(state)
	if summary.RequiredTotal == 0 {
		return summary, fmt.Errorf("goal contract has no required items; enumerate user_required or goal_necessary items before declaring completion")
	}
	if summary.RequiredRemaining > 0 {
		return summary, fmt.Errorf("goal contract still has %d unfinished required item(s)", summary.RequiredRemaining)
	}
	for _, item := range state.Items {
		if !isRequiredGoalContractKind(strings.TrimSpace(item.Kind)) {
			continue
		}
		if strings.TrimSpace(item.Status) != goalContractStatusDone {
			continue
		}
		switch strings.TrimSpace(item.SatisfactionBasis) {
		case goalSatisfactionPreexisting, goalSatisfactionRunChange, goalSatisfactionMixed:
		default:
			return summary, fmt.Errorf("goal contract item %s is done but missing valid satisfaction_basis", item.ID)
		}
		if (strings.TrimSpace(item.SatisfactionBasis) == goalSatisfactionRunChange || strings.TrimSpace(item.SatisfactionBasis) == goalSatisfactionMixed) && len(item.Evidence) == 0 {
			return summary, fmt.Errorf("goal contract item %s claims %s but has no evidence", item.ID, item.SatisfactionBasis)
		}
	}
	return summary, nil
}

func ValidateGoalContractAgainstCompletion(state *GoalContractState, completion *CompletionState) error {
	if state == nil || completion == nil {
		return nil
	}
	for _, item := range state.Items {
		if !isRequiredGoalContractKind(strings.TrimSpace(item.Kind)) {
			continue
		}
		if strings.TrimSpace(item.Status) != goalContractStatusDone {
			continue
		}
		basis := strings.TrimSpace(item.SatisfactionBasis)
		if !completion.CodeChanged && (basis == goalSatisfactionRunChange || basis == goalSatisfactionMixed) {
			return fmt.Errorf("goal contract item %s claims %s but current HEAD is unchanged since run start", item.ID, basis)
		}
	}
	return nil
}

func isRequiredGoalContractKind(kind string) bool {
	switch kind {
	case goalContractKindUserRequired, goalContractKindGoalNecessary:
		return true
	default:
		return false
	}
}
