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

func TestNudgeSubmitKeyQueuesBusyCodex(t *testing.T) {
	pane := strings.Join([]string{
		"• Working (1m 36s • esc to interrupt)",
		"",
		"› goalx-hb",
		"",
		"  tab to queue message",
	}, "\n")

	if got := nudgeSubmitKey("codex", pane); got != "Tab" {
		t.Fatalf("nudgeSubmitKey(codex, busy pane) = %q, want %q", got, "Tab")
	}
	if got := nudgeSubmitKey("claude-code", pane); got != "Enter" {
		t.Fatalf("nudgeSubmitKey(claude-code, busy pane) = %q, want %q", got, "Enter")
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
	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), []byte("name: pulse-run\nmode: develop\nobjective: ship it\nmaster:\n  engine: codex\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
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
	if gotTarget == "" || gotEngine != "codex" {
		t.Fatalf("nudge target=%q engine=%q, want codex target", gotTarget, gotEngine)
	}
}
