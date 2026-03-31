package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestRefreshDisplayFactsRepairsCloseoutReadyRunWhenWorkerWindowKeepsTmuxSessionAlive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := prepareCloseoutReadyActiveRun(t, repo)

	shellPID, childPID, waitDone := startSleepProcessTree(t)
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", shellPID); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}

	logPath := installLingeringWorkerTmux(t)

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
		t.Fatal("leased process tree did not exit after completed-run repair")
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

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "kill-session -t "+goalx.TmuxSessionName(repo, runName)) {
		t.Fatalf("tmux log missing kill-session for repaired completed run:\n%s", string(logData))
	}

	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	if _, ok := reg.ActiveRuns[runName]; ok {
		t.Fatalf("run %q still registered active after repair", runName)
	}
}

func TestTellRejectsCloseoutReadyRunBeforeExplicitFinalization(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := prepareCloseoutReadyActiveRun(t, repo)
	installFakePresenceTmux(t, false, "", "")

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

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState.GoalState != "completed" || controlState.ContinuityState != "stopped" {
		t.Fatalf("control state = %+v, want completed/stopped", controlState)
	}
}

func prepareCloseoutReadyActiveRun(t *testing.T, repo string) (string, string) {
	t.Helper()

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
	return runName, runDir
}

func installLingeringWorkerTmux(t *testing.T) string {
	t.Helper()

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	statePath := filepath.Join(fakeBin, "tmux-session")
	if err := os.WriteFile(statePath, []byte("live\n"), 0o644); err != nil {
		t.Fatalf("write tmux state: %v", err)
	}
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    if [ -f "$TMUX_STATE_PATH" ]; then
      exit 0
    fi
    exit 1
    ;;
  list-windows)
    if [ -f "$TMUX_STATE_PATH" ]; then
      printf 'session-1\n'
    fi
    exit 0
    ;;
  list-panes)
    if [ -f "$TMUX_STATE_PATH" ]; then
      printf '%%1\tsession-1\n'
    fi
    exit 0
    ;;
  kill-session)
    rm -f "$TMUX_STATE_PATH"
    exit 0
    ;;
  capture-pane|new-window|kill-window|send-keys)
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("TMUX_STATE_PATH", statePath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}
