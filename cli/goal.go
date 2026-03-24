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
	goalItemStateOpen    = "open"
	goalItemStateClaimed = "claimed"
	goalItemStateWaived  = "waived"

	goalItemSourceUser   = "user"
	goalItemSourceMaster = "master"
)

type GoalState struct {
	Version   int        `json:"version"`
	UpdatedAt string     `json:"updated_at,omitempty"`
	Required  []GoalItem `json:"required,omitempty"`
	Optional  []GoalItem `json:"optional,omitempty"`
}

type GoalItem struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Source        string   `json:"source,omitempty"`
	State         string   `json:"state,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	Note          string   `json:"note,omitempty"`
	UserApproved  bool     `json:"user_approved,omitempty"`
}

type GoalSummary struct {
	Version           int
	RequiredTotal     int
	RequiredSatisfied int
	RequiredRemaining int
	OptionalOpen      int
}

func GoalPath(runDir string) string {
	return filepath.Join(runDir, "goal.json")
}

func GoalLogPath(runDir string) string {
	return filepath.Join(runDir, "goal-log.jsonl")
}

func LoadGoalState(path string) (*GoalState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var state GoalState
	if len(strings.TrimSpace(string(data))) == 0 {
		state = *NewGoalState()
		return &state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse goal state: %w", err)
	}
	normalizeGoalState(&state)
	return &state, nil
}

func SaveGoalState(path string, state *GoalState) error {
	if state == nil {
		return fmt.Errorf("goal state is nil")
	}
	normalizeGoalState(state)
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

func NewGoalState() *GoalState {
	state := &GoalState{
		Version:   1,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Required:  []GoalItem{},
		Optional:  []GoalItem{},
	}
	normalizeGoalState(state)
	return state
}

func EnsureGoalState(runDir string) (*GoalState, error) {
	path := GoalPath(runDir)
	state, err := LoadGoalState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = NewGoalState()
		if err := SaveGoalState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	if err := SaveGoalState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}

func EnsureGoalLog(runDir string) error {
	path := GoalLogPath(runDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

func SummarizeGoalState(state *GoalState) GoalSummary {
	var summary GoalSummary
	if state == nil {
		return summary
	}
	normalizeGoalState(state)
	summary.Version = state.Version
	for _, item := range state.Required {
		summary.RequiredTotal++
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			summary.RequiredSatisfied++
		case goalItemStateWaived:
			if item.UserApproved {
				summary.RequiredSatisfied++
			} else {
				summary.RequiredRemaining++
			}
		default:
			summary.RequiredRemaining++
		}
	}
	for _, item := range state.Optional {
		if normalizeGoalItemState(item.State) == goalItemStateOpen {
			summary.OptionalOpen++
		}
	}
	return summary
}

func ValidateGoalStateForVerification(state *GoalState) (GoalSummary, error) {
	summary := SummarizeGoalState(state)
	if state == nil {
		return summary, fmt.Errorf("goal state is missing")
	}
	if summary.RequiredTotal == 0 {
		return summary, fmt.Errorf("goal state has no required outcomes")
	}

	for _, item := range state.Required {
		if strings.TrimSpace(item.ID) == "" {
			return summary, fmt.Errorf("goal state has required item with empty id")
		}
		if strings.TrimSpace(item.Text) == "" {
			return summary, fmt.Errorf("goal item %s is missing text", item.ID)
		}
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			if len(trimmedGoalEvidencePaths(item.EvidencePaths)) == 0 {
				return summary, fmt.Errorf("goal item %s is claimed but has no evidence_paths", item.ID)
			}
		case goalItemStateWaived:
			if !item.UserApproved {
				return summary, fmt.Errorf("goal item %s is waived without explicit user approval", item.ID)
			}
		default:
			return summary, fmt.Errorf("goal item %s remains open", item.ID)
		}
	}

	return summary, nil
}

func normalizeGoalState(state *GoalState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Required == nil {
		state.Required = []GoalItem{}
	}
	if state.Optional == nil {
		state.Optional = []GoalItem{}
	}
	for i := range state.Required {
		normalizeGoalItem(&state.Required[i], goalItemSourceUser)
	}
	for i := range state.Optional {
		normalizeGoalItem(&state.Optional[i], goalItemSourceMaster)
	}
}

func normalizeGoalItem(item *GoalItem, defaultSource string) {
	if item == nil {
		return
	}
	item.Source = normalizeGoalItemSource(item.Source, defaultSource)
	item.State = normalizeGoalItemState(item.State)
	item.EvidencePaths = trimmedGoalEvidencePaths(item.EvidencePaths)
}

func normalizeGoalItemSource(source, defaultSource string) string {
	switch strings.TrimSpace(source) {
	case goalItemSourceUser:
		return goalItemSourceUser
	case goalItemSourceMaster:
		return goalItemSourceMaster
	default:
		return defaultSource
	}
}

func normalizeGoalItemState(state string) string {
	switch strings.TrimSpace(state) {
	case goalItemStateClaimed:
		return goalItemStateClaimed
	case goalItemStateWaived:
		return goalItemStateWaived
	default:
		return goalItemStateOpen
	}
}

func trimmedGoalEvidencePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
