package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type ActivitySnapshot struct {
	Version   int                             `json:"version"`
	CheckedAt string                          `json:"checked_at,omitempty"`
	Run       ActivityRunInfo                 `json:"run"`
	Lifecycle ActivityLifecycle               `json:"lifecycle"`
	Queue     ActivityQueue                   `json:"queue"`
	Budget    ActivityBudget                  `json:"budget,omitempty"`
	Coverage  RequiredCoverage                `json:"coverage,omitempty"`
	Operations map[string]ControlOperationTarget `json:"operations,omitempty"`
	Attention map[string]TargetAttentionFacts `json:"attention,omitempty"`
	Root      WorktreeDiffStat                `json:"root"`
	Targets   map[string]TargetPresenceFacts  `json:"targets,omitempty"`
	Actors    map[string]ActivityActor        `json:"actors,omitempty"`
	Sessions  map[string]ActivitySession      `json:"sessions,omitempty"`
}

type ActivityRunInfo struct {
	ProjectID   string `json:"project_id,omitempty"`
	RunName     string `json:"run_name,omitempty"`
	RunID       string `json:"run_id,omitempty"`
	Epoch       int    `json:"epoch,omitempty"`
	TmuxSession string `json:"tmux_session,omitempty"`
}

type ActivityLifecycle struct {
	ControlState string `json:"control_state,omitempty"`
	RuntimePhase string `json:"runtime_phase,omitempty"`
	RunActive    bool   `json:"run_active,omitempty"`
}

type ActivityQueue struct {
	MasterUnread       int    `json:"master_unread,omitempty"`
	UrgentUnread       bool   `json:"urgent_unread,omitempty"`
	RemindersDue       int    `json:"reminders_due,omitempty"`
	DeliveriesFailed   int    `json:"deliveries_failed,omitempty"`
	LastMasterSubmitAt string `json:"last_master_submit_at,omitempty"`
}

type ActivityBudget struct {
	MaxDurationSeconds int64  `json:"max_duration_seconds,omitempty"`
	StartedAt          string `json:"started_at,omitempty"`
	DeadlineAt         string `json:"deadline_at,omitempty"`
	ElapsedSeconds     int64  `json:"elapsed_seconds,omitempty"`
	RemainingSeconds   int64  `json:"remaining_seconds,omitempty"`
	Exhausted          bool   `json:"exhausted,omitempty"`
}

type ActivityActor struct {
	Lease              string `json:"lease,omitempty"`
	PID                int    `json:"pid,omitempty"`
	PIDAlive           bool   `json:"pid_alive,omitempty"`
	Transport          string `json:"transport,omitempty"`
	RenewedAt          string `json:"renewed_at,omitempty"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	PanePresent        bool   `json:"pane_present,omitempty"`
	PaneHash           string `json:"pane_hash,omitempty"`
	LastOutputChangeAt string `json:"last_output_change_at,omitempty"`
}

type ActivitySession struct {
	Lease                 string `json:"lease,omitempty"`
	PID                   int    `json:"pid,omitempty"`
	PIDAlive              bool   `json:"pid_alive,omitempty"`
	Transport             string `json:"transport,omitempty"`
	RenewedAt             string `json:"renewed_at,omitempty"`
	ExpiresAt             string `json:"expires_at,omitempty"`
	JournalStaleMinute    int    `json:"journal_stale_minutes,omitempty"`
	DirtyFiles            int    `json:"dirty_files,omitempty"`
	Insertions            int    `json:"insertions,omitempty"`
	Deletions             int    `json:"deletions,omitempty"`
	WorktreeFingerprint   string `json:"worktree_fingerprint,omitempty"`
	LastWorktreeChangeAt  string `json:"last_worktree_change_at,omitempty"`
	PanePresent           bool   `json:"pane_present,omitempty"`
	PaneHash              string `json:"pane_hash,omitempty"`
	LastOutputChangeAt    string `json:"last_output_change_at,omitempty"`
	InboxLastID           int64  `json:"inbox_last_id,omitempty"`
	CursorLastSeenID      int64  `json:"cursor_last_seen_id,omitempty"`
	Unread                int    `json:"unread,omitempty"`
	TransportState        string `json:"transport_state,omitempty"`
	InputContainsWake     bool   `json:"input_contains_wake,omitempty"`
	QueuedMessageVisible  bool   `json:"queued_message_visible,omitempty"`
	LastSubmitAttemptAt   string `json:"last_submit_attempt_at,omitempty"`
	LastTransportAcceptAt string `json:"last_transport_accept_at,omitempty"`
}

func ActivityPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "activity.json")
}

func LoadActivitySnapshot(path string) (*ActivitySnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state := &ActivitySnapshot{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, err
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveActivitySnapshot(runDir string, snapshot *ActivitySnapshot) error {
	if snapshot == nil {
		return nil
	}
	if snapshot.Version == 0 {
		snapshot.Version = 1
	}
	if snapshot.CheckedAt == "" {
		snapshot.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(ActivityPath(runDir), snapshot)
}

func BuildActivitySnapshot(projectRoot, runName, runDir string) (*ActivitySnapshot, error) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return nil, err
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	previous, _ := LoadActivitySnapshot(ActivityPath(runDir))
	tmuxSession := goalx.TmuxSessionName(projectRoot, runName)
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return nil, err
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	sessionState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	goalState, err := LoadGoalState(GoalPath(runDir))
	if err != nil {
		return nil, err
	}
	coordinationState, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		return nil, err
	}
	remindersDue, deliveriesFailed := controlQueueSummary(runDir)
	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: now,
		Run: ActivityRunInfo{
			ProjectID:   goalx.ProjectID(projectRoot),
			RunName:     cfg.Name,
			TmuxSession: tmuxSession,
		},
		Lifecycle: ActivityLifecycle{},
		Queue: ActivityQueue{
			MasterUnread:       unreadControlInboxCount(MasterInboxPath(runDir), MasterCursorPath(runDir)),
			UrgentUnread:       hasUrgentUnread(runDir),
			RemindersDue:       remindersDue,
			DeliveriesFailed:   deliveriesFailed,
			LastMasterSubmitAt: lastDeliveryAttemptAt(runDir, "master-wake"),
		},
		Actors:   map[string]ActivityActor{},
		Sessions: map[string]ActivitySession{},
	}
	if meta != nil {
		snapshot.Run.RunID = meta.RunID
		snapshot.Run.Epoch = meta.Epoch
	}
	if controlState != nil {
		snapshot.Lifecycle.ControlState = controlState.LifecycleState
		if snapshot.Lifecycle.ControlState == "" {
			snapshot.Lifecycle.ControlState = controlState.Phase
		}
	}
	operationsState, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil {
		return nil, err
	}
	if operationsState != nil && len(operationsState.Targets) > 0 {
		snapshot.Operations = cloneControlOperationsState(operationsState).Targets
	}
	if runtimeState != nil {
		snapshot.Lifecycle.RuntimePhase = runtimeState.Phase
		snapshot.Lifecycle.RunActive = runtimeState.Active
	}
	snapshot.Budget = buildActivityBudget(cfg, runtimeState, meta, snapshot.CheckedAt)
	targets, err := BuildTargetPresenceFacts(runDir, tmuxSession)
	if err != nil {
		return nil, err
	}
	snapshot.Targets = targets
	snapshot.Actors["master"] = buildActivityActor(runDir, "master", tmuxSession, "master", previousActor(previous, "master"), snapshot.CheckedAt)
	snapshot.Actors["sidecar"] = buildActivityActor(runDir, "sidecar", tmuxSession, "", previousActor(previous, "sidecar"), snapshot.CheckedAt)

	liveness, err := LoadLivenessState(LivenessPath(runDir))
	if err != nil {
		return nil, err
	}
	worktreeSnapshot, _ := LoadWorktreeSnapshot(WorktreeSnapshotPath(runDir))
	transportFacts, _ := LoadTransportFacts(TransportFactsPath(runDir))
	if worktreeSnapshot != nil {
		snapshot.Root = worktreeSnapshot.Root
	}
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		name := SessionName(idx)
		session := ActivitySession{}
		lease, _ := LoadControlLease(ControlLeasePath(runDir, name))
		if lease != nil {
			session.Lease = actorLeaseSummary(runDir, name, "missing")
			session.PID = lease.PID
			session.PIDAlive = processAlive(lease.PID)
			session.Transport = lease.Transport
			session.RenewedAt = lease.RenewedAt
			session.ExpiresAt = lease.ExpiresAt
		}
		if liveness != nil && liveness.Sessions != nil {
			if live, ok := liveness.Sessions[name]; ok {
				session.JournalStaleMinute = live.JournalStaleMinutes
				if session.Lease == "" {
					session.Lease = live.Lease
				}
				if !session.PIDAlive {
					session.PIDAlive = live.PIDAlive
				}
			}
		}
		if worktreeSnapshot != nil && worktreeSnapshot.Sessions != nil {
			if diff, ok := worktreeSnapshot.Sessions[name]; ok {
				session.DirtyFiles = diff.DirtyFiles
				session.Insertions = diff.Insertions
				session.Deletions = diff.Deletions
				session.WorktreeFingerprint = diff.DiffFingerprint
				session.LastWorktreeChangeAt = carryWorktreeChangeTime(previousSession(previous, name), session, snapshot.CheckedAt)
			}
		}
		inboxState := readControlInboxState(ControlInboxPath(runDir, name), SessionCursorPath(runDir, name))
		session.InboxLastID = inboxState.LastID
		session.CursorLastSeenID = inboxState.LastSeenID
		session.Unread = inboxState.Unread
		if transport := latestSessionTransportFacts(transportFacts, name); transport.TransportState != "" || transport.InputContainsWake || transport.QueuedMessageVisible || transport.LastSubmitAttemptAt != "" || transport.LastTransportAcceptAt != "" {
			session.TransportState = transport.TransportState
			session.InputContainsWake = transport.InputContainsWake
			session.QueuedMessageVisible = transport.QueuedMessageVisible
			session.LastSubmitAttemptAt = transport.LastSubmitAttemptAt
			session.LastTransportAcceptAt = transport.LastTransportAcceptAt
		}
		paneHash, panePresent := capturePaneHash(tmuxSession, name)
		session.PanePresent = panePresent
		session.PaneHash = paneHash
		session.LastOutputChangeAt = carryPaneChangeTime(previousSession(previous, name), paneHash, panePresent, snapshot.CheckedAt)
		snapshot.Sessions[name] = session
	}
	if len(snapshot.Sessions) == 0 {
		snapshot.Sessions = nil
	}
	attention, err := buildTargetAttentionFacts(runDir, snapshot, sessionState, liveness)
	if err != nil {
		return nil, err
	}
	if len(attention) > 0 {
		snapshot.Attention = attention
	}
	snapshot.Coverage = buildRequiredCoverage(goalState, coordinationState, sessionState, coverageSessionRoster(runDir, sessionState))
	return snapshot, nil
}

func buildActivityBudget(cfg *goalx.Config, runtimeState *RunRuntimeState, meta *RunMetadata, checkedAt string) ActivityBudget {
	if cfg == nil || cfg.Budget.MaxDuration <= 0 {
		return ActivityBudget{}
	}

	budget := ActivityBudget{
		MaxDurationSeconds: int64(cfg.Budget.MaxDuration / time.Second),
	}
	startedAt := ""
	if runtimeState != nil {
		startedAt = strings.TrimSpace(runtimeState.StartedAt)
	}
	if startedAt == "" && meta != nil {
		startedAt = strings.TrimSpace(meta.StartedAt)
	}
	if startedAt == "" {
		return budget
	}
	budget.StartedAt = startedAt

	startTime, err := time.Parse(time.RFC3339, startedAt)
	if err != nil {
		return budget
	}
	checkedTime, err := time.Parse(time.RFC3339, checkedAt)
	if err != nil {
		checkedTime = time.Now().UTC()
	}
	if checkedTime.Before(startTime) {
		checkedTime = startTime
	}

	elapsed := checkedTime.Sub(startTime)
	remaining := cfg.Budget.MaxDuration - elapsed
	budget.DeadlineAt = startTime.Add(cfg.Budget.MaxDuration).UTC().Format(time.RFC3339)
	budget.ElapsedSeconds = int64(elapsed / time.Second)
	budget.RemainingSeconds = int64(remaining / time.Second)
	budget.Exhausted = elapsed >= cfg.Budget.MaxDuration
	return budget
}

func buildActivityActor(runDir, holder, tmuxSession, window string, previous ActivityActor, checkedAt string) ActivityActor {
	actor := ActivityActor{}
	lease, _ := LoadControlLease(ControlLeasePath(runDir, holder))
	if lease != nil {
		actor.Lease = actorLeaseSummary(runDir, holder, "missing")
		actor.PID = lease.PID
		actor.PIDAlive = processAlive(lease.PID)
		actor.Transport = lease.Transport
		actor.RenewedAt = lease.RenewedAt
		actor.ExpiresAt = lease.ExpiresAt
	}
	if strings.TrimSpace(window) != "" {
		actor.PaneHash, actor.PanePresent = capturePaneHash(tmuxSession, window)
		actor.LastOutputChangeAt = carryPaneChangeTime(previous, actor.PaneHash, actor.PanePresent, checkedAt)
	}
	return actor
}

func previousActor(previous *ActivitySnapshot, holder string) ActivityActor {
	if previous == nil || previous.Actors == nil {
		return ActivityActor{}
	}
	return previous.Actors[holder]
}

func previousSession(previous *ActivitySnapshot, name string) ActivitySession {
	if previous == nil || previous.Sessions == nil {
		return ActivitySession{}
	}
	return previous.Sessions[name]
}

func carryPaneChangeTime[T interface {
	getPaneHash() string
	getLastOutputChangeAt() string
}](previous T, paneHash string, panePresent bool, fallback string) string {
	if !panePresent {
		return ""
	}
	if previous.getPaneHash() != "" && previous.getPaneHash() == paneHash && previous.getLastOutputChangeAt() != "" {
		return previous.getLastOutputChangeAt()
	}
	return fallback
}

func (a ActivityActor) getPaneHash() string           { return a.PaneHash }
func (a ActivityActor) getLastOutputChangeAt() string { return a.LastOutputChangeAt }
func (s ActivitySession) getPaneHash() string         { return s.PaneHash }
func (s ActivitySession) getLastOutputChangeAt() string {
	return s.LastOutputChangeAt
}

func carryWorktreeChangeTime(previous, current ActivitySession, fallback string) string {
	if current.DirtyFiles == 0 || strings.TrimSpace(current.WorktreeFingerprint) == "" {
		return ""
	}
	if previous.WorktreeFingerprint != "" && previous.WorktreeFingerprint == current.WorktreeFingerprint && previous.LastWorktreeChangeAt != "" {
		return previous.LastWorktreeChangeAt
	}
	return fallback
}

func capturePaneHash(tmuxSession, window string) (string, bool) {
	if strings.TrimSpace(tmuxSession) == "" || strings.TrimSpace(window) == "" {
		return "", false
	}
	out, err := exec.Command("tmux", "capture-pane", "-t", tmuxSession+":"+window, "-p", "-S", "-200").Output()
	if err != nil {
		return "", false
	}
	sum := sha256.Sum256(out)
	return "sha256:" + hex.EncodeToString(sum[:]), true
}

func lastDeliveryAttemptAt(runDir, dedupeKey string) string {
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil || deliveries == nil {
		return ""
	}
	last := ""
	for _, item := range deliveries.Items {
		if item.DedupeKey != dedupeKey {
			continue
		}
		if item.AttemptedAt > last {
			last = item.AttemptedAt
		}
	}
	return last
}
