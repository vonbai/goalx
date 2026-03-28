package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestRunSidecarTickRenewsLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	if activity == nil || activity.Run.RunName != cfg.Name {
		t.Fatalf("activity snapshot not written correctly: %+v", activity)
	}
	index, err := LoadContextIndex(ContextIndexPath(runDir))
	if err != nil {
		t.Fatalf("LoadContextIndex: %v", err)
	}
	if index == nil || index.RunName != cfg.Name {
		t.Fatalf("context index not written correctly: %+v", index)
	}
	affordances, err := LoadAffordances(AffordancesJSONPath(runDir))
	if err != nil {
		t.Fatalf("LoadAffordances: %v", err)
	}
	if affordances == nil || affordances.RunName != cfg.Name {
		t.Fatalf("affordances not written correctly: %+v", affordances)
	}
	transportFacts, err := LoadTransportFacts(TransportFactsPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportFacts: %v", err)
	}
	if transportFacts == nil {
		t.Fatal("transport facts not written")
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "sidecar"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.Holder != "sidecar" {
		t.Fatalf("lease holder = %q, want sidecar", lease.Holder)
	}
	if lease.RunID != meta.RunID {
		t.Fatalf("lease run id = %q, want %q", lease.RunID, meta.RunID)
	}
	if lease.Epoch != meta.Epoch {
		t.Fatalf("lease epoch = %d, want %d", lease.Epoch, meta.Epoch)
	}
	if lease.PID != 4242 {
		t.Fatalf("lease pid = %d, want 4242", lease.PID)
	}
	if lease.RenewedAt == "" || lease.ExpiresAt == "" {
		t.Fatalf("lease timestamps missing: %+v", lease)
	}
	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "heartbeat.json")); !os.IsNotExist(err) {
		t.Fatalf("legacy heartbeat state should not exist, stat err = %v", err)
	}
}

func TestRunSidecarTickDeliversDueMasterWakeReminder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master; exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '%%0\tmaster\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		gotTarget, gotEngine = target, engine
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
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
	if deliveries.Items[0].Status != "sent" || deliveries.Items[0].DedupeKey != "master-wake" {
		t.Fatalf("unexpected delivery: %+v", deliveries.Items[0])
	}
}

func TestRunSidecarTickQueuesRefreshContextWhenIdentityFenceChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	coord, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	coord.Version++
	coord.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	coord.OpenQuestions = append(coord.OpenQuestions, "new priority")
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState updated: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	found := false
	for _, item := range reminders.Items {
		if item.DedupeKey == "refresh-context" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected refresh-context reminder after identity fence change: %+v", reminders.Items)
	}
}

func TestRunSidecarTickRelaunchesMissingMasterWindowOnActiveRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$TMUX_LOG\"\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' session-1; exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '%%1\tsession-1\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantSession := goalx.TmuxSessionName(repo, cfg.Name)
	for _, want := range []string{
		"new-window -t " + wantSession + " -n master -c " + RunWorktreePath(runDir),
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "kill-window") {
		t.Fatalf("missing-master relaunch should not kill window:\n%s", logText)
	}

	auditData, err := os.ReadFile(filepath.Join(runDir, "sidecar.log"))
	if err != nil {
		t.Fatalf("read sidecar audit log: %v", err)
	}
	auditText := string(auditData)
	for _, want := range []string{
		"target_relaunch_attempt target=master cause=window_missing",
		"target_relaunch_result target=master result=success cause=window_missing",
	} {
		if !strings.Contains(auditText, want) {
			t.Fatalf("sidecar audit log missing %q:\n%s", want, auditText)
		}
	}
}

func TestRunSidecarTickAlertsMasterOnceWhenSessionWindowMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master; exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '%%0\tmaster\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then printf 'prompt\n'; exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	for i := 0; i < 2; i++ {
		if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
			t.Fatalf("runSidecarTick #%d: %v", i+1, err)
		}
	}

	inbox, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inbox), `"type":"target-missing"`); got != 1 {
		t.Fatalf("target-missing alerts = %d, want 1:\n%s", got, string(inbox))
	}
	if _, err := os.Stat(SessionIdentityPath(runDir, "session-2")); !os.IsNotExist(err) {
		t.Fatalf("unexpected replacement worker created, stat err = %v", err)
	}

	recoveryData, err := os.ReadFile(TransportRecoveryPath(runDir))
	if err != nil {
		t.Fatalf("read transport recovery: %v", err)
	}
	recoveryText := string(recoveryData)
	for _, want := range []string{
		"session-1",
		"window_missing",
		"current_missing_first_seen_at",
		"current_missing_last_alert_at",
	} {
		if !strings.Contains(recoveryText, want) {
			t.Fatalf("transport recovery missing %q:\n%s", want, recoveryText)
		}
	}
}

func TestRunSidecarTickProjectsSessionJournalStateIntoRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Mode:         string(goalx.ModeResearch),
		WorktreePath: WorktreePath(runDir, cfg.Name, 1),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := os.MkdirAll(WorktreePath(runDir, cfg.Name, 1), 0o755); err != nil {
		t.Fatalf("mkdir session worktree: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":2,"desc":"awaiting master","status":"idle","owner_scope":"db race triage"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	sess := state.Sessions["session-1"]
	if sess.State != "idle" {
		t.Fatalf("session state = %q, want idle", sess.State)
	}
	if sess.LastJournalState != "idle" {
		t.Fatalf("session last journal state = %q, want idle", sess.LastJournalState)
	}
	if sess.LastRound != 2 {
		t.Fatalf("session last round = %d, want 2", sess.LastRound)
	}
	if sess.OwnerScope != "db race triage" {
		t.Fatalf("session owner scope = %q, want db race triage", sess.OwnerScope)
	}
}

func TestRunSidecarTickDoesNotQueueRefreshContextWhenIdentityFenceUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 0; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	for _, item := range reminders.Items {
		if item.DedupeKey == "refresh-context" {
			t.Fatalf("unexpected refresh-context reminder without fence change: %+v", reminders.Items)
		}
	}
}

func TestStopTerminatesSidecar(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if err := RegisterActiveRun(repo, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	var gotRunDir string
	stopRunSidecar = func(runDir string) error {
		gotRunDir = runDir
		return nil
	}

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if gotRunDir != runDir {
		t.Fatalf("stopRunSidecar runDir = %q, want %q", gotRunDir, runDir)
	}
}

func TestStopTerminalizesControlStateWhenRunIsAlreadyInactive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if err := RegisterActiveRun(repo, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := RenewControlLease(runDir, "master", "run_demo", 1, time.Minute, "tmux", exitedProcessPID(t)); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", exitedProcessPID(t)); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}
	if err := SaveControlReminders(ControlRemindersPath(runDir), &ControlReminders{
		Version: 1,
		Items: []ControlReminder{
			{ReminderID: "rem-1", DedupeKey: "master-wake", Reason: "control-cycle", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlReminders: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "master-wake", Status: "failed", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	runState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if runState.LifecycleState != "stopped" {
		t.Fatalf("lifecycle_state = %q, want stopped", runState.LifecycleState)
	}
	masterLease, err := LoadControlLease(ControlLeasePath(runDir, "master"))
	if err != nil {
		t.Fatalf("LoadControlLease master: %v", err)
	}
	if masterLease.PID != 0 || masterLease.RunID != "" {
		t.Fatalf("master lease not expired: %+v", masterLease)
	}
	sessionLease, err := LoadControlLease(ControlLeasePath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadControlLease session-1: %v", err)
	}
	if sessionLease.PID != 0 || sessionLease.RunID != "" {
		t.Fatalf("session lease not expired: %+v", sessionLease)
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 1 || !reminders.Items[0].Suppressed {
		t.Fatalf("unexpected reminders: %+v", reminders.Items)
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "cancelled" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
	}
	sessionsState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := sessionsState.Sessions["session-1"].State; got != "stopped" {
		t.Fatalf("session-1 state = %q, want stopped", got)
	}
}

func TestDropTerminatesSidecarBeforeRemovingRunDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName := "drop-run"
	runDir := goalx.RunDir(repo, runName)
	for _, dir := range []string{
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "worktrees"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: drop-run\nmode: research\nobjective: demo\ntarget:\n  files: [\"report.md\"]\nlocal_validation:\n  command: \"test -f base.txt\"\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	var gotRunDir string
	stopRunSidecar = func(runDir string) error {
		gotRunDir = runDir
		return nil
	}

	if err := Drop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	if gotRunDir != runDir {
		t.Fatalf("stopRunSidecar runDir = %q, want %q", gotRunDir, runDir)
	}
}

func TestSidecarRenewsLeaseUntilContextStops(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runSidecarLoop(ctx, repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	time.Sleep(120 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runSidecarLoop: %v", err)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "sidecar"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.RenewedAt == "" {
		t.Fatalf("sidecar lease renewed_at empty: %+v", lease)
	}
}

func TestSidecarStopsWhenRunIdentityChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runSidecarLoop(ctx, repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	time.Sleep(60 * time.Millisecond)
	meta.RunID = newRunID()
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runSidecarLoop: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("sidecar did not stop after run identity changed")
	}
}

func TestSidecarFinalizesCompletedRunWhenMasterSessionIsGone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if err := RegisterActiveRun(repo, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}

	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	runState.Active = true
	runState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), runState); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(CompletionStatePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir proof dir: %v", err)
	}
	if err := os.WriteFile(CompletionStatePath(runDir), []byte(`{"completed_at":"2026-03-27T16:02:03Z"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := RenewControlLease(runDir, "master", "run_demo", 1, time.Minute, "tmux", exitedProcessPID(t)); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", exitedProcessPID(t)); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}
	if err := SaveControlReminders(ControlRemindersPath(runDir), &ControlReminders{
		Version: 1,
		Items: []ControlReminder{
			{ReminderID: "rem-1", DedupeKey: "master-wake", Reason: "control-cycle", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlReminders: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "master-wake", Status: "failed", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runSidecarLoop(ctx, repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runSidecarLoop: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("sidecar did not stop after completed run lost its master session")
	}

	runState, err = LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState after: %v", err)
	}
	if runState.Active {
		t.Fatalf("run state still active: %+v", runState)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState after: %v", err)
	}
	if controlState.LifecycleState != "completed" {
		t.Fatalf("lifecycle_state = %q, want completed", controlState.LifecycleState)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "master"))
	if err != nil {
		t.Fatalf("LoadControlLease master: %v", err)
	}
	if lease.PID != 0 || lease.RunID != "" {
		t.Fatalf("master lease not expired: %+v", lease)
	}
	lease, err = LoadControlLease(ControlLeasePath(runDir, "sidecar"))
	if err != nil {
		t.Fatalf("LoadControlLease sidecar: %v", err)
	}
	if lease.PID != 0 || lease.RunID != "" {
		t.Fatalf("sidecar lease not expired: %+v", lease)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders after: %v", err)
	}
	if len(reminders.Items) != 1 || !reminders.Items[0].Suppressed {
		t.Fatalf("unexpected reminders after finalize: %+v", reminders.Items)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries after: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "cancelled" {
		t.Fatalf("unexpected deliveries after finalize: %+v", deliveries.Items)
	}

	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	if _, ok := reg.ActiveRuns[runName]; ok {
		t.Fatalf("run %q still registered active after completion", runName)
	}
}

func TestSidecarKeepsCompletedRunAliveWhenUnreadMasterInboxExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}

	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	runState.Active = true
	runState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), runState); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(CompletionStatePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir proof dir: %v", err)
	}
	if err := os.WriteFile(CompletionStatePath(runDir), []byte(`{"completed_at":"2026-03-27T16:02:03Z"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "reopen and fix verification", false); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 1
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.LifecycleState == "completed" {
		t.Fatalf("lifecycle_state = %q, want run kept active for unread master inbox", controlState.LifecycleState)
	}
	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "handoffs", "master.json")); !os.IsNotExist(err) {
		t.Fatalf("sidecar relaunch should not create legacy handoff file, stat err = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantSession := goalx.TmuxSessionName(repo, runName)
	for _, want := range []string{
		"new-session -d -s " + wantSession + " -n master -c " + RunWorktreePath(runDir),
		filepath.Join(runDir, "master.md"),
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}
}

func TestSidecarFinalizesCompletedRunWhenMasterWindowMissingButWorkerWindowsRemain(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if err := RegisterActiveRun(repo, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}

	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	runState.Active = true
	runState.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), runState); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(CompletionStatePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir proof dir: %v", err)
	}
	if err := os.WriteFile(CompletionStatePath(runDir), []byte(`{"completed_at":"2026-03-27T16:02:03Z"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := RenewControlLease(runDir, "master", "run_demo", 1, time.Minute, "tmux", exitedProcessPID(t)); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", exitedProcessPID(t)); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}

	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf 'session-1\n'
    exit 0
    ;;
  list-panes)
    printf '%s\t%%1\tsession-1\n' "$TMUX_SESSION_NAME"
    exit 0
    ;;
  kill-session)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("TMUX_SESSION_NAME", goalx.TmuxSessionName(repo, runName))
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, 4242); err == nil || !errors.Is(err, errSidecarCompleted) {
		t.Fatalf("runSidecarTick err = %v, want %v", err, errSidecarCompleted)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.LifecycleState != "completed" {
		t.Fatalf("lifecycle_state = %q, want completed", controlState.LifecycleState)
	}

	runState, err = LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState after: %v", err)
	}
	if runState.Active {
		t.Fatalf("run state still active: %+v", runState)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "kill-session -t "+goalx.TmuxSessionName(repo, runName)) {
		t.Fatalf("tmux log missing kill-session:\n%s", string(logData))
	}
}

func TestRunSidecarTickAlertsMissingSessionWindowOnceWithoutRespawn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	logPath := installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick first: %v", err)
	}
	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick second: %v", err)
	}

	recovery, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportRecovery: %v", err)
	}
	sessionRecovery := recovery.Targets["session-1"]
	if sessionRecovery.CurrentMissingState != TargetPresenceWindowMissing {
		t.Fatalf("session recovery state = %q, want %q", sessionRecovery.CurrentMissingState, TargetPresenceWindowMissing)
	}
	if sessionRecovery.CurrentMissingFirstSeenAt == "" || sessionRecovery.CurrentMissingLastAlertAt == "" {
		t.Fatalf("session recovery timestamps missing: %+v", sessionRecovery)
	}
	if !strings.Contains(sessionRecovery.CurrentMissingLastAlertReason, "session-1") {
		t.Fatalf("session alert reason missing target name: %+v", sessionRecovery)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inboxData), `"type":"target-missing"`); got != 1 {
		t.Fatalf("master inbox target-missing count = %d, want 1\n%s", got, string(inboxData))
	}
	if !strings.Contains(string(inboxData), "do_not_respawn_worker") {
		t.Fatalf("master inbox missing no-respawn guidance:\n%s", string(inboxData))
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, cfg.Name)+" -n session-1") {
		t.Fatalf("missing session should not respawn worker:\n%s", string(logData))
	}

	auditData, err := os.ReadFile(filepath.Join(runDir, "sidecar.log"))
	if err != nil {
		t.Fatalf("read sidecar audit log: %v", err)
	}
	if strings.Count(string(auditData), "target_missing_alert target=session-1 cause=window_missing") != 1 {
		t.Fatalf("missing session alert should be recorded once:\n%s", string(auditData))
	}
}

func TestRunSidecarTickDoesNotAlertMissingParkedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	logPath := installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "parked",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{State: "parked", Scope: "reusable slice"}
	coord.Version++
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if strings.Contains(string(inboxData), `"type":"target-missing"`) {
		t.Fatalf("parked session should not trigger missing alert:\n%s", string(inboxData))
	}

	auditData, err := os.ReadFile(filepath.Join(runDir, "sidecar.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read sidecar audit log: %v", err)
	}
	if strings.Contains(string(auditData), "target_missing_alert target=session-1") {
		t.Fatalf("parked session should not emit missing alert:\n%s", string(auditData))
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, cfg.Name)+" -n session-1") {
		t.Fatalf("parked session should not respawn worker:\n%s", string(logData))
	}
}

func TestRunSidecarTickQueuesSessionWakeForUnreadSessionInbox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master session-1; exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '%%0\tmaster\n%%1\tsession-1\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	found := false
	wantTarget := goalx.TmuxSessionName(repo, runName) + ":session-1"
	for _, item := range deliveries.Items {
		if item.DedupeKey == "session-wake:session-1" {
			found = true
			if item.Target != wantTarget || item.Status != "sent" {
				t.Fatalf("unexpected session wake delivery: %+v", item)
			}
		}
	}
	if !found {
		t.Fatalf("expected session wake delivery, got %+v", deliveries.Items)
	}
	found = false
	for _, nudge := range nudges {
		if nudge == wantTarget+"|codex" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected session wake nudge to %q, got %v", wantTarget, nudges)
	}
}

func TestRunSidecarTickDoesNotQueueSessionWakeWhenCursorCaughtUp(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveMasterCursorState(SessionCursorPath(runDir, "session-1"), &MasterCursorState{LastSeenID: 1}); err != nil {
		t.Fatalf("SaveMasterCursorState: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master session-1; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	for _, item := range deliveries.Items {
		if item.DedupeKey == "session-wake:session-1" {
			t.Fatalf("unexpected session wake delivery after cursor caught up: %+v", item)
		}
	}
	for _, nudge := range nudges {
		if strings.Contains(nudge, ":session-1|") {
			t.Fatalf("unexpected session wake nudge after cursor caught up: %v", nudges)
		}
	}
}

func TestRunSidecarTickSkipsSessionWakeWhenWindowMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	for _, item := range deliveries.Items {
		if item.DedupeKey == "session-wake:session-1" {
			t.Fatalf("unexpected session wake delivery with missing window: %+v", item)
		}
	}
	for _, nudge := range nudges {
		if strings.Contains(nudge, ":session-1|") {
			t.Fatalf("unexpected session wake nudge with missing window: %v", nudges)
		}
	}
}

func TestRunSidecarTickSkipsSessionWakeDuringRecentSuccessfulTellGrace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "tell", "user", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{
				DeliveryID:  "del-1",
				MessageID:   "session-inbox:session-1:1",
				DedupeKey:   "session-inbox:session-1:1",
				Target:      goalx.TmuxSessionName(repo, runName) + ":session-1",
				Adapter:     "tmux",
				Status:      "sent",
				AttemptedAt: time.Now().UTC().Format(time.RFC3339),
				AcceptedAt:  time.Now().UTC().Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master session-1; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	count := 0
	for _, item := range deliveries.Items {
		if item.DedupeKey == "session-wake:session-1" {
			count++
		}
	}
	if count != 0 {
		t.Fatalf("unexpected session wake deliveries during recent tell grace: %+v", deliveries.Items)
	}
	for _, nudge := range nudges {
		if strings.Contains(nudge, ":session-1|") {
			t.Fatalf("unexpected session wake nudge during recent tell grace: %v", nudges)
		}
	}
}

func TestRunSidecarTickQueuesSessionRepairWhenTransportStillBuffered(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "tell", "user", "repair buffered wake"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{
				DeliveryID:  "del-1",
				MessageID:   "session-inbox:session-1:1",
				DedupeKey:   "session-inbox:session-1:1",
				Target:      goalx.TmuxSessionName(repo, runName) + ":session-1",
				Adapter:     "tmux",
				Status:      "sent",
				AcceptedAt:  time.Now().UTC().Format(time.RFC3339),
				AttemptedAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("› [[GOALX_WAKE_CHECK_INBOX]]\n  gpt-5.4 xhigh\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master session-1; exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '%%0\tmaster\n%%1\tsession-1\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then\n" +
		"  target=\"\"\n" +
		"  while [ $# -gt 0 ]; do\n" +
		"    if [ \"$1\" = \"-t\" ]; then target=\"$2\"; shift 2; continue; fi\n" +
		"    shift\n" +
		"  done\n" +
		"  case \"$target\" in\n" +
		"    *:master) cat \"$TMUX_MASTER_CAPTURE\" ;;\n" +
		"    *:session-1) cat \"$TMUX_SESSION1_CAPTURE\" ;;\n" +
		"  esac\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	found := false
	for _, item := range deliveries.Items {
		if item.DedupeKey == "session-wake:session-1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected session repair wake after buffered transport, got %+v", deliveries.Items)
	}
}

func TestRunSidecarTickSkipsSessionWakeWhenTransportAlreadyAccepted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "tell", "user", "already accepted"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("• Working (24s)\n• Messages to be submitted after next tool call\n  ↳ [[GOALX_WAKE_CHECK_INBOX]]\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	script := "#!/bin/sh\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then printf '%s\n' master session-1; exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '%%0\tmaster\n%%1\tsession-1\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then\n" +
		"  target=\"\"\n" +
		"  while [ $# -gt 0 ]; do\n" +
		"    if [ \"$1\" = \"-t\" ]; then target=\"$2\"; shift 2; continue; fi\n" +
		"    shift\n" +
		"  done\n" +
		"  case \"$target\" in\n" +
		"    *:master) cat \"$TMUX_MASTER_CAPTURE\" ;;\n" +
		"    *:session-1) cat \"$TMUX_SESSION1_CAPTURE\" ;;\n" +
		"  esac\n" +
		"  exit 0\n" +
		"fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runSidecarTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	for _, item := range deliveries.Items {
		if item.DedupeKey == "session-wake:session-1" {
			t.Fatalf("unexpected session wake delivery after accepted transport: %+v", item)
		}
	}
}

func TestRunSidecarTickElevatesProviderDialogToUrgentMasterFact(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Please authenticate in browser to continue\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("ReadFile master inbox: %v", err)
	}
	if !strings.Contains(string(inboxData), `"type":"provider-dialog-visible"`) {
		t.Fatalf("master inbox missing provider-dialog-visible fact:\n%s", string(inboxData))
	}
	if !strings.Contains(string(inboxData), `"urgent":true`) {
		t.Fatalf("master inbox missing urgent provider dialog fact:\n%s", string(inboxData))
	}
	if !strings.Contains(string(inboxData), `target=session-1 engine=codex kind=auth_prompt`) {
		t.Fatalf("master inbox missing provider dialog body details:\n%s", string(inboxData))
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.ProviderDialogAlerts["session-1"] == "" {
		t.Fatalf("provider dialog alert fingerprint missing from control state: %+v", controlState.ProviderDialogAlerts)
	}

	transportFacts, err := LoadTransportFacts(TransportFactsPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportFacts: %v", err)
	}
	if !transportFacts.Targets["session-1"].ProviderDialogVisible {
		t.Fatalf("transport facts missing provider dialog visibility: %+v", transportFacts.Targets["session-1"])
	}
}

func TestRunSidecarTickElevatesCapacityPickerDialogToUrgentMasterFact(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Choose a model to continue\nModel capacity picker\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("ReadFile master inbox: %v", err)
	}
	if !strings.Contains(string(inboxData), `"type":"provider-dialog-visible"`) {
		t.Fatalf("master inbox missing provider-dialog-visible fact:\n%s", string(inboxData))
	}
	if !strings.Contains(string(inboxData), `"urgent":true`) {
		t.Fatalf("master inbox missing urgent provider dialog fact:\n%s", string(inboxData))
	}
	if !strings.Contains(string(inboxData), `target=session-1 engine=codex kind=capacity_picker`) {
		t.Fatalf("master inbox missing capacity picker body details:\n%s", string(inboxData))
	}
}

func bootstrapSidecarIdentityFixture(t *testing.T, runDir, repo string, cfg *goalx.Config, meta *RunMetadata) {
	t.Helper()

	goalState, err := EnsureGoalState(runDir)
	if err != nil {
		t.Fatalf("EnsureGoalState: %v", err)
	}
	if err := EnsureGoalLog(runDir); err != nil {
		t.Fatalf("EnsureGoalLog: %v", err)
	}
	if _, err := EnsureAcceptanceState(runDir, cfg, goalState.Version); err != nil {
		t.Fatalf("EnsureAcceptanceState: %v", err)
	}
	charter, err := NewRunCharter(runDir, cfg.Name, cfg.Objective, meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		t.Fatalf("hashRunCharter: %v", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if _, err := EnsureCoordinationState(runDir, cfg.Objective); err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	fence, err := NewIdentityFence(runDir, meta)
	if err != nil {
		t.Fatalf("NewIdentityFence: %v", err)
	}
	if err := SaveIdentityFence(IdentityFencePath(runDir), fence); err != nil {
		t.Fatalf("SaveIdentityFence: %v", err)
	}
}
