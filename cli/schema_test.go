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
		`"required_remaining": 0`,
		"goalx durable write status --run NAME --body-file /abs/path.json",
		"Authoring format: `json`",
		"Storage format: `json`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"version": 1`) {
		t.Fatalf("status schema should not expose storage-only version field in authoring example:\n%s", out)
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
		"goalx durable write goal --run NAME --body-file /abs/path.json",
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
		"goalx durable write coordination --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsSuccessModelContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"success-model"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: success-model",
		"structured_state",
		"replace",
		`"dimensions": [`,
		`"anti_goals": [`,
		"goalx durable write success-model --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsProofPlanContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"proof-plan"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: proof-plan",
		"structured_state",
		"replace",
		`"covers_dimensions": [`,
		`"kind": "acceptance_check"`,
		"goalx durable write proof-plan --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsWorkflowPlanContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"workflow-plan"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: workflow-plan",
		"structured_state",
		"replace",
		`"required_roles": [`,
		`"gates": [`,
		"goalx durable write workflow-plan --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsInterventionLogContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"intervention-log"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: intervention-log",
		"event_log",
		"append",
		`"message": "Do not stop at route cutover only."`,
		"goalx durable write intervention-log --run NAME --kind user_redirect --actor master --body-file /abs/path.json",
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
		`"decision": "initial_boundary_shape_selection"`,
		"goalx durable write goal-log --run NAME --kind decision --actor master --body-file /abs/path.json",
		"Authoring format: `json`",
		"Storage format: `jsonl`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{`"kind": "decision"`, `"version": 1`, `"actor": "master"`} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("goal-log schema should not expose storage envelope field %q in authoring example:\n%s", unwanted, out)
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
	if got := payload["authoring_format"]; got != string(DurableSurfaceSchemaFormatJSON) {
		t.Fatalf("authoring_format = %#v, want %q", got, DurableSurfaceSchemaFormatJSON)
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
