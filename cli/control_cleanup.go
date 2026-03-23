package cli

import "time"

func FinalizeControlRun(runDir, lifecycle string) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	runState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	runState.LifecycleState = lifecycle
	runState.UpdatedAt = now
	if err := SaveControlRunState(ControlRunStatePath(runDir), runState); err != nil {
		return err
	}

	if err := ExpireControlLease(runDir, "master"); err != nil {
		return err
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		return err
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	for i := range reminders.Items {
		reminders.Items[i].Suppressed = true
		reminders.Items[i].AckedAt = now
		reminders.Items[i].CooldownUntil = now
	}
	if err := SaveControlReminders(ControlRemindersPath(runDir), reminders); err != nil {
		return err
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		return err
	}
	for i := range deliveries.Items {
		if deliveries.Items[i].Status != "sent" {
			deliveries.Items[i].Status = "cancelled"
		}
		if deliveries.Items[i].AckedAt == "" {
			deliveries.Items[i].AckedAt = now
		}
	}
	return SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries)
}
