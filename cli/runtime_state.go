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

// RunRuntimeState holds master-written runtime state for a run.
// All derived fields (GoalSatisfied, CompletionMode, etc.) are removed —
// master writes status.json directly with whatever it decides.
type RunRuntimeState struct {
	Version        int    `json:"version"`
	Run            string `json:"run"`
	Mode           string `json:"mode,omitempty"`
	Active         bool   `json:"active"`
	Phase          string `json:"phase,omitempty"`
	Recommendation string `json:"recommendation,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	StoppedAt      string `json:"stopped_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
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
	return os.WriteFile(path, data, 0o644)
}

func EnsureSessionsRuntimeState(runDir string) (*SessionsRuntimeState, error) {
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		return nil, err
	}
	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = &SessionsRuntimeState{
			Version:   1,
			Sessions:  map[string]SessionRuntimeState{},
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := SaveSessionsRuntimeState(SessionsRuntimeStatePath(runDir), state); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func LoadSessionsRuntimeState(path string) (*SessionsRuntimeState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
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
	if state.Sessions == nil {
		state.Sessions = map[string]SessionRuntimeState{}
	}
	return state, nil
}

func SaveSessionsRuntimeState(path string, state *SessionsRuntimeState) error {
	if state == nil {
		return fmt.Errorf("sessions runtime state is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Sessions == nil {
		state.Sessions = map[string]SessionRuntimeState{}
	}
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func UpsertSessionRuntimeState(runDir string, next SessionRuntimeState) error {
	state, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return err
	}
	current := state.Sessions[next.Name]
	mergeSessionRuntimeState(&current, next)
	current.Name = next.Name
	current.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	state.Sessions[next.Name] = current
	return SaveSessionsRuntimeState(SessionsRuntimeStatePath(runDir), state)
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
	if src.BlockedBy != "" || src.State == "blocked" {
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
	return snapshot, nil
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
