package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAppendEvidenceLogEventRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := EvidenceLogPath(runDir)
	body := EvidenceEventBody{
		ScenarioID:  "scenario-cli-first-run",
		Scope:       "run-root",
		Revision:    "def456",
		HarnessKind: "cli",
		OracleResult: map[string]any{
			"exit_code": 0,
		},
		ArtifactRefs: []string{"reports/assurance/stdout.txt"},
	}

	if err := AppendEvidenceLogEvent(path, "scenario.executed", "master", body); err != nil {
		t.Fatalf("AppendEvidenceLogEvent: %v", err)
	}
	events, err := LoadEvidenceLog(path)
	if err != nil {
		t.Fatalf("LoadEvidenceLog: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %#v, want one event", events)
	}
	var got EvidenceEventBody
	if err := json.Unmarshal(events[0].Body, &got); err != nil {
		t.Fatalf("json.Unmarshal body: %v", err)
	}
	if got.ScenarioID != "scenario-cli-first-run" {
		t.Fatalf("scenario_id = %q, want scenario-cli-first-run", got.ScenarioID)
	}
}

func TestLoadEvidenceLogRejectsMissingScenarioID(t *testing.T) {
	path := EvidenceLogPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{"version":1,"kind":"scenario.executed","at":"2026-04-01T00:00:00Z","actor":"master","body":{"scope":"run-root","revision":"def456","harness_kind":"cli"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadEvidenceLog(path)
	if err == nil {
		t.Fatal("LoadEvidenceLog should reject missing scenario_id")
	}
	if !strings.Contains(err.Error(), "scenario_id") {
		t.Fatalf("LoadEvidenceLog error = %v, want scenario_id hint", err)
	}
}
