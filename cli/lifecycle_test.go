package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestParkMarksSessionParkedAndStopsWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master session-1")
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

	logPath := installFakeTmuxWithKillAction(t, "master session-1", fmt.Sprintf("rm -rf %q", wtPath))
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

	logPath := installFakeTmux(t, "master")
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

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("expected session identity to exist after resume")
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
	if !strings.Contains(logText, "new-window -t "+wantSession+" -n session-1 -c "+WorktreePath(runDir, runName, 1)+" env ") {
		t.Fatalf("tmux log missing new-window:\n%s", logText)
	}
	for _, want := range []string{"/bin/bash -c ", "lease-loop --run", "--holder", "session-1"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "send-keys -t "+wantSession+":session-1") {
		t.Fatalf("resume should launch directly, not via send-keys:\n%s", logText)
	}
}

func TestResumeUsesRunWorktreeWhenSessionHasNoDedicatedWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	if err := os.RemoveAll(WorktreePath(runDir, runName, 1)); err != nil {
		t.Fatalf("remove session worktree: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:       "session-1",
		State:      "parked",
		Mode:       string(goalx.ModeDevelop),
		Branch:     "goalx/" + runName + "/1",
		OwnerScope: "db race triage",
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

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

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	sess := state.Sessions["session-1"]
	if sess.WorktreePath != "" {
		t.Fatalf("session worktree path = %q, want empty", sess.WorktreePath)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantSession := goalx.TmuxSessionName(repo, runName)
	if !strings.Contains(logText, "new-window -t "+wantSession+" -n session-1 -c "+runWT+" env ") {
		t.Fatalf("tmux log missing run worktree launch:\n%s", logText)
	}
}

func TestResumeRequiresExistingSessionIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity: %v", err)
	}

	err := Resume(repo, []string{"--run", runName, "session-1"})
	if err == nil || !strings.Contains(err.Error(), "session identity") {
		t.Fatalf("Resume error = %v, want missing session identity", err)
	}
}

func TestResumeRejectsMismatchedSessionIdentityCharter(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	identity.OriginCharterID = "charter_other"
	if err := writeJSONFile(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("rewrite session identity: %v", err)
	}

	err = Resume(repo, []string{"--run", runName, "session-1"})
	if err == nil || !strings.Contains(err.Error(), "charter linkage") {
		t.Fatalf("Resume error = %v, want charter linkage failure", err)
	}
}

func TestResumeUsesDurableSessionIdentityInsteadOfCurrentConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	cfg.Engine = "claude-code"
	cfg.Model = "opus"
	cfg.Sessions = []goalx.SessionConfig{{
		Engine: "claude-code",
		Model:  "opus",
		Mode:   goalx.ModeResearch,
		Target: &goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}},
		Harness: &goalx.HarnessConfig{
			Command: "test -s report.md",
		},
		Hint: "mutated after park",
	}}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	if err := Resume(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	out, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(out)
	if strings.Contains(logText, "claude ") {
		t.Fatalf("resume launch should not recompute current config engine:\n%s", logText)
	}
	if !strings.Contains(logText, "exec codex ") {
		t.Fatalf("resume launch should use durable session identity engine:\n%s", logText)
	}
}

func TestResumeUsesRunLaunchEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
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
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+"/tmp/goalx-bin:/usr/bin")
	t.Setenv("OPENAI_API_KEY", "sk-before")
	t.Setenv("FOO_TOOLCHAIN_ROOT", "/opt/resume-before")

	runName, runDir := writeLifecycleRunFixture(t, repo)
	writeTestLaunchEnvSnapshot(t, runDir, map[string]string{
		"HOME":               home,
		"PATH":               fakeBin + string(os.PathListSeparator) + "/tmp/goalx-bin:/usr/bin",
		"OPENAI_API_KEY":     "sk-before",
		"FOO_TOOLCHAIN_ROOT": "/opt/resume-before",
	})

	t.Setenv("OPENAI_API_KEY", "sk-after")
	t.Setenv("FOO_TOOLCHAIN_ROOT", "/opt/resume-after")

	if err := Resume(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	out, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(out)
	for _, want := range []string{
		"FOO_TOOLCHAIN_ROOT='/opt/resume-before'",
		"OPENAI_API_KEY='sk-before'",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("resume launch missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "OPENAI_API_KEY='sk-after'") || strings.Contains(logText, "FOO_TOOLCHAIN_ROOT='/opt/resume-after'") {
		t.Fatalf("resume should use stored run launch env, not caller env:\n%s", logText)
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

func TestParkExpiresSessionLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakeTmux(t, "master session-1")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := RenewControlLease(runDir, "session-1", "run_demo", 1, time.Minute, "tmux", 2345); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}

	if err := Park(repo, []string{"--run", runName, "session-1"}); err != nil {
		t.Fatalf("Park: %v", err)
	}

	lease, err := LoadControlLease(ControlLeasePath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}
	if lease.RunID != "" || lease.PID != 0 {
		t.Fatalf("session lease not expired: %+v", lease)
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
	writeTestLaunchEnvSnapshotFromCurrent(t, runDir)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	goalState, err := EnsureGoalState(runDir)
	if err != nil {
		t.Fatalf("EnsureGoalState: %v", err)
	}
	if err := EnsureGoalLog(runDir); err != nil {
		t.Fatalf("EnsureGoalLog: %v", err)
	}
	if _, err := EnsureAcceptanceState(runDir, &cfg, goalState.Version); err != nil {
		t.Fatalf("EnsureAcceptanceState: %v", err)
	}
	charter, err := NewRunCharter(runDir, runName, cfg.Objective, meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		t.Fatalf("hashRunCharter: %v", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	fence, err := NewIdentityFence(runDir, meta)
	if err != nil {
		t.Fatalf("NewIdentityFence: %v", err)
	}
	if err := SaveIdentityFence(IdentityFencePath(runDir), fence); err != nil {
		t.Fatalf("SaveIdentityFence: %v", err)
	}
	identity, err := NewSessionIdentity(runDir, "session-1", "master-derived-develop", goalx.ModeDevelop, "codex", "codex", cfg.Target)
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	return runName, runDir
}
