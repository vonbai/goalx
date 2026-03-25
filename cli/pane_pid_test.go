package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestStartPersistsMasterPanePID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target:  goalx.TargetConfig{Files: []string{"README.md"}},
		Harness: goalx.HarnessConfig{Command: "test -f README.md"},
		Master:  goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	cfgPath := filepath.Join(goalxDir, "goalx.yaml")
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	installFakePaneTmux(t, false, "4321", 0)

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Start(repo, []string{"--config", cfgPath}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	runDir := goalx.RunDir(repo, cfg.Name)
	paneData, err := os.ReadFile(filepath.Join(ControlDir(runDir), "pane-pids", "master"))
	if err != nil {
		t.Fatalf("read persisted master pane pid: %v", err)
	}
	if got := strings.TrimSpace(string(paneData)); got != "4321" {
		t.Fatalf("persisted master pane pid = %q, want 4321", got)
	}
}

func TestAddPersistsPanePIDForNewSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakePaneTmux(t, true, "9876", 0)

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: codex
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"--run", runName, "--mode", "develop", "second direction"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	paneData, err := os.ReadFile(filepath.Join(ControlDir(runDir), "pane-pids", "session-2"))
	if err != nil {
		t.Fatalf("read persisted session pane pid: %v", err)
	}
	if got := strings.TrimSpace(string(paneData)); got != "9876" {
		t.Fatalf("persisted session pane pid = %q, want 9876", got)
	}
}

func TestStopKillsLivePaneProcessTreesBeforeTmuxSession(t *testing.T) {
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
	installFakePaneTmux(t, true, strconv.Itoa(shellPID), shellPID)

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("live pane process tree did not exit")
	}

	waitForProcessExit(t, childPID)
}

func TestStopKillsPersistedPaneProcessTreesWhenTmuxSessionMissing(t *testing.T) {
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
	writePersistedPanePID(t, runDir, "session-1", shellPID)
	installFakePaneTmux(t, false, "", 0)

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	if err := Stop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("persisted pane process tree did not exit")
	}

	waitForProcessExit(t, childPID)
}

func TestDropKillsPersistedPaneProcessTreesWhenTmuxSessionMissing(t *testing.T) {
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
	writePersistedPanePID(t, runDir, "session-1", shellPID)
	installFakePaneTmux(t, false, "", 0)

	origStopSidecar := stopRunSidecar
	defer func() { stopRunSidecar = origStopSidecar }()
	stopRunSidecar = func(runDir string) error { return nil }

	if err := Drop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Drop: %v", err)
	}

	select {
	case <-waitDone:
	case <-time.After(5 * time.Second):
		t.Fatal("persisted pane process tree did not exit after drop")
	}

	waitForProcessExit(t, childPID)
}

func TestScanLivenessReportsJournalStaleMinutes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "sidecar-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	sessionWT := WorktreePath(runDir, cfg.Name, 1)
	if err := os.MkdirAll(sessionWT, 0o755); err != nil {
		t.Fatalf("mkdir session worktree: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		WorktreePath: sessionWT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte("{\"round\":1}\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
	staleAt := time.Now().Add(-(16*time.Minute + 10*time.Second))
	if err := os.Chtimes(JournalPath(runDir, "session-1"), staleAt, staleAt); err != nil {
		t.Fatalf("chtimes session journal: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease session: %v", err)
	}

	state, err := ScanLiveness(runDir)
	if err != nil {
		t.Fatalf("ScanLiveness: %v", err)
	}
	session := state.Sessions["session-1"]
	if session.JournalStaleMinutes < 16 {
		t.Fatalf("session journal stale minutes = %d, want >= 16", session.JournalStaleMinutes)
	}
}

func TestRunSidecarLoopWritesAuditLogOnError(t *testing.T) {
	runDir := t.TempDir()
	if err := os.WriteFile(RunMetadataPath(runDir), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write invalid metadata: %v", err)
	}

	err := runSidecarLoop(context.Background(), t.TempDir(), "demo", runDir, "run_demo", 1, time.Second)
	if err == nil {
		t.Fatal("expected runSidecarLoop error")
	}

	logData, readErr := os.ReadFile(filepath.Join(runDir, "sidecar.log"))
	if readErr != nil {
		t.Fatalf("read sidecar audit log: %v", readErr)
	}
	logText := string(logData)
	for _, want := range []string{
		"sidecar started pid=",
		"sidecar error:",
		"sidecar exiting reason=",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("sidecar audit log missing %q:\n%s", want, logText)
		}
	}
}

func TestRunLeaseLoopWritesExitAuditLog(t *testing.T) {
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

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := runLeaseLoop(ctx, runDir, "master", meta.RunID, meta.Epoch, 2*time.Second, "tmux", os.Getpid()); err != nil {
		t.Fatalf("runLeaseLoop: %v", err)
	}

	logData, err := os.ReadFile(filepath.Join(runDir, "sidecar.log"))
	if err != nil {
		t.Fatalf("read sidecar audit log: %v", err)
	}
	if !strings.Contains(string(logData), "lease-loop exiting holder=master reason=context canceled") {
		t.Fatalf("lease-loop audit log missing exit reason:\n%s", string(logData))
	}
}

func installFakePaneTmux(t *testing.T, hasSession bool, panePIDs string, expectDeadPID int) {
	t.Helper()

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    if [ "${TMUX_HAS_SESSION:-0}" = "1" ]; then
      exit 0
    fi
    exit 1
    ;;
  list-panes)
    if [ -n "${TMUX_PANE_PIDS:-}" ]; then
      printf '%s\n' $TMUX_PANE_PIDS
    fi
    exit 0
    ;;
  kill-session)
    if [ -n "${TMUX_EXPECT_DEAD_PID:-}" ] && kill -0 "$TMUX_EXPECT_DEAD_PID" 2>/dev/null; then
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

	hasSessionValue := "0"
	if hasSession {
		hasSessionValue = "1"
	}
	t.Setenv("TMUX_LOG", filepath.Join(fakeBin, "tmux.log"))
	t.Setenv("TMUX_HAS_SESSION", hasSessionValue)
	t.Setenv("TMUX_PANE_PIDS", panePIDs)
	if expectDeadPID > 0 {
		t.Setenv("TMUX_EXPECT_DEAD_PID", strconv.Itoa(expectDeadPID))
	} else {
		t.Setenv("TMUX_EXPECT_DEAD_PID", "")
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func writePersistedPanePID(t *testing.T, runDir, holder string, pid int) {
	t.Helper()

	paneDir := filepath.Join(ControlDir(runDir), "pane-pids")
	if err := os.MkdirAll(paneDir, 0o755); err != nil {
		t.Fatalf("mkdir pane pid dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(paneDir, holder), []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		t.Fatalf("write persisted pane pid: %v", err)
	}
}
