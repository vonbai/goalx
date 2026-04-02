package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeBoundaryFixture(t *testing.T, runDir string, state *GoalState) error {
	t.Helper()
	if state == nil {
		t.Fatal("writeBoundaryFixture requires state")
	}
	objectiveContract, err := LoadObjectiveContract(ObjectiveContractPath(runDir))
	if err != nil {
		t.Fatalf("LoadObjectiveContract: %v", err)
	}
	objectiveHash := ""
	objectiveText := ""
	if objectiveContract != nil {
		objectiveHash = objectiveContract.ObjectiveHash
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err == nil && meta != nil {
		objectiveText = meta.Objective
	}
	if objectiveHash == "" {
		objectiveHash = firstNonEmpty(hashObjectiveText(objectiveText), "test-boundary")
	}
	model := obligationModelFromGoalState(state, objectiveContract, objectiveHash, objectiveText)
	model.UpdatedAt = ""
	if err := SaveObligationModel(ObligationModelPath(runDir), model); err != nil {
		t.Fatalf("SaveObligationModel: %v", err)
	}
	return nil
}

func writeAssuranceFixture(t *testing.T, runDir string, state *AcceptanceState) error {
	t.Helper()
	if state == nil {
		t.Fatal("writeAssuranceFixture requires state")
	}
	plan := assurancePlanFromAcceptanceState(state)
	plan.UpdatedAt = ""
	if err := SaveAssurancePlan(AssurancePlanPath(runDir), plan); err != nil {
		t.Fatalf("SaveAssurancePlan: %v", err)
	}
	if strings.TrimSpace(state.LastResult.CheckedAt) == "" && len(state.LastResult.CheckResults) == 0 {
		return nil
	}
	refs := []string{}
	if strings.TrimSpace(state.LastResult.EvidencePath) != "" {
		refs = append(refs, state.LastResult.EvidencePath)
	}
	for _, result := range state.LastResult.CheckResults {
		if strings.TrimSpace(result.EvidencePath) != "" {
			refs = append(refs, result.EvidencePath)
		}
	}
	exitCode := 0
	if state.LastResult.ExitCode != nil {
		exitCode = *state.LastResult.ExitCode
	}
	scenarioID := "scenario-manual"
	if len(plan.Scenarios) > 0 && strings.TrimSpace(plan.Scenarios[0].ID) != "" {
		scenarioID = plan.Scenarios[0].ID
	}
	body, err := json.Marshal(EvidenceEventBody{
		ScenarioID:   scenarioID,
		HarnessKind:  "cli",
		OracleResult: map[string]any{"exit_code": exitCode},
		ArtifactRefs: refs,
	})
	if err != nil {
		t.Fatalf("marshal evidence body: %v", err)
	}
	event := DurableLogEvent{
		Version: 1,
		Kind:    "scenario.executed",
		At:      firstNonEmpty(strings.TrimSpace(state.LastResult.CheckedAt), "2026-03-27T08:00:00Z"),
		Actor:   "test",
		Body:    body,
	}
	line, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal evidence event: %v", err)
	}
	if err := os.WriteFile(EvidenceLogPath(runDir), append(line, '\n'), 0o644); err != nil {
		t.Fatalf("write evidence log: %v", err)
	}
	return nil
}

func writeAssuranceArtifact(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir artifact dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
}
