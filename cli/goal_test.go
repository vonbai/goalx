package cli

import (
	"os"
	"strings"
	"testing"
)

func TestLoadGoalStateRejectsUnknownFields(t *testing.T) {
	path := t.TempDir() + "/goal.json"
	payload := []byte(`{
  "version": 2,
  "required_items": [
    {
      "id": "req-1",
      "text": "ship feature",
      "state": "done"
    }
  ],
  "improvements": []
}`)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	_, err := LoadGoalState(path)
	if err == nil {
		t.Fatal("expected LoadGoalState to fail")
	}
	for _, want := range []string{"parse goal state", "unknown field", "goalx schema goal"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadGoalState error = %v, want %q", err, want)
		}
	}
}

func TestLoadGoalStateRejectsEmptyFile(t *testing.T) {
	path := t.TempDir() + "/goal.json"
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("write empty goal state: %v", err)
	}

	_, err := LoadGoalState(path)
	if err == nil {
		t.Fatal("expected LoadGoalState to fail")
	}
	if !strings.Contains(err.Error(), "goal state is empty") {
		t.Fatalf("LoadGoalState error = %v, want empty-file error", err)
	}
}

func TestLoadGoalStateRejectsInvalidItemState(t *testing.T) {
	path := t.TempDir() + "/goal.json"
	payload := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "done"
    }
  ],
  "optional": []
}`)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	_, err := LoadGoalState(path)
	if err == nil {
		t.Fatal("expected LoadGoalState to fail")
	}
	if !strings.Contains(err.Error(), `invalid goal item state "done"`) {
		t.Fatalf("LoadGoalState error = %v, want invalid-state error", err)
	}
}

func TestEnsureGoalStateDoesNotRewriteExistingGoal(t *testing.T) {
	runDir := t.TempDir()
	goalBefore := []byte(`{
  "version": 1,
  "updated_at": "2026-03-27T00:00:00Z",
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "claimed",
      "evidence_paths": ["/tmp/e2e.txt"]
    }
  ],
  "optional": []
}`)
	if err := os.WriteFile(GoalPath(runDir), goalBefore, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	state, err := EnsureGoalState(runDir)
	if err != nil {
		t.Fatalf("EnsureGoalState: %v", err)
	}
	if state == nil || len(state.Required) != 1 {
		t.Fatalf("EnsureGoalState returned %#v, want one required item", state)
	}

	assertFileUnchanged(t, GoalPath(runDir), goalBefore)
}
