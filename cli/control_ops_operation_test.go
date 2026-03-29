package cli

import "testing"

func TestApplyPendingControlOpsPublishesOperationTargetsInOrder(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	runTarget := BoundaryEstablishmentOperationKey()
	sessionTarget := SessionDispatchOperationKey("session-2")
	if err := submitControlOperationTarget(runDir, runTarget, ControlOperationTarget{
		Kind:              ControlOperationKindBoundaryEstablishment,
		State:             ControlOperationStateAwaitingAgent,
		Summary:           "boundary still draft",
		PendingConditions: []string{"objective_contract_locked"},
	}); err != nil {
		t.Fatalf("submitControlOperationTarget(runTarget): %v", err)
	}
	if err := submitControlOperationTarget(runDir, sessionTarget, ControlOperationTarget{
		Kind:              ControlOperationKindSessionDispatch,
		State:             ControlOperationStateHandshaking,
		Summary:           "waiting for first frame",
		PendingConditions: []string{"pane_present", "transport_first_frame"},
	}); err != nil {
		t.Fatalf("submitControlOperationTarget(sessionTarget): %v", err)
	}

	state, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlOperationsState: %v", err)
	}
	if got := state.Targets[runTarget].State; got != ControlOperationStateAwaitingAgent {
		t.Fatalf("run state = %q, want %q", got, ControlOperationStateAwaitingAgent)
	}
	if got := state.Targets[sessionTarget].State; got != ControlOperationStateHandshaking {
		t.Fatalf("session-2 state = %q, want %q", got, ControlOperationStateHandshaking)
	}
}

func TestApplyPendingControlOpsUpsertDoesNotClobberOtherTargets(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	bootstrapTarget := RunBootstrapOperationKey()
	boundaryTarget := BoundaryEstablishmentOperationKey()
	if err := submitControlOperationTarget(runDir, bootstrapTarget, ControlOperationTarget{
		Kind:  ControlOperationKindRunBootstrap,
		State: ControlOperationStateCommitted,
	}); err != nil {
		t.Fatalf("submitControlOperationTarget(run): %v", err)
	}
	if err := submitControlOperationTarget(runDir, boundaryTarget, ControlOperationTarget{
		Kind:  ControlOperationKindBoundaryEstablishment,
		State: ControlOperationStateAwaitingAgent,
	}); err != nil {
		t.Fatalf("submitControlOperationTarget(boundary): %v", err)
	}

	state, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlOperationsState: %v", err)
	}
	if got := state.Targets[bootstrapTarget].State; got != ControlOperationStateCommitted {
		t.Fatalf("run state = %q, want %q", got, ControlOperationStateCommitted)
	}
	if got := state.Targets[boundaryTarget].State; got != ControlOperationStateAwaitingAgent {
		t.Fatalf("boundary state = %q, want %q", got, ControlOperationStateAwaitingAgent)
	}
}

func TestApplyPendingControlOpsClearsExplicitTarget(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	target := SessionDispatchOperationKey("session-3")
	if err := submitControlOperationTarget(runDir, target, ControlOperationTarget{
		Kind:  ControlOperationKindSessionDispatch,
		State: ControlOperationStateFailed,
	}); err != nil {
		t.Fatalf("submitControlOperationTarget: %v", err)
	}
	if err := clearControlOperationTarget(runDir, target); err != nil {
		t.Fatalf("clearControlOperationTarget: %v", err)
	}

	state, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlOperationsState: %v", err)
	}
	if _, ok := state.Targets[target]; ok {
		t.Fatal("session-3 target still present after clear")
	}
}
