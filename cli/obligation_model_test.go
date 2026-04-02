package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveObligationModelRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := ObligationModelPath(runDir)
	model := &ObligationModel{
		Version:               1,
		ObjectiveContractHash: "sha256:objective",
		Required: []ObligationItem{
			{
				ID:                "obl-first-run",
				Text:              "first run succeeds",
				Kind:              "outcome",
				CoversClauses:     []string{"ucl-first-run"},
				AssuranceRequired: true,
			},
		},
		Guardrails: []ObligationItem{
			{
				ID:            "obl-no-corruption",
				Text:          "no state corruption",
				Kind:          "guardrail",
				CoversClauses: []string{"ucl-no-corruption"},
			},
		},
	}

	if err := SaveObligationModel(path, model); err != nil {
		t.Fatalf("SaveObligationModel: %v", err)
	}
	loaded, err := LoadObligationModel(path)
	if err != nil {
		t.Fatalf("LoadObligationModel: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadObligationModel returned nil model")
	}
	if len(loaded.Required) != 1 || loaded.Required[0].ID != "obl-first-run" {
		t.Fatalf("required = %#v, want one round-tripped obligation", loaded.Required)
	}
}

func TestLoadObligationModelRejectsMissingClauseCoverage(t *testing.T) {
	path := ObligationModelPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "objective_contract_hash": "sha256:objective",
  "required": [
    {
      "id": "obl-first-run",
      "text": "first run succeeds",
      "kind": "outcome",
      "assurance_required": true
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadObligationModel(path)
	if err == nil {
		t.Fatal("LoadObligationModel should reject missing covers_clauses")
	}
	if !strings.Contains(err.Error(), "covers_clauses") {
		t.Fatalf("LoadObligationModel error = %v, want covers_clauses hint", err)
	}
}
