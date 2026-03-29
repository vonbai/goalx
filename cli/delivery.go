package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type TransportDeliveryOutcome struct {
	SubmitMode     string
	TransportState string
}

type TransportDeliverFunc func(target, engine string) (TransportDeliveryOutcome, error)

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

func latestSessionInboxDelivery(runDir, sessionName string) (ControlDelivery, bool) {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil || deliveries == nil {
		return ControlDelivery{}, false
	}
	prefix := "session-inbox:" + sessionName + ":"
	var latest ControlDelivery
	found := false
	for _, item := range deliveries.Items {
		if !strings.HasPrefix(item.DedupeKey, prefix) {
			continue
		}
		if !found || item.AttemptedAt > latest.AttemptedAt {
			latest = item
			found = true
		}
	}
	return latest, found
}

func latestTargetDelivery(runDir, logicalTarget string) (ControlDelivery, bool) {
	logicalTarget = strings.TrimSpace(logicalTarget)
	if logicalTarget == "" {
		return ControlDelivery{}, false
	}
	if logicalTarget == "master" {
		deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
		if err != nil || deliveries == nil {
			return ControlDelivery{}, false
		}
		var latest ControlDelivery
		found := false
		for _, item := range deliveries.Items {
			if !strings.HasPrefix(item.DedupeKey, "master-") && !strings.HasPrefix(item.DedupeKey, "session-") {
				continue
			}
			if !strings.Contains(item.Target, ":master") {
				continue
			}
			if !found || item.AttemptedAt > latest.AttemptedAt {
				latest = item
				found = true
			}
		}
		return latest, found
	}
	return latestSessionDelivery(runDir, logicalTarget)
}

func deliveryAcceptedWithin(delivery ControlDelivery, window time.Duration, now time.Time) bool {
	return deliveryTimestampWithin(delivery.AcceptedAt, window, now)
}

func deliveryTimestampWithin(ts string, window time.Duration, now time.Time) bool {
	if window <= 0 || strings.TrimSpace(ts) == "" {
		return false
	}
	at, err := time.Parse(time.RFC3339, strings.TrimSpace(ts))
	if err != nil {
		return false
	}
	return now.Sub(at) < window
}

func DeliverControlNudge(runDir, messageID, dedupeKey, target, engine string, deliver TransportDeliverFunc) (*ControlDelivery, error) {
	return deliverControlNudge(runDir, messageID, dedupeKey, target, engine, true, deliver)
}

func reconcileControlDeliveries(runDir string) error {
	if cursor, err := LoadMasterCursorState(MasterCursorPath(runDir)); err == nil {
		if err := submitAndApplyControlOp(runDir, controlOpDeliveryReconcileTarget, controlDeliveryReconcileTargetBody{
			Target:     "master",
			LastSeenID: cursor.LastSeenID,
			AcceptedAt: cursor.UpdatedAt,
		}); err != nil {
			return err
		}
	}
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		cursor, err := LoadMasterCursorState(SessionCursorPath(runDir, sessionName))
		if err != nil {
			continue
		}
		if err := submitAndApplyControlOp(runDir, controlOpDeliveryReconcileTarget, controlDeliveryReconcileTargetBody{
			Target:     sessionName,
			LastSeenID: cursor.LastSeenID,
			AcceptedAt: cursor.UpdatedAt,
		}); err != nil {
			return err
		}
	}
	return nil
}

func reconcileTargetDeliveries(runDir, target string, cursor *MasterCursorState) error {
	if cursor == nil {
		return nil
	}
	if err := submitAndApplyControlOp(runDir, controlOpDeliveryReconcileTarget, controlDeliveryReconcileTargetBody{
		Target:     target,
		LastSeenID: cursor.LastSeenID,
		AcceptedAt: cursor.UpdatedAt,
	}); err != nil {
		return err
	}
	return nil
}

func reconcileInboxDeliveryAcceptance(deliveries *ControlDeliveries, target string, cursor *MasterCursorState) bool {
	if deliveries == nil || cursor == nil || cursor.LastSeenID <= 0 {
		return false
	}
	acceptedAt := strings.TrimSpace(cursor.UpdatedAt)
	if acceptedAt == "" {
		acceptedAt = time.Now().UTC().Format(time.RFC3339)
	}
	updated := false
	for i := range deliveries.Items {
		item := &deliveries.Items[i]
		if item.AcceptedAt != "" {
			continue
		}
		switch strings.TrimSpace(item.Status) {
		case "failed", "cancelled":
			continue
		}
		messageID, ok := controlInboxDeliveryMessageID(target, item.MessageID)
		if !ok {
			messageID, ok = controlInboxDeliveryMessageID(target, item.DedupeKey)
		}
		if !ok || messageID > cursor.LastSeenID {
			continue
		}
		item.Status = "accepted"
		item.LastError = ""
		item.AcceptedAt = acceptedAt
		if strings.TrimSpace(item.AttemptedAt) == "" {
			item.AttemptedAt = acceptedAt
		}
		updated = true
	}
	return updated
}

func controlInboxDeliveryMessageID(target, raw string) (int64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	prefix := "master-inbox:"
	if target != "master" {
		prefix = "session-inbox:" + target + ":"
	}
	if !strings.HasPrefix(raw, prefix) {
		return 0, false
	}
	id, err := strconv.ParseInt(strings.TrimPrefix(raw, prefix), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

func deliverControlNudge(runDir, messageID, dedupeKey, target, engine string, dedupeOnSuccess bool, deliver TransportDeliverFunc) (*ControlDelivery, error) {
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

	now := time.Now().UTC().Format(time.RFC3339)
	if err := submitAndApplyControlOp(runDir, controlOpDeliveryPrepare, controlDeliveryPrepareBody{
		MessageID:       messageID,
		DedupeKey:       dedupeKey,
		Target:          target,
		DedupeOnSuccess: dedupeOnSuccess,
		AttemptedAt:     now,
	}); err != nil {
		return nil, err
	}
	item, err := findControlDeliveryByDedupeKey(runDir, dedupeKey)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, fmt.Errorf("delivery %q missing after prepare", dedupeKey)
	}
	if dedupeOnSuccess && item.Status == "accepted" && item.AcceptedAt != "" {
		return item, nil
	}

	outcome := TransportDeliveryOutcome{}
	if deliver != nil {
		result, err := deliver(target, engine)
		outcome = result
		if err != nil {
			if saveErr := submitAndApplyControlOp(runDir, controlOpDeliveryComplete, controlDeliveryCompleteBody{
				DedupeKey:       dedupeKey,
				SubmitMode:      outcome.SubmitMode,
				TransportState:  "",
				Status:          "failed",
				LastError:       err.Error(),
				AcceptedAt:      "",
				LastAttemptedAt: now,
			}); saveErr != nil {
				return nil, saveErr
			}
			copy, loadErr := findControlDeliveryByDedupeKey(runDir, dedupeKey)
			if loadErr != nil {
				return nil, loadErr
			}
			return copy, err
		}
	}

	transportState := strings.TrimSpace(outcome.TransportState)
	if transportState == "" {
		transportState = string(TUIStateUnknown)
	}
	status := transportState
	acceptedAt := ""
	if isAcceptedTUITransportState(transportState) {
		status = "accepted"
		acceptedAt = now
	}
	if err := submitAndApplyControlOp(runDir, controlOpDeliveryComplete, controlDeliveryCompleteBody{
		DedupeKey:       dedupeKey,
		SubmitMode:      outcome.SubmitMode,
		TransportState:  transportState,
		Status:          status,
		LastError:       "",
		AcceptedAt:      acceptedAt,
		LastAttemptedAt: now,
	}); err != nil {
		return nil, err
	}
	return findControlDeliveryByDedupeKey(runDir, dedupeKey)
}

func findControlDeliveryByDedupeKey(runDir, dedupeKey string) (*ControlDelivery, error) {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		return nil, err
	}
	for i := range deliveries.Items {
		if deliveries.Items[i].DedupeKey != dedupeKey {
			continue
		}
		copy := deliveries.Items[i]
		return &copy, nil
	}
	return nil, nil
}

func newControlObjectID(prefix string) string {
	id := newRunID()
	if strings.HasPrefix(id, runIDPrefix) {
		return prefix + id[len(runIDPrefix):]
	}
	return prefix + "_" + id
}
