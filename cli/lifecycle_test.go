package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestParkMarksSessionParkedAndStopsWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master heartbeat session-1")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":4,"desc":"db race triage","status":"stuck","owner_scope":"db race triage","blocked_by":"postgres lock timeout"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}

	if err := Park(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("Park: %v", err)
	}

	state, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	sess := state.Sessions["session-1"]
	if sess.State != "parked" {
		t.Fatalf("session state = %q, want parked", sess.State)
	}
	if sess.Scope != "db race triage" {
		t.Fatalf("session scope = %q, want db race triage", sess.Scope)
	}
	if sess.BlockedBy != "postgres lock timeout" {
		t.Fatalf("session blocked_by = %q, want postgres lock timeout", sess.BlockedBy)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "kill-window -t "+goalx.TmuxSessionName(repo, runName)+":session-1") {
		t.Fatalf("tmux log missing kill-window call:\n%s", string(logData))
	}
}

func TestParkSnapshotsDirtyWorktreeBeforeWindowTermination(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName, runDir := writeLifecycleRunFixture(t, repo)
	wtPath := WorktreePath(runDir, runName, 1)
	runGit(t, wtPath, "init")
	runGit(t, wtPath, "config", "user.email", "test@example.com")
	runGit(t, wtPath, "config", "user.name", "Test User")
	if err := os.WriteFile(filepath.Join(wtPath, "tracked.txt"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write tracked file: %v", err)
	}
	runGit(t, wtPath, "add", "tracked.txt")
	runGit(t, wtPath, "commit", "-m", "seed tracked file")
	if err := os.WriteFile(filepath.Join(wtPath, "tracked.txt"), []byte("after\n"), 0o644); err != nil {
		t.Fatalf("modify tracked file: %v", err)
	}

	logPath := installFakeTmuxWithKillAction(t, "master heartbeat session-1", fmt.Sprintf("rm -rf %q", wtPath))
	if err := Park(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("Park: %v", err)
	}

	state, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	sess := state.Sessions["session-1"]
	if sess.State != "parked" {
		t.Fatalf("session runtime state = %q, want parked", sess.State)
	}
	if sess.DirtyFiles == 0 {
		t.Fatalf("expected parked snapshot to retain dirty worktree details, got %+v", sess)
	}
	if !strings.Contains(sess.DiffStat, "tracked.txt") {
		t.Fatalf("expected diff stat to mention tracked.txt, got %q", sess.DiffStat)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if !strings.Contains(string(logData), "kill-window -t "+goalx.TmuxSessionName(repo, runName)+":session-1") {
		t.Fatalf("tmux log missing kill-window call:\n%s", string(logData))
	}
}

func TestResumeRelaunchesParkedSessionAndMarksActive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master heartbeat")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	coord, err := EnsureCoordinationState(runDir, "fix pipeline")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{
		State: "parked",
		Scope: "db race triage",
	}
	coord.Version++
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	if err := Resume(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	state, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	sess := state.Sessions["session-1"]
	if sess.State != "active" {
		t.Fatalf("session state = %q, want active", sess.State)
	}
	if sess.Scope != "db race triage" {
		t.Fatalf("session scope = %q, want db race triage", sess.Scope)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantSession := goalx.TmuxSessionName(repo, runName)
	if !strings.Contains(logText, "new-window -t "+wantSession+" -n session-1") {
		t.Fatalf("tmux log missing new-window:\n%s", logText)
	}
	if !strings.Contains(logText, "send-keys -t "+wantSession+":session-1") {
		t.Fatalf("tmux log missing launch send-keys:\n%s", logText)
	}
}

func TestStatusShowsParkedSessionStateFromCoordination(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":3,"desc":"awaiting master","status":"idle","owner_scope":"db race triage"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, "fix pipeline")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{
		State:     "parked",
		Scope:     "db race triage",
		LastRound: 3,
	}
	coord.Version++
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	if !strings.Contains(out, "session-1") || !strings.Contains(out, "parked") {
		t.Fatalf("status output missing parked session:\n%s", out)
	}
	if !strings.Contains(out, "parked: db race triage") {
		t.Fatalf("status output missing parked summary:\n%s", out)
	}
}

func installFakeTmux(t *testing.T, windows string) string {
	t.Helper()
	return installFakeTmuxWithKillAction(t, windows, "")
}

func installFakeTmuxWithKillAction(t *testing.T, windows, killAction string) string {
	t.Helper()

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    if [ -n "$TMUX_WINDOWS" ]; then
      printf '%s\n' $TMUX_WINDOWS
    fi
    exit 0
    ;;
  capture-pane)
    printf 'pane output\n'
    exit 0
    ;;
  kill-window)
    if [ -n "$TMUX_KILL_ACTION" ]; then
      eval "$TMUX_KILL_ACTION"
    fi
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
	t.Setenv("TMUX_WINDOWS", windows)
	t.Setenv("TMUX_KILL_ACTION", killAction)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}

func writeLifecycleRunFixture(t *testing.T, repo string) (string, string) {
	t.Helper()

	runName := "lifecycle-run"
	runDir := goalx.RunDir(repo, runName)
	for _, dir := range []string{
		runDir,
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "guidance"),
		filepath.Join(runDir, "worktrees"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeDevelop,
		Objective: "fix pipeline",
		Engine:    "codex",
		Model:     "codex",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
		Sessions: []goalx.SessionConfig{
			{Hint: "db race triage"},
		},
		Target:  goalx.TargetConfig{Files: []string{"."}},
		Harness: goalx.HarnessConfig{Command: "go test ./..."},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-1"), nil, 0o644); err != nil {
		t.Fatalf("seed session journal: %v", err)
	}
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if _, err := EnsureCoordinationState(runDir, cfg.Objective); err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	return runName, runDir
}
