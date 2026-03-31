package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveProofPlanRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := ProofPlanPath(runDir)
	plan := &ProofPlan{
		Version:    1,
		CompiledAt: "2026-03-31T08:00:00Z",
		Items: []ProofPlanItem{
			{
				ID:               "proof-clarity-visual",
				CoversDimensions: []string{"dim-product-clarity"},
				Kind:             "visual_evidence",
				Required:         true,
				SourceSurface:    "artifact",
			},
		},
	}

	if err := SaveProofPlan(path, plan); err != nil {
		t.Fatalf("SaveProofPlan: %v", err)
	}
	loaded, err := LoadProofPlan(path)
	if err != nil {
		t.Fatalf("LoadProofPlan: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadProofPlan returned nil plan")
	}
	if len(loaded.Items) != 1 || loaded.Items[0].Kind != "visual_evidence" {
		t.Fatalf("items = %#v, want one round-tripped proof item", loaded.Items)
	}
}

func TestLoadProofPlanRejectsMissingDimensionCoverage(t *testing.T) {
	path := ProofPlanPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "items": [
    {
      "id": "proof-1",
      "kind": "acceptance_check",
      "required": true,
      "source_surface": "acceptance"
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadProofPlan(path)
	if err == nil {
		t.Fatal("LoadProofPlan should reject missing covers_dimensions")
	}
	if !strings.Contains(err.Error(), "covers_dimensions") {
		t.Fatalf("LoadProofPlan error = %v, want covers_dimensions hint", err)
	}
}
