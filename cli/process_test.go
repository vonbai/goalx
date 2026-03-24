package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestKillProcessTree(t *testing.T) {
	shellPID, childPID, waitDone := startSleepProcessTree(t)
	KillProcessTree(shellPID)

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("process tree did not exit")
	}

	if processAlive(shellPID) {
		t.Fatalf("shell pid %d still alive", shellPID)
	}
	waitForProcessExit(t, childPID)
}

func TestKillProcessTreeHandlesDeadProcess(t *testing.T) {
	cmd := exec.Command("sleep", "0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait sleep: %v", err)
	}

	KillProcessTree(pid)
}

func startSleepProcessTree(t *testing.T) (int, int, <-chan error) {
	t.Helper()

	pidPath := filepath.Join(t.TempDir(), "child.pid")
	cmd := exec.Command("bash", "-c", "sleep 30 & echo $! >\"$1\"; wait", "bash", pidPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatalf("start shell: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- cmd.Wait()
	}()

	return cmd.Process.Pid, waitForPIDFile(t, pidPath), waitDone
}

func exitedProcessPID(t *testing.T) int {
	t.Helper()

	cmd := exec.Command("sleep", "0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait sleep: %v", err)
	}
	return pid
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if convErr != nil {
				t.Fatalf("parse pid %q: %v", strings.TrimSpace(string(data)), convErr)
			}
			if pid <= 0 {
				t.Fatalf("pid = %d, want > 0", pid)
			}
			return pid
		}
		if !os.IsNotExist(err) {
			t.Fatalf("read pid file: %v", err)
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("pid file %s not created", path)
	return 0
}

func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}

	t.Fatalf("pid %d still alive", pid)
}
