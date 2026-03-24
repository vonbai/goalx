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
