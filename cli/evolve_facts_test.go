package cli

import (
	"os"
	"reflect"
	"testing"
)

func TestBuildEvolveFactsActiveFrontier(t *testing.T) {
	runDir := t.TempDir()
	writeEvolveMetadataForTest(t, runDir)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)

	facts, err := BuildEvolveFacts(runDir)
	if err != nil {
		t.Fatalf("BuildEvolveFacts: %v", err)
	}
	if facts == nil {
		t.Fatal("BuildEvolveFacts returned nil facts")
	}
	if facts.FrontierState != EvolveFrontierActive {
		t.Fatalf("FrontierState = %q, want %q", facts.FrontierState, EvolveFrontierActive)
	}
	if got := facts.OpenCandidateIDs; !reflect.DeepEqual(got, []string{"exp-1"}) {
		t.Fatalf("OpenCandidateIDs = %v, want [exp-1]", got)
	}
}

func TestBuildEvolveFactsStoppedFrontier(t *testing.T) {
	runDir := t.TempDir()
	writeEvolveMetadataForTest(t, runDir)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"evolve.stopped","at":"2026-03-29T10:05:00Z","actor":"master","body":{"reason_code":"diminishing_returns","reason":"no meaningful upside remains","stopped_at":"2026-03-29T10:05:00Z"}}`)

	facts, err := BuildEvolveFacts(runDir)
	if err != nil {
		t.Fatalf("BuildEvolveFacts: %v", err)
	}
	if facts == nil {
		t.Fatal("BuildEvolveFacts returned nil facts")
	}
	if facts.FrontierState != EvolveFrontierStopped {
		t.Fatalf("FrontierState = %q, want %q", facts.FrontierState, EvolveFrontierStopped)
	}
	if facts.LastStopReasonCode != "diminishing_returns" {
		t.Fatalf("LastStopReasonCode = %q, want diminishing_returns", facts.LastStopReasonCode)
	}
	if facts.LastStopAt != "2026-03-29T10:05:00Z" {
		t.Fatalf("LastStopAt = %q, want stop timestamp", facts.LastStopAt)
	}
}

func TestBuildEvolveFactsReopensAfterNewExperiment(t *testing.T) {
	runDir := t.TempDir()
	writeEvolveMetadataForTest(t, runDir)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"evolve.stopped","at":"2026-03-29T10:05:00Z","actor":"master","body":{"reason_code":"diminishing_returns","reason":"no meaningful upside remains","stopped_at":"2026-03-29T10:05:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:10:00Z","actor":"master","body":{"experiment_id":"exp-2","created_at":"2026-03-29T10:10:00Z"}}`)

	facts, err := BuildEvolveFacts(runDir)
	if err != nil {
		t.Fatalf("BuildEvolveFacts: %v", err)
	}
	if facts == nil {
		t.Fatal("BuildEvolveFacts returned nil facts")
	}
	if facts.FrontierState != EvolveFrontierActive {
		t.Fatalf("FrontierState = %q, want %q", facts.FrontierState, EvolveFrontierActive)
	}
	if got := facts.OpenCandidateIDs; !reflect.DeepEqual(got, []string{"exp-1", "exp-2"}) {
		t.Fatalf("OpenCandidateIDs = %v, want [exp-1 exp-2]", got)
	}
}

func TestBuildEvolveFactsExcludesClosedCandidates(t *testing.T) {
	runDir := t.TempDir()
	writeEvolveMetadataForTest(t, runDir)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:02:00Z","actor":"master","body":{"experiment_id":"exp-2","created_at":"2026-03-29T10:02:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.closed","at":"2026-03-29T10:03:00Z","actor":"master","body":{"experiment_id":"exp-1","disposition":"rejected","reason":"loses on latency","closed_at":"2026-03-29T10:03:00Z"}}`)

	facts, err := BuildEvolveFacts(runDir)
	if err != nil {
		t.Fatalf("BuildEvolveFacts: %v", err)
	}
	if facts == nil {
		t.Fatal("BuildEvolveFacts returned nil facts")
	}
	if got := facts.OpenCandidateIDs; !reflect.DeepEqual(got, []string{"exp-2"}) {
		t.Fatalf("OpenCandidateIDs = %v, want [exp-2]", got)
	}
}

func TestBuildEvolveFactsUsesIntegratedBestExperiment(t *testing.T) {
	runDir := t.TempDir()
	writeEvolveMetadataForTest(t, runDir)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:             1,
		CurrentExperimentID: "exp-1",
		CurrentBranch:       "goalx/demo/root",
		CurrentCommit:       "abc1234",
		UpdatedAt:           "2026-03-29T10:00:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}

	facts, err := BuildEvolveFacts(runDir)
	if err != nil {
		t.Fatalf("BuildEvolveFacts: %v", err)
	}
	if facts == nil {
		t.Fatal("BuildEvolveFacts returned nil facts")
	}
	if facts.BestExperimentID != "exp-1" {
		t.Fatalf("BestExperimentID = %q, want exp-1", facts.BestExperimentID)
	}
}

func writeEvolveMetadataForTest(t *testing.T, runDir string) {
	t.Helper()
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version: 1,
		Intent:  runIntentEvolve,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
}

func appendExperimentEventForTest(t *testing.T, runDir, payload string) {
	t.Helper()
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := AppendDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments, []byte(payload)); err != nil {
		t.Fatalf("AppendDurableLog: %v", err)
	}
}
