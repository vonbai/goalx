package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestEnsureMasterControlCreatesFiles(t *testing.T) {
	runDir := t.TempDir()

	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	for _, path := range []string{
		MasterInboxPath(runDir),
		MasterCursorPath(runDir),
		ControlRunIdentityPath(runDir),
		ControlRunStatePath(runDir),
		ControlLeasePath(runDir, "master"),
		ControlLeasePath(runDir, "sidecar"),
		ControlInboxPath(runDir, "master"),
		ControlRemindersPath(runDir),
		ControlDeliveriesPath(runDir),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "events.jsonl")); !os.IsNotExist(err) {
		t.Fatalf("legacy event log should not exist, stat err = %v", err)
	}
}

func TestAppendMasterInboxMessageAssignsMonotonicIDs(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	first, err := AppendMasterInboxMessage(runDir, "control-cycle", "system", "tick")
	if err != nil {
		t.Fatalf("AppendMasterInboxMessage first: %v", err)
	}
	second, err := AppendMasterInboxMessage(runDir, "tell", "user", "focus on e2e")
	if err != nil {
		t.Fatalf("AppendMasterInboxMessage second: %v", err)
	}

	if second.ID <= first.ID {
		t.Fatalf("second.ID = %d, want > %d", second.ID, first.ID)
	}

	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	text := string(data)
	for _, want := range []string{`"type":"control-cycle"`, `"type":"tell"`, `"source":"user"`, `"body":"focus on e2e"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("inbox missing %q:\n%s", want, text)
		}
	}
}

func TestSendAgentNudgeAlwaysUsesExplicitWakePayload(t *testing.T) {
	origSend := sendAgentKeys
	defer func() { sendAgentKeys = origSend }()

	tests := []struct {
		name   string
		engine string
	}{
		{name: "codex", engine: "codex"},
		{name: "claude", engine: "claude-code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotTarget, gotKeys, gotSubmit string
			sendAgentKeys = func(target, keys, submitKey string) error {
				gotTarget, gotKeys, gotSubmit = target, keys, submitKey
				return nil
			}

			if err := SendAgentNudge("gx-demo:master", tt.engine); err != nil {
				t.Fatalf("SendAgentNudge: %v", err)
			}
			if gotTarget != "gx-demo:master" {
				t.Fatalf("target = %q, want gx-demo:master", gotTarget)
			}
			if gotKeys != masterWakeMessage || gotSubmit != "Enter" {
				t.Fatalf("SendAgentNudge used keys=%q submit=%q, want explicit wake payload + Enter", gotKeys, gotSubmit)
			}
		})
	}
}

func TestPulseQueuesMasterWakeReminderWithoutLegacyHeartbeatState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	runName := "pulse-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: pulse-run\nmode: develop\nobjective: ship it\nmaster:\n  engine: codex\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"pulse-run","mode":"develop","objective":"ship it","active":false,"phase":"working","recommendation":"keep going","updated_at":"2026-03-23T00:00:00Z"}`)
	if err := os.WriteFile(RunRuntimeStatePath(runDir), runStateBefore, 0o644); err != nil {
		t.Fatalf("write run state: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	called := false
	orig := sendAgentNudge
	sendAgentNudge = func(target, engine string) error {
		called = true
		return nil
	}
	defer func() { sendAgentNudge = orig }()

	if err := Pulse(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Pulse: %v", err)
	}

	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "heartbeat.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy heartbeat state should not exist, stat err = %v", err)
	}
	gotRunState, err := os.ReadFile(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("read run state: %v", err)
	}
	if string(gotRunState) != string(runStateBefore) {
		t.Fatalf("run state changed:\nwant %s\ngot  %s", string(runStateBefore), string(gotRunState))
	}
	if called {
		t.Fatal("Pulse should not deliver wake directly")
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 {
		t.Fatalf("reminders len = %d, want 1", len(reminders.Items))
	}
	if reminders.Items[0].DedupeKey != "master-wake" || reminders.Items[0].Reason != "control-cycle" {
		t.Fatalf("unexpected reminder: %+v", reminders.Items[0])
	}
}

func TestPulseDedupesMasterWakeReminderAcrossRepeatedCycles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	runName := "pulse-lag"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: pulse-lag\nmode: develop\nobjective: ship it\nmaster:\n  engine: codex\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"pulse-lag","mode":"develop","objective":"ship it","active":false,"phase":"working","recommendation":"keep going","updated_at":"2026-03-23T00:00:00Z"}`)
	if err := os.WriteFile(RunRuntimeStatePath(runDir), runStateBefore, 0o644); err != nil {
		t.Fatalf("write run state: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	orig := sendAgentNudge
	sendAgentNudge = func(target, engine string) error { return nil }
	defer func() { sendAgentNudge = orig }()

	for i := 0; i < 3; i++ {
		if err := Pulse(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Pulse #%d: %v", i+1, err)
		}
	}

	cursor, err := LoadMasterCursorState(MasterCursorPath(runDir))
	if err != nil {
		t.Fatalf("LoadMasterCursorState: %v", err)
	}
	if cursor.LastSeenID != 0 {
		t.Fatalf("cursor last_seen_id = %d, want 0", cursor.LastSeenID)
	}
	gotRunState, err := os.ReadFile(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("read run state: %v", err)
	}
	if string(gotRunState) != string(runStateBefore) {
		t.Fatalf("run state changed:\nwant %s\ngot  %s", string(runStateBefore), string(gotRunState))
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 {
		t.Fatalf("reminders len = %d, want 1", len(reminders.Items))
	}
	if reminders.Items[0].Reason != "control-cycle" {
		t.Fatalf("reminder reason = %q, want control-cycle", reminders.Items[0].Reason)
	}
}
