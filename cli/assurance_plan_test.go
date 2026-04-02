package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveAssurancePlanRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := AssurancePlanPath(runDir)
	plan := &AssurancePlan{
		Version:        1,
		ObligationRefs: []string{"obl-first-run"},
		Scenarios: []AssuranceScenario{
			{
				ID:                "scenario-cli-first-run",
				CoversObligations: []string{"obl-first-run"},
				Harness: AssuranceHarness{
					Kind:    "cli",
					Command: "goalx run \"demo goal\"",
				},
				Oracle: AssuranceOracle{
					Kind: "exit_code",
					CheckDefinitions: []AssuranceOracleCheck{
						{Kind: "exit_code", Equals: "0"},
					},
				},
				Evidence: []AssuranceEvidenceRequirement{
					{Kind: "stdout"},
				},
				GatePolicy: AssuranceGatePolicy{
					Closeout:              "required",
					RequiredCognitionTier: "repo-native",
				},
			},
		},
	}

	if err := SaveAssurancePlan(path, plan); err != nil {
		t.Fatalf("SaveAssurancePlan: %v", err)
	}
	loaded, err := LoadAssurancePlan(path)
	if err != nil {
		t.Fatalf("LoadAssurancePlan: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadAssurancePlan returned nil plan")
	}
	if len(loaded.Scenarios) != 1 || loaded.Scenarios[0].Harness.Kind != "cli" {
		t.Fatalf("scenarios = %#v, want one round-tripped scenario", loaded.Scenarios)
	}
}

func TestLoadAssurancePlanRejectsScenarioWithoutCoverage(t *testing.T) {
	path := AssurancePlanPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "scenarios": [
    {
      "id": "scenario-cli-first-run",
      "harness": {"kind": "cli", "command": "goalx run \"demo goal\""},
      "oracle": {"kind": "exit_code", "checks": [{"kind": "exit_code", "equals": "0"}]},
      "evidence": [{"kind": "stdout"}]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadAssurancePlan(path)
	if err == nil {
		t.Fatal("LoadAssurancePlan should reject scenario without covers_obligations")
	}
	if !strings.Contains(err.Error(), "covers_obligations") {
		t.Fatalf("LoadAssurancePlan error = %v, want covers_obligations hint", err)
	}
}
