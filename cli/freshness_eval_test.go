package cli

import "testing"

func TestEvaluateFreshnessStateMarksRequiredEvidenceStaleOnTouchpointOverlap(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveCognitionState(CognitionStatePath(runDir), &CognitionState{
		Version: 1,
		Scopes: []CognitionScopeState{
			{
				Scope: "run-root",
				Providers: []CognitionProviderState{
					{Name: "repo-native", InvocationKind: "builtin", Available: true, HeadRevision: "def456", Capabilities: []string{"git_diff"}},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCognitionState: %v", err)
	}
	if err := SaveImpactState(ImpactStatePath(runDir), &ImpactState{
		Version:          1,
		Scope:            "run-root",
		BaselineRevision: "abc123",
		HeadRevision:     "def456",
		ResolverKind:     "repo-native",
		ChangedFiles:     []string{"cli/start.go"},
	}); err != nil {
		t.Fatalf("SaveImpactState: %v", err)
	}
	if err := SaveAssurancePlan(AssurancePlanPath(runDir), &AssurancePlan{
		Version: 1,
		Scenarios: []AssuranceScenario{
			{
				ID:                "scenario-cli-first-run",
				CoversObligations: []string{"obl-1"},
				Harness:           AssuranceHarness{Kind: "cli", Command: "printf ok"},
				Oracle:            AssuranceOracle{Kind: "exit_code", CheckDefinitions: []AssuranceOracleCheck{{Kind: "exit_code", Equals: "0"}}},
				Evidence:          []AssuranceEvidenceRequirement{{Kind: "stdout"}},
				Touchpoints:       AssuranceTouchpoints{Files: []string{"cli/start.go"}},
				GatePolicy:        AssuranceGatePolicy{Closeout: "required", RequiredCognitionTier: "repo-native"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveAssurancePlan: %v", err)
	}
	if err := AppendEvidenceLogEvent(EvidenceLogPath(runDir), "scenario.executed", "master", EvidenceEventBody{
		ScenarioID:   "scenario-cli-first-run",
		Revision:     "abc123",
		HarnessKind:  "cli",
		OracleResult: map[string]any{"exit_code": 0},
	}); err != nil {
		t.Fatalf("AppendEvidenceLogEvent: %v", err)
	}

	state, err := EvaluateFreshnessState(runDir)
	if err != nil {
		t.Fatalf("EvaluateFreshnessState: %v", err)
	}
	if len(state.Evidence) != 1 || state.Evidence[0].State != freshnessStateStale {
		t.Fatalf("evidence freshness = %#v, want stale", state.Evidence)
	}
}

func TestEvaluateFreshnessStateMarksSeededGitNexusCacheUnknownUntilLocallyAnalyzed(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := SaveCognitionState(CognitionStatePath(runDir), &CognitionState{
		Version: 1,
		Scopes: []CognitionScopeState{
			{
				Scope: "run-root",
				Providers: []CognitionProviderState{
					{Name: "repo-native", InvocationKind: "builtin", Available: true, IndexState: "fresh", HeadRevision: "def456", Capabilities: []string{"git_diff"}},
					{Name: "gitnexus", InvocationKind: "binary", Available: true, IndexState: "fresh", IndexProvenance: "seeded", IndexedRevision: "def456", HeadRevision: "def456", Capabilities: []string{"query", "context", "impact"}},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCognitionState: %v", err)
	}

	state, err := EvaluateFreshnessState(runDir)
	if err != nil {
		t.Fatalf("EvaluateFreshnessState: %v", err)
	}
	if len(state.Cognition) != 2 {
		t.Fatalf("cognition freshness = %#v, want repo-native and gitnexus entries", state.Cognition)
	}
	var gitnexusItem CognitionFreshnessItem
	for _, item := range state.Cognition {
		if item.Provider == "gitnexus" {
			gitnexusItem = item
		}
	}
	if gitnexusItem.State != freshnessStateUnknown || gitnexusItem.Reason != "seeded_cache_unverified" {
		t.Fatalf("gitnexus freshness = %+v, want unknown seeded_cache_unverified", gitnexusItem)
	}
}
