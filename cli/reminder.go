package cli

import "time"

func QueueControlReminder(runDir, dedupeKey, reason, target string) (*ControlReminder, error) {
	return QueueControlReminderWithEngine(runDir, dedupeKey, reason, target, "")
}

func QueueControlReminderWithEngine(runDir, dedupeKey, reason, target, engine string) (*ControlReminder, error) {
	if err := EnsureControlState(runDir); err != nil {
		return nil, err
	}
	if err := submitAndApplyControlOp(runDir, controlOpReminderQueue, controlReminderQueueBody{
		DedupeKey: dedupeKey,
		Reason:    reason,
		Target:    target,
		Engine:    engine,
	}); err != nil {
		return nil, err
	}
	return findControlReminderByDedupeKey(runDir, dedupeKey)
}

func SuppressControlReminder(runDir, dedupeKey string) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	return submitAndApplyControlOp(runDir, controlOpReminderSuppress, controlReminderSuppressBody{
		DedupeKey: dedupeKey,
	})
}

func DeliverDueControlReminders(runDir, engine string, interval time.Duration, deliver TransportDeliverFunc) error {
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
		if item.Suppressed || item.ResolvedAt != "" {
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
		delivery, _ := deliverControlNudge(runDir, item.ReminderID, item.DedupeKey, item.Target, deliveryEngine, false, deliver)
		nextAttempts := item.Attempts + 1
		nextCooldown := now.Add(controlReminderCooldown(interval, nextAttempts, delivery)).Format(time.RFC3339)
		if err := submitAndApplyControlOp(runDir, controlOpReminderRecordAttempt, controlReminderRecordAttemptBody{
			DedupeKey:     item.DedupeKey,
			AttemptDelta:  1,
			CooldownUntil: nextCooldown,
		}); err != nil {
			return err
		}
		changed = true
	}
	if !changed {
		return nil
	}
	return nil
}

func controlReminderCooldown(interval time.Duration, attempts int, delivery *ControlDelivery) time.Duration {
	base := interval
	if base < time.Minute {
		base = time.Minute
	}
	if delivery == nil {
		return base
	}
	switch delivery.Status {
	case string(TUIStateBufferedInput):
		cooldown := interval / 4
		if cooldown < 5*time.Second {
			cooldown = 5 * time.Second
		}
		if cooldown > 30*time.Second {
			cooldown = 30 * time.Second
		}
		return cooldown
	case "accepted":
		return base
	case "failed":
		multiplier := attempts
		if multiplier < 1 {
			multiplier = 1
		}
		if multiplier > 4 {
			multiplier = 4
		}
		return time.Duration(multiplier) * base
	}
	return base
}

func findControlReminderByDedupeKey(runDir, dedupeKey string) (*ControlReminder, error) {
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return nil, err
	}
	for i := range reminders.Items {
		if reminders.Items[i].DedupeKey != dedupeKey {
			continue
		}
		copy := reminders.Items[i]
		return &copy, nil
	}
	return nil, nil
}
