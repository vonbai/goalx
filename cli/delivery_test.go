package cli

import (
	"errors"
	"testing"
)

func TestDeliverControlNudgeRecordsSentAndDedupesByKey(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	calls := 0
	send := func(target, engine string) error {
		calls++
		return nil
	}

	if _, err := DeliverControlNudge(runDir, "tell:1", "tell:1", "gx-demo:master", "codex", send); err != nil {
		t.Fatalf("DeliverControlNudge first: %v", err)
	}
	if _, err := DeliverControlNudge(runDir, "tell:1", "tell:1", "gx-demo:master", "codex", send); err != nil {
		t.Fatalf("DeliverControlNudge second: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 {
		t.Fatalf("deliveries len = %d, want 1", len(deliveries.Items))
	}
	if deliveries.Items[0].Status != "sent" || deliveries.Items[0].DedupeKey != "tell:1" {
		t.Fatalf("unexpected delivery: %+v", deliveries.Items[0])
	}
	if calls != 1 {
		t.Fatalf("deliver calls = %d, want 1", calls)
	}
}

func TestDeliverControlNudgeRecordsFailure(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	_, err := DeliverControlNudge(runDir, "tell:2", "tell:2", "gx-demo:master", "codex", func(target, engine string) error {
		return errors.New("tmux unavailable")
	})
	if err == nil {
		t.Fatal("DeliverControlNudge error = nil, want failure")
	}

	deliveries, loadErr := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if loadErr != nil {
		t.Fatalf("LoadControlDeliveries: %v", loadErr)
	}
	if len(deliveries.Items) != 1 {
		t.Fatalf("deliveries len = %d, want 1", len(deliveries.Items))
	}
	if deliveries.Items[0].Status != "failed" || deliveries.Items[0].LastError == "" {
		t.Fatalf("unexpected delivery: %+v", deliveries.Items[0])
	}
}
