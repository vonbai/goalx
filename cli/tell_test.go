package cli

import (
	"fmt"
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
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()

	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		gotTarget, gotEngine = target, engine
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
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
	if deliveries.Items[0].Status != "accepted" || deliveries.Items[0].Target != wantTarget {
		t.Fatalf("unexpected delivery: %+v", deliveries.Items[0])
	}
}

func TestTellKeepsDurableSessionMessageWhenImmediateNudgeFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	sendAgentNudge = func(target, engine string) error {
		return fmt.Errorf("tmux target missing")
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{}, fmt.Errorf("tmux target missing")
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
	if !strings.Contains(string(inboxData), `"body":"focus on db race triage"`) {
		t.Fatalf("session inbox missing message:\n%s", string(inboxData))
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "failed" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
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
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")

	repoB := initNamedGitRepo(t, "project-b")
	writeAndCommit(t, repoB, "other.txt", "other", "other commit")

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	called := false
	sendAgentNudge = func(target, engine string) error {
		called = true
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		called = true
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
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
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "accepted" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
	}
}

func TestTellUrgentWritesUrgentMasterInboxMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()

	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		gotTarget, gotEngine = target, engine
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
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

func TestTellRejectsCompletedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "completed", ContinuityState: "stopped"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	err := Tell(repo, []string{"--run", runName, "master", "reopen and fix verification"})
	if err == nil || !strings.Contains(err.Error(), `run "`+runName+`" is completed`) {
		t.Fatalf("Tell error = %v, want completed-run rejection", err)
	}

	data, readErr := os.ReadFile(MasterInboxPath(runDir))
	if readErr != nil {
		t.Fatalf("read master inbox: %v", readErr)
	}
	if strings.TrimSpace(string(data)) != "" {
		t.Fatalf("master inbox = %q, want empty", string(data))
	}
}

func TestTellHelpDoesNotDeliverAnything(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()

	called := false
	sendAgentNudge = func(target, engine string) error {
		called = true
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		called = true
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
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

func TestAckInboxAcknowledgesMasterAndRefreshesActivity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	msg, err := AppendMasterInboxMessage(runDir, "tell", "user", "process the worker results")
	if err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}

	out := captureStdout(t, func() {
		if err := AckInbox(repo, []string{"--run", runName, "master"}); err != nil {
			t.Fatalf("AckInbox: %v", err)
		}
	})
	if !strings.Contains(out, "Acknowledged master inbox") {
		t.Fatalf("ack output = %q, want master ack confirmation", out)
	}

	cursor, err := LoadMasterCursorState(MasterCursorPath(runDir))
	if err != nil {
		t.Fatalf("LoadMasterCursorState: %v", err)
	}
	if cursor == nil || cursor.LastSeenID != msg.ID {
		t.Fatalf("master cursor = %+v, want last_seen_id=%d", cursor, msg.ID)
	}
	if unread := unreadControlInboxCount(MasterInboxPath(runDir), MasterCursorPath(runDir)); unread != 0 {
		t.Fatalf("master unread = %d, want 0", unread)
	}

	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	if activity == nil || activity.Queue.MasterUnread != 0 {
		t.Fatalf("activity queue = %+v, want master_unread=0", activity)
	}
}

func TestAckInboxAcknowledgesSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	msg, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take next slice")
	if err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	if err := AckInbox(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("AckInbox session: %v", err)
	}
	cursor, err := LoadMasterCursorState(SessionCursorPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadMasterCursorState(session): %v", err)
	}
	if cursor == nil || cursor.LastSeenID != msg.ID {
		t.Fatalf("session cursor after ack-inbox = %+v, want last_seen_id=%d", cursor, msg.ID)
	}
}

func TestAckInboxMarksLatestInboxEntryConsumed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if _, err := AppendControlInboxMessage(runDir, "session-1", "tell", "user", "focus on db race triage"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	if err := AckInbox(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("AckInbox: %v", err)
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

func TestStatusShowsSessionQueueFactsWithoutDerivedInboxState(t *testing.T) {
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

	for _, want := range []string{"idle", "unread=1", "cursor=0/1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestRenderSubagentProtocolIncludesSessionInboxAckPath(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
		JournalPath:            "/tmp/journals/session-1.jsonl",
		Target:                 goalx.TargetConfig{Files: []string{"."}},
		LocalValidationCommand: "go test ./...",
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
		"goalx ack-inbox --run demo session-1",
		"cd /tmp/project && goalx ack-inbox --run demo session-1",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q:\n%s", want, text)
		}
	}
}
