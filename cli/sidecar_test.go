package cli

import (
	"context"
	"os"
	"path/filepath"
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

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()
	var gotTarget, gotEngine string
	sendAgentNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
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
	if err := RenewControlLease(runDir, "master", "run_demo", 1, time.Minute, "tmux", 123); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", 456); err != nil {
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
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: drop-run\nmode: research\nobjective: demo\ntarget:\n  files: [\"report.md\"]\nharness:\n  command: \"test -f base.txt\"\n"), 0o644); err != nil {
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
