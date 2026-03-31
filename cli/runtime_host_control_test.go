package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestRunRuntimeHostTickRenewsLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := EnsureSuccessCompilation(repo, runDir, cfg, meta); err != nil {
		t.Fatalf("EnsureSuccessCompilation: %v", err)
	}
	if _, _, err := RefreshIdentityFence(runDir, meta); err != nil {
		t.Fatalf("RefreshIdentityFence: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

	lease, err := LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.Holder != "runtime-host" {
		t.Fatalf("lease holder = %q, want runtime-host", lease.Holder)
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

func TestRunRuntimeHostTickAppliesPendingControlOpsBeforeMaintenance(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := appendControlOp(runDir, controlOpReminderQueue, controlReminderQueueBody{
		DedupeKey: "pending-op",
		Reason:    "queued-before-runtime-host",
		Target:    "gx-demo:master",
		Engine:    "codex",
	}); err != nil {
		t.Fatalf("appendControlOp: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nif [ \"$1\" = \"has-session\" ]; then exit 1; fi\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	found := false
	for _, item := range reminders.Items {
		if item.DedupeKey == "pending-op" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("pending control op not applied into reminders: %+v", reminders.Items)
	}
}

func TestRunRuntimeHostTickWritesEvolveFactsOnlyForEvolveRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick evolve: %v", err)
	}
	if _, err := os.Stat(EvolveFactsPath(runDir)); err != nil {
		t.Fatalf("expected evolve facts file, stat err = %v", err)
	}

	runDir2 := writeRunSpecFixture(t, repo, &goalx.Config{
		Name:      "runtime-host-run-non-evolve",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	})
	meta2, err := EnsureRunMetadata(runDir2, repo, "ship feature")
	if err != nil {
		t.Fatalf("EnsureRunMetadata non-evolve: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir2, repo, &goalx.Config{
		Name:      "runtime-host-run-non-evolve",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}, meta2)
	if _, err := EnsureRuntimeState(runDir2, &goalx.Config{
		Name:      "runtime-host-run-non-evolve",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}); err != nil {
		t.Fatalf("EnsureRuntimeState non-evolve: %v", err)
	}
	if err := EnsureControlState(runDir2); err != nil {
		t.Fatalf("EnsureControlState non-evolve: %v", err)
	}
	if err := runRuntimeHostTick(repo, "runtime-host-run-non-evolve", runDir2, meta2.RunID, meta2.Epoch, 2*time.Minute, 4343); err != nil {
		t.Fatalf("runRuntimeHostTick non-evolve: %v", err)
	}
	if _, err := os.Stat(EvolveFactsPath(runDir2)); !os.IsNotExist(err) {
		t.Fatalf("expected no evolve facts for non-evolve run, stat err = %v", err)
	}
}

func TestRunRuntimeHostTickDeliversDueMasterWakeReminder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickQueuesRefreshContextWhenIdentityFenceChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickRelaunchesMissingMasterWindowOnActiveRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})

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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

	auditData, err := os.ReadFile(filepath.Join(runDir, "runtime-host.log"))
	if err != nil {
		t.Fatalf("read runtime-host audit log: %v", err)
	}
	auditText := string(auditData)
	for _, want := range []string{
		"target_relaunch_attempt target=master cause=window_missing",
		"target_relaunch_result target=master result=success cause=window_missing",
	} {
		if !strings.Contains(auditText, want) {
			t.Fatalf("runtime-host audit log missing %q:\n%s", want, auditText)
		}
	}
}

func TestRunRuntimeHostTickAlertsMasterOnceWhenSessionWindowMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})

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
		if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
			t.Fatalf("runRuntimeHostTick #%d: %v", i+1, err)
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

func TestRunRuntimeHostTickRelaunchesMissingMasterSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$TMUX_LOG\"\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 1; fi\n" +
		"if [ \"$1\" = \"new-session\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantSession := goalx.TmuxSessionName(repo, cfg.Name)
	if !strings.Contains(logText, "new-session -d -s "+wantSession+" -n master -c "+RunWorktreePath(runDir)) {
		t.Fatalf("tmux log missing master session relaunch:\n%s", logText)
	}

	auditData, err := os.ReadFile(filepath.Join(runDir, "runtime-host.log"))
	if err != nil {
		t.Fatalf("read runtime-host audit log: %v", err)
	}
	auditText := string(auditData)
	if !strings.Contains(auditText, "target_relaunch_attempt target=master cause=session_missing") {
		t.Fatalf("runtime-host audit log missing session_missing relaunch attempt:\n%s", auditText)
	}
}

func TestRunRuntimeHostTickProjectsSessionJournalStateIntoRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Mode:         string(goalx.ModeWorker),
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickDoesNotQueueRefreshContextWhenIdentityFenceUnchanged(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	if err := EnsureSuccessCompilation(repo, runDir, cfg, meta); err != nil {
		t.Fatalf("EnsureSuccessCompilation: %v", err)
	}
	if _, err := RefreshRunGuidance(repo, cfg.Name, runDir); err != nil {
		t.Fatalf("RefreshRunGuidance: %v", err)
	}
	if _, _, err := RefreshIdentityFence(runDir, meta); err != nil {
		t.Fatalf("RefreshIdentityFence: %v", err)
	}
	if err := os.Remove(ControlRemindersPath(runDir)); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove reminders: %v", err)
	}

	installGuidanceFakeTmux(t, nil)

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestStopTerminatesRuntimeHost(t *testing.T) {
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

	origRuntimeSupervisor := runtimeSupervisor
	defer func() { runtimeSupervisor = origRuntimeSupervisor }()
	runtimeSupervisor = &runtimeSupervisorStub{
		stopErr: nil,
	}
	supervisor := runtimeSupervisor.(*runtimeSupervisorStub)

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if supervisor.stopCalls != 1 || supervisor.lastStopRunDir != runDir {
		t.Fatalf("runtime supervisor stop runDir = %q, want %q", supervisor.lastStopRunDir, runDir)
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
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

	_ = stubRuntimeSupervisor(t)

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	runState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if runState.GoalState != "open" || runState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want open/stopped", runState)
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

func TestDropTerminatesRuntimeHostBeforeRemovingRunDir(t *testing.T) {
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
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: drop-run\nmode: worker\nobjective: demo\ntarget:\n  files: [\"report.md\"]\nlocal_validation:\n  command: \"test -f base.txt\"\n"), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}

	origRuntimeSupervisor := runtimeSupervisor
	defer func() { runtimeSupervisor = origRuntimeSupervisor }()
	runtimeSupervisor = &runtimeSupervisorStub{
		stopErr: nil,
	}
	supervisor := runtimeSupervisor.(*runtimeSupervisorStub)

	if err := Drop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Drop: %v", err)
	}
	if supervisor.stopCalls != 1 || supervisor.lastStopRunDir != runDir {
		t.Fatalf("runtime supervisor stop runDir = %q, want %q", supervisor.lastStopRunDir, runDir)
	}
}

func TestRuntimeHostRenewsLeaseUntilContextStops(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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
		done <- runRuntimeHostLoop(ctx, repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	time.Sleep(120 * time.Millisecond)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("runRuntimeHostLoop: %v", err)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.RenewedAt == "" {
		t.Fatalf("runtime host renewed_at empty: %+v", lease)
	}
}

func TestRuntimeHostStopsWhenRunIdentityChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
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
		done <- runRuntimeHostLoop(ctx, repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	time.Sleep(60 * time.Millisecond)
	meta.RunID = newRunID()
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runRuntimeHostLoop: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("runtime host did not stop after run identity changed")
	}
}

func TestRuntimeHostFinalizesCompletedRunWhenMasterSessionIsGone(t *testing.T) {
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
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
		done <- runRuntimeHostLoop(ctx, repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runRuntimeHostLoop: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("runtime host did not stop after completed run lost its master session")
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
	if controlState.GoalState != "completed" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want completed/stopped", controlState)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "master"))
	if err != nil {
		t.Fatalf("LoadControlLease master: %v", err)
	}
	if lease.PID != 0 || lease.RunID != "" {
		t.Fatalf("master lease not expired: %+v", lease)
	}
	lease, err = LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
	if err != nil {
		t.Fatalf("LoadControlLease runtime-host: %v", err)
	}
	if lease.PID != 0 || lease.RunID != "" {
		t.Fatalf("runtime host not expired: %+v", lease)
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

func TestRuntimeHostFinalizesCompletedRunWhenMasterSessionStillExists(t *testing.T) {
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
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
    printf 'master\nsession-1\nsession-2\n'
    exit 0
    ;;
  list-panes)
    printf '%%0\tmaster\n%%1\tsession-1\n%%2\tsession-2\n'
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runRuntimeHostLoop(ctx, repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runRuntimeHostLoop: %v", err)
		}
	case <-time.After(300 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("runtime host did not stop after completed run with live master session")
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
	if controlState.GoalState != "completed" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want completed/stopped", controlState)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "kill-session -t "+goalx.TmuxSessionName(repo, runName)) {
		t.Fatalf("tmux log missing kill-session for completed run:\n%s", string(logData))
	}

	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	if _, ok := reg.ActiveRuns[runName]; ok {
		t.Fatalf("run %q still registered active after completion", runName)
	}
}

func TestRuntimeHostKeepsCompletedRunAliveWhenUnreadMasterInboxExists(t *testing.T) {
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
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

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.GoalState == "completed" {
		t.Fatalf("control state = %+v, want run kept open for unread master inbox", controlState)
	}
	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "handoffs", "master.json")); !os.IsNotExist(err) {
		t.Fatalf("runtime host relaunch should not create legacy handoff file, stat err = %v", err)
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

func TestRuntimeHostFinalizesCompletedRunWhenMasterWindowMissingButWorkerWindowsRemain(t *testing.T) {
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
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

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, 4242); err == nil || !errors.Is(err, errRuntimeHostCompleted) {
		t.Fatalf("runRuntimeHostTick err = %v, want %v", err, errRuntimeHostCompleted)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.GoalState != "completed" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want completed/stopped", controlState)
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

func TestRunRuntimeHostTickAlertsMissingSessionWindowOnceWithoutRespawn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	logPath := installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick first: %v", err)
	}
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick second: %v", err)
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

	auditData, err := os.ReadFile(filepath.Join(runDir, "runtime-host.log"))
	if err != nil {
		t.Fatalf("read runtime-host audit log: %v", err)
	}
	if strings.Count(string(auditData), "target_missing_alert target=session-1 cause=window_missing") != 1 {
		t.Fatalf("missing session alert should be recorded once:\n%s", string(auditData))
	}
}

func TestRunRuntimeHostTickDoesNotAlertMissingParkedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	logPath := installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "parked",
		Mode:  string(goalx.ModeWorker),
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 50*time.Millisecond, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if strings.Contains(string(inboxData), `"type":"target-missing"`) {
		t.Fatalf("parked session should not trigger missing alert:\n%s", string(inboxData))
	}

	auditData, err := os.ReadFile(filepath.Join(runDir, "runtime-host.log"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read runtime-host audit log: %v", err)
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

func TestProcessTargetAttentionAlertsMasterOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})
	if err := SaveActivitySnapshot(runDir, &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Attention: map[string]TargetAttentionFacts{
			"session-1": {
				Target:              "session-1",
				AttentionState:      TargetAttentionTransportBlocked,
				Unread:              1,
				CursorLag:           1,
				JournalStaleMinutes: 24,
				RuntimeState:        "progress",
			},
		},
	}); err != nil {
		t.Fatalf("SaveActivitySnapshot: %v", err)
	}

	presence := map[string]TargetPresenceFacts{
		"master":    {Target: "master", State: TargetPresencePresent},
		"session-1": {Target: "session-1", State: TargetPresencePresent},
	}
	if _, err := processTargetAttentionAlerts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine, presence); err != nil {
		t.Fatalf("processTargetAttentionAlerts first: %v", err)
	}
	if _, err := processTargetAttentionAlerts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine, presence); err != nil {
		t.Fatalf("processTargetAttentionAlerts second: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inboxData), `"type":"target-attention"`); got != 1 {
		t.Fatalf("master inbox target-attention count = %d, want 1\n%s", got, string(inboxData))
	}
	if !strings.Contains(string(inboxData), "session-1") || !strings.Contains(string(inboxData), "transport_blocked") {
		t.Fatalf("master inbox missing blocked target details:\n%s", string(inboxData))
	}

	recovery, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportRecovery: %v", err)
	}
	got := recovery.Targets["session-1"]
	if got.CurrentAttentionState != TargetAttentionTransportBlocked {
		t.Fatalf("attention state = %q, want %q", got.CurrentAttentionState, TargetAttentionTransportBlocked)
	}
	if got.CurrentAttentionLastAlertAt == "" {
		t.Fatalf("attention alert timestamp missing: %+v", got)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	found := false
	for _, item := range deliveries.Items {
		if strings.HasPrefix(item.DedupeKey, "master-attention:session-1:transport_blocked") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected immediate master-attention delivery, got %+v", deliveries.Items)
	}
}

func TestRunRuntimeHostTickAlertsBlockedSessionAttentionOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("❯\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 24},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	old := time.Now().UTC().Add(-24 * time.Minute)
	if err := os.Chtimes(JournalPath(runDir, "session-1"), old, old); err != nil {
		t.Fatalf("Chtimes session journal: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "session-wake:session-1", Status: "accepted", Target: goalx.TmuxSessionName(repo, cfg.Name) + ":session-1", AttemptedAt: time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339), AcceptedAt: time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339), TransportState: string(TUIStateQueued)},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick first: %v", err)
	}
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick second: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inboxData), `"type":"target-attention"`); got != 1 {
		t.Fatalf("master inbox target-attention count = %d, want 1\n%s", got, string(inboxData))
	}
	if !strings.Contains(string(inboxData), `"urgent":true`) {
		t.Fatalf("master inbox missing urgent target-attention fact:\n%s", string(inboxData))
	}
	if !strings.Contains(string(inboxData), "state=transport_blocked") {
		t.Fatalf("master inbox missing blocked attention state:\n%s", string(inboxData))
	}

	recovery, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportRecovery: %v", err)
	}
	target := recovery.Targets["session-1"]
	if target.CurrentAttentionState != TargetAttentionTransportBlocked {
		t.Fatalf("current attention state = %q, want %q", target.CurrentAttentionState, TargetAttentionTransportBlocked)
	}
	if target.CurrentAttentionFirstSeenAt == "" || target.CurrentAttentionLastAlertAt == "" {
		t.Fatalf("attention timestamps missing: %+v", target)
	}

	auditData, err := os.ReadFile(filepath.Join(runDir, "runtime-host.log"))
	if err != nil {
		t.Fatalf("read runtime-host audit log: %v", err)
	}
	if got := strings.Count(string(auditData), "target_attention_alert target=session-1 state=transport_blocked"); got != 1 {
		t.Fatalf("target attention alert should be recorded once:\n%s", string(auditData))
	}
}

func TestRunRuntimeHostTickAlertsRequiredFrontierGapsOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "finish integration", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "verify remote system", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-3", Text: "ship remaining slice", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "master",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
			"req-2": {
				Owner:          "master",
				ExecutionState: coordinationRequiredExecutionStateBlocked,
				BlockedBy:      "claimed blocker before runtime exhausted",
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceExhausted,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfaceExhausted,
					WebResearch:    coordinationRequiredSurfaceExhausted,
					ExternalSystem: coordinationRequiredSurfaceUnreachable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-4",
		State: "idle",
		Mode:  string(goalx.ModeWorker),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}

	orig := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudgeDetailedInRunFunc = orig }()
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick first: %v", err)
	}
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick second: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inboxData), `"type":"master-alert"`); got != 3 {
		t.Fatalf("master inbox master-alert count = %d, want 3\n%s", got, string(inboxData))
	}
	for _, want := range []string{
		"required=req-1 fact=master_orphaned",
		"required=req-2 fact=premature_blocked",
		"required=req-3 fact=unmapped_required",
	} {
		if !strings.Contains(string(inboxData), want) {
			t.Fatalf("master inbox missing %q:\n%s", want, string(inboxData))
		}
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if len(controlState.MasterAlerts) != 3 {
		t.Fatalf("master alerts = %+v, want 3 entries", controlState.MasterAlerts)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	count := 0
	for _, item := range deliveries.Items {
		if strings.HasPrefix(item.DedupeKey, "master-alert:") {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("required frontier deliveries = %d, want 3: %+v", count, deliveries.Items)
	}
}

func TestRunRuntimeHostTickRealertsRequiredFrontierWhenStateChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "finish integration", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	writeCoordination := func(state string) {
		t.Helper()
		if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
			Version: 1,
			Required: map[string]CoordinationRequiredItem{
				"req-1": {
					Owner:          "master",
					ExecutionState: state,
					Surfaces: CoordinationRequiredSurfaces{
						Repo:           coordinationRequiredSurfaceActive,
						Runtime:        coordinationRequiredSurfacePending,
						RunArtifacts:   coordinationRequiredSurfacePending,
						WebResearch:    coordinationRequiredSurfacePending,
						ExternalSystem: coordinationRequiredSurfaceNotApplicable,
					},
				},
			},
		}); err != nil {
			t.Fatalf("SaveCoordinationState: %v", err)
		}
	}
	writeCoordination(coordinationRequiredExecutionStateProbing)
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-4",
		State: "idle",
		Mode:  string(goalx.ModeWorker),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}

	orig := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudgeDetailedInRunFunc = orig }()
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick probing: %v", err)
	}
	writeCoordination(coordinationRequiredExecutionStateWaiting)
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick waiting: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inboxData), `"type":"master-alert"`); got != 2 {
		t.Fatalf("master inbox master-alert count = %d, want 2\n%s", got, string(inboxData))
	}
	if !strings.Contains(string(inboxData), "execution_state=probing") || !strings.Contains(string(inboxData), "execution_state=waiting") {
		t.Fatalf("master inbox missing frontier state change:\n%s", string(inboxData))
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if len(controlState.MasterAlerts) != 1 {
		t.Fatalf("master alerts = %+v, want 1 entry", controlState.MasterAlerts)
	}
	for _, fingerprint := range controlState.MasterAlerts {
		if !strings.Contains(fingerprint, "execution_state=waiting") {
			t.Fatalf("required frontier fingerprint = %q, want waiting state", fingerprint)
		}
	}
}

func TestRunRuntimeHostTickAlertsControlGapFacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship cockpit", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "ship research spine", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"working","required_remaining":2,"open_required_ids":["req-1"],"active_sessions":["session-9"],"updated_at":"2026-03-30T19:12:54Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version:   1,
		UpdatedAt: "2026-03-30T19:12:54Z",
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-5",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfaceActive,
					RunArtifacts:   coordinationRequiredSurfaceActive,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfacePending,
				},
			},
			"req-2": {
				Owner:          "session-5",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfaceActive,
					RunArtifacts:   coordinationRequiredSurfaceActive,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfacePending,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:             1,
		CurrentExperimentID: "exp-2",
		CurrentBranch:       "goalx/guidance-run/root",
		CurrentCommit:       "abc123",
		UpdatedAt:           "2026-03-31T01:05:35Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}
	for _, session := range []SessionRuntimeState{
		{Name: "session-5", State: "idle", Mode: string(goalx.ModeWorker)},
		{Name: "session-4", State: "parked", Mode: string(goalx.ModeWorker)},
	} {
		if err := UpsertSessionRuntimeState(runDir, session); err != nil {
			t.Fatalf("UpsertSessionRuntimeState %s: %v", session.Name, err)
		}
	}

	orig := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudgeDetailedInRunFunc = orig }()
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	for _, want := range []string{
		"fact=status_drift",
		"fact=coordination_stale",
		"fact=serialized_required_frontier",
	} {
		if !strings.Contains(string(inboxData), want) {
			t.Fatalf("master inbox missing %q:\n%s", want, string(inboxData))
		}
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if len(controlState.MasterAlerts) != 3 {
		t.Fatalf("master alerts = %+v, want 3 entries", controlState.MasterAlerts)
	}
}

func TestRunRuntimeHostTickRealertsControlGapWhenFingerprintChanges(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship cockpit", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "ship research spine", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseWorking,
		RequiredRemaining: intPtr(2),
		OpenRequiredIDs:   []string{"req-1", "req-2"},
		ActiveSessions:    []string{"session-5"},
		UpdatedAt:         "2026-03-30T19:12:54Z",
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-5",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfaceActive,
					RunArtifacts:   coordinationRequiredSurfaceActive,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfacePending,
				},
			},
			"req-2": {
				Owner:          "session-5",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfaceActive,
					RunArtifacts:   coordinationRequiredSurfaceActive,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfacePending,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-5", State: "idle", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-5: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-4", State: "parked", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}

	orig := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudgeDetailedInRunFunc = orig }()
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick first: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-3", State: "idle", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-3: %v", err)
	}
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick second: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	if got := strings.Count(string(inboxData), "fact=serialized_required_frontier"); got != 2 {
		t.Fatalf("serialized control-gap alert count = %d, want 2\n%s", got, string(inboxData))
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	found := false
	for key, fingerprint := range controlState.MasterAlerts {
		if key != "control_gap:serialized_required_frontier" {
			continue
		}
		found = true
		if !strings.Contains(fingerprint, "reusable_sessions=session-3,session-4") {
			t.Fatalf("serialized control-gap fingerprint = %q, want updated reusable sessions", fingerprint)
		}
	}
	if !found {
		t.Fatalf("serialized control-gap fingerprint missing: %+v", controlState.MasterAlerts)
	}
}

func TestRunRuntimeHostTickRefreshesDomainPackFromPromotedSuccessPrior(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)

	installGuidanceFakeTmux(t, nil)

	now := time.Date(2026, time.March, 31, 13, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:        "prop_success_prior_guidance",
			State:     "proposed",
			Kind:      MemoryKindSuccessPrior,
			Statement: "operator-console runs require critique and finisher passes before closeout",
			Selectors: map[string]string{"project_id": goalx.ProjectID(repo)},
			Evidence: []MemoryEvidence{
				{Kind: "intervention_log", Path: "/tmp/intervention-log.jsonl"},
				{Kind: "summary", Path: "/tmp/summary.md"},
			},
			SourceRuns: []string{"run-1", "run-2"},
			CreatedAt:  "2026-03-31T13:00:00Z",
			UpdatedAt:  "2026-03-31T13:00:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindSuccessPrior)
	if len(entries) != 1 {
		t.Fatalf("success prior entries = %+v, want one promoted entry", entries)
	}

	pack, err := LoadDomainPack(DomainPackPath(runDir))
	if err != nil {
		t.Fatalf("LoadDomainPack: %v", err)
	}
	if pack == nil || len(pack.PriorEntryIDs) != 1 {
		t.Fatalf("domain pack = %+v, want one prior entry id", pack)
	}
	if len(pack.Signals) == 0 || !slices.Contains(pack.Signals, "success_prior_present") {
		t.Fatalf("domain pack signals = %v, want success_prior_present", pack.Signals)
	}
	composition, err := buildProtocolComposition(runDir, ProtocolComposition{})
	if err != nil {
		t.Fatalf("buildProtocolComposition: %v", err)
	}
	if len(composition.SelectedPriorRefs) != 1 {
		t.Fatalf("protocol composition selected prior refs = %v, want one selected prior", composition.SelectedPriorRefs)
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
		t.Fatalf("expected refresh-context reminder after success-context change: %+v", reminders.Items)
	}
}

func TestRunRuntimeHostTickDoesNotRefreshCompilerContextOnIrrelevantMemoryChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("repo policy"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
	if err := EnsureSuccessCompilation(repo, runDir, cfg, meta); err != nil {
		t.Fatalf("EnsureSuccessCompilation: %v", err)
	}

	installGuidanceFakeTmux(t, nil)

	now := time.Date(2026, time.March, 31, 13, 30, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:         "prop_fact_irrelevant",
			State:      "proposed",
			Kind:       MemoryKindFact,
			Statement:  "deploy host is ops-7",
			Selectors:  map[string]string{"project_id": "other-project"},
			Evidence:   []MemoryEvidence{{Kind: "summary", Path: "/tmp/summary.md"}},
			SourceRuns: []string{"run-1"},
			CreatedAt:  "2026-03-31T13:30:00Z",
			UpdatedAt:  "2026-03-31T13:30:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	for _, item := range reminders.Items {
		if item.DedupeKey == "refresh-context" {
			t.Fatalf("unexpected refresh-context reminder after irrelevant memory change: %+v", reminders.Items)
		}
	}
}

func TestRunRuntimeHostTickAlertsQualityDebtAndEvolveManagementGap(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship cockpit", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "ship polish", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version: 1,
		LastResult: AcceptanceResult{
			CheckedAt: "2026-03-31T10:05:00Z",
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	if err := SaveSuccessModel(SuccessModelPath(runDir), &SuccessModel{
		Version:               1,
		ObjectiveContractHash: "sha256:objective",
		GoalHash:              "sha256:goal",
		Dimensions: []SuccessDimension{
			{ID: "req-1", Kind: "goal_item", Text: "ship cockpit", Required: true},
			{ID: "req-2", Kind: "goal_item", Text: "ship polish", Required: true},
		},
		CloseoutRequirements: []string{"quality_debt_zero"},
	}); err != nil {
		t.Fatalf("SaveSuccessModel: %v", err)
	}
	if err := SaveProofPlan(ProofPlanPath(runDir), &ProofPlan{
		Version: 1,
		Items: []ProofPlanItem{
			{ID: "proof-summary", CoversDimensions: []string{"req-1", "req-2"}, Kind: "run_artifact", Required: true, SourceSurface: "summary"},
		},
	}); err != nil {
		t.Fatalf("SaveProofPlan: %v", err)
	}
	if err := SaveWorkflowPlan(WorkflowPlanPath(runDir), &WorkflowPlan{
		Version: 1,
		RequiredRoles: []WorkflowRoleRequirement{
			{ID: "builder", Required: true},
			{ID: "critic", Required: true},
			{ID: "finisher", Required: true},
		},
		Gates: []string{"builder_result_present", "critic_review_present", "finisher_pass_present"},
	}); err != nil {
		t.Fatalf("SaveWorkflowPlan: %v", err)
	}
	if err := SaveDomainPack(DomainPackPath(runDir), &DomainPack{Version: 1, Domain: "evolve"}); err != nil {
		t.Fatalf("SaveDomainPack: %v", err)
	}
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseWorking,
		RequiredRemaining: intPtr(2),
		OpenRequiredIDs:   []string{"req-1", "req-2"},
		UpdatedAt:         "2026-03-31T10:05:00Z",
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version:   1,
		UpdatedAt: "2026-03-31T10:05:00Z",
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-5",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfaceActive,
					RunArtifacts:   coordinationRequiredSurfaceActive,
					WebResearch:    coordinationRequiredSurfaceNotApplicable,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-31T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-31T10:00:00Z"}}`)

	orig := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudgeDetailedInRunFunc = orig }()
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	inboxData, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	inbox := string(inboxData)
	for _, want := range []string{
		"fact=quality_debt",
		"fact=evolve_management_gap",
		"gap=missing_stop_or_dispatch",
	} {
		if !strings.Contains(inbox, want) {
			t.Fatalf("master inbox missing %q:\n%s", want, inbox)
		}
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if len(controlState.MasterAlerts) == 0 {
		t.Fatalf("master alerts unexpectedly empty: %+v", controlState)
	}
}

func TestRunRuntimeHostTickQueuesSessionWakeForUnreadSessionInbox(t *testing.T) {
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
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickDoesNotQueueSessionWakeWhenCursorCaughtUp(t *testing.T) {
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
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickSkipsSessionWakeWhenWindowMissing(t *testing.T) {
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
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickSkipsSessionWakeDuringRecentSuccessfulTellGrace(t *testing.T) {
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
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	var nudges []string
	sendAgentNudge = func(target, engine string) error {
		nudges = append(nudges, target+"|"+engine)
		return nil
	}
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickQueuesSessionRepairWhenTransportStillBuffered(t *testing.T) {
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

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickSkipsSessionWakeWhenTransportAlreadyAccepted(t *testing.T) {
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

	if err := runRuntimeHostTick(repo, runName, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickElevatesProviderDialogToUrgentMasterFact(t *testing.T) {
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickElevatesCapacityPickerDialogToUrgentMasterFact(t *testing.T) {
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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

func TestRunRuntimeHostTickImmediatelyNudgesMasterOnceForProviderDialog(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Needs your permission\nYes, don't ask again\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	orig := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudgeDetailedInRunFunc = orig }()
	var nudges []string
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		nudges = append(nudges, target+"|"+engine)
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "sent"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick first: %v", err)
	}
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick second: %v", err)
	}

	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	count := 0
	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
	for _, item := range deliveries.Items {
		if strings.HasPrefix(item.DedupeKey, "provider-dialog:session-1:") {
			count++
			if item.Target != wantTarget {
				t.Fatalf("provider dialog nudge target = %q, want %q", item.Target, wantTarget)
			}
		}
	}
	if count != 1 {
		t.Fatalf("provider dialog immediate nudge count = %d, want 1; deliveries=%+v", count, deliveries.Items)
	}
	found := false
	for _, nudge := range nudges {
		if nudge == wantTarget+"|codex" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected immediate master nudge to %q, got %v", wantTarget, nudges)
	}
}

func bootstrapRuntimeHostIdentityFixture(t *testing.T, runDir, repo string, cfg *goalx.Config, meta *RunMetadata) {
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
