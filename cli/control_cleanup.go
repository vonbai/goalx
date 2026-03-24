package cli

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

func FinalizeControlRun(runDir, lifecycle string) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	killAllLeasedProcesses(runDir)

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
	if err := finalizeSessionRuntimeStates(runDir, lifecycle, now); err != nil {
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

func killAllLeasedProcesses(runDir string) {
	entries, err := os.ReadDir(ControlLeasesDir(runDir))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lease, err := LoadControlLease(filepath.Join(ControlLeasesDir(runDir), entry.Name()))
		if err == nil && lease.PID > 0 {
			KillProcessTree(lease.PID)
		}
	}
}

func finalizeSessionRuntimeStates(runDir, lifecycle, now string) error {
	state, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return err
	}
	if state.Sessions == nil {
		state.Sessions = map[string]SessionRuntimeState{}
	}
	if len(state.Sessions) == 0 {
		indexes, err := existingSessionIndexes(runDir)
		if err != nil {
			return err
		}
		for _, num := range indexes {
			name := SessionName(num)
			state.Sessions[name] = SessionRuntimeState{Name: name}
		}
	}
	for name, session := range state.Sessions {
		session.State = lifecycle
		session.UpdatedAt = now
		state.Sessions[name] = session
	}
	return SaveSessionsRuntimeState(SessionsRuntimeStatePath(runDir), state)
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
