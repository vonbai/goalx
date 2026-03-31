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
		Version:         1,
		GoalState:       "open",
		ContinuityState: "running",
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

	origRuntimeSupervisor := runtimeSupervisor
	defer func() { runtimeSupervisor = origRuntimeSupervisor }()
	supervisor := &runtimeSupervisorStub{}
	runtimeSupervisor = supervisor

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

	origRuntimeSupervisor := runtimeSupervisor
	defer func() { runtimeSupervisor = origRuntimeSupervisor }()
	supervisor := &runtimeSupervisorStub{}
	runtimeSupervisor = supervisor

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
	if runState.GoalState != "open" || runState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want open/stopped", runState)
	}
	if supervisor.stopCalls != 1 || supervisor.lastStopRunDir != runDir {
		t.Fatalf("runtime supervisor stop = calls:%d runDir:%q, want calls=1 runDir=%q", supervisor.stopCalls, supervisor.lastStopRunDir, runDir)
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

	_ = stubRuntimeSupervisor(t)

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
	if controlState.GoalState != "completed" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want completed/stopped", controlState)
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "stopped"}); err != nil {
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
	if controlState.GoalState != "completed" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want completed/stopped", controlState)
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

func TestRefreshDisplayFactsDoesNotRepairCompletedEvolveRunWithoutManagedStop(t *testing.T) {
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
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
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
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "stopped"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
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

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.GoalState != "open" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want open/stopped when evolve frontier is still open", controlState)
	}
}

func TestRefreshDisplayFactsDoesNotRepairCompletedRunWhenQualityDebtRemains(t *testing.T) {
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
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship cockpit", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:     2,
		GoalVersion: 1,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Label: "acceptance", Command: "printf ok", State: acceptanceCheckStateActive},
		},
		LastResult: AcceptanceResult{CheckedAt: "2026-03-31T02:00:00Z"},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}
	if err := SaveSuccessModel(SuccessModelPath(runDir), &SuccessModel{
		Version:               1,
		ObjectiveContractHash: "sha256:objective",
		GoalHash:              "sha256:goal",
		Dimensions: []SuccessDimension{
			{ID: "req-1", Kind: "outcome", Text: "ship cockpit", Required: true},
		},
	}); err != nil {
		t.Fatalf("SaveSuccessModel: %v", err)
	}
	if err := SaveProofPlan(ProofPlanPath(runDir), &ProofPlan{
		Version: 1,
		Items: []ProofPlanItem{
			{ID: "proof-acceptance", CoversDimensions: []string{"req-1"}, Kind: "acceptance_check", Required: true, SourceSurface: "acceptance"},
		},
	}); err != nil {
		t.Fatalf("SaveProofPlan: %v", err)
	}
	if err := SaveWorkflowPlan(WorkflowPlanPath(runDir), &WorkflowPlan{
		Version: 1,
		RequiredRoles: []WorkflowRoleRequirement{
			{ID: "critic", Required: true},
			{ID: "finisher", Required: true},
		},
		Gates: []string{"critic_review_present", "finisher_pass_present"},
	}); err != nil {
		t.Fatalf("SaveWorkflowPlan: %v", err)
	}
	if err := SaveDomainPack(DomainPackPath(runDir), &DomainPack{Version: 1, Domain: "generic"}); err != nil {
		t.Fatalf("SaveDomainPack: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"complete","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-31T02:00:00Z"}`), 0o644); err != nil {
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "stopped"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
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

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.GoalState != "open" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want open/stopped when quality debt remains", controlState)
	}
}

func TestRefreshDisplayFactsStrandsOpenRunWhenRuntimeHostStopped(t *testing.T) {
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
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:         1,
		GoalState:       "open",
		ContinuityState: "running",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(cfg.Mode),
		Active:    true,
		Phase:     "working",
		StartedAt: "2026-03-31T00:00:00Z",
		UpdatedAt: "2026-03-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}
	if err := SaveRunHostState(RunHostStatePath(runDir), &RunHostState{
		Version:   1,
		Kind:      "runtime_host",
		Launcher:  "process",
		RunDir:    runDir,
		RunName:   runName,
		Running:   true,
		PID:       4242,
		UpdatedAt: "2026-03-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunHostState: %v", err)
	}

	origRuntimeSupervisor := runtimeSupervisor
	defer func() { runtimeSupervisor = origRuntimeSupervisor }()
	runtimeSupervisor = &runtimeSupervisorStub{
		inspectState: &RunHostState{
			Version:   1,
			Kind:      "runtime_host",
			Launcher:  "process",
			RunDir:    runDir,
			RunName:   runName,
			Running:   false,
			PID:       0,
			UpdatedAt: "2026-03-31T00:05:00Z",
		},
	}

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
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

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.GoalState != "open" || controlState.ContinuityState != "stranded" {
		t.Fatalf("control state = %+v, want open/stranded", controlState)
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	if runtimeState.Active {
		t.Fatalf("runtime state should be inactive after stranding: %+v", runtimeState)
	}
	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	if _, ok := reg.ActiveRuns[runName]; ok {
		t.Fatalf("run %q still registered active after stranding", runName)
	}
}
