package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	oomScoreAdjRuntimeHost = -900
	oomScoreAdjMaster      = -700
	oomScoreAdjWorker      = 300
)

var writeOOMScoreAdj = func(pid int, value int) error {
	path := filepath.Join("/proc", fmt.Sprintf("%d", pid), "oom_score_adj")
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", value)), 0o644)
}

func oomScoreAdjForHolder(holder string) (int, bool) {
	switch trimmed := strings.TrimSpace(holder); {
	case trimmed == "runtime-host":
		return oomScoreAdjRuntimeHost, true
	case trimmed == "master":
		return oomScoreAdjMaster, true
	case strings.HasPrefix(trimmed, "session-"):
		return oomScoreAdjWorker, true
	default:
		return 0, false
	}
}

func applyOOMScoreAdj(pid int, value int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid oom target pid %d", pid)
	}
	return writeOOMScoreAdj(pid, value)
}

func applyOOMPriorityForHolder(holder string, pid int) error {
	value, ok := oomScoreAdjForHolder(holder)
	if !ok {
		return nil
	}
	return applyOOMScoreAdj(pid, value)
}

func applyOOMPriorityBestEffort(runDir, holder string, pid int) {
	if err := applyOOMPriorityForHolder(holder, pid); err != nil {
		if errorsIsPermissionDenied(err) {
			appendAuditLog(runDir, "oom priority skipped holder=%s pid=%d reason=permission_denied", holder, pid)
			return
		}
		appendAuditLog(runDir, "oom priority skipped holder=%s pid=%d reason=%v", holder, pid, err)
		return
	}
	appendAuditLog(runDir, "oom priority applied holder=%s pid=%d", holder, pid)
}

func errorsIsPermissionDenied(err error) bool {
	return os.IsPermission(err) || err == syscall.EPERM || err == syscall.EACCES
}
