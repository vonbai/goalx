package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestCoordinationStatePreservesExecutionStateAndDispatchableSlices(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state := &CoordinationState{
		Version:   1,
		Objective: "audit auth flow",
		Owners:    map[string]string{"req-1": "master"},
		Sessions: map[string]CoordinationSession{
			"session-1": {
				State:          "active",
				Scope:          "inspect db retries",
				ExecutionState: "waiting_external",
				DispatchableSlices: []goalx.DispatchableSlice{
					{
						Title:          "split retry triage",
						Why:            "unblocks independent backend work",
						Mode:           "develop",
						SuggestedOwner: "session-2",
					},
				},
			},
		},
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), state); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	loaded, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	got := loaded.Sessions["session-1"]
	if got.ExecutionState != "waiting_external" {
		t.Fatalf("ExecutionState = %q, want waiting_external", got.ExecutionState)
	}
	if len(got.DispatchableSlices) != 1 {
		t.Fatalf("DispatchableSlices len = %d, want 1", len(got.DispatchableSlices))
	}
	if got.DispatchableSlices[0].Title != "split retry triage" {
		t.Fatalf("DispatchableSlices[0].Title = %q", got.DispatchableSlices[0].Title)
	}
}
