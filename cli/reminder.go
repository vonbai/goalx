package cli

import "time"

func QueueControlReminder(runDir, dedupeKey, reason, target string) (*ControlReminder, error) {
	return QueueControlReminderWithEngine(runDir, dedupeKey, reason, target, "")
}

func QueueControlReminderWithEngine(runDir, dedupeKey, reason, target, engine string) (*ControlReminder, error) {
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
			if item.Engine == "" && engine != "" {
				item.Engine = engine
				if err := SaveControlReminders(ControlRemindersPath(runDir), reminders); err != nil {
					return nil, err
				}
			}
			copy := *item
			return &copy, nil
		}
	}
	item := ControlReminder{
		ReminderID: newControlObjectID("reminder"),
		DedupeKey:  dedupeKey,
		Reason:     reason,
		Target:     target,
		Engine:     engine,
	}
	reminders.Items = append(reminders.Items, item)
	if err := SaveControlReminders(ControlRemindersPath(runDir), reminders); err != nil {
		return nil, err
	}
	copy := item
	return &copy, nil
}

func SuppressControlReminder(runDir, dedupeKey string) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	changed := false
	for i := range reminders.Items {
		item := &reminders.Items[i]
		if item.DedupeKey != dedupeKey || item.Suppressed {
			continue
		}
		item.Suppressed = true
		changed = true
	}
	if !changed {
		return nil
	}
	return SaveControlReminders(ControlRemindersPath(runDir), reminders)
}

func DeliverDueControlReminders(runDir, engine string, interval time.Duration, deliver func(target, engine string) error) error {
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
		deliveryEngine := item.Engine
		if deliveryEngine == "" {
			deliveryEngine = engine
		}
		_, _ = deliverControlNudge(runDir, item.ReminderID, item.DedupeKey, item.Target, deliveryEngine, false, deliver)
		item.Attempts++
		item.CooldownUntil = now.Add(controlReminderCooldown(interval)).Format(time.RFC3339)
		changed = true
	}
	if !changed {
		return nil
	}
	return SaveControlReminders(ControlRemindersPath(runDir), reminders)
}

func controlReminderCooldown(interval time.Duration) time.Duration {
	if interval < time.Minute {
		return time.Minute
	}
	return interval
}
