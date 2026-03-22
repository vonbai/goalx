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
		MasterStatePath(runDir),
		HeartbeatStatePath(runDir),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestAppendMasterInboxMessageAssignsMonotonicIDs(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	first, err := AppendMasterInboxMessage(runDir, "heartbeat", "system", "tick")
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
	for _, want := range []string{`"type":"heartbeat"`, `"type":"tell"`, `"source":"user"`, `"body":"focus on e2e"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("inbox missing %q:\n%s", want, text)
		}
	}
}

func TestRecordHeartbeatTickIncrementsSequence(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	first, err := RecordHeartbeatTick(runDir)
	if err != nil {
		t.Fatalf("RecordHeartbeatTick first: %v", err)
	}
	second, err := RecordHeartbeatTick(runDir)
	if err != nil {
		t.Fatalf("RecordHeartbeatTick second: %v", err)
	}

	if second.Seq != first.Seq+1 {
		t.Fatalf("second.Seq = %d, want %d", second.Seq, first.Seq+1)
	}
}

func TestPlanAgentNudgeForBusyCodexStates(t *testing.T) {
	pane := strings.Join([]string{
		"• Working (1m 36s • esc to interrupt)",
		"",
		"  tab to queue message",
	}, "\n")

	plan := planAgentNudge("codex", pane)
	if !plan.Skip {
		t.Fatalf("planAgentNudge(codex, absent) = %#v", plan)
	}

	draftedPane := strings.Join([]string{
		"• Working (1m 36s • esc to interrupt)",
		"",
		"› goalx-hb",
		"",
		"  tab to queue message",
	}, "\n")
	plan = planAgentNudge("codex", draftedPane)
	if plan.Keys != "" || plan.SubmitKey != "Enter" || plan.Skip {
		t.Fatalf("planAgentNudge(codex, drafted) = %#v, want submit existing draft", plan)
	}

	queuedPane := strings.Join([]string{
		"• Working (1m 36s • esc to interrupt)",
		"",
		"› goalx-hb",
		"  goalx-hb",
		"  goalx-hb",
		"",
		"  tab to queue message",
	}, "\n")
	plan = planAgentNudge("codex", queuedPane)
	if plan.Keys != "" || plan.SubmitKey != "Enter" || plan.Skip {
		t.Fatalf("planAgentNudge(codex, queued) = %#v, want submit queued wake", plan)
	}

	readyDraftPane := strings.Join([]string{
		"› goalx-hb",
		"",
		"Ready for input",
	}, "\n")
	plan = planAgentNudge("codex", readyDraftPane)
	if plan.Keys != "" || plan.SubmitKey != "Enter" || plan.Skip {
		t.Fatalf("planAgentNudge(codex, ready drafted) = %#v", plan)
	}

	plan = planAgentNudge("claude-code", draftedPane)
	if plan.Keys != masterWakeMessage || plan.SubmitKey != "Enter" || plan.Skip {
		t.Fatalf("planAgentNudge(claude-code, busy pane) = %#v", plan)
	}
}

func TestSendAgentNudgeSubmitsQueuedBusyCodexWake(t *testing.T) {
	origCapture := captureAgentPane
	origSend := sendAgentKeys
	defer func() {
		captureAgentPane = origCapture
		sendAgentKeys = origSend
	}()

	captureAgentPane = func(target string) (string, error) {
		return strings.Join([]string{
			"• Working (5m 59s • esc to interrupt)",
			"",
			"› goalx-hb",
			"  goalx-hb",
			"  goalx-hb",
			"",
			"  tab to queue message",
		}, "\n"), nil
	}
	var gotKeys, gotSubmit string
	sendAgentKeys = func(target, keys, submitKey string) error {
		gotKeys, gotSubmit = keys, submitKey
		return nil
	}

	if err := SendAgentNudge("gx-demo:master", "codex"); err != nil {
		t.Fatalf("SendAgentNudge: %v", err)
	}
	if gotKeys != "" || gotSubmit != "Enter" {
		t.Fatalf("SendAgentNudge used keys=%q submit=%q, want empty keys + Enter for queued busy codex", gotKeys, gotSubmit)
	}
}

func TestSendAgentNudgeSubmitsBusyCodexDraft(t *testing.T) {
	origCapture := captureAgentPane
	origSend := sendAgentKeys
	defer func() {
		captureAgentPane = origCapture
		sendAgentKeys = origSend
	}()

	captureAgentPane = func(target string) (string, error) {
		return strings.Join([]string{
			"• Working (5m 59s • esc to interrupt)",
			"",
			"› goalx-hb",
			"",
			"  tab to queue message",
		}, "\n"), nil
	}
	var gotKeys, gotSubmit string
	sendAgentKeys = func(target, keys, submitKey string) error {
		gotKeys, gotSubmit = keys, submitKey
		return nil
	}

	if err := SendAgentNudge("gx-demo:master", "codex"); err != nil {
		t.Fatalf("SendAgentNudge: %v", err)
	}
	if gotKeys != "" || gotSubmit != "Enter" {
		t.Fatalf("SendAgentNudge used keys=%q submit=%q, want empty keys + Enter for drafted busy codex", gotKeys, gotSubmit)
	}
}

func TestSendAgentNudgeSubmitsExistingDraftForReadyCodex(t *testing.T) {
	origCapture := captureAgentPane
	origSend := sendAgentKeys
	defer func() {
		captureAgentPane = origCapture
		sendAgentKeys = origSend
	}()

	captureAgentPane = func(target string) (string, error) {
		return strings.Join([]string{
			"› goalx-hb",
			"",
			"Ready for input",
		}, "\n"), nil
	}
	var gotKeys, gotSubmit string
	sendAgentKeys = func(target, keys, submitKey string) error {
		gotKeys, gotSubmit = keys, submitKey
		return nil
	}

	if err := SendAgentNudge("gx-demo:master", "codex"); err != nil {
		t.Fatalf("SendAgentNudge: %v", err)
	}
	if gotKeys != "" || gotSubmit != "Enter" {
		t.Fatalf("SendAgentNudge used keys=%q submit=%q, want empty keys + Enter", gotKeys, gotSubmit)
	}
}

func TestPulseRecordsHeartbeatAndUsesControlNudge(t *testing.T) {
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

	var gotTarget, gotEngine string
	orig := sendAgentNudge
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}
	defer func() { sendAgentNudge = orig }()

	if err := Pulse(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Pulse: %v", err)
	}

	state, err := LoadHeartbeatState(HeartbeatStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadHeartbeatState: %v", err)
	}
	if state.Seq != 1 {
		t.Fatalf("heartbeat seq = %d, want 1", state.Seq)
	}
	gotRunState, err := os.ReadFile(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("read run state: %v", err)
	}
	if string(gotRunState) != string(runStateBefore) {
		t.Fatalf("run state changed:\nwant %s\ngot  %s", string(runStateBefore), string(gotRunState))
	}
	if gotTarget == "" || gotEngine != "codex" {
		t.Fatalf("nudge target=%q engine=%q, want codex target", gotTarget, gotEngine)
	}
}

func TestPulseTracksHeartbeatLagAndStaleState(t *testing.T) {
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

	for i := 0; i < heartbeatLagStaleThreshold; i++ {
		if err := Pulse(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Pulse #%d: %v", i+1, err)
		}
	}

	state, err := LoadMasterState(MasterStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadMasterState: %v", err)
	}
	if state.HeartbeatLag != heartbeatLagStaleThreshold {
		t.Fatalf("heartbeat lag = %d, want %d", state.HeartbeatLag, heartbeatLagStaleThreshold)
	}
	if !state.WakePending {
		t.Fatalf("wake pending = false, want true")
	}
	if state.StaleSince == "" {
		t.Fatalf("stale since empty, want value")
	}
	gotRunState, err := os.ReadFile(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("read run state: %v", err)
	}
	if string(gotRunState) != string(runStateBefore) {
		t.Fatalf("run state changed:\nwant %s\ngot  %s", string(runStateBefore), string(gotRunState))
	}

	statusData, err := os.ReadFile(filepath.Join(repo, ".goalx", "status.json"))
	if err != nil {
		t.Fatalf("read status.json: %v", err)
	}
	statusText := string(statusData)
	for _, want := range []string{
		`"heartbeat_lag":3`,
		`"master_wake_pending":true`,
		`"master_stale":true`,
	} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("status.json missing %q:\n%s", want, statusText)
		}
	}
}
