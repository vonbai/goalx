package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

// RunRuntimeState holds framework-owned runtime facts for a run.
// The separate run-scoped status record is master-written and carries any
// agent-authored progress summary without project-level sharing.
type RunRuntimeState struct {
	Version   int    `json:"version"`
	Run       string `json:"run"`
	Mode      string `json:"mode,omitempty"`
	Active    bool   `json:"active"`
	Phase     string `json:"phase,omitempty"`
	StartedAt string `json:"started_at,omitempty"`
	StoppedAt string `json:"stopped_at,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type SessionRuntimeState struct {
	Name             string `json:"name"`
	State            string `json:"state,omitempty"`
	Mode             string `json:"mode,omitempty"`
	Branch           string `json:"branch,omitempty"`
	WorktreePath     string `json:"worktree_path,omitempty"`
	OwnerScope       string `json:"owner_scope,omitempty"`
	BlockedBy        string `json:"blocked_by,omitempty"`
	DirtyFiles       int    `json:"dirty_files,omitempty"`
	DiffStat         string `json:"diff_stat,omitempty"`
	LastRound        int    `json:"last_round,omitempty"`
	LastJournalState string `json:"last_journal_state,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type SessionsRuntimeState struct {
	Version   int                            `json:"version"`
	Sessions  map[string]SessionRuntimeState `json:"sessions"`
	UpdatedAt string                         `json:"updated_at,omitempty"`
}

func StateDir(runDir string) string {
	return filepath.Join(runDir, "state")
}

func RunRuntimeStatePath(runDir string) string {
	return filepath.Join(StateDir(runDir), "run.json")
}

func SessionsRuntimeStatePath(runDir string) string {
	return filepath.Join(StateDir(runDir), "sessions.json")
}

func EnsureRuntimeState(runDir string, cfg *goalx.Config) (*RunRuntimeState, error) {
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		return nil, err
	}
	state, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	if state == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		state = &RunRuntimeState{
			Version:   1,
			Run:       cfg.Name,
			Mode:      string(cfg.Mode),
			Active:    true,
			StartedAt: now,
			UpdatedAt: now,
		}
		if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func LoadRunRuntimeState(path string) (*RunRuntimeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseRunRuntimeState(data)
}

func parseRunRuntimeState(data []byte) (*RunRuntimeState, error) {
	state := &RunRuntimeState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse run runtime state: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveRunRuntimeState(path string, state *RunRuntimeState) error {
	if state == nil {
		return fmt.Errorf("run runtime state is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func putRunRuntimeStateDirect(runDir string, next RunRuntimeState) error {
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		return err
	}
	state := next
	return SaveRunRuntimeState(RunRuntimeStatePath(runDir), &state)
}

func UpsertRunRuntimeState(runDir string, next RunRuntimeState) error {
	return submitAndApplyControlOp(runDir, controlOpRunRuntimeUpsert, controlRunRuntimeUpsertBody{State: next})
}

func EnsureSessionsRuntimeState(runDir string) (*SessionsRuntimeState, error) {
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		return nil, err
	}
	path := SessionsRuntimeStatePath(runDir)
	var ensured *SessionsRuntimeState
	if err := mutateStructuredFile(
		path,
		0o644,
		func(data []byte) (*SessionsRuntimeState, error) {
			return parseSessionsRuntimeState(data)
		},
		func() *SessionsRuntimeState {
			return &SessionsRuntimeState{
				Version:   1,
				Sessions:  map[string]SessionRuntimeState{},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}
		},
		func(state *SessionsRuntimeState) error {
			ensured = cloneSessionsRuntimeState(state)
			return nil
		},
		func(state *SessionsRuntimeState) ([]byte, error) {
			normalizeSessionsRuntimeState(state)
			if state.UpdatedAt == "" {
				state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			}
			return json.MarshalIndent(state, "", "  ")
		},
	); err != nil {
		return nil, err
	}
	return ensured, nil
}

func LoadSessionsRuntimeState(path string) (*SessionsRuntimeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseSessionsRuntimeState(data)
}

func parseSessionsRuntimeState(data []byte) (*SessionsRuntimeState, error) {
	state := &SessionsRuntimeState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		state.Sessions = map[string]SessionRuntimeState{}
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse sessions runtime state: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	normalizeSessionsRuntimeState(state)
	return state, nil
}

func SaveSessionsRuntimeState(path string, state *SessionsRuntimeState) error {
	if state == nil {
		return fmt.Errorf("sessions runtime state is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	normalizeSessionsRuntimeState(state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func upsertSessionRuntimeStateDirect(runDir string, next SessionRuntimeState) error {
	return mutateStructuredFile(
		SessionsRuntimeStatePath(runDir),
		0o644,
		func(data []byte) (*SessionsRuntimeState, error) {
			return parseSessionsRuntimeState(data)
		},
		func() *SessionsRuntimeState {
			return &SessionsRuntimeState{
				Version:  1,
				Sessions: map[string]SessionRuntimeState{},
			}
		},
		func(state *SessionsRuntimeState) error {
			normalizeSessionsRuntimeState(state)
			current := state.Sessions[next.Name]
			mergeSessionRuntimeState(&current, next)
			current.Name = next.Name
			current.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			state.Sessions[next.Name] = current
			return nil
		},
		func(state *SessionsRuntimeState) ([]byte, error) {
			normalizeSessionsRuntimeState(state)
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return json.MarshalIndent(state, "", "  ")
		},
	)
}

func UpsertSessionRuntimeState(runDir string, next SessionRuntimeState) error {
	return submitAndApplyControlOp(runDir, controlOpSessionRuntimeUpsert, controlSessionRuntimeUpsertBody{State: next})
}

func removeSessionRuntimeStateDirect(runDir, sessionName string) error {
	return mutateStructuredFile(
		SessionsRuntimeStatePath(runDir),
		0o644,
		func(data []byte) (*SessionsRuntimeState, error) {
			return parseSessionsRuntimeState(data)
		},
		func() *SessionsRuntimeState {
			return &SessionsRuntimeState{
				Version:  1,
				Sessions: map[string]SessionRuntimeState{},
			}
		},
		func(state *SessionsRuntimeState) error {
			normalizeSessionsRuntimeState(state)
			delete(state.Sessions, sessionName)
			return nil
		},
		func(state *SessionsRuntimeState) ([]byte, error) {
			normalizeSessionsRuntimeState(state)
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return json.MarshalIndent(state, "", "  ")
		},
	)
}

func RemoveSessionRuntimeState(runDir, sessionName string) error {
	return submitAndApplyControlOp(runDir, controlOpSessionRuntimeRemove, controlSessionRuntimeRemoveBody{Name: sessionName})
}

func normalizeSessionsRuntimeState(state *SessionsRuntimeState) {
	if state == nil {
		return
	}
	if state.Sessions == nil {
		state.Sessions = map[string]SessionRuntimeState{}
	}
}

func cloneSessionsRuntimeState(state *SessionsRuntimeState) *SessionsRuntimeState {
	if state == nil {
		return nil
	}
	cloned := &SessionsRuntimeState{
		Version:   state.Version,
		Sessions:  make(map[string]SessionRuntimeState, len(state.Sessions)),
		UpdatedAt: state.UpdatedAt,
	}
	for name, session := range state.Sessions {
		cloned.Sessions[name] = session
	}
	return cloned
}

func mergeSessionRuntimeState(dst *SessionRuntimeState, src SessionRuntimeState) {
	if src.State != "" {
		dst.State = src.State
	}
	if src.Mode != "" {
		dst.Mode = src.Mode
	}
	if src.Branch != "" {
		dst.Branch = src.Branch
	}
	if src.WorktreePath != "" {
		dst.WorktreePath = src.WorktreePath
	}
	if src.OwnerScope != "" {
		dst.OwnerScope = src.OwnerScope
	}
	if src.BlockedBy != "" || src.State != "" {
		dst.BlockedBy = src.BlockedBy
	}
	if src.DirtyFiles != 0 || src.DiffStat != "" {
		dst.DirtyFiles = src.DirtyFiles
		dst.DiffStat = src.DiffStat
	}
	if src.LastRound != 0 {
		dst.LastRound = src.LastRound
	}
	if src.LastJournalState != "" {
		dst.LastJournalState = src.LastJournalState
	}
}

func finalizeSessionRuntimeStatesDirect(runDir, lifecycle, now string) error {
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
	if strings.TrimSpace(now) == "" {
		now = time.Now().UTC().Format(time.RFC3339)
	}
	for name, session := range state.Sessions {
		session.State = lifecycle
		session.UpdatedAt = now
		state.Sessions[name] = session
	}
	return SaveSessionsRuntimeState(SessionsRuntimeStatePath(runDir), state)
}

func SnapshotSessionRuntime(runDir, sessionName, worktreePath string) (SessionRuntimeState, error) {
	dirtyFiles, diffStat, err := snapshotWorktreeState(worktreePath)
	if err != nil {
		return SessionRuntimeState{}, err
	}
	journalEntries, _ := goalx.LoadJournal(JournalPath(runDir, sessionName))
	lastRound := 0
	lastJournalState := ""
	if len(journalEntries) > 0 {
		last := journalEntries[len(journalEntries)-1]
		lastRound = last.Round
		lastJournalState = last.Status
	}
	snapshot := SessionRuntimeState{
		Name:             sessionName,
		WorktreePath:     worktreePath,
		DirtyFiles:       dirtyFiles,
		DiffStat:         diffStat,
		LastRound:        lastRound,
		LastJournalState: lastJournalState,
	}
	if len(journalEntries) > 0 {
		last := journalEntries[len(journalEntries)-1]
		snapshot.State = sessionLifecycleStateFromJournalStatus(last.Status)
		snapshot.OwnerScope = last.OwnerScope
		snapshot.BlockedBy = last.BlockedBy
	}
	return snapshot, nil
}

func sessionLifecycleStateFromJournalStatus(status string) string {
	status = strings.TrimSpace(status)
	if status == "" || isControlOnlySessionJournalStatus(status) {
		return ""
	}
	return status
}

func isControlOnlySessionJournalStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "ack-session":
		return true
	default:
		return false
	}
}

func RefreshSessionRuntimeProjection(runDir, runName string) error {
	state, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return err
	}
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		worktreePath := resolvedSessionWorktreePath(runDir, runName, sessionName, state)
		snapshot, err := SnapshotSessionRuntime(runDir, sessionName, worktreePath)
		if err != nil {
			return err
		}
		if session, ok := state.Sessions[sessionName]; ok && session.Mode != "" {
			snapshot.Mode = session.Mode
		}
		if session, ok := state.Sessions[sessionName]; ok && shouldPreserveSessionRuntimeState(session, snapshot) {
			snapshot.State = strings.TrimSpace(session.State)
		}
		if err := UpsertSessionRuntimeState(runDir, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func shouldPreserveSessionRuntimeState(current, snapshot SessionRuntimeState) bool {
	switch strings.TrimSpace(current.State) {
	case "parked", "stopped":
		return true
	case "active":
		return current.LastRound > 0 &&
			current.LastRound == snapshot.LastRound &&
			strings.TrimSpace(current.LastJournalState) != "" &&
			strings.TrimSpace(current.LastJournalState) == strings.TrimSpace(snapshot.LastJournalState)
	default:
		return false
	}
}

func snapshotWorktreeState(worktreePath string) (int, string, error) {
	if strings.TrimSpace(worktreePath) == "" {
		return 0, "", nil
	}
	statusOut, err := exec.Command("git", "-C", worktreePath, "status", "--porcelain").CombinedOutput()
	if err != nil {
		if os.IsNotExist(err) || bytes.Contains(bytes.ToLower(statusOut), []byte("not a git repository")) {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf("git status in %s: %w: %s", worktreePath, err, statusOut)
	}
	dirty := 0
	for _, line := range strings.Split(strings.TrimSpace(string(statusOut)), "\n") {
		if strings.TrimSpace(line) != "" {
			dirty++
		}
	}
	diffOut, err := exec.Command("git", "-C", worktreePath, "diff", "--stat").CombinedOutput()
	if err != nil {
		if bytes.Contains(bytes.ToLower(diffOut), []byte("not a git repository")) {
			return dirty, "", nil
		}
		return dirty, "", fmt.Errorf("git diff --stat in %s: %w: %s", worktreePath, err, diffOut)
	}
	return dirty, strings.TrimSpace(string(diffOut)), nil
}

func sortedSessionStates(state *SessionsRuntimeState) []SessionRuntimeState {
	if state == nil {
		return nil
	}
	list := make([]SessionRuntimeState, 0, len(state.Sessions))
	for _, sess := range state.Sessions {
		list = append(list, sess)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].Name < list[j].Name
	})
	return list
}

func normalizeDiffStat(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func mergeDiffStat(lines string) string {
	if strings.TrimSpace(lines) == "" {
		return ""
	}
	var buf bytes.Buffer
	for _, line := range strings.Split(strings.TrimSpace(lines), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString(" | ")
		}
		buf.WriteString(normalizeDiffStat(line))
	}
	return buf.String()
}
