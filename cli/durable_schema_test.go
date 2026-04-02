package cli

import (
	"slices"
	"strings"
	"testing"
)

func TestLookupDurableContractReturnsSchemaMetadata(t *testing.T) {
	contract, err := LookupDurableContract("status")
	if err != nil {
		t.Fatalf("LookupDurableContract(status): %v", err)
	}
	if contract.Surface != DurableSurfaceStatus {
		t.Fatalf("contract.Surface = %s, want %s", contract.Surface, DurableSurfaceStatus)
	}
	if contract.Class != DurableSurfaceClassStructuredState {
		t.Fatalf("contract.Class = %s, want %s", contract.Class, DurableSurfaceClassStructuredState)
	}
	if contract.WriteMode != DurableSurfaceWriteModeReplace {
		t.Fatalf("contract.WriteMode = %s, want %s", contract.WriteMode, DurableSurfaceWriteModeReplace)
	}
	if !contract.Strict {
		t.Fatal("contract.Strict = false, want true")
	}
	if strings.TrimSpace(contract.Summary) == "" {
		t.Fatal("contract.Summary is empty")
	}
	if len(contract.Notes) == 0 {
		t.Fatal("contract.Notes is empty")
	}
	record, err := parseStatusAuthoringBody([]byte(contract.Example))
	if err != nil {
		t.Fatalf("parseStatusAuthoringBody(status example): %v\n%s", err, contract.Example)
	}
	if record.Version != 1 {
		t.Fatalf("record.Version = %d, want 1", record.Version)
	}
}

func TestLookupDurableContractObligationModelExampleSatisfiesValidator(t *testing.T) {
	contract, err := LookupDurableContract("obligation-model")
	if err != nil {
		t.Fatalf("LookupDurableContract(obligation-model): %v", err)
	}
	model, err := parseObligationModelAuthoringBody([]byte(contract.Example))
	if err != nil {
		t.Fatalf("parseObligationModelAuthoringBody(obligation-model example): %v\n%s", err, contract.Example)
	}
	if model.Version != 1 {
		t.Fatalf("model.Version = %d, want 1", model.Version)
	}
}

func TestLookupDurableContractAssurancePlanExampleSatisfiesValidator(t *testing.T) {
	contract, err := LookupDurableContract("assurance-plan")
	if err != nil {
		t.Fatalf("LookupDurableContract(assurance-plan): %v", err)
	}
	plan, err := parseAssurancePlanAuthoringBody([]byte(contract.Example))
	if err != nil {
		t.Fatalf("parseAssurancePlanAuthoringBody(assurance-plan example): %v\n%s", err, contract.Example)
	}
	if plan.Version != 1 {
		t.Fatalf("plan.Version = %d, want 1", plan.Version)
	}
}

func TestLookupDurableContractSupportsEventLogs(t *testing.T) {
	contract, err := LookupDurableContract("obligation-log")
	if err != nil {
		t.Fatalf("LookupDurableContract(obligation-log): %v", err)
	}
	if contract.Class != DurableSurfaceClassEventLog {
		t.Fatalf("contract.Class = %s, want %s", contract.Class, DurableSurfaceClassEventLog)
	}
	if contract.WriteMode != DurableSurfaceWriteModeAppend {
		t.Fatalf("contract.WriteMode = %s, want %s", contract.WriteMode, DurableSurfaceWriteModeAppend)
	}
	if !strings.Contains(contract.Example, `"decision": "initial_obligation_boundary"`) {
		t.Fatalf("contract.Example missing decision body:\n%s", contract.Example)
	}
	if !strings.Contains(contract.Example, `"chosen_shape"`) {
		t.Fatalf("contract.Example missing obligation-log body:\n%s", contract.Example)
	}
	if !slices.Equal(contract.AllowedKinds, []string{"decision", "checkpoint", "waiver", "closeout", "update"}) {
		t.Fatalf("contract.AllowedKinds = %v", contract.AllowedKinds)
	}
}

func TestLookupDurableContractRejectsUnknownSurface(t *testing.T) {
	if _, err := LookupDurableContract("mystery"); err == nil {
		t.Fatal("LookupDurableContract should reject unknown surfaces")
	}
}

func TestRenderDurableContractIncludesSummaryNotesAndExample(t *testing.T) {
	text, err := RenderDurableContract("coordination")
	if err != nil {
		t.Fatalf("RenderDurableContract(coordination): %v", err)
	}
	for _, want := range []string{
		"Surface: `coordination`",
		"Write mode: `replace`",
		"Unknown fields are fatal.",
		`"required": {`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered contract missing %q:\n%s", want, text)
		}
	}
}

func TestLookupDurableContractCoordinationExampleSatisfiesValidator(t *testing.T) {
	contract, err := LookupDurableContract("coordination")
	if err != nil {
		t.Fatalf("LookupDurableContract(coordination): %v", err)
	}
	record, err := parseCoordinationAuthoringBody([]byte(contract.Example))
	if err != nil {
		t.Fatalf("parseCoordinationAuthoringBody(coordination example): %v\n%s", err, contract.Example)
	}
	if record.Version != 1 {
		t.Fatalf("record.Version = %d, want 1", record.Version)
	}
}
