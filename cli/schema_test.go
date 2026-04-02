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

func TestSchemaRejectsLegacyGoalSurface(t *testing.T) {
	err := Schema(t.TempDir(), []string{"goal"})
	if err == nil || !strings.Contains(err.Error(), "obligation-model") {
		t.Fatalf("Schema error = %v, want obligation-model migration hint", err)
	}
}

func TestSchemaPrintsObligationModelContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"obligation-model"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: obligation-model",
		`"covers_clauses": [`,
		`"assurance_required": true`,
		`"guardrails": [`,
		"goalx durable write obligation-model --run NAME --body-file /abs/path.json",
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

func TestSchemaPrintsCognitionStateContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"cognition-state"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: cognition-state",
		`"providers": [`,
		`"invocation_kind": "builtin"`,
		`"index_state": "fresh"`,
		`"index_provenance": "seeded"`,
		`"read_transports_supported": [`,
		`"mcp_server_command": "gitnexus mcp"`,
		`"mcp_tools_supported": [`,
		`"mcp_resources_supported": [`,
		`"registry_name": "demo-repo"`,
		`"last_refresh_error": "status parse warning"`,
		`"analyzed_in_scope_at": "2026-04-01T00:00:00Z"`,
		`"capabilities": [`,
		"goalx durable write cognition-state --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsAssurancePlanContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"assurance-plan"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: assurance-plan",
		`"scenarios": [`,
		`"harness": {`,
		`"oracle": {`,
		`"required_cognition_tier": "repo-native"`,
		"goalx durable write assurance-plan --run NAME --body-file /abs/path.json",
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

func TestSchemaPrintsImpactStateContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"impact-state"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: impact-state",
		`"resolver_kind": "repo-native"`,
		`"changed_files": [`,
		`"changed_symbols": [`,
		"goalx durable write impact-state --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsFreshnessStateContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"freshness-state"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: freshness-state",
		`"cognition": [`,
		`"evidence": [`,
		`"state": "stale"`,
		"goalx durable write freshness-state --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsResourceStateContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"resource-state"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: resource-state",
		`"host": {`,
		`"psi": {`,
		`"cgroup": {`,
		`"goalx_processes": {`,
		`"headroom_bytes": 17179869184`,
		`"state": "healthy"`,
		"goalx durable write resource-state --run NAME --body-file /abs/path.json",
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
		`"kind": "assurance_check"`,
		"goalx durable write proof-plan --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsObligationLogContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"obligation-log"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: obligation-log",
		"event_log",
		"append",
		`"decision": "initial_obligation_boundary"`,
		"goalx durable write obligation-log --run NAME --kind decision --actor master --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsEvidenceLogContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"evidence-log"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: evidence-log",
		"event_log",
		"append",
		`"scenario_id": "scenario-cli-first-run"`,
		"goalx durable write evidence-log --run NAME --kind scenario.executed --actor master --body-file /abs/path.json",
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

func TestSchemaPrintsCompilerInputContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"compiler-input"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: compiler-input",
		`"source_slots": [`,
		`"selected_prior_refs": [`,
		"goalx durable write compiler-input --run NAME --body-file /abs/path.json",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("schema output missing %q:\n%s", want, out)
		}
	}
}

func TestSchemaPrintsCompilerReportContract(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Schema(t.TempDir(), []string{"compiler-report"}); err != nil {
			t.Fatalf("Schema: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Schema: compiler-report",
		`"available_source_slots": [`,
		`"rejected_priors": [`,
		`"reason_code": "no_selector_match"`,
		"goalx durable write compiler-report --run NAME --body-file /abs/path.json",
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

func TestSchemaRejectsLegacyGoalLogSurface(t *testing.T) {
	err := Schema(t.TempDir(), []string{"goal-log"})
	if err == nil || !strings.Contains(err.Error(), "obligation-log") {
		t.Fatalf("Schema error = %v, want obligation-log migration hint", err)
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
