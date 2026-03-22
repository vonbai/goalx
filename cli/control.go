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

const masterWakeMessage = "goalx-hb"

type MasterInboxMessage struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type MasterState struct {
	LastSeenID       int64  `json:"last_seen_id"`
	LastHeartbeatSeq int64  `json:"last_heartbeat_seq"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type HeartbeatState struct {
	Seq       int64  `json:"seq"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

var sendAgentNudge = SendAgentNudge

func ControlDir(runDir string) string {
	return filepath.Join(runDir, "control")
}

func MasterInboxPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "master-inbox.jsonl")
}

func MasterStatePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "master-state.json")
}

func HeartbeatStatePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "heartbeat.json")
}

func EnsureMasterControl(runDir string) error {
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		return fmt.Errorf("mkdir control dir: %w", err)
	}
	if err := ensureEmptyFile(MasterInboxPath(runDir)); err != nil {
		return err
	}
	if _, err := LoadMasterState(MasterStatePath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveMasterState(MasterStatePath(runDir), &MasterState{}); err != nil {
			return err
		}
	}
	if _, err := LoadHeartbeatState(HeartbeatStatePath(runDir)); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := SaveHeartbeatState(HeartbeatStatePath(runDir), &HeartbeatState{}); err != nil {
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

func LoadMasterState(path string) (*MasterState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state MasterState
	if len(strings.TrimSpace(string(data))) == 0 {
		return &state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse master state: %w", err)
	}
	return &state, nil
}

func SaveMasterState(path string, state *MasterState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal master state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write master state: %w", err)
	}
	return nil
}

func LoadHeartbeatState(path string) (*HeartbeatState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var state HeartbeatState
	if len(strings.TrimSpace(string(data))) == 0 {
		return &state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse heartbeat state: %w", err)
	}
	return &state, nil
}

func SaveHeartbeatState(path string, state *HeartbeatState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal heartbeat state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write heartbeat state: %w", err)
	}
	return nil
}

func RecordHeartbeatTick(runDir string) (*HeartbeatState, error) {
	if err := EnsureMasterControl(runDir); err != nil {
		return nil, err
	}
	state, err := LoadHeartbeatState(HeartbeatStatePath(runDir))
	if err != nil {
		return nil, err
	}
	state.Seq++
	state.UpdatedAt = time.Now().Format(time.RFC3339)
	if err := SaveHeartbeatState(HeartbeatStatePath(runDir), state); err != nil {
		return nil, err
	}
	return state, nil
}

func SendAgentNudge(target, engine string) error {
	pane, err := CapturePaneTargetOutput(target)
	if err != nil {
		pane = ""
	}
	return sendKeysWithSubmit(target, masterWakeMessage, nudgeSubmitKey(engine, pane))
}

func nudgeSubmitKey(engine, pane string) string {
	if engine == "codex" && strings.Contains(strings.ToLower(pane), "tab to queue message") {
		return "Tab"
	}
	return "Enter"
}
