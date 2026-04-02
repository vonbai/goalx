package cli

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestBuildEngineExecCommandUsesExec(t *testing.T) {
	got := buildEngineExecCommand("codex -m gpt-5.4", "/tmp/master.md")
	if !strings.HasPrefix(got, "exec codex -m gpt-5.4 ") {
		t.Fatalf("buildEngineExecCommand = %q, want exec-prefixed command", got)
	}
}

func TestOOMScoreAdjForHolder(t *testing.T) {
	cases := []struct {
		holder string
		want   int
		ok     bool
	}{
		{holder: "runtime-host", want: oomScoreAdjRuntimeHost, ok: true},
		{holder: "master", want: oomScoreAdjMaster, ok: true},
		{holder: "session-1", want: oomScoreAdjWorker, ok: true},
		{holder: "mystery", want: 0, ok: false},
	}
	for _, tc := range cases {
		got, ok := oomScoreAdjForHolder(tc.holder)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("oomScoreAdjForHolder(%q) = (%d,%t), want (%d,%t)", tc.holder, got, ok, tc.want, tc.ok)
		}
	}
}

func TestApplyOOMPriorityBestEffortWritesExpectedValue(t *testing.T) {
	runDir := t.TempDir()
	prev := writeOOMScoreAdj
	t.Cleanup(func() { writeOOMScoreAdj = prev })
	var gotPID, gotValue int
	writeOOMScoreAdj = func(pid int, value int) error {
		gotPID = pid
		gotValue = value
		return nil
	}

	applyOOMPriorityBestEffort(runDir, "session-1", 4242)
	if gotPID != 4242 || gotValue != oomScoreAdjWorker {
		t.Fatalf("oom priority write = pid:%d value:%d, want pid:%d value:%d", gotPID, gotValue, 4242, oomScoreAdjWorker)
	}
}

func TestApplyOOMPriorityBestEffortIgnoresPermissionDenied(t *testing.T) {
	runDir := t.TempDir()
	prev := writeOOMScoreAdj
	t.Cleanup(func() { writeOOMScoreAdj = prev })
	writeOOMScoreAdj = func(pid int, value int) error {
		return &os.PathError{Op: "write", Path: "/proc/123/oom_score_adj", Err: syscall.EPERM}
	}

	applyOOMPriorityBestEffort(runDir, "master", 123)
	logData, err := os.ReadFile(auditLogPath(runDir))
	if err != nil {
		t.Fatalf("ReadFile audit log: %v", err)
	}
	if !strings.Contains(string(logData), "oom priority skipped holder=master pid=123 reason=permission_denied") {
		t.Fatalf("audit log missing permission-denied message:\n%s", string(logData))
	}
}

func TestErrorsIsPermissionDenied(t *testing.T) {
	if !errorsIsPermissionDenied(syscall.EPERM) {
		t.Fatal("EPERM should be treated as permission denied")
	}
	if !errorsIsPermissionDenied(&os.PathError{Op: "write", Path: "/proc/1/oom_score_adj", Err: syscall.EACCES}) {
		t.Fatal("wrapped EACCES should be treated as permission denied")
	}
	if errorsIsPermissionDenied(errors.New("boom")) {
		t.Fatal("generic error should not be treated as permission denied")
	}
}
