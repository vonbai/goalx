package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type RunIntake struct {
	Version       int      `json:"version"`
	CreatedAt     string   `json:"created_at,omitempty"`
	Objective     string   `json:"objective,omitempty"`
	Intent        string   `json:"intent,omitempty"`
	Readonly      bool     `json:"readonly,omitempty"`
	ContextFiles  []string `json:"context_files,omitempty"`
	ContextRefs   []string `json:"context_refs,omitempty"`
	SuccessHints  []string `json:"success_hints,omitempty"`
	AntiGoals     []string `json:"anti_goals,omitempty"`
	WorkflowHints []string `json:"workflow_hints,omitempty"`
	ProofHints    []string `json:"proof_hints,omitempty"`
}

func IntakePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "intake.json")
}

func LegacyGuidedIntakePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "guided-intake.json")
}

func SavedRunIntakePath(runDir string) string {
	return filepath.Join(runDir, "intake.json")
}

func LoadRunIntake(path string) (*RunIntake, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var intake RunIntake
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&intake); err != nil {
		return nil, fmt.Errorf("parse intake: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("parse intake: %w", err)
	}
	normalizeRunIntake(&intake)
	return &intake, nil
}

func LoadLiveRunIntake(runDir string) (*RunIntake, error) {
	intake, err := LoadRunIntake(IntakePath(runDir))
	if err != nil {
		return nil, err
	}
	if intake == nil && fileExists(LegacyGuidedIntakePath(runDir)) {
		return nil, fmt.Errorf("legacy run uses removed intake surface %s; recreate or migrate this run to %s", LegacyGuidedIntakePath(runDir), IntakePath(runDir))
	}
	return intake, nil
}

func RequireSavedRunIntake(savedRunDir string) (*RunIntake, error) {
	intake, err := LoadRunIntake(SavedRunIntakePath(savedRunDir))
	if err != nil {
		return nil, err
	}
	if intake == nil {
		return nil, fmt.Errorf("legacy saved run missing canonical intake %s", SavedRunIntakePath(savedRunDir))
	}
	return intake, nil
}

func SaveRunIntake(path string, intake *RunIntake) error {
	if intake == nil {
		return fmt.Errorf("run intake is nil")
	}
	normalizeRunIntake(intake)
	if intake.Version <= 0 {
		intake.Version = 1
	}
	if intake.CreatedAt == "" {
		intake.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(intake, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func normalizeRunIntake(intake *RunIntake) {
	if intake == nil {
		return
	}
	if intake.Version <= 0 {
		intake.Version = 1
	}
	intake.CreatedAt = strings.TrimSpace(intake.CreatedAt)
	intake.Objective = strings.TrimSpace(intake.Objective)
	intake.Intent = strings.TrimSpace(intake.Intent)
	intake.ContextFiles = compactStrings(intake.ContextFiles)
	intake.ContextRefs = compactStrings(intake.ContextRefs)
	intake.SuccessHints = compactStrings(intake.SuccessHints)
	intake.AntiGoals = compactStrings(intake.AntiGoals)
	intake.WorkflowHints = compactStrings(intake.WorkflowHints)
	intake.ProofHints = compactStrings(intake.ProofHints)
}

func BuildRunIntake(cfg *goalx.Config, meta *RunMetadata) *RunIntake {
	if cfg == nil || meta == nil {
		return nil
	}
	intent := strings.TrimSpace(meta.Intent)
	if intent == "" {
		intent = runIntentDeliver
	}
	intake := &RunIntake{
		Version:      1,
		Objective:    strings.TrimSpace(cfg.Objective),
		Intent:       intent,
		Readonly:     len(cfg.Target.Readonly) > 0,
		ContextFiles: append([]string(nil), cfg.Context.Files...),
		ContextRefs:  append([]string(nil), cfg.Context.Refs...),
	}
	switch intent {
	case runIntentExplore:
		intake.SuccessHints = append(intake.SuccessHints, "expand_evidence_before_implementation")
		intake.AntiGoals = append(intake.AntiGoals, "do_not_collapse_explore_into_implementation_only")
		intake.WorkflowHints = append(intake.WorkflowHints, "compare_paths_before_commitment")
		intake.ProofHints = append(intake.ProofHints, "capture_evidence_artifacts")
	case runIntentEvolve:
		intake.SuccessHints = append(intake.SuccessHints, "keep_frontier_moving_until_explicit_stop")
		intake.AntiGoals = append(intake.AntiGoals, "do_not_idle_without_frontier_decision")
		intake.WorkflowHints = append(intake.WorkflowHints, "record_experiment_lineage")
		intake.ProofHints = append(intake.ProofHints, "preserve_decisive_iteration_evidence")
	default:
		intake.SuccessHints = append(intake.SuccessHints, "ship_verified_outcome")
		intake.AntiGoals = append(intake.AntiGoals, "do_not_stop_at_correctness_only")
		intake.WorkflowHints = append(intake.WorkflowHints, "dispatch_before_self_implementation")
		intake.ProofHints = append(intake.ProofHints, "preserve_closeout_evidence")
	}
	if intake.Readonly {
		intake.AntiGoals = append(intake.AntiGoals, "preserve_declared_readonly_boundary")
	}
	if len(intake.ContextFiles) > 0 || len(intake.ContextRefs) > 0 {
		intake.WorkflowHints = append(intake.WorkflowHints, "declared_context_is_part_of_initial_success_input")
	}
	normalizeRunIntake(intake)
	return intake
}

func ensureRunIntake(runDir string, cfg *goalx.Config, meta *RunMetadata) error {
	intake := BuildRunIntake(cfg, meta)
	if intake == nil {
		return nil
	}
	return SaveRunIntake(IntakePath(runDir), intake)
}
