package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestCoordinationStatePreservesExecutionStateAndDispatchableSlices(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state := &CoordinationState{
		Version: 1,
		Owners:  map[string]string{"req-1": "master"},
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

func TestCoordinationStatePreservesDecisionRecord(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state := &CoordinationState{
		Version: 1,
		Decision: &CoordinationDecision{
			RootCause:        "master keeps waiting on external blockers",
			LocalPath:        "patch the current flow",
			CompatiblePath:   "preserve existing contract semantics",
			ArchitecturePath: "separate waiting_external from dispatchable coverage",
			ChosenPath:       "architecture_path",
			ChosenPathReason: "it reduces idle waiting without sacrificing correctness",
		},
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), state); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	loaded, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if loaded.Decision == nil {
		t.Fatal("Decision = nil, want populated record")
	}
	if loaded.Decision.RootCause != "master keeps waiting on external blockers" {
		t.Fatalf("Decision.RootCause = %q", loaded.Decision.RootCause)
	}
	if loaded.Decision.CompatiblePath != "preserve existing contract semantics" {
		t.Fatalf("Decision.CompatiblePath = %q", loaded.Decision.CompatiblePath)
	}
	if loaded.Decision.ArchitecturePath != "separate waiting_external from dispatchable coverage" {
		t.Fatalf("Decision.ArchitecturePath = %q", loaded.Decision.ArchitecturePath)
	}
	if loaded.Decision.ChosenPath != "architecture_path" {
		t.Fatalf("Decision.ChosenPath = %q", loaded.Decision.ChosenPath)
	}
	if loaded.Decision.ChosenPathReason != "it reduces idle waiting without sacrificing correctness" {
		t.Fatalf("Decision.ChosenPathReason = %q", loaded.Decision.ChosenPathReason)
	}
}

func TestSaveCoordinationStatePreservesDigestFieldsVerbatim(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state := &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {
				State:          "active",
				ExecutionState: "active",
				Scope:          "  " + strings.Repeat("root-cause narration ", 30) + "  ",
				BlockedBy:      strings.Repeat("blocked because the remote deploy is still pending and we keep repeating the same analysis. ", 8),
			},
		},
		Blocked: []string{
			"  " + strings.Repeat("same blocker ", 20) + "  ",
			"  " + strings.Repeat("same blocker ", 20) + "  ",
		},
		Decision: &CoordinationDecision{
			RootCause:        strings.Repeat("analysis ", 25),
			LocalPath:        strings.Repeat("local ", 25),
			CompatiblePath:   strings.Repeat("compatible ", 25),
			ArchitecturePath: strings.Repeat("architecture ", 25),
			ChosenPath:       "architecture_path",
			ChosenPathReason: strings.Repeat("because better ", 25),
		},
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), state); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	loaded, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if loaded.Sessions["session-1"].Scope == "" {
		t.Fatal("Scope should be preserved")
	}
	if loaded.Sessions["session-1"].BlockedBy == "" {
		t.Fatal("BlockedBy should be preserved")
	}
	if loaded.Decision == nil {
		t.Fatal("Decision = nil")
	}
}

func TestLoadCoordinationStateRejectsLegacySessionSchema(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	legacy := `{
  "version": 2,
  "objective": "ship it",
  "active": [
    {
      "owner": "session-1",
      "scope": ["req-1", "req-2"],
      "status": "active"
    },
    {
      "owner": "master",
      "scope": ["req-3"],
      "status": "active",
      "note": "master is covering the remainder"
    }
  ],
  "blocked": [
    {
      "owner": "session-2",
      "scope": ["req-4"],
      "reason": "waiting on external api"
    }
  ],
  "parked": [
    {
      "session": "session-3",
      "scope": "held for reuse",
      "state": "parked"
    }
  ],
  "waiting_external": [
    {
      "session": "master",
      "scope": "remote deploy",
      "state": "waiting_external"
    }
  ]
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	_, err := LoadCoordinationState(CoordinationPath(runDir))
	if err == nil {
		t.Fatal("LoadCoordinationState should reject legacy aliases")
	}
	for _, want := range []string{"unknown field", "goalx schema coordination"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadCoordinationState error = %v, want %q", err, want)
		}
	}
}
