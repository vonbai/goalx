package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type ControlRunIdentity struct {
	Version     int    `json:"version"`
	RunID       string `json:"run_id,omitempty"`
	ProjectID   string `json:"project_id,omitempty"`
	ProjectRoot string `json:"project_root,omitempty"`
	RunName     string `json:"run_name,omitempty"`
	Epoch       int    `json:"epoch,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

type ControlRunState struct {
	Version            int    `json:"version"`
	LifecycleState     string `json:"lifecycle_state,omitempty"`
	Phase              string `json:"phase,omitempty"`
	Recommendation     string `json:"recommendation,omitempty"`
	ActiveSessionCount int    `json:"active_session_count,omitempty"`
	LastEventID        int64  `json:"last_event_id,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
}

type ControlLease struct {
	Version   int    `json:"version"`
	Holder    string `json:"holder,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	Epoch     int    `json:"epoch,omitempty"`
	RenewedAt string `json:"renewed_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	PID       int    `json:"pid,omitempty"`
	Transport string `json:"transport,omitempty"`
}

type ControlReminder struct {
	ReminderID    string `json:"reminder_id,omitempty"`
	DedupeKey     string `json:"dedupe_key,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Target        string `json:"target,omitempty"`
	CooldownUntil string `json:"cooldown_until,omitempty"`
	Attempts      int    `json:"attempts,omitempty"`
	AckedAt       string `json:"acked_at,omitempty"`
	Suppressed    bool   `json:"suppressed,omitempty"`
}

type ControlReminders struct {
	Version   int               `json:"version"`
	Items     []ControlReminder `json:"items"`
	UpdatedAt string            `json:"updated_at,omitempty"`
}

type ControlDelivery struct {
	DeliveryID  string `json:"delivery_id,omitempty"`
	MessageID   string `json:"message_id,omitempty"`
	DedupeKey   string `json:"dedupe_key,omitempty"`
	Target      string `json:"target,omitempty"`
	Adapter     string `json:"adapter,omitempty"`
	Status      string `json:"status,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	AttemptedAt string `json:"attempted_at,omitempty"`
	AckedAt     string `json:"acked_at,omitempty"`
}

type ControlDeliveries struct {
	Version   int               `json:"version"`
	Items     []ControlDelivery `json:"items"`
	UpdatedAt string            `json:"updated_at,omitempty"`
}

func ControlRunIdentityPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "run-identity.json")
}

func ControlRunStatePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "run-state.json")
}

func ControlEventsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "events.jsonl")
}

func ControlLeasesDir(runDir string) string {
	return filepath.Join(ControlDir(runDir), "leases")
}

func ControlLeasePath(runDir, holder string) string {
	return filepath.Join(ControlLeasesDir(runDir), holder+".json")
}

func ControlInboxDir(runDir string) string {
	return filepath.Join(ControlDir(runDir), "inbox")
}

func ControlInboxPath(runDir, target string) string {
	return filepath.Join(ControlInboxDir(runDir), target+".jsonl")
}

func ControlRemindersPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "reminders.json")
}

func ControlDeliveriesPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "deliveries.json")
}

func EnsureControlState(runDir string) error {
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		return fmt.Errorf("mkdir control dir: %w", err)
	}
	if err := os.MkdirAll(ControlLeasesDir(runDir), 0o755); err != nil {
		return fmt.Errorf("mkdir leases dir: %w", err)
	}
	if err := os.MkdirAll(ControlInboxDir(runDir), 0o755); err != nil {
		return fmt.Errorf("mkdir inbox dir: %w", err)
	}
	for _, path := range []string{
		ControlEventsPath(runDir),
		ControlInboxPath(runDir, "master"),
	} {
		if err := ensureEmptyFile(path); err != nil {
			return err
		}
	}
	if _, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveControlRunIdentity(ControlRunIdentityPath(runDir), deriveControlRunIdentity(runDir)); err != nil {
			return err
		}
	}
	if _, err := LoadControlRunState(ControlRunStatePath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveControlRunState(ControlRunStatePath(runDir), deriveControlRunState(runDir)); err != nil {
			return err
		}
	}
	if _, err := LoadControlLease(ControlLeasePath(runDir, "master")); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveControlLease(ControlLeasePath(runDir, "master"), &ControlLease{Version: 1, Holder: "master"}); err != nil {
			return err
		}
	}
	if _, err := LoadControlLease(ControlLeasePath(runDir, "sidecar")); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveControlLease(ControlLeasePath(runDir, "sidecar"), &ControlLease{Version: 1, Holder: "sidecar"}); err != nil {
			return err
		}
	}
	if _, err := LoadControlReminders(ControlRemindersPath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveControlReminders(ControlRemindersPath(runDir), &ControlReminders{Version: 1, Items: []ControlReminder{}}); err != nil {
			return err
		}
	}
	if _, err := LoadControlDeliveries(ControlDeliveriesPath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{Version: 1, Items: []ControlDelivery{}}); err != nil {
			return err
		}
	}
	return nil
}

func LoadControlRunIdentity(path string) (*ControlRunIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &ControlRunIdentity{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse control run identity: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveControlRunIdentity(path string, state *ControlRunIdentity) error {
	if state == nil {
		return fmt.Errorf("control run identity is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.CreatedAt == "" {
		state.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(path, state)
}

func LoadControlRunState(path string) (*ControlRunState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &ControlRunState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse control run state: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveControlRunState(path string, state *ControlRunState) error {
	if state == nil {
		return fmt.Errorf("control run state is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(path, state)
}

func LoadControlLease(path string) (*ControlLease, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &ControlLease{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse control lease: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveControlLease(path string, state *ControlLease) error {
	if state == nil {
		return fmt.Errorf("control lease is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return writeJSONFile(path, state)
}

func RenewControlLease(runDir, holder, runID string, epoch int, ttl time.Duration, transport string, pid int) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	now := time.Now().UTC()
	return SaveControlLease(ControlLeasePath(runDir, holder), &ControlLease{
		Version:   1,
		Holder:    holder,
		RunID:     runID,
		Epoch:     epoch,
		RenewedAt: now.Format(time.RFC3339),
		ExpiresAt: now.Add(ttl).Format(time.RFC3339),
		PID:       pid,
		Transport: transport,
	})
}

func ExpireControlLease(runDir, holder string) error {
	now := time.Now().UTC()
	lease, err := LoadControlLease(ControlLeasePath(runDir, holder))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		lease = &ControlLease{Version: 1, Holder: holder}
	}
	lease.Holder = holder
	lease.RenewedAt = now.Format(time.RFC3339)
	lease.ExpiresAt = now.Format(time.RFC3339)
	lease.RunID = ""
	lease.PID = 0
	return SaveControlLease(ControlLeasePath(runDir, holder), lease)
}

func LoadControlReminders(path string) (*ControlReminders, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &ControlReminders{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		state.Items = []ControlReminder{}
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse control reminders: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Items == nil {
		state.Items = []ControlReminder{}
	}
	return state, nil
}

func SaveControlReminders(path string, state *ControlReminders) error {
	if state == nil {
		return fmt.Errorf("control reminders is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Items == nil {
		state.Items = []ControlReminder{}
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func LoadControlDeliveries(path string) (*ControlDeliveries, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &ControlDeliveries{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		state.Items = []ControlDelivery{}
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse control deliveries: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Items == nil {
		state.Items = []ControlDelivery{}
	}
	return state, nil
}

func SaveControlDeliveries(path string, state *ControlDeliveries) error {
	if state == nil {
		return fmt.Errorf("control deliveries is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Items == nil {
		state.Items = []ControlDelivery{}
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func deriveControlRunIdentity(runDir string) *ControlRunIdentity {
	now := time.Now().UTC().Format(time.RFC3339)
	identity := &ControlRunIdentity{
		Version:   1,
		RunID:     newRunID(),
		RunName:   filepath.Base(runDir),
		Epoch:     1,
		CreatedAt: now,
	}
	if cfg, err := LoadRunSpec(runDir); err == nil && cfg != nil && cfg.Name != "" {
		identity.RunName = cfg.Name
	}
	if meta, err := LoadRunMetadata(RunMetadataPath(runDir)); err == nil && meta != nil {
		if meta.RunID != "" {
			identity.RunID = meta.RunID
		}
		if meta.ProjectRoot != "" {
			identity.ProjectRoot = meta.ProjectRoot
			identity.ProjectID = goalx.ProjectID(meta.ProjectRoot)
		}
		if meta.Epoch > 0 {
			identity.Epoch = meta.Epoch
		}
		if meta.StartedAt != "" {
			identity.CreatedAt = meta.StartedAt
		}
	}
	return identity
}

func deriveControlRunState(runDir string) *ControlRunState {
	now := time.Now().UTC().Format(time.RFC3339)
	state := &ControlRunState{
		Version:        1,
		LifecycleState: "active",
		UpdatedAt:      now,
	}
	if runtime, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir)); err == nil && runtime != nil {
		switch {
		case runtime.StoppedAt != "":
			state.LifecycleState = "stopped"
		case runtime.Active:
			state.LifecycleState = "active"
		default:
			state.LifecycleState = "inactive"
		}
		state.Phase = runtime.Phase
		state.Recommendation = runtime.Recommendation
		if runtime.UpdatedAt != "" {
			state.UpdatedAt = runtime.UpdatedAt
		}
	}
	if sessions, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir)); err == nil && sessions != nil {
		state.ActiveSessionCount = len(sessions.Sessions)
	}
	return state
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
