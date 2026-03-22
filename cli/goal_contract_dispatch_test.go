package cli

import "testing"

func TestSummarizeGoalDispatchSeparatesWaitingExternalFromDispatchable(t *testing.T) {
	state := &GoalContractState{
		Items: []GoalContractItem{
			{
				ID:             "req-1",
				Kind:           goalContractKindUserRequired,
				Requirement:    "resolve the external dependency",
				Status:         goalContractStatusQueued,
				ExecutionState: "waiting_external",
				Owner:          "session-1",
			},
			{
				ID:             "req-2",
				Kind:           goalContractKindGoalNecessary,
				Requirement:    "ship the next slice",
				Status:         goalContractStatusDelegated,
				ExecutionState: "dispatchable",
				Owner:          "session-2",
			},
			{
				ID:          "req-3",
				Kind:        goalContractKindUserRequired,
				Requirement: "fix the blocker",
				Status:      goalContractStatusBlocked,
				Owner:       "master",
			},
			{
				ID:          "enh-1",
				Kind:        goalContractKindGoalEnhancement,
				Requirement: "polish later",
				Status:      goalContractStatusQueued,
			},
		},
	}

	summary := SummarizeGoalDispatch(state)
	if summary.RequiredTotal != 3 {
		t.Fatalf("RequiredTotal = %d, want 3", summary.RequiredTotal)
	}
	if summary.RequiredRemaining != 3 {
		t.Fatalf("RequiredRemaining = %d, want 3", summary.RequiredRemaining)
	}
	if summary.WaitingExternal != 1 {
		t.Fatalf("WaitingExternal = %d, want 1", summary.WaitingExternal)
	}
	if summary.Dispatchable != 1 {
		t.Fatalf("Dispatchable = %d, want 1", summary.Dispatchable)
	}
	if summary.Blocked != 1 {
		t.Fatalf("Blocked = %d, want 1", summary.Blocked)
	}
	if !summary.HasDispatchableWork() {
		t.Fatal("HasDispatchableWork = false, want true")
	}
}
