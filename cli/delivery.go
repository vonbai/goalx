package cli

import (
	"fmt"
	"strings"
	"time"
)

func latestSessionDelivery(runDir, sessionName string) (ControlDelivery, bool) {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil || deliveries == nil {
		return ControlDelivery{}, false
	}
	prefix := "session-inbox:" + sessionName + ":"
	dedupe := "session-wake:" + sessionName
	var latest ControlDelivery
	found := false
	for _, item := range deliveries.Items {
		if !strings.HasPrefix(item.DedupeKey, prefix) && item.DedupeKey != dedupe {
			continue
		}
		if !found || item.AttemptedAt > latest.AttemptedAt {
			latest = item
			found = true
		}
	}
	return latest, found
}

func DeliverControlNudge(runDir, messageID, dedupeKey, target, engine string, deliver func(target, engine string) error) (*ControlDelivery, error) {
	return deliverControlNudge(runDir, messageID, dedupeKey, target, engine, true, deliver)
}

func deliverControlNudge(runDir, messageID, dedupeKey, target, engine string, ackOnSuccess bool, deliver func(target, engine string) error) (*ControlDelivery, error) {
	if err := EnsureControlState(runDir); err != nil {
		return nil, err
	}
	dedupeKey = strings.TrimSpace(dedupeKey)
	if dedupeKey == "" {
		dedupeKey = strings.TrimSpace(messageID)
	}
	if dedupeKey == "" {
		return nil, fmt.Errorf("delivery dedupe key is required")
	}
	messageID = strings.TrimSpace(messageID)
	if messageID == "" {
		messageID = dedupeKey
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("delivery target is required")
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		return nil, err
	}

	idx := -1
	for i := range deliveries.Items {
		if deliveries.Items[i].DedupeKey == dedupeKey {
			idx = i
			break
		}
	}
	if idx == -1 {
		deliveries.Items = append(deliveries.Items, ControlDelivery{
			DeliveryID: newControlObjectID("delivery"),
			DedupeKey:  dedupeKey,
		})
		idx = len(deliveries.Items) - 1
	}

	item := &deliveries.Items[idx]
	item.MessageID = messageID
	item.DedupeKey = dedupeKey
	item.Target = target
	item.Adapter = "tmux"
	if item.DeliveryID == "" {
		item.DeliveryID = newControlObjectID("delivery")
	}
	if item.Status == "sent" && item.AckedAt != "" {
		if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries); err != nil {
			return nil, err
		}
		copy := *item
		return &copy, nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	item.AttemptedAt = now
	item.Status = "pending"
	item.LastError = ""
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries); err != nil {
		return nil, err
	}

	if deliver != nil {
		if err := deliver(target, engine); err != nil {
			item.Status = "failed"
			item.LastError = err.Error()
			item.AckedAt = ""
			if saveErr := SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries); saveErr != nil {
				return nil, saveErr
			}
			copy := *item
			return &copy, err
		}
	}

	item.Status = "sent"
	item.LastError = ""
	if ackOnSuccess {
		item.AckedAt = now
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries); err != nil {
		return nil, err
	}
	copy := *item
	return &copy, nil
}

func newControlObjectID(prefix string) string {
	id := newRunID()
	if strings.HasPrefix(id, runIDPrefix) {
		return prefix + id[len(runIDPrefix):]
	}
	return prefix + "_" + id
}
