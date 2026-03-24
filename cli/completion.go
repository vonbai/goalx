package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	completionModeVerificationOnly              = "verification_only"
	completionModeImplementationAndVerification = "implementation_and_verification"
)

type CompletionState struct {
	Version           int                   `json:"version"`
	GoalVersion       int                   `json:"goal_version"`
	AcceptanceStatus  string                `json:"acceptance_status,omitempty"`
	GoalSatisfied     bool                  `json:"goal_satisfied"`
	RequiredTotal     int                   `json:"required_total"`
	RequiredSatisfied int                   `json:"required_satisfied"`
	RequiredRemaining int                   `json:"required_remaining"`
	OptionalOpen      int                   `json:"optional_open"`
	BaseRevision      string                `json:"base_revision,omitempty"`
	HeadRevision      string                `json:"head_revision,omitempty"`
	CodeChanged       bool                  `json:"code_changed"`
	CompletionMode    string                `json:"completion_mode,omitempty"`
	ChangedFiles      []string              `json:"changed_files,omitempty"`
	KeptSession       string                `json:"kept_session,omitempty"`
	KeptBranch        string                `json:"kept_branch,omitempty"`
	Items             []CompletionProofItem `json:"items,omitempty"`
	UpdatedAt         string                `json:"updated_at,omitempty"`
}

func CompletionStatePath(runDir string) string {
	return filepath.Join(runDir, "proof", "completion.json")
}

func SaveCompletionState(path string, state *CompletionState) error {
	if state == nil {
		return fmt.Errorf("completion state is nil")
	}
	if state.Version <= 0 {
		state.Version = 1
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

func DetectCompletionState(projectRoot, runDir string, goal *GoalState, acceptance *AcceptanceState) (*CompletionState, error) {
	meta, err := EnsureRunMetadata(runDir, projectRoot, "")
	if err != nil {
		return nil, err
	}
	headRevision, err := gitHeadRevision(projectRoot)
	if err != nil {
		return nil, err
	}
	changedFiles, err := gitChangedFilesSince(projectRoot, meta.BaseRevision, headRevision)
	if err != nil {
		return nil, err
	}
	selection, _ := loadSelectionFile(filepath.Join(runDir, "selection.json"))
	codeChanged := len(changedFiles) > 0

	state := &CompletionState{
		Version:      1,
		BaseRevision: meta.BaseRevision,
		HeadRevision: headRevision,
		ChangedFiles: changedFiles,
		CodeChanged:  codeChanged,
	}
	if selection != nil {
		state.KeptSession = selection.Kept
		state.KeptBranch = selection.Branch
		if state.KeptSession != "" || state.KeptBranch != "" {
			state.CodeChanged = true
		}
	}
	if state.CodeChanged {
		state.CompletionMode = completionModeImplementationAndVerification
	} else {
		state.CompletionMode = completionModeVerificationOnly
	}
	if acceptance != nil {
		state.AcceptanceStatus = acceptanceStatus(acceptance)
	}
	if goal != nil {
		summary := SummarizeGoalState(goal)
		state.GoalVersion = summary.Version
		state.RequiredTotal = summary.RequiredTotal
		state.OptionalOpen = summary.OptionalOpen
		state.Items = BuildCompletionProofItems(goal, state.CodeChanged)
		for _, item := range state.Items {
			switch item.Verdict {
			case completionVerdictSatisfied:
				state.RequiredSatisfied++
			case completionVerdictWaived:
				if item.UserApproved {
					state.RequiredSatisfied++
				}
			}
		}
		state.RequiredRemaining = state.RequiredTotal - state.RequiredSatisfied
	}
	state.GoalSatisfied = state.AcceptanceStatus == acceptanceStatusPassed && state.RequiredTotal > 0 && state.RequiredRemaining == 0
	return state, nil
}

func gitChangedFilesSince(projectRoot, baseRevision, headRevision string) ([]string, error) {
	if strings.TrimSpace(baseRevision) == "" || strings.TrimSpace(headRevision) == "" {
		return nil, nil
	}
	if strings.TrimSpace(baseRevision) == strings.TrimSpace(headRevision) {
		return nil, nil
	}
	out, err := exec.Command("git", "-C", projectRoot, "diff", "--name-only", baseRevision+".."+headRevision).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("diff changed files %s..%s: %w\n%s", baseRevision, headRevision, err, strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		files = append(files, line)
	}
	return files, nil
}

func loadSelectionFile(path string) (*selectionJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var selection selectionJSON
	if err := json.Unmarshal(data, &selection); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if selection.Kept == "" && selection.Branch == "" {
		return nil, nil
	}
	return &selection, nil
}
