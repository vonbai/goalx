package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCoordinationStateCreatesDigest(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state, err := EnsureCoordinationState(runDir, "audit auth flow")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	if state.Objective != "" {
		t.Fatalf("Objective = %q, want empty display-only coordination state", state.Objective)
	}
	if state.Version <= 0 {
		t.Fatalf("Version = %d, want > 0", state.Version)
	}
	if state.Sessions == nil {
		t.Fatal("Sessions = nil, want initialized map")
	}
	if _, err := os.Stat(CoordinationPath(runDir)); err != nil {
		t.Fatalf("coordination path missing: %v", err)
	}
}

func TestCoordinationStripsObjectiveFromSavedState(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version:     1,
		Objective:   "stale duplicate objective",
		PlanSummary: []string{"compare paths"},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	state, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if state.Objective != "" {
		t.Fatalf("Objective = %q, want stripped duplicate objective", state.Objective)
	}
	if len(state.PlanSummary) != 1 || state.PlanSummary[0] != "compare paths" {
		t.Fatalf("PlanSummary = %#v", state.PlanSummary)
	}
}
