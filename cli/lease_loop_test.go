package cli

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestBuildMasterLaunchCommandWrapsLeaseLoop(t *testing.T) {
	t.Setenv("HOME", "/tmp/goalx-home")
	t.Setenv("PATH", "/tmp/goalx-bin:/usr/bin")

	cmd := buildMasterLaunchCommand(
		"/root/go/bin/goalx",
		"demo-run",
		"run_demo",
		7,
		4*time.Minute,
		"codex -m gpt-5.4 -a never -s danger-full-access",
		"/tmp/run/master.md",
	)

	for _, want := range []string{
		"env ",
		"/bin/bash -c ",
		"lease-loop --run",
		"--holder master",
		"--run-id",
		"--epoch 7",
		"--ttl-seconds 240",
		"--transport tmux",
		"--pid $$",
		"exec codex -m gpt-5.4 -a never -s danger-full-access",
		"/tmp/run/master.md",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("master launch command missing %q:\n%s", want, cmd)
		}
	}
}

func TestLeaseLoopRenewsAndExpiresMasterLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	cfg := &goalx.Config{
		Name:      "lease-run",
		Mode:      goalx.ModeDevelop,
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
