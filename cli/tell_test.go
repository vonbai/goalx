package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestTellWritesSessionInboxAndNudges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()

	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}

	out := captureStdout(t, func() {
		if err := Tell(repo, []string{"--run", runName, "session-1", "focus on db race triage"}); err != nil {
			t.Fatalf("Tell: %v", err)
		}
	})

	if !strings.Contains(out, "session-1") {
		t.Fatalf("tell output = %q, want session target", out)
	}

	inboxData, err := os.ReadFile(ControlInboxPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("read session inbox: %v", err)
	}
	text := string(inboxData)
	for _, want := range []string{`"type":"tell"`, `"source":"user"`, `"body":"focus on db race triage"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("session inbox missing %q:\n%s", want, text)
		}
	}
	if _, err := os.Stat(filepath.Join(runDir, "guidance")); !os.IsNotExist(err) {
		t.Fatalf("legacy guidance directory should not exist, stat err = %v", err)
	}

	wantTarget := goalx.TmuxSessionName(repo, runName) + ":" + sessionWindowName(runName, 1)
	if gotTarget != wantTarget || gotEngine != "codex" {
		t.Fatalf("sendAgentNudge target=%q engine=%q, want %q codex", gotTarget, gotEngine, wantTarget)
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 {
		t.Fatalf("deliveries len = %d, want 1", len(deliveries.Items))
	}
	if deliveries.Items[0].Status != "sent" || deliveries.Items[0].Target != wantTarget {
		t.Fatalf("unexpected delivery: %+v", deliveries.Items[0])
	}
}

func TestTellResolvesExplicitProjectSelectorOutsideProjectRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repoA := initNamedGitRepo(t, "project-a")
	writeAndCommit(t, repoA, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repoA)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if _, err := EnsureRunMetadata(runDir, repoA, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := RegisterActiveRun(repoA, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}

	repoB := initNamedGitRepo(t, "project-b")
	writeAndCommit(t, repoB, "other.txt", "other", "other commit")

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()
	called := false
	sendAgentNudge = func(target, engine string) error {
		called = true
		return nil
	}

	if err := Tell(repoB, []string{"--run", goalx.ProjectID(repoA) + "/" + runName, "master", "decide and execute"}); err != nil {
		t.Fatalf("Tell: %v", err)
	}

	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if !strings.Contains(string(data), "decide and execute") {
		t.Fatalf("master inbox = %q, want delivered message", string(data))
	}
	if !called {
		t.Fatal("sendAgentNudge was not called")
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "sent" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
	}
}

func TestTellUrgentWritesUrgentMasterInboxMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()

	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}

	out := captureStdout(t, func() {
		if err := Tell(repo, []string{"--run", runName, "--urgent", "drop everything and triage"}); err != nil {
			t.Fatalf("Tell urgent: %v", err)
		}
	})

	if !strings.Contains(out, "Told master") {
		t.Fatalf("tell output = %q, want master target", out)
	}

	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	text := string(data)
	for _, want := range []string{`"type":"tell"`, `"source":"user"`, `"body":"drop everything and triage"`, `"urgent":true`} {
		if !strings.Contains(text, want) {
			t.Fatalf("master inbox missing %q:\n%s", want, text)
		}
	}

	wantTarget := goalx.TmuxSessionName(repo, runName) + ":master"
	if gotTarget != wantTarget || gotEngine != "codex" {
		t.Fatalf("sendAgentNudge target=%q engine=%q, want %q codex", gotTarget, gotEngine, wantTarget)
	}
}

func TestTellHelpDoesNotDeliverAnything(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()

	called := false
	sendAgentNudge = func(target, engine string) error {
		called = true
		return nil
	}

	out := captureStdout(t, func() {
		if err := Tell(repo, []string{"--run", runName, "--help"}); err != nil {
			t.Fatalf("Tell --help: %v", err)
		}
	})

	if !strings.Contains(out, tellUsage) {
		t.Fatalf("tell help output = %q, want %q", out, tellUsage)
	}
	if called {
		t.Fatal("Tell --help should not nudge any target")
	}
	if _, err := os.Stat(ControlInboxPath(runDir, "session-1")); !os.IsNotExist(err) {
		t.Fatalf("session inbox should not be created by help, stat err = %v", err)
	}
}

func TestAckSessionMarksLatestInboxEntryConsumed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if _, err := AppendControlInboxMessage(runDir, "session-1", "tell", "user", "focus on db race triage"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	if err := AckSession(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("AckSession: %v", err)
	}

	state, err := LoadMasterCursorState(SessionCursorPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadMasterCursorState: %v", err)
	}
	if state == nil {
		t.Fatal("session cursor missing")
	}
	if state.LastSeenID != 1 {
		t.Fatalf("last seen id = %d, want 1", state.LastSeenID)
	}
	if state.UpdatedAt == "" {
		t.Fatalf("updated at empty")
	}
}

func TestStatusShowsInboxPendingForSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":2,"desc":"awaiting master","status":"idle"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
	if err := os.MkdirAll(ControlInboxDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir control inbox dir: %v", err)
	}
	if err := os.WriteFile(ControlInboxPath(runDir, "session-1"), []byte(`{"id":1,"type":"tell","source":"user","body":"focus on db race triage","created_at":"2026-03-24T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session inbox: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	if !strings.Contains(out, "inbox-pending") {
		t.Fatalf("status output missing inbox-pending:\n%s", out)
	}
}

func TestRenderSubagentProtocolIncludesSessionInboxAckPath(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeDevelop,
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
		JournalPath:       "/tmp/journals/session-1.jsonl",
		Target:            goalx.TargetConfig{Files: []string{"."}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"/tmp/control/inbox/session-1.jsonl",
		"/tmp/control/session-1-cursor.json",
		"goalx ack-session --run demo session-1",
		"cd /tmp/project && goalx ack-session --run demo session-1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q:\n%s", want, text)
		}
	}
}
