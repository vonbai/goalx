package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveSuccessModelRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := SuccessModelPath(runDir)
	model := &SuccessModel{
		Version:               1,
		CompiledAt:            "2026-03-31T08:00:00Z",
		CompilerVersion:       "smc-v1",
		ObjectiveContractHash: "sha256:objective",
		ObligationModelHash:              "sha256:goal",
		Dimensions: []SuccessDimension{
			{
				ID:           "dim-product-clarity",
				Kind:         "quality",
				Text:         "Operators orient quickly.",
				Required:     true,
				FailureModes: []string{"correct_but_unclear"},
			},
		},
		AntiGoals: []SuccessAntiGoal{
			{ID: "anti-proof-only", Text: "Do not collapse into proof only."},
		},
		CloseoutRequirements: []string{"quality_debt_zero"},
	}

	if err := SaveSuccessModel(path, model); err != nil {
		t.Fatalf("SaveSuccessModel: %v", err)
	}
	loaded, err := LoadSuccessModel(path)
	if err != nil {
		t.Fatalf("LoadSuccessModel: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadSuccessModel returned nil model")
	}
	if loaded.CompilerVersion != "smc-v1" {
		t.Fatalf("compiler_version = %q, want smc-v1", loaded.CompilerVersion)
	}
	if len(loaded.Dimensions) != 1 || loaded.Dimensions[0].ID != "dim-product-clarity" {
		t.Fatalf("dimensions = %#v, want one round-tripped dimension", loaded.Dimensions)
	}
}

func TestLoadSuccessModelRejectsUnknownFields(t *testing.T) {
	path := SuccessModelPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "objective_contract_hash": "sha256:objective",
  "obligation_model_hash": "sha256:goal",
  "dimensions": [],
  "unexpected": true
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadSuccessModel(path)
	if err == nil {
		t.Fatal("LoadSuccessModel should reject unknown fields")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadSuccessModel error = %v, want unknown field hint", err)
	}
}
