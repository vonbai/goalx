package cli

import "time"

const controlReminderCooldown = 30 * time.Second

func QueueControlReminder(runDir, dedupeKey, reason, target string) (*ControlReminder, error) {
	if err := EnsureControlState(runDir); err != nil {
		return nil, err
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return nil, err
	}
	for i := range reminders.Items {
		item := &reminders.Items[i]
		if item.DedupeKey == dedupeKey && !item.Suppressed && item.AckedAt == "" {
			copy := *item
			return &copy, nil
		}
	}
	item := ControlReminder{
		ReminderID: newControlObjectID("reminder"),
		DedupeKey:  dedupeKey,
		Reason:     reason,
		Target:     target,
	}
	reminders.Items = append(reminders.Items, item)
	if err := SaveControlReminders(ControlRemindersPath(runDir), reminders); err != nil {
		return nil, err
	}
	copy := item
	return &copy, nil
}

func DeliverDueControlReminders(runDir, engine string, deliver func(target, engine string) error) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	changed := false
	for i := range reminders.Items {
		item := &reminders.Items[i]
		if item.Suppressed || item.AckedAt != "" {
			continue
		}
		if item.CooldownUntil != "" {
			cooldownUntil, err := time.Parse(time.RFC3339, item.CooldownUntil)
			if err == nil && cooldownUntil.After(now) {
				continue
			}
		}
		_, _ = deliverControlNudge(runDir, item.ReminderID, item.DedupeKey, item.Target, engine, false, deliver)
		item.Attempts++
		item.CooldownUntil = now.Add(controlReminderCooldown).Format(time.RFC3339)
		changed = true
	}
	if !changed {
		return nil
	}
	return SaveControlReminders(ControlRemindersPath(runDir), reminders)
}
