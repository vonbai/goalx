package cli

import "testing"

func TestLoadCanonicalGoalStateUsesObligationModelWhenPresent(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveObligationModel(ObligationModelPath(runDir), &ObligationModel{
		Version:               1,
		ObjectiveContractHash: "sha256:objective",
		Required: []ObligationItem{
			{ID: "obl-1", Text: "ship feature", Kind: "outcome", CoversClauses: []string{"ucl-1"}},
		},
	}); err != nil {
		t.Fatalf("SaveObligationModel: %v", err)
	}

	state, err := LoadCanonicalGoalState(runDir)
	if err != nil {
		t.Fatalf("LoadCanonicalGoalState: %v", err)
	}
	if state == nil || len(state.Required) != 1 || state.Required[0].ID != "obl-1" {
		t.Fatalf("canonical goal state = %#v, want projected obligation", state)
	}
}
