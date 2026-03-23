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
	CreatedAt string `json:"created_at"`
}

type MasterCursorState struct {
	LastSeenID int64  `json:"last_seen_id"`
	UpdatedAt  string `json:"updated_at,omitempty"`
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
	if err := EnsureMasterControl(runDir); err != nil {
		return MasterInboxMessage{}, err
	}
	nextID, err := nextMasterInboxID(MasterInboxPath(runDir))
	if err != nil {
		return MasterInboxMessage{}, err
	}
	msg := MasterInboxMessage{
		ID:        nextID,
		Type:      typ,
		Source:    source,
		Body:      body,
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	f, err := os.OpenFile(MasterInboxPath(runDir), os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return MasterInboxMessage{}, fmt.Errorf("open master inbox: %w", err)
	}
	defer f.Close()
	line, err := json.Marshal(msg)
	if err != nil {
		return MasterInboxMessage{}, fmt.Errorf("marshal master inbox message: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return MasterInboxMessage{}, fmt.Errorf("append master inbox: %w", err)
	}
	return msg, nil
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
