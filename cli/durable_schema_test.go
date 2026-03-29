package cli

import (
	"encoding/json"
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
	var record RunStatusRecord
	if err := json.Unmarshal([]byte(contract.Example), &record); err != nil {
		t.Fatalf("json.Unmarshal(status example): %v\n%s", err, contract.Example)
	}
	if err := validateRunStatusRecord(&record); err != nil {
		t.Fatalf("status example must satisfy validator: %v\n%s", err, contract.Example)
	}
}

func TestLookupDurableContractSupportsEventLogs(t *testing.T) {
	contract, err := LookupDurableContract("goal-log")
	if err != nil {
		t.Fatalf("LookupDurableContract(goal-log): %v", err)
	}
	if contract.Class != DurableSurfaceClassEventLog {
		t.Fatalf("contract.Class = %s, want %s", contract.Class, DurableSurfaceClassEventLog)
	}
	if contract.WriteMode != DurableSurfaceWriteModeAppend {
		t.Fatalf("contract.WriteMode = %s, want %s", contract.WriteMode, DurableSurfaceWriteModeAppend)
	}
	if !strings.Contains(contract.Example, `"kind": "decision"`) {
		t.Fatalf("contract.Example missing decision envelope:\n%s", contract.Example)
	}
	if !strings.Contains(contract.Example, `"boundary_shapes_compared"`) {
		t.Fatalf("contract.Example missing goal-log envelope:\n%s", contract.Example)
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
	var record CoordinationState
	if err := json.Unmarshal([]byte(contract.Example), &record); err != nil {
		t.Fatalf("json.Unmarshal(coordination example): %v\n%s", err, contract.Example)
	}
	if err := validateCoordinationState(&record); err != nil {
		t.Fatalf("coordination example must satisfy validator: %v\n%s", err, contract.Example)
	}
}
