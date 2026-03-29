package cli

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestFinalizeControlRunKillsLeasedProcesses(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	shellPID, childPID, waitDone := startSleepProcessTree(t)
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", shellPID); err != nil {
		t.Fatalf("RenewControlLease: %v", err)
	}

	if err := FinalizeControlRun(runDir, "stopped"); err != nil {
		t.Fatalf("FinalizeControlRun: %v", err)
	}

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("leased process tree did not exit")
	}

	waitForProcessExit(t, childPID)

	lease, err := LoadControlLease(ControlLeasePath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.PID != 0 || lease.RunID != "" {
		t.Fatalf("lease not expired after finalize: %+v", lease)
	}
}

func TestFinalizeControlRunRecordsControlSurfaceFinalizeOp(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:        1,
		LifecycleState: "active",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
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
			{DeliveryID: "del-2", DedupeKey: "tell:1", Status: "sent", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	if err := FinalizeControlRun(runDir, "stopped"); err != nil {
		t.Fatalf("FinalizeControlRun: %v", err)
	}

	ops, err := loadControlOps(ControlOpsPath(runDir))
	if err != nil {
		t.Fatalf("loadControlOps: %v", err)
	}
	if len(ops) == 0 || ops[len(ops)-1].Kind != "control.finalize_surfaces" {
		t.Fatalf("finalize should record control-surface finalize op, ops=%+v", ops)
	}
	cursor, err := LoadControlOpsCursor(ControlOpsCursorPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlOpsCursor: %v", err)
	}
	if cursor.LastAppliedID != ops[len(ops)-1].ID {
		t.Fatalf("control ops cursor = %d, want %d", cursor.LastAppliedID, ops[len(ops)-1].ID)
	}
}

func TestStopKillsLeasedProcessesBeforeTmuxSession(t *testing.T) {
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

	shellPID, childPID, waitDone := startSleepProcessTree(t)
	if err := RenewControlLease(runDir, "master", "run_demo", 1, time.Minute, "tmux", shellPID); err != nil {
		t.Fatalf("RenewControlLease: %v", err)
	}

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 0
    ;;
  kill-session)
    if kill -0 "$LEASE_PID" 2>/dev/null; then
      exit 23
    fi
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
	t.Setenv("LEASE_PID", strconv.Itoa(shellPID))
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("leased process tree did not exit")
	}

	waitForProcessExit(t, childPID)
}

func TestStopSucceedsWhenTmuxSessionAlreadyExited(t *testing.T) {
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

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	fakeBin := t.TempDir()
	statePath := filepath.Join(fakeBin, "tmux-session")
	if err := os.WriteFile(statePath, []byte("live\n"), 0o644); err != nil {
		t.Fatalf("write tmux state: %v", err)
	}
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    if [ -f "$TMUX_STATE_PATH" ]; then
      exit 0
    fi
    exit 1
    ;;
  kill-session)
    rm -f "$TMUX_STATE_PATH"
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
	t.Setenv("TMUX_STATE_PATH", statePath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
}

func TestStopPreservesCompletedLifecycleWhenCloseoutExists(t *testing.T) {
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
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(CompletionStatePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir proof dir: %v", err)
	}
	if err := os.WriteFile(CompletionStatePath(runDir), []byte(`{"verdict":"complete"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.LifecycleState != "completed" {
		t.Fatalf("lifecycle_state = %q, want completed", controlState.LifecycleState)
	}

	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	if runtimeState.Phase != "complete" {
		t.Fatalf("run phase = %q, want complete", runtimeState.Phase)
	}
	if runtimeState.Active {
		t.Fatalf("run should be inactive after stop: %+v", runtimeState)
	}
}

func TestRefreshDisplayFactsRepairsCompletedRunAndCleansLeases(t *testing.T) {
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
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(CompletionStatePath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir proof dir: %v", err)
	}
	if err := os.WriteFile(CompletionStatePath(runDir), []byte(`{"verdict":"complete"}`), 0o644); err != nil {
		t.Fatalf("write completion proof: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "stopped"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	shellPID, childPID, waitDone := startSleepProcessTree(t)
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", shellPID); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 1
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	rc, err := buildRunContext(repo, runDir, runName)
	if err != nil {
		t.Fatalf("buildRunContext: %v", err)
	}
	if err := refreshDisplayFacts(rc); err != nil {
		t.Fatalf("refreshDisplayFacts: %v", err)
	}

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("leased process tree did not exit after closeout repair")
	}
	waitForProcessExit(t, childPID)

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.LifecycleState != "completed" {
		t.Fatalf("lifecycle_state = %q, want completed", controlState.LifecycleState)
	}

	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	if runtimeState.Phase != "complete" {
		t.Fatalf("run phase = %q, want complete", runtimeState.Phase)
	}
	if runtimeState.Active {
		t.Fatalf("run should be inactive after repair: %+v", runtimeState)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.PID != 0 || lease.RunID != "" {
		t.Fatalf("session lease not expired after repair: %+v", lease)
	}
}
