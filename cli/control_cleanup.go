package cli

import (
	"os"
	"strings"
	"time"
)

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

	if err := expireAllControlLeases(runDir); err != nil {
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

func expireAllControlLeases(runDir string) error {
	if err := ExpireControlLease(runDir, "master"); err != nil {
		return err
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		return err
	}
	entries, err := os.ReadDir(ControlLeasesDir(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		holder := strings.TrimSuffix(entry.Name(), ".json")
		if holder == "" || holder == "master" || holder == "sidecar" {
			continue
		}
		if err := ExpireControlLease(runDir, holder); err != nil {
			return err
		}
	}
	return nil
}
