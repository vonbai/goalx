package cli

import "testing"

func TestApplyPendingControlOpsAppliesQueuedReminderOpsInOrder(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	if _, err := appendControlOp(runDir, controlOpReminderQueue, controlReminderQueueBody{
		DedupeKey: "master-wake",
		Reason:    "control-cycle",
		Target:    "gx-demo:master",
	}); err != nil {
		t.Fatalf("append queue op: %v", err)
	}
	if _, err := appendControlOp(runDir, controlOpReminderSuppress, controlReminderSuppressBody{
		DedupeKey: "master-wake",
	}); err != nil {
		t.Fatalf("append suppress op: %v", err)
	}

	if err := ApplyPendingControlOps(runDir); err != nil {
		t.Fatalf("ApplyPendingControlOps: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 {
		t.Fatalf("reminders len = %d, want 1", len(reminders.Items))
	}
	if !reminders.Items[0].Suppressed {
		t.Fatalf("reminder should be suppressed after ordered ops: %+v", reminders.Items[0])
	}
}

func TestApplyPendingControlOpsIsIdempotent(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	if _, err := appendControlOp(runDir, controlOpReminderQueue, controlReminderQueueBody{
		DedupeKey: "session-wake:session-1",
		Reason:    "session-inbox-unread",
		Target:    "gx-demo:session-1",
		Engine:    "codex",
	}); err != nil {
		t.Fatalf("append queue op: %v", err)
	}

	if err := ApplyPendingControlOps(runDir); err != nil {
		t.Fatalf("ApplyPendingControlOps first: %v", err)
	}
	if err := ApplyPendingControlOps(runDir); err != nil {
		t.Fatalf("ApplyPendingControlOps second: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 {
		t.Fatalf("reminders len = %d, want 1", len(reminders.Items))
	}
}

func TestApplyPendingControlOpsUpsertsRunAndSessionRuntimeState(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	if _, err := appendControlOp(runDir, controlOpRunRuntimeUpsert, controlRunRuntimeUpsertBody{
		State: RunRuntimeState{
			Version:   1,
			Run:       "demo-run",
			Mode:      "develop",
			Active:    true,
			StartedAt: "2026-03-29T12:00:00Z",
		},
	}); err != nil {
		t.Fatalf("append run runtime op: %v", err)
	}
	if _, err := appendControlOp(runDir, controlOpSessionRuntimeUpsert, controlSessionRuntimeUpsertBody{
		State: SessionRuntimeState{
			Name:       "session-1",
			State:      "active",
			Mode:       "develop",
			OwnerScope: "triage auth flow",
		},
	}); err != nil {
		t.Fatalf("append session runtime op: %v", err)
	}

	if err := ApplyPendingControlOps(runDir); err != nil {
		t.Fatalf("ApplyPendingControlOps: %v", err)
	}

	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	if runState == nil || runState.Run != "demo-run" || !runState.Active {
		t.Fatalf("run runtime state = %+v, want active demo-run", runState)
	}
	sessions, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := sessions.Sessions["session-1"].State; got != "active" {
		t.Fatalf("session-1 state = %q, want active", got)
	}
}

func TestApplyPendingControlOpsFinalizesSessionRuntimeStates(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveSessionsRuntimeState(SessionsRuntimeStatePath(runDir), &SessionsRuntimeState{
		Version: 1,
		Sessions: map[string]SessionRuntimeState{
			"session-1": {Name: "session-1", State: "active"},
			"session-2": {Name: "session-2", State: "parked"},
		},
	}); err != nil {
		t.Fatalf("SaveSessionsRuntimeState: %v", err)
	}

	if _, err := appendControlOp(runDir, controlOpSessionsRuntimeFinalize, controlSessionsRuntimeFinalizeBody{
		Lifecycle: "completed",
		UpdatedAt: "2026-03-29T12:34:00Z",
	}); err != nil {
		t.Fatalf("append finalize op: %v", err)
	}

	if err := ApplyPendingControlOps(runDir); err != nil {
		t.Fatalf("ApplyPendingControlOps: %v", err)
	}

	sessions, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	for _, name := range []string{"session-1", "session-2"} {
		if got := sessions.Sessions[name].State; got != "completed" {
			t.Fatalf("%s state = %q, want completed", name, got)
		}
	}
}
