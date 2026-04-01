package cli

import (
	"os"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildRunIntakeIncludesIntentHints(t *testing.T) {
	cfg := &goalx.Config{
		Objective: "ship auth boundary",
		Context: goalx.ContextConfig{
			Files: []string{"README.md"},
			Refs:  []string{"ref:ticket-123"},
		},
		Target: goalx.TargetConfig{
			Readonly: []string{"."},
		},
	}
	meta := &RunMetadata{Intent: runIntentExplore}
	intake := BuildRunIntake(cfg, meta)
	if intake == nil {
		t.Fatal("BuildRunIntake returned nil")
	}
	if intake.Intent != runIntentExplore {
		t.Fatalf("intent = %q, want %q", intake.Intent, runIntentExplore)
	}
	if intake.Objective != "ship auth boundary" {
		t.Fatalf("objective = %q, want ship auth boundary", intake.Objective)
	}
	for _, want := range []string{
		"expand_evidence_before_implementation",
		"preserve_declared_readonly_boundary",
		"declared_context_is_part_of_initial_success_input",
	} {
		if !containsString(intake.WorkflowHints, want) && !containsString(intake.AntiGoals, want) && !containsString(intake.SuccessHints, want) {
			t.Fatalf("run intake missing %q: %+v", want, intake)
		}
	}
}

func TestSaveRunIntakeRoundTrip(t *testing.T) {
	path := IntakePath(t.TempDir())
	intake := &RunIntake{
		Version:       1,
		Objective:     "ship it",
		Intent:        runIntentDeliver,
		SuccessHints:  []string{"ship_verified_outcome"},
		AntiGoals:     []string{"do_not_stop_at_correctness_only"},
		WorkflowHints: []string{"dispatch_before_self_implementation"},
	}
	if err := SaveRunIntake(path, intake); err != nil {
		t.Fatalf("SaveRunIntake: %v", err)
	}
	loaded, err := LoadRunIntake(path)
	if err != nil {
		t.Fatalf("LoadRunIntake: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadRunIntake returned nil")
	}
	if loaded.Objective != "ship it" {
		t.Fatalf("loaded intake = %+v, want ship it", loaded)
	}
}

func TestBuildBootstrapCompilerSourcesIncludesIntakeRunContextRef(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	runDir := t.TempDir()
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	if err := SaveRunIntake(IntakePath(runDir), &RunIntake{
		Version: 1,
		Intent:  runIntentDeliver,
	}); err != nil {
		t.Fatalf("SaveRunIntake: %v", err)
	}

	sources, err := buildBootstrapCompilerSources(repo, runDir)
	if err != nil {
		t.Fatalf("buildBootstrapCompilerSources: %v", err)
	}
	if sources == nil {
		t.Fatal("sources is nil")
	}
	found := false
	for _, slot := range sources.SourceSlots {
		if slot.Slot != CompilerInputSlotRunContext {
			continue
		}
		for _, ref := range slot.Refs {
			if strings.Contains(ref, "intake.json") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("source slots = %+v, want intake run-context ref", sources.SourceSlots)
	}
}
