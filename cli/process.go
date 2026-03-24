package cli

import (
	"syscall"
	"time"
)

// KillProcessTree sends SIGTERM to the process group rooted at pid, waits up to
// 2 seconds, then SIGKILL if the process is still alive. It is best-effort and
// silently skips dead processes.
func KillProcessTree(pid int) {
	if pid <= 0 || !processAlive(pid) {
		return
	}

	_ = syscall.Kill(-pid, syscall.SIGTERM)
	_ = syscall.Kill(pid, syscall.SIGTERM)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processAlive(pid) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}

	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
