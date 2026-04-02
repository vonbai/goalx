package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestObjectiveContractPath(t *testing.T) {
	runDir := t.TempDir()
	got := ObjectiveContractPath(runDir)
	want := filepath.Join(runDir, "objective-contract.json")
	if got != want {
		t.Fatalf("ObjectiveContractPath() = %q, want %q", got, want)
	}
}

func TestParseObjectiveContractRejectsDuplicateClauseIDs(t *testing.T) {
	_, err := parseObjectiveContract([]byte(`{
  "version": 1,
  "state": "draft",
  "objective_hash": "sha256:demo",
  "clauses": [
    {
      "id": "ucl-1",
      "text": "ship a real flow",
      "kind": "delivery",
      "source_excerpt": "ship a real flow",
      "required_surfaces": ["goal"]
    },
    {
      "id": "ucl-1",
      "text": "verify the flow",
      "kind": "verification",
      "source_excerpt": "verify the flow",
      "required_surfaces": ["acceptance"]
    }
  ]
}`))
	if err == nil {
		t.Fatal("parseObjectiveContract should reject duplicate clause ids")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate clause error = %v, want duplicate hint", err)
	}
}

func TestParseObjectiveContractRejectsMissingSourceExcerpt(t *testing.T) {
	_, err := parseObjectiveContract([]byte(`{
  "version": 1,
  "state": "draft",
  "objective_hash": "sha256:demo",
  "clauses": [
    {
      "id": "ucl-1",
      "text": "ship a real flow",
      "kind": "delivery",
      "required_surfaces": ["goal"]
    }
  ]
}`))
	if err == nil {
		t.Fatal("parseObjectiveContract should reject missing source_excerpt")
	}
	if !strings.Contains(err.Error(), "source_excerpt") {
		t.Fatalf("missing source_excerpt error = %v, want source_excerpt hint", err)
	}
}

func TestParseObjectiveContractRejectsEmptyRequiredSurfaces(t *testing.T) {
	_, err := parseObjectiveContract([]byte(`{
  "version": 1,
  "state": "draft",
  "objective_hash": "sha256:demo",
  "clauses": [
    {
      "id": "ucl-1",
      "text": "ship a real flow",
      "kind": "delivery",
      "source_excerpt": "ship a real flow",
      "required_surfaces": []
    }
  ]
}`))
	if err == nil {
		t.Fatal("parseObjectiveContract should reject empty required_surfaces")
	}
	if !strings.Contains(err.Error(), "required_surfaces") {
		t.Fatalf("empty required_surfaces error = %v, want required_surfaces hint", err)
	}
}

func TestParseObjectiveContractRejectsInvalidState(t *testing.T) {
	_, err := parseObjectiveContract([]byte(`{
  "version": 1,
  "state": "mutable",
  "objective_hash": "sha256:demo",
  "clauses": [
    {
      "id": "ucl-1",
      "text": "ship a real flow",
      "kind": "delivery",
      "source_excerpt": "ship a real flow",
      "required_surfaces": ["goal"]
    }
  ]
}`))
	if err == nil {
		t.Fatal("parseObjectiveContract should reject invalid state")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Fatalf("invalid state error = %v, want state hint", err)
	}
}

func TestSaveObjectiveContractRejectsLockedRewrite(t *testing.T) {
	runDir := t.TempDir()
	path := ObjectiveContractPath(runDir)
	contract := &ObjectiveContract{
		Version:       1,
		State:         objectiveContractStateLocked,
		ObjectiveHash: "sha256:demo",
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-1",
				Text:             "ship a real flow",
				Kind:             objectiveClauseKindDelivery,
				SourceExcerpt:    "ship a real flow",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
		},
	}
	if err := SaveObjectiveContract(path, contract); err != nil {
		t.Fatalf("SaveObjectiveContract(first): %v", err)
	}
	if err := SaveObjectiveContract(path, contract); err == nil {
		t.Fatal("SaveObjectiveContract should reject rewriting a locked contract")
	}
}

func TestBuildObjectiveIntegritySummaryReportsMissingCoverage(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), &ObjectiveContract{
		Version:       1,
		State:         objectiveContractStateLocked,
		ObjectiveHash: "sha256:demo",
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-goal",
				Text:             "ship the user-facing outcome",
				Kind:             objectiveClauseKindDelivery,
				SourceExcerpt:    "ship the user-facing outcome",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
			{
				ID:               "ucl-accept",
				Text:             "verify the user-facing outcome",
				Kind:             objectiveClauseKindVerification,
				SourceExcerpt:    "verify the user-facing outcome",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceAcceptance},
			},
		},
	}); err != nil {
		t.Fatalf("SaveObjectiveContract: %v", err)
	}

	summary, err := BuildObjectiveIntegritySummary(runDir)
	if err != nil {
		t.Fatalf("BuildObjectiveIntegritySummary: %v", err)
	}
	if !summary.ContractPresent || !summary.ContractLocked {
		t.Fatalf("summary = %+v, want present locked contract", summary)
	}
	if summary.IntegrityOK() {
		t.Fatalf("IntegrityOK() = true, want false when coverage is missing: %+v", summary)
	}
	if len(summary.MissingGoalClauseIDs) != 1 || summary.MissingGoalClauseIDs[0] != "ucl-goal" {
		t.Fatalf("missing goal clauses = %#v, want ucl-goal", summary.MissingGoalClauseIDs)
	}
	if len(summary.MissingAcceptanceClauseIDs) != 1 || summary.MissingAcceptanceClauseIDs[0] != "ucl-accept" {
		t.Fatalf("missing acceptance clauses = %#v, want ucl-accept", summary.MissingAcceptanceClauseIDs)
	}
}

func TestBuildObjectiveIntegritySummaryUsesObligationModelCoverage(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), &ObjectiveContract{
		Version:       1,
		State:         objectiveContractStateLocked,
		ObjectiveHash: "sha256:demo",
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-goal",
				Text:             "ship the user-facing outcome",
				Kind:             objectiveClauseKindDelivery,
				SourceExcerpt:    "ship the user-facing outcome",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
			{
				ID:               "ucl-accept",
				Text:             "verify the user-facing outcome",
				Kind:             objectiveClauseKindVerification,
				SourceExcerpt:    "verify the user-facing outcome",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceAcceptance},
			},
		},
	}); err != nil {
		t.Fatalf("SaveObjectiveContract: %v", err)
	}
	if err := SaveObligationModel(ObligationModelPath(runDir), &ObligationModel{
		Version:               1,
		ObjectiveContractHash: "sha256:demo",
		Required: []ObligationItem{
			{
				ID:                "obl-goal",
				Text:              "ship the outcome",
				Kind:              "outcome",
				CoversClauses:     []string{"ucl-goal"},
				AssuranceRequired: true,
			},
		},
	}); err != nil {
		t.Fatalf("SaveObligationModel: %v", err)
	}
	if err := writeAssuranceFixture(t, runDir, &AcceptanceState{
		Version: 2,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Command: "printf ok", Covers: []string{"ucl-accept"}, State: acceptanceCheckStateActive},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}

	summary, err := BuildObjectiveIntegritySummary(runDir)
	if err != nil {
		t.Fatalf("BuildObjectiveIntegritySummary: %v", err)
	}
	if !summary.IntegrityOK() {
		t.Fatalf("IntegrityOK() = false, want true with obligation+acceptance coverage: %+v", summary)
	}
	if summary.GoalCoveredCount != 1 || summary.AcceptanceCoveredCount != 1 {
		t.Fatalf("coverage counts = %+v, want 1/1", summary)
	}
}
