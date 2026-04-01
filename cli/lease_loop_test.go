package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

type targetRunnerStub struct {
	calls    int
	lastSpec TargetRunnerLaunchSpec
	command  string
	err      error
}

func (s *targetRunnerStub) BuildCommand(spec TargetRunnerLaunchSpec) (string, error) {
	s.calls++
	s.lastSpec = spec
	if s.err != nil {
		return "", s.err
	}
	if s.command != "" {
		return s.command, nil
	}
	return "stub-launch", nil
}

func TestBuildMasterLaunchCommandUsesTargetRunnerProcess(t *testing.T) {
	t.Setenv("HOME", "/tmp/goalx-home")
	t.Setenv("PATH", "/tmp/goalx-bin:/usr/bin")

	cmd := buildMasterLaunchCommand(
		"/root/go/bin/goalx",
		"demo-run",
		"/tmp/run-dir",
		"run_demo",
		7,
		4*time.Minute,
		"codex -m gpt-5.4 -a never -s danger-full-access",
		"/tmp/run/master.md",
	)

	for _, want := range []string{
		"env ",
		"target-runner --run",
		"--run-dir",
		"/tmp/run-dir",
		"--holder",
		"master",
		"--run-id",
		"--epoch 7",
		"--ttl-seconds 240",
		"--transport tmux",
		"--engine-command",
		"codex -m gpt-5.4 -a never -s danger-full-access",
		"--prompt",
		"/tmp/run/master.md",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("master launch command missing %q:\n%s", want, cmd)
		}
	}
}

func TestBuildMasterLaunchCommandUsesTargetRunner(t *testing.T) {
	origTargetRunner := targetRunner
	defer func() { targetRunner = origTargetRunner }()
	stub := &targetRunnerStub{command: "runner-command"}
	targetRunner = stub

	cmd := buildMasterLaunchCommand(
		"/root/go/bin/goalx",
		"demo-run",
		"/tmp/run-dir",
		"run_demo",
		7,
		4*time.Minute,
		"codex -m gpt-5.4 -a never -s danger-full-access",
		"/tmp/run/master.md",
	)

	if cmd != "runner-command" {
		t.Fatalf("command = %q, want runner-command", cmd)
	}
	if stub.calls != 1 {
		t.Fatalf("target runner calls = %d, want 1", stub.calls)
	}
	if stub.lastSpec.Holder != "master" || stub.lastSpec.RunName != "demo-run" || stub.lastSpec.RunDir != "/tmp/run-dir" {
		t.Fatalf("target runner spec = %+v", stub.lastSpec)
	}
	if stub.lastSpec.TTL != 4*time.Minute {
		t.Fatalf("target runner ttl = %v, want %v", stub.lastSpec.TTL, 4*time.Minute)
	}
}

func TestLeaseLoopRenewsAndExpiresMasterLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	cfg := &goalx.Config{
		Name:      "lease-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	cmd := exec.Command("sleep", "1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() { _ = cmd.Process.Kill() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runLeaseLoop(ctx, runDir, "master", meta.RunID, meta.Epoch, 2*time.Second, "tmux", cmd.Process.Pid)
	}()

	time.Sleep(200 * time.Millisecond)

	lease, err := LoadControlLease(ControlLeasePath(runDir, "master"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.RunID != meta.RunID || lease.Epoch != meta.Epoch || lease.PID != cmd.Process.Pid {
		t.Fatalf("unexpected lease after renew: %+v", lease)
	}
	if lease.Transport != "tmux" {
		t.Fatalf("transport = %q, want tmux", lease.Transport)
	}

	if err := cmd.Wait(); err != nil {
		t.Fatalf("wait sleep: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runLeaseLoop: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("lease loop did not stop after target pid exited")
	}

	lease, err = LoadControlLease(ControlLeasePath(runDir, "master"))
	if err != nil {
		t.Fatalf("LoadControlLease expired: %v", err)
	}
	if lease.RunID != "" || lease.PID != 0 {
		t.Fatalf("expected expired lease, got %+v", lease)
	}
}

func TestTargetRunnerRenewsLeaseUntilChildExit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	cfg := &goalx.Config{
		Name:      "target-runner",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	if err := TargetRunnerCommand(repo, []string{
		"--run", cfg.Name,
		"--run-dir", runDir,
		"--holder", "master",
		"--run-id", meta.RunID,
		"--epoch", "1",
		"--ttl-seconds", "2",
		"--transport", "tmux",
		"--engine-command", "sleep",
		"--prompt", "1",
	}); err != nil {
		t.Fatalf("TargetRunnerCommand: %v", err)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "master"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.RunID != "" || lease.PID != 0 {
		t.Fatalf("expected expired lease after target runner exit, got %+v", lease)
	}
}
