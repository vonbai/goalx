package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoordinationStatePreservesExecutionStateAndDispatchableSlices(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	payload := `{
  "version": 1,
  "required": {
    "req-1": {
      "owner": "master",
      "execution_state": "waiting",
      "blocked_by": "need remote confirmation",
      "surfaces": {
        "repo": "exhausted",
        "runtime": "available",
        "run_artifacts": "pending",
        "web_research": "pending",
        "external_system": "not_applicable"
      },
      "updated_at": "2026-03-30T10:00:00Z"
    }
  },
  "sessions": {
    "session-1": {
      "state": "active",
      "scope": "inspect db retries",
      "dispatchable_slices": [
        {
          "title": "split retry triage",
          "why": "unblocks independent backend work",
          "mode": "develop",
          "suggested_owner": "session-2"
        }
      ]
    }
  }
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(payload), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	loaded, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), loaded); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	saved, err := os.ReadFile(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("read saved coordination state: %v", err)
	}
	text := string(saved)
	for _, want := range []string{
		`"required": {`,
		`"execution_state": "waiting"`,
		`"runtime": "available"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("saved coordination state missing %q:\n%s", want, text)
		}
	}
	got := loaded.Sessions["session-1"]
	if got.State != "active" {
		t.Fatalf("State = %q, want active", got.State)
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

	payload := `{
  "version": 1,
  "required": {
    "req-1": {
      "owner": "master",
      "execution_state": "blocked",
      "blocked_by": "` + strings.Repeat("blocked because the remote deploy is still pending and we keep repeating the same analysis. ", 8) + `",
      "surfaces": {
        "repo": "exhausted",
        "runtime": "exhausted",
        "run_artifacts": "exhausted",
        "web_research": "exhausted",
        "external_system": "unreachable"
      }
    }
  },
  "sessions": {
    "session-1": {
      "state": "active",
      "scope": "  ` + strings.Repeat("root-cause narration ", 30) + `  "
    }
  },
  "decision": {
    "root_cause": "` + strings.Repeat("analysis ", 25) + `",
    "local_path": "` + strings.Repeat("local ", 25) + `",
    "compatible_path": "` + strings.Repeat("compatible ", 25) + `",
    "architecture_path": "` + strings.Repeat("architecture ", 25) + `",
    "chosen_path": "architecture_path",
    "chosen_path_reason": "` + strings.Repeat("because better ", 25) + `"
  }
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(payload), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	loaded, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), loaded); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	saved, err := os.ReadFile(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("read saved coordination state: %v", err)
	}
	if !strings.Contains(string(saved), `"blocked_by": "`) {
		t.Fatalf("saved coordination state missing blocked_by:\n%s", string(saved))
	}
	if loaded.Sessions["session-1"].Scope == "" {
		t.Fatal("Scope should be preserved")
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

func TestLoadCoordinationStateRejectsLegacyOwnersField(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	legacy := `{
  "version": 1,
  "owners": {
    "req-1": "master"
  }
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	_, err := LoadCoordinationState(CoordinationPath(runDir))
	if err == nil {
		t.Fatal("LoadCoordinationState should reject legacy owners")
	}
	for _, want := range []string{"unknown field", "goalx schema coordination"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadCoordinationState error = %v, want %q", err, want)
		}
	}
}

func TestLoadCoordinationStateRejectsLegacySessionExecutionStateField(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	legacy := `{
  "version": 1,
  "sessions": {
    "session-1": {
      "state": "active",
      "execution_state": "waiting_external"
    }
  }
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	_, err := LoadCoordinationState(CoordinationPath(runDir))
	if err == nil {
		t.Fatal("LoadCoordinationState should reject legacy session execution_state")
	}
	for _, want := range []string{"unknown field", "goalx schema coordination"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadCoordinationState error = %v, want %q", err, want)
		}
	}
}

func TestLoadCoordinationStateRejectsLegacySessionBlockedByField(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	legacy := `{
  "version": 1,
  "sessions": {
    "session-1": {
      "state": "active",
      "blocked_by": "wait for remote system"
    }
  }
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	_, err := LoadCoordinationState(CoordinationPath(runDir))
	if err == nil {
		t.Fatal("LoadCoordinationState should reject legacy session blocked_by")
	}
	for _, want := range []string{"unknown field", "goalx schema coordination"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadCoordinationState error = %v, want %q", err, want)
		}
	}
}

func TestLoadCoordinationStateRejectsLegacyBlockedField(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	legacy := `{
  "version": 1,
  "blocked": [
    "req-1"
  ]
}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(legacy), 0o644); err != nil {
		t.Fatalf("write coordination state: %v", err)
	}

	_, err := LoadCoordinationState(CoordinationPath(runDir))
	if err == nil {
		t.Fatal("LoadCoordinationState should reject legacy blocked")
	}
	for _, want := range []string{"unknown field", "goalx schema coordination"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadCoordinationState error = %v, want %q", err, want)
		}
	}
}
