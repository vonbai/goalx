package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveFreshnessStateRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := FreshnessStatePath(runDir)
	state := &FreshnessState{
		Version: 1,
		Cognition: []CognitionFreshnessItem{
			{Scope: "run-root", Provider: "repo-native", State: "fresh"},
		},
		Evidence: []EvidenceFreshnessItem{
			{ScenarioID: "scenario-cli-first-run", State: "stale"},
		},
	}

	if err := SaveFreshnessState(path, state); err != nil {
		t.Fatalf("SaveFreshnessState: %v", err)
	}
	loaded, err := LoadFreshnessState(path)
	if err != nil {
		t.Fatalf("LoadFreshnessState: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadFreshnessState returned nil state")
	}
	if len(loaded.Cognition) != 1 || len(loaded.Evidence) != 1 {
		t.Fatalf("freshness = %#v, want cognition and evidence entries", loaded)
	}
}

func TestLoadFreshnessStateRejectsInvalidState(t *testing.T) {
	path := FreshnessStatePath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "cognition": [
    {
      "scope": "run-root",
      "provider": "repo-native",
      "state": "broken"
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadFreshnessState(path)
	if err == nil {
		t.Fatal("LoadFreshnessState should reject invalid state")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Fatalf("LoadFreshnessState error = %v, want state hint", err)
	}
}
