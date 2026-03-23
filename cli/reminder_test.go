package cli

import (
	"testing"
	"time"
)

func TestQueueControlReminderDedupesByKey(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	first, err := QueueControlReminder(runDir, "master-wake", "control-cycle", "gx-demo:master")
	if err != nil {
		t.Fatalf("QueueControlReminder first: %v", err)
	}
	second, err := QueueControlReminder(runDir, "master-wake", "control-cycle", "gx-demo:master")
	if err != nil {
		t.Fatalf("QueueControlReminder second: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 {
		t.Fatalf("reminders len = %d, want 1", len(reminders.Items))
	}
	if second.ReminderID != first.ReminderID {
		t.Fatalf("second reminder id = %q, want %q", second.ReminderID, first.ReminderID)
	}
}

func TestDeliverDueControlRemindersRespectsCooldownAndCreatesDelivery(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := QueueControlReminder(runDir, "master-wake", "control-cycle", "gx-demo:master"); err != nil {
		t.Fatalf("QueueControlReminder: %v", err)
	}

	calls := 0
	send := func(target, engine string) error {
		calls++
		return nil
	}

	start := time.Now().UTC()
	if err := DeliverDueControlReminders(runDir, "codex", 5*time.Minute, send); err != nil {
		t.Fatalf("DeliverDueControlReminders first: %v", err)
	}
	if err := DeliverDueControlReminders(runDir, "codex", 5*time.Minute, send); err != nil {
		t.Fatalf("DeliverDueControlReminders second: %v", err)
	}

	if calls != 1 {
		t.Fatalf("deliver calls = %d, want 1", calls)
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 {
		t.Fatalf("reminders len = %d, want 1", len(reminders.Items))
	}
	if reminders.Items[0].Attempts != 1 || reminders.Items[0].CooldownUntil == "" {
		t.Fatalf("unexpected reminder: %+v", reminders.Items[0])
	}
	cooldownUntil, err := time.Parse(time.RFC3339, reminders.Items[0].CooldownUntil)
	if err != nil {
		t.Fatalf("parse cooldown_until: %v", err)
	}
	if cooldownUntil.Before(start.Add(4 * time.Minute)) {
		t.Fatalf("cooldown_until = %s, want interval-derived cooldown well above 4 minutes", reminders.Items[0].CooldownUntil)
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "sent" || deliveries.Items[0].DedupeKey != "master-wake" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
	}
}
