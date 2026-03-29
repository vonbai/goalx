package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	controlOpReminderQueue                  = "reminder.queue"
	controlOpReminderSuppress               = "reminder.suppress"
	controlOpReminderRecordAttempt          = "reminder.record_attempt"
	controlOpDeliveryPrepare                = "delivery.prepare"
	controlOpDeliveryComplete               = "delivery.complete"
	controlOpDeliveryReconcileTarget        = "delivery.reconcile_target"
	controlOpRunStateProviderDialogAlerts   = "run_state.provider_dialog_alerts"
	controlOpRunStateRequiredFrontierAlerts = "run_state.required_frontier_alerts"
	controlOpFinalizeControlSurfaces        = "control.finalize_surfaces"
	controlOpRunRuntimeUpsert               = "runtime.run.upsert"
	controlOpSessionRuntimeUpsert           = "runtime.session.upsert"
	controlOpSessionRuntimeRemove           = "runtime.session.remove"
	controlOpSessionsRuntimeFinalize        = "runtime.sessions.finalize"
)

type ControlOp struct {
	ID        int64           `json:"id"`
	Kind      string          `json:"kind"`
	CreatedAt string          `json:"created_at"`
	Body      json.RawMessage `json:"body"`
}

type ControlOpsCursor struct {
	LastAppliedID int64  `json:"last_applied_id"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

type controlReminderQueueBody struct {
	DedupeKey string `json:"dedupe_key"`
	Reason    string `json:"reason"`
	Target    string `json:"target"`
	Engine    string `json:"engine,omitempty"`
}

type controlReminderSuppressBody struct {
	DedupeKey string `json:"dedupe_key"`
}

type controlReminderRecordAttemptBody struct {
	DedupeKey     string `json:"dedupe_key"`
	AttemptDelta  int    `json:"attempt_delta"`
	CooldownUntil string `json:"cooldown_until,omitempty"`
}

type controlDeliveryPrepareBody struct {
	MessageID       string `json:"message_id"`
	DedupeKey       string `json:"dedupe_key"`
	Target          string `json:"target"`
	DedupeOnSuccess bool   `json:"dedupe_on_success,omitempty"`
	AttemptedAt     string `json:"attempted_at"`
}

type controlDeliveryCompleteBody struct {
	DedupeKey       string `json:"dedupe_key"`
	SubmitMode      string `json:"submit_mode,omitempty"`
	TransportState  string `json:"transport_state,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	Status          string `json:"status,omitempty"`
	AcceptedAt      string `json:"accepted_at,omitempty"`
	LastAttemptedAt string `json:"attempted_at,omitempty"`
}

type controlDeliveryReconcileTargetBody struct {
	Target     string `json:"target"`
	LastSeenID int64  `json:"last_seen_id"`
	AcceptedAt string `json:"accepted_at,omitempty"`
}

type controlRunStateProviderDialogAlertsBody struct {
	Alerts map[string]string `json:"alerts,omitempty"`
}

type controlRunStateRequiredFrontierAlertsBody struct {
	Alerts map[string]string `json:"alerts,omitempty"`
}

type controlFinalizeControlSurfacesBody struct {
	Lifecycle string `json:"lifecycle"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type controlRunRuntimeUpsertBody struct {
	State RunRuntimeState `json:"state"`
}

type controlSessionRuntimeUpsertBody struct {
	State SessionRuntimeState `json:"state"`
}

type controlSessionRuntimeRemoveBody struct {
	Name string `json:"name"`
}

type controlSessionsRuntimeFinalizeBody struct {
	Lifecycle string `json:"lifecycle"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func ControlOpsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "ops.jsonl")
}

func ControlOpsCursorPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "ops-cursor.json")
}

func controlWriterLockKey(runDir string) string {
	return filepath.Join(ControlDir(runDir), "writer")
}

func LoadControlOpsCursor(path string) (*ControlOpsCursor, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ControlOpsCursor{}, nil
		}
		return nil, err
	}
	state := &ControlOpsCursor{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse control ops cursor: %w", err)
	}
	return state, nil
}

func SaveControlOpsCursor(path string, state *ControlOpsCursor) error {
	if state == nil {
		return fmt.Errorf("control ops cursor is nil")
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal control ops cursor: %w", err)
	}
	return writeFileAtomic(path, data, 0o644)
}

func appendControlOp(runDir, kind string, body any) (ControlOp, error) {
	if err := EnsureControlState(runDir); err != nil {
		return ControlOp{}, err
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return ControlOp{}, err
	}

	var op ControlOp
	err = withExclusiveFileLock(ControlOpsPath(runDir), func() error {
		nextID, err := nextControlOpID(ControlOpsPath(runDir))
		if err != nil {
			return err
		}
		op = ControlOp{
			ID:        nextID,
			Kind:      kind,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Body:      bodyJSON,
		}
		line, err := json.Marshal(op)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(ControlOpsPath(runDir), os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = f.Write(append(line, '\n'))
		return err
	})
	if err != nil {
		return ControlOp{}, err
	}
	return op, nil
}

func ApplyPendingControlOps(runDir string) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	return withExclusiveFileLock(controlWriterLockKey(runDir), func() error {
		cursor, err := LoadControlOpsCursor(ControlOpsCursorPath(runDir))
		if err != nil {
			return err
		}
		var ops []ControlOp
		if err := withExclusiveFileLock(ControlOpsPath(runDir), func() error {
			loaded, err := loadControlOps(ControlOpsPath(runDir))
			if err != nil {
				return err
			}
			ops = loaded
			return nil
		}); err != nil {
			return err
		}
		for _, op := range ops {
			if op.ID <= cursor.LastAppliedID {
				continue
			}
			if err := applyControlOp(runDir, op); err != nil {
				return fmt.Errorf("apply control op %d %s: %w", op.ID, op.Kind, err)
			}
			cursor.LastAppliedID = op.ID
			cursor.UpdatedAt = ""
			if err := SaveControlOpsCursor(ControlOpsCursorPath(runDir), cursor); err != nil {
				return err
			}
		}
		return nil
	})
}

func submitAndApplyControlOp(runDir, kind string, body any) error {
	if _, err := appendControlOp(runDir, kind, body); err != nil {
		return err
	}
	return ApplyPendingControlOps(runDir)
}

func nextControlOpID(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var lastID int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var op ControlOp
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			return 0, err
		}
		if op.ID > lastID {
			lastID = op.ID
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return lastID + 1, nil
}

func loadControlOps(path string) ([]ControlOp, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var ops []ControlOp
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var op ControlOp
		if err := json.Unmarshal([]byte(line), &op); err != nil {
			return nil, err
		}
		ops = append(ops, op)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ops, nil
}

func applyControlOp(runDir string, op ControlOp) error {
	switch op.Kind {
	case controlOpReminderQueue:
		var body controlReminderQueueBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyReminderQueueOp(runDir, body)
	case controlOpReminderSuppress:
		var body controlReminderSuppressBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyReminderSuppressOp(runDir, body)
	case controlOpReminderRecordAttempt:
		var body controlReminderRecordAttemptBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyReminderRecordAttemptOp(runDir, body)
	case controlOpDeliveryPrepare:
		var body controlDeliveryPrepareBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyDeliveryPrepareOp(runDir, body)
	case controlOpDeliveryComplete:
		var body controlDeliveryCompleteBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyDeliveryCompleteOp(runDir, body)
	case controlOpDeliveryReconcileTarget:
		var body controlDeliveryReconcileTargetBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyDeliveryReconcileTargetOp(runDir, body)
	case controlOpRunStateProviderDialogAlerts:
		var body controlRunStateProviderDialogAlertsBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyRunStateProviderDialogAlertsOp(runDir, body)
	case controlOpRunStateRequiredFrontierAlerts:
		var body controlRunStateRequiredFrontierAlertsBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyRunStateRequiredFrontierAlertsOp(runDir, body)
	case controlOpFinalizeControlSurfaces:
		var body controlFinalizeControlSurfacesBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyFinalizeControlSurfacesOp(runDir, body)
	case controlOpRunRuntimeUpsert:
		var body controlRunRuntimeUpsertBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applyRunRuntimeUpsertOp(runDir, body)
	case controlOpSessionRuntimeUpsert:
		var body controlSessionRuntimeUpsertBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applySessionRuntimeUpsertOp(runDir, body)
	case controlOpSessionRuntimeRemove:
		var body controlSessionRuntimeRemoveBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applySessionRuntimeRemoveOp(runDir, body)
	case controlOpSessionsRuntimeFinalize:
		var body controlSessionsRuntimeFinalizeBody
		if err := json.Unmarshal(op.Body, &body); err != nil {
			return err
		}
		return applySessionsRuntimeFinalizeOp(runDir, body)
	}
	return fmt.Errorf("unknown control op kind %q", op.Kind)
}

func applyReminderQueueOp(runDir string, body controlReminderQueueBody) error {
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	for i := range reminders.Items {
		item := &reminders.Items[i]
		if item.DedupeKey != body.DedupeKey {
			continue
		}
		if item.Reason != body.Reason {
			item.Reason = body.Reason
		}
		if item.Target != body.Target {
			item.Target = body.Target
		}
		if item.Engine != body.Engine {
			item.Engine = body.Engine
		}
		if item.Suppressed || item.ResolvedAt != "" {
			item.Suppressed = false
			item.ResolvedAt = ""
			item.CooldownUntil = ""
			item.Attempts = 0
		}
		return SaveControlReminders(ControlRemindersPath(runDir), reminders)
	}
	reminders.Items = append(reminders.Items, ControlReminder{
		ReminderID: newControlObjectID("reminder"),
		DedupeKey:  body.DedupeKey,
		Reason:     body.Reason,
		Target:     body.Target,
		Engine:     body.Engine,
	})
	return SaveControlReminders(ControlRemindersPath(runDir), reminders)
}

func applyReminderSuppressOp(runDir string, body controlReminderSuppressBody) error {
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	changed := false
	for i := range reminders.Items {
		item := &reminders.Items[i]
		if item.DedupeKey != body.DedupeKey || item.Suppressed {
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

func applyReminderRecordAttemptOp(runDir string, body controlReminderRecordAttemptBody) error {
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	for i := range reminders.Items {
		item := &reminders.Items[i]
		if item.DedupeKey != body.DedupeKey {
			continue
		}
		if item.Suppressed || item.ResolvedAt != "" {
			return nil
		}
		item.Attempts += body.AttemptDelta
		if item.Attempts < 0 {
			item.Attempts = 0
		}
		item.CooldownUntil = body.CooldownUntil
		return SaveControlReminders(ControlRemindersPath(runDir), reminders)
	}
	return nil
}

func applyDeliveryPrepareOp(runDir string, body controlDeliveryPrepareBody) error {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		return err
	}
	idx := -1
	for i := range deliveries.Items {
		if deliveries.Items[i].DedupeKey == body.DedupeKey {
			idx = i
			break
		}
	}
	if idx == -1 {
		deliveries.Items = append(deliveries.Items, ControlDelivery{
			DeliveryID: newControlObjectID("delivery"),
			DedupeKey:  body.DedupeKey,
		})
		idx = len(deliveries.Items) - 1
	}

	item := &deliveries.Items[idx]
	item.MessageID = body.MessageID
	item.DedupeKey = body.DedupeKey
	item.Target = body.Target
	item.Adapter = "tmux"
	if item.DeliveryID == "" {
		item.DeliveryID = newControlObjectID("delivery")
	}
	if body.DedupeOnSuccess && item.Status == "accepted" && item.AcceptedAt != "" {
		return SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries)
	}
	item.AttemptedAt = body.AttemptedAt
	item.Status = "pending"
	item.SubmitMode = ""
	item.TransportState = ""
	item.LastError = ""
	item.AcceptedAt = ""
	return SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries)
}

func applyDeliveryCompleteOp(runDir string, body controlDeliveryCompleteBody) error {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		return err
	}
	for i := range deliveries.Items {
		item := &deliveries.Items[i]
		if item.DedupeKey != body.DedupeKey {
			continue
		}
		item.SubmitMode = body.SubmitMode
		item.TransportState = body.TransportState
		item.Status = body.Status
		item.LastError = body.LastError
		item.AcceptedAt = body.AcceptedAt
		if strings.TrimSpace(item.AttemptedAt) == "" {
			item.AttemptedAt = body.LastAttemptedAt
		}
		return SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries)
	}
	return nil
}

func applyDeliveryReconcileTargetOp(runDir string, body controlDeliveryReconcileTargetBody) error {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		return err
	}
	cursor := &MasterCursorState{
		LastSeenID: body.LastSeenID,
		UpdatedAt:  body.AcceptedAt,
	}
	if !reconcileInboxDeliveryAcceptance(deliveries, body.Target, cursor) {
		return nil
	}
	return SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries)
}

func applyRunStateProviderDialogAlertsOp(runDir string, body controlRunStateProviderDialogAlertsBody) error {
	state, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	if len(body.Alerts) == 0 {
		state.ProviderDialogAlerts = nil
	} else {
		state.ProviderDialogAlerts = body.Alerts
	}
	state.UpdatedAt = ""
	return SaveControlRunState(ControlRunStatePath(runDir), state)
}

func applyRunStateRequiredFrontierAlertsOp(runDir string, body controlRunStateRequiredFrontierAlertsBody) error {
	state, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	if len(body.Alerts) == 0 {
		state.RequiredFrontierAlerts = nil
	} else {
		state.RequiredFrontierAlerts = body.Alerts
	}
	state.UpdatedAt = ""
	return SaveControlRunState(ControlRunStatePath(runDir), state)
}

func applyFinalizeControlSurfacesOp(runDir string, body controlFinalizeControlSurfacesBody) error {
	if strings.TrimSpace(body.UpdatedAt) == "" {
		body.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	runState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	runState.LifecycleState = body.Lifecycle
	runState.UpdatedAt = body.UpdatedAt
	if err := SaveControlRunState(ControlRunStatePath(runDir), runState); err != nil {
		return err
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		return err
	}
	for i := range reminders.Items {
		reminders.Items[i].Suppressed = true
		reminders.Items[i].ResolvedAt = body.UpdatedAt
		reminders.Items[i].CooldownUntil = body.UpdatedAt
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
		if deliveries.Items[i].Status == "sent" && deliveries.Items[i].AcceptedAt == "" {
			deliveries.Items[i].AcceptedAt = body.UpdatedAt
		}
	}
	return SaveControlDeliveries(ControlDeliveriesPath(runDir), deliveries)
}

func applyRunRuntimeUpsertOp(runDir string, body controlRunRuntimeUpsertBody) error {
	return putRunRuntimeStateDirect(runDir, body.State)
}

func applySessionRuntimeUpsertOp(runDir string, body controlSessionRuntimeUpsertBody) error {
	return upsertSessionRuntimeStateDirect(runDir, body.State)
}

func applySessionRuntimeRemoveOp(runDir string, body controlSessionRuntimeRemoveBody) error {
	return removeSessionRuntimeStateDirect(runDir, body.Name)
}

func applySessionsRuntimeFinalizeOp(runDir string, body controlSessionsRuntimeFinalizeBody) error {
	return finalizeSessionRuntimeStatesDirect(runDir, body.Lifecycle, body.UpdatedAt)
}
