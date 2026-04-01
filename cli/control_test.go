package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
	if _, err := os.Stat(ControlLeasePath(runDir, "runtime-host")); !os.IsNotExist(err) {
		t.Fatalf("runtime host should not be precreated, stat err = %v", err)
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

func TestAppendMasterInboxMessageConcurrentWritersPreserveUniqueMonotonicIDs(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	const writers = 24
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, writers)
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := AppendMasterInboxMessage(runDir, "tell", "user", fmt.Sprintf("msg-%02d", i))
			errCh <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("AppendMasterInboxMessage concurrent append: %v", err)
		}
	}

	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read inbox: %v", err)
	}
	lines := splitNonEmptyLines(string(data))
	if len(lines) != writers {
		t.Fatalf("inbox lines = %d, want %d", len(lines), writers)
	}
	ids := make([]int, 0, len(lines))
	seen := make(map[int]struct{}, len(lines))
	for _, line := range lines {
		var msg MasterInboxMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			t.Fatalf("unmarshal inbox line: %v", err)
		}
		ids = append(ids, int(msg.ID))
		if _, ok := seen[int(msg.ID)]; ok {
			t.Fatalf("duplicate message id %d in inbox:\n%s", msg.ID, string(data))
		}
		seen[int(msg.ID)] = struct{}{}
	}
	sort.Ints(ids)
	for i, id := range ids {
		if want := i + 1; id != want {
			t.Fatalf("sorted ids[%d] = %d, want %d", i, id, want)
		}
	}
}

func TestUnreadControlInboxCountReturnsZeroWhenCursorCaughtUp(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "focus on e2e"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}
	if err := SaveMasterCursorState(MasterCursorPath(runDir), &MasterCursorState{LastSeenID: 1}); err != nil {
		t.Fatalf("SaveMasterCursorState: %v", err)
	}

	if got := unreadControlInboxCount(MasterInboxPath(runDir), MasterCursorPath(runDir)); got != 0 {
		t.Fatalf("unreadControlInboxCount = %d, want 0", got)
	}
}

func TestSendAgentNudgeCodexStagesPayloadAndSubmitSeparately(t *testing.T) {
	origSend := sendAgentKeys
	origCapture := captureAgentPane
	defer func() { sendAgentKeys = origSend }()
	defer func() { captureAgentPane = origCapture }()

	var calls []struct {
		target string
		keys   string
		submit string
	}
	sendAgentKeys = func(target, keys, submitKey string) error {
		calls = append(calls, struct {
			target string
			keys   string
			submit string
		}{target: target, keys: keys, submit: submitKey})
		return nil
	}
	captureAgentPane = func(target string) (string, error) {
		return "› ", nil
	}

	if err := SendAgentNudge("gx-demo:master", "codex"); err != nil {
		t.Fatalf("SendAgentNudge: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("sendAgentKeys calls = %d, want 2", len(calls))
	}
	if calls[0].target != "gx-demo:master" || calls[0].keys != transportWakeToken || calls[0].submit != "" {
		t.Fatalf("first codex send = %+v, want wake payload without submit", calls[0])
	}
	if calls[1].target != "gx-demo:master" || calls[1].keys != "" || calls[1].submit != "Enter" {
		t.Fatalf("second codex send = %+v, want Enter-only submit", calls[1])
	}
}

func TestSendAgentNudgeCodexRepairsBufferedWakeWithEnterOnly(t *testing.T) {
	origSend := sendAgentKeys
	origCapture := captureAgentPane
	defer func() { sendAgentKeys = origSend }()
	defer func() { captureAgentPane = origCapture }()

	var calls []struct {
		target string
		keys   string
		submit string
	}
	sendAgentKeys = func(target, keys, submitKey string) error {
		calls = append(calls, struct {
			target string
			keys   string
			submit string
		}{target: target, keys: keys, submit: submitKey})
		return nil
	}
	captureAgentPane = func(target string) (string, error) {
		return "› [[GOALX_WAKE_CHECK_INBOX]]\n  gpt-5.4 xhigh", nil
	}

	outcome, err := SendAgentNudgeDetailed("gx-demo:session-2", "codex")
	if err != nil {
		t.Fatalf("SendAgentNudgeDetailed: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("sendAgentKeys calls = %d, want 1", len(calls))
	}
	if calls[0].target != "gx-demo:session-2" || calls[0].keys != "" || calls[0].submit != "Enter" {
		t.Fatalf("codex repair send = %+v, want Enter-only submit", calls[0])
	}
	if outcome.SubmitMode != "enter_only_repair" || outcome.TransportState != "buffered_input" {
		t.Fatalf("codex repair outcome = %+v, want enter_only_repair/buffered_input", outcome)
	}
}

func TestSendAgentNudgeClaudeTreatsQueuedWakeAsAccepted(t *testing.T) {
	origSend := sendAgentKeys
	origCapture := captureAgentPane
	defer func() { sendAgentKeys = origSend }()
	defer func() { captureAgentPane = origCapture }()

	called := false
	sendAgentKeys = func(target, keys, submitKey string) error {
		called = true
		return nil
	}
	captureAgentPane = func(target string) (string, error) {
		return "❯ [[GOALX_WAKE_CHECK_INBOX]]\nPress up to edit queued messages", nil
	}

	outcome, err := SendAgentNudgeDetailed("gx-demo:session-2", "claude-code")
	if err != nil {
		t.Fatalf("SendAgentNudgeDetailed: %v", err)
	}
	if called {
		t.Fatal("sendAgentKeys called, want queued Claude wake treated as already accepted")
	}
	if outcome.SubmitMode != "accepted_existing_queue" || outcome.TransportState != "queued" {
		t.Fatalf("claude queued outcome = %+v, want accepted_existing_queue/queued", outcome)
	}
}

func TestSendAgentNudgeNonCodexUsesExplicitWakePayload(t *testing.T) {
	origSend := sendAgentKeys
	origCapture := captureAgentPane
	defer func() { sendAgentKeys = origSend }()
	defer func() { captureAgentPane = origCapture }()

	tests := []struct {
		name   string
		engine string
	}{
		{name: "claude", engine: "claude-code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotTarget, gotKeys, gotSubmit string
			sendAgentKeys = func(target, keys, submitKey string) error {
				gotTarget, gotKeys, gotSubmit = target, keys, submitKey
				return nil
			}
			captureAgentPane = func(target string) (string, error) {
				return "› ", nil
			}

			if err := SendAgentNudge("gx-demo:master", tt.engine); err != nil {
				t.Fatalf("SendAgentNudge: %v", err)
			}
			if gotTarget != "gx-demo:master" {
				t.Fatalf("target = %q, want gx-demo:master", gotTarget)
			}
			if gotKeys != transportWakeToken || gotSubmit != "Enter" {
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
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: pulse-run\nmode: worker\nobjective: ship it\nmaster:\n  engine: codex\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"pulse-run","mode":"worker","objective":"ship it","active":false,"phase":"working","updated_at":"2026-03-23T00:00:00Z"}`)
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
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: pulse-lag\nmode: worker\nobjective: ship it\nmaster:\n  engine: codex\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"pulse-lag","mode":"worker","objective":"ship it","active":false,"phase":"working","updated_at":"2026-03-23T00:00:00Z"}`)
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
