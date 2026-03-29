package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaPrintsStructuredSurfaceContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"status"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: status",
		"structured_state",
		"replace",
		`"version": 1`,
		`"required_remaining": 0`,
		"goalx durable replace status --run NAME --file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsGoalContractWithObligationGrammar(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"goal"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: goal",
		`"role": "outcome"`,
		`"source": "master"`,
		"goalx durable replace goal --run NAME --file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsCoordinationContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"coordination"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: coordination",
		"structured_state",
		"replace",
		`"required": {`,
		`"decision": {`,
		"goalx durable replace coordination --run NAME --file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsEventLogContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"goal-log"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: goal-log",
		"event_log",
		"append",
		`"kind": "`,
		`"body": {`,
		"goalx durable append goal-log --run NAME --file /abs/path.jsonl",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaJSONOutput(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"status", "--json"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\n%s", err, out)
	}
	if got := payload["surface"]; got != "status" {
		t.Fatalf("surface = %#v, want status", got)
	}
	if got := payload["class"]; got != string(DurableSurfaceClassStructuredState) {
		t.Fatalf("class = %#v, want %q", got, DurableSurfaceClassStructuredState)
	}
	if got := payload["write_mode"]; got != string(DurableSurfaceWriteModeReplace) {
		t.Fatalf("write_mode = %#v, want %q", got, DurableSurfaceWriteModeReplace)
	}
}

func TestSchemaRejectsUnknownSurface(t *testing.T) {
	err := Schema(t.TempDir(), []string{"mystery"})
	if err == nil || !strings.Contains(err.Error(), `unknown durable surface "mystery"`) {
		t.Fatalf("Schema error = %v, want unknown surface", err)
	}
}

func TestSchemaHelpShowsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Schema --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx schema <surface> [--json]") {
		t.Fatalf("help missing usage:\n%s", out)
	}
}
