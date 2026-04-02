package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type LivenessState struct {
	CheckedAt string                   `json:"checked_at"`
	Master    LivenessEntry            `json:"master"`
	Sessions  map[string]LivenessEntry `json:"sessions,omitempty"`
}

type LivenessEntry struct {
	Lease               string `json:"lease"`
	PIDAlive            bool   `json:"pid_alive"`
	HasWorktree         bool   `json:"has_worktree"`
	JournalStaleMinutes int    `json:"journal_stale_minutes,omitempty"`
}

func ScanLiveness(runDir string) (*LivenessState, error) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return nil, err
	}
	sessionsState, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return nil, fmt.Errorf("load session runtime state: %w", err)
	}
	nonWindowExpectedStates, err := loadNonWindowExpectedSessionStates(runDir)
	if err != nil {
		return nil, err
	}
	previous, err := LoadLivenessState(LivenessPath(runDir))
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	state := &LivenessState{
		CheckedAt: now.Format(time.RFC3339),
		Master:    scanLivenessEntry(runDir, "master", pathDirExists(RunWorktreePath(runDir)), now),
	}

	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	if len(indexes) > 0 {
		state.Sessions = make(map[string]LivenessEntry, len(indexes))
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		worktreePath := resolvedSessionWorktreePath(runDir, cfg.Name, sessionName, sessionsState)
		entry := scanLivenessEntry(runDir, sessionName, worktreePath != "", now)
		state.Sessions[sessionName] = entry
		if nonWindowExpectedStates[sessionName] != "" {
			continue
		}
		if shouldNotifySessionDied(previous, sessionName, entry) {
			if _, err := AppendMasterInboxMessage(runDir, "session-died", "goalx runtime-host", fmt.Sprintf("%s lease expired and its process is no longer alive.", sessionName)); err != nil {
				return nil, err
			}
		}
	}
	if len(state.Sessions) == 0 {
		state.Sessions = nil
	}
	return state, nil
}

func LoadLivenessState(path string) (*LivenessState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state := &LivenessState{}
	if len(data) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse liveness state: %w", err)
	}
	return state, nil
}

func SaveLivenessState(runDir string, state *LivenessState) error {
	if state == nil {
		return fmt.Errorf("liveness state is nil")
	}
	if state.CheckedAt == "" {
		state.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(LivenessPath(runDir), state)
}

func scanLivenessEntry(runDir, holder string, hasWorktree bool, now time.Time) LivenessEntry {
	entry := LivenessEntry{
		Lease:       "expired",
		HasWorktree: hasWorktree,
	}
	if stale, ok := journalStaleMinutes(runDir, holder, now); ok {
		entry.JournalStaleMinutes = stale
	}
	lease, err := LoadControlLease(ControlLeasePath(runDir, holder))
	if err != nil {
		return entry
	}
	entry.PIDAlive = processAlive(lease.PID)
	if controlLeaseHealthyAt(lease, now) {
		entry.Lease = "healthy"
	}
	return entry
}

func controlLeaseHealthyAt(lease *ControlLease, now time.Time) bool {
	if lease == nil || lease.ExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, lease.ExpiresAt)
	if err != nil {
		return false
	}
	return expiresAt.After(now)
}

func shouldNotifySessionDied(previous *LivenessState, sessionName string, current LivenessEntry) bool {
	if previous == nil || previous.Sessions == nil {
		return false
	}
	prev, ok := previous.Sessions[sessionName]
	if !ok {
		return false
	}
	return prev.Lease == "healthy" && prev.PIDAlive && current.Lease == "expired" && !current.PIDAlive
}

func pathDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func journalStaleMinutes(runDir, holder string, now time.Time) (int, bool) {
	journalPath := JournalPath(runDir, holder)
	if holder == "master" {
		journalPath = filepath.Join(runDir, "master.jsonl")
	}
	info, err := os.Stat(journalPath)
	if err != nil || info.IsDir() {
		return 0, false
	}
	modTime := info.ModTime()
	if modTime.IsZero() || modTime.After(now) {
		return 0, true
	}
	return int(now.Sub(modTime).Minutes()), true
}
