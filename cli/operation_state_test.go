package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureControlOperationsStateBootstrapsEmptyState(t *testing.T) {
	runDir := t.TempDir()

	state, err := EnsureControlOperationsState(runDir)
	if err != nil {
		t.Fatalf("EnsureControlOperationsState: %v", err)
	}
	if state == nil {
		t.Fatal("state is nil")
	}
	if state.Version != 1 {
		t.Fatalf("version = %d, want 1", state.Version)
	}
	if len(state.Targets) != 0 {
		t.Fatalf("targets = %v, want empty", state.Targets)
	}

	if _, err := os.Stat(ControlOperationsPath(runDir)); err != nil {
		t.Fatalf("stat operations path: %v", err)
	}
}

func TestParseControlOperationsStateRejectsUnknownField(t *testing.T) {
	_, err := parseControlOperationsState([]byte(`{
  "version": 1,
  "targets": {},
  "extra": true
}`))
	if err == nil {
		t.Fatal("parseControlOperationsState should reject unknown field")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("error = %v, want unknown field", err)
	}
}

func TestParseControlOperationsStateRejectsInvalidKindAndState(t *testing.T) {
	_, err := parseControlOperationsState([]byte(`{
  "version": 1,
  "targets": {
    "run": {
      "kind": "not-real",
      "state": "also-not-real"
    }
  }
}`))
	if err == nil {
		t.Fatal("parseControlOperationsState should reject invalid kind/state")
	}
	if !strings.Contains(err.Error(), "invalid kind") && !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("error = %v, want invalid kind/state", err)
	}
}

func TestSaveControlOperationsStateNormalizesPendingConditions(t *testing.T) {
	runDir := t.TempDir()
	path := filepath.Join(runDir, "operations.json")
	state := &ControlOperationsState{
		Version: 1,
		Targets: map[string]ControlOperationTarget{
			"session-1": {
				Kind:              ControlOperationKindSessionDispatch,
				State:             ControlOperationStatePreparing,
				PendingConditions: []string{" pane_present ", "", "pane_present", "transport_frame"},
			},
		},
	}

	if err := SaveControlOperationsState(path, state); err != nil {
		t.Fatalf("SaveControlOperationsState: %v", err)
	}
	loaded, err := LoadControlOperationsState(path)
	if err != nil {
		t.Fatalf("LoadControlOperationsState: %v", err)
	}
	got := loaded.Targets["session-1"].PendingConditions
	want := []string{"pane_present", "transport_frame"}
	if len(got) != len(want) {
		t.Fatalf("pending_conditions len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pending_conditions[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
