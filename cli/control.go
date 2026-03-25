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

const masterWakeMessage = "goalx-wake"

type MasterInboxMessage struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Body      string `json:"body"`
	Urgent    bool   `json:"urgent,omitempty"`
	CreatedAt string `json:"created_at"`
}

type MasterCursorState struct {
	LastSeenID int64  `json:"last_seen_id"`
	UpdatedAt  string `json:"updated_at,omitempty"`
}

type ControlInboxState struct {
	LastID     int64
	LastSeenID int64
	Unread     int
}

var sendAgentNudge = SendAgentNudge
var sendAgentKeys = sendKeysWithSubmit

func ControlDir(runDir string) string {
	return filepath.Join(runDir, "control")
}

func MasterInboxPath(runDir string) string {
	return ControlInboxPath(runDir, "master")
}

func MasterCursorPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "master-cursor.json")
}

func SessionCursorPath(runDir, sessionName string) string {
	return filepath.Join(ControlDir(runDir), sessionName+"-cursor.json")
}

func EnsureMasterControl(runDir string) error {
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		return fmt.Errorf("mkdir control dir: %w", err)
	}
	if err := ensureEmptyFile(MasterInboxPath(runDir)); err != nil {
		return err
	}
	if _, err := LoadMasterCursorState(MasterCursorPath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveMasterCursorState(MasterCursorPath(runDir), &MasterCursorState{}); err != nil {
			return err
		}
	}
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	return nil
}

func EnsureSessionControl(runDir, sessionName string) error {
	if err := EnsureMasterControl(runDir); err != nil {
		return err
	}
	if err := ensureEmptyFile(ControlInboxPath(runDir, sessionName)); err != nil {
		return err
	}
	if _, err := LoadMasterCursorState(SessionCursorPath(runDir, sessionName)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveMasterCursorState(SessionCursorPath(runDir, sessionName), &MasterCursorState{}); err != nil {
			return err
		}
	}
	return nil
}

func ensureEmptyFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func AppendMasterInboxMessage(runDir, typ, source, body string) (MasterInboxMessage, error) {
	return appendControlInboxMessage(runDir, "master", typ, source, body, false)
}

func AppendControlInboxMessage(runDir, target, typ, source, body string) (MasterInboxMessage, error) {
	return appendControlInboxMessage(runDir, target, typ, source, body, false)
}

func appendControlInboxMessage(runDir, target, typ, source, body string, urgent bool) (MasterInboxMessage, error) {
	if err := EnsureMasterControl(runDir); err != nil {
		return MasterInboxMessage{}, err
	}
	if target != "master" {
		if err := EnsureSessionControl(runDir, target); err != nil {
			return MasterInboxMessage{}, err
		}
	}
	inboxPath := MasterInboxPath(runDir)
	if target != "master" {
		inboxPath = ControlInboxPath(runDir, target)
	}
	nextID, err := nextMasterInboxID(inboxPath)
	if err != nil {
		return MasterInboxMessage{}, err
	}
	msg := MasterInboxMessage{
		ID:        nextID,
		Type:      typ,
		Source:    source,
		Body:      body,
		Urgent:    urgent,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return MasterInboxMessage{}, fmt.Errorf("open control inbox: %w", err)
	}
	defer f.Close()
	line, err := json.Marshal(msg)
	if err != nil {
		return MasterInboxMessage{}, fmt.Errorf("marshal master inbox message: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return MasterInboxMessage{}, fmt.Errorf("append control inbox: %w", err)
	}
	return msg, nil
}

func AckControlInbox(runDir, target string) (*MasterCursorState, error) {
	inboxPath := MasterInboxPath(runDir)
	cursorPath := MasterCursorPath(runDir)
	if target != "master" {
		if err := EnsureSessionControl(runDir, target); err != nil {
			return nil, err
		}
		inboxPath = ControlInboxPath(runDir, target)
		cursorPath = SessionCursorPath(runDir, target)
	} else {
		if err := EnsureMasterControl(runDir); err != nil {
			return nil, err
		}
	}
	lastID, err := nextMasterInboxID(inboxPath)
	if err != nil {
		return nil, err
	}
	cursor := &MasterCursorState{}
	if lastID > 0 {
		cursor.LastSeenID = lastID - 1
	}
	if err := SaveMasterCursorState(cursorPath, cursor); err != nil {
		return nil, err
	}
	return cursor, nil
}

func unreadControlInboxCount(inboxPath, cursorPath string) int {
	return readControlInboxState(inboxPath, cursorPath).Unread
}

func readControlInboxState(inboxPath, cursorPath string) ControlInboxState {
	cursor, _ := LoadMasterCursorState(cursorPath)
	state := ControlInboxState{}
	if cursor != nil {
		state.LastSeenID = cursor.LastSeenID
	}
	f, err := os.ReadFile(inboxPath)
	if err != nil {
		return state
	}
	for _, line := range splitNonEmptyLines(string(f)) {
		var msg MasterInboxMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID > state.LastID {
			state.LastID = msg.ID
		}
	}
	if state.LastID == 0 {
		return state
	}
	if cursor == nil {
		state.Unread = int(state.LastID)
		return state
	}
	if state.LastID <= state.LastSeenID {
		return state
	}
	state.Unread = int(state.LastID - state.LastSeenID)
	return state
}

func hasUrgentUnread(runDir string) bool {
	cursor, _ := LoadMasterCursorState(MasterCursorPath(runDir))
	lastSeen := int64(0)
	if cursor != nil {
		lastSeen = cursor.LastSeenID
	}
	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		return false
	}
	for _, line := range splitNonEmptyLines(string(data)) {
		var msg MasterInboxMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.ID > lastSeen && msg.Urgent {
			return true
		}
	}
	return false
}

func nextMasterInboxID(path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open master inbox: %w", err)
	}
	defer f.Close()

	var lastID int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var msg MasterInboxMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return 0, fmt.Errorf("parse master inbox: %w", err)
		}
		if msg.ID > lastID {
			lastID = msg.ID
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan master inbox: %w", err)
	}
	return lastID + 1, nil
}

func LoadMasterCursorState(path string) (*MasterCursorState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state MasterCursorState
	if len(strings.TrimSpace(string(data))) == 0 {
		return &state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse master cursor state: %w", err)
	}
	return &state, nil
}

func SaveMasterCursorState(path string, state *MasterCursorState) error {
	if state == nil {
		return fmt.Errorf("master cursor state is nil")
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal master cursor state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write master cursor state: %w", err)
	}
	return nil
}

func SendAgentNudge(target, engine string) error {
	return sendAgentKeys(target, masterWakeMessage, "Enter")
}
