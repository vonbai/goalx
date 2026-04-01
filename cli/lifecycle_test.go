package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"github.com/vonbai/goalx/internal/slowtest"
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

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	sess := state.Sessions["session-1"]
	if sess.State != "parked" {
		t.Fatalf("session state = %q, want parked", sess.State)
	}
	if sess.OwnerScope != "db race triage" {
		t.Fatalf("session owner scope = %q, want db race triage", sess.OwnerScope)
	}
	if sess.BlockedBy != "postgres lock timeout" {
		t.Fatalf("session blocked_by = %q, want postgres lock timeout", sess.BlockedBy)
	}
	coord, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if _, ok := coord.Sessions["session-1"]; ok {
		t.Fatalf("coordination should not publish parked runtime state: %+v", coord.Sessions["session-1"])
	}
	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "handoffs", "session-1.json")); !os.IsNotExist(err) {
		t.Fatalf("park should not create legacy handoff file, stat err = %v", err)
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
	if sess.State != "parked" {
		t.Fatalf("coordination session state = %q, want original parked semantic state preserved", sess.State)
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
	for _, want := range []string{"target-runner --run", "--holder", "session-1"} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "send-keys -t "+wantSession+":session-1") {
		t.Fatalf("resume should launch directly, not via send-keys:\n%s", logText)
	}

	inboxData, err := os.ReadFile(ControlInboxPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("read session inbox: %v", err)
	}
	if strings.Contains(string(inboxData), `"type":"handoff"`) {
		t.Fatalf("resume should not queue legacy handoff message:\n%s", string(inboxData))
	}
}

func TestResumeUsesRunWorktreeWhenSessionHasNoDedicatedWorktree(t *testing.T) {
	slowtest.Require(t, "tmux/worktree lifecycle integration test")
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
		Mode:       string(goalx.ModeWorker),
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

func TestResumeKeepsSessionParkedWhenLaunchHandshakeFails(t *testing.T) {
	slowtest.Require(t, "tmux/worktree lifecycle integration test")
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf 'master\n'
    exit 0
    ;;
  new-window)
    exit 0
    ;;
  capture-pane)
    exit 0
    ;;
  kill-window)
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "parked",
		Mode:         string(goalx.ModeWorker),
		Branch:       "goalx/" + runName + "/1",
		WorktreePath: WorktreePath(runDir, runName, 1),
		OwnerScope:   "db race triage",
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

	err = Resume(repo, []string{"--run", runName, "session-1"})
	if err == nil || !strings.Contains(err.Error(), "launch handshake") {
		t.Fatalf("Resume error = %v, want launch handshake failure", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := state.Sessions["session-1"].State; got != "parked" {
		t.Fatalf("session runtime state = %q, want parked after failed resume handshake", got)
	}
	coord, err = LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if got := coord.Sessions["session-1"].State; got != "parked" {
		t.Fatalf("coordination session-1 state = %q, want parked after failed resume handshake", got)
	}
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read tmux log: %v", readErr)
	}
	if !strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n session-1") {
		t.Fatalf("tmux log missing resume launch attempt:\n%s", string(logData))
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
	cfg.Roles.Worker.Engine = "claude-code"
	cfg.Roles.Worker.Model = "opus"
	cfg.Sessions = []goalx.SessionConfig{{
		Engine: "claude-code",
		Model:  "opus",
		Mode:   goalx.ModeWorker,
		Target: &goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}},
		LocalValidation: &goalx.LocalValidationConfig{
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
	if !strings.Contains(logText, "--engine-command 'codex ") {
		t.Fatalf("resume launch should use durable session identity engine:\n%s", logText)
	}
	protocolText, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	if !strings.Contains(string(protocolText), "go test ./...") {
		t.Fatalf("resume protocol missing durable local validation command:\n%s", string(protocolText))
	}
	if strings.Contains(string(protocolText), "test -s report.md") {
		t.Fatalf("resume protocol should not recompute session local validation from current config:\n%s", string(protocolText))
	}
}

func TestResumeUsesCurrentProcessEnvWithoutSnapshot(t *testing.T) {
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
  capture-pane)
    printf '❯ ready\n'
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

	runName, _ := writeLifecycleRunFixture(t, repo)
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
		"FOO_TOOLCHAIN_ROOT='/opt/resume-after'",
		"OPENAI_API_KEY='sk-after'",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("resume launch missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "OPENAI_API_KEY='sk-before'") || strings.Contains(logText, "FOO_TOOLCHAIN_ROOT='/opt/resume-before'") {
		t.Fatalf("resume should use current process env, not a stored snapshot:\n%s", logText)
	}
}

func TestReplaceCreatesReplacementSessionWithExplicitOverrideAndLineage(t *testing.T) {
	slowtest.Require(t, "tmux/worktree lifecycle integration test")
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master session-1")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	session1WT := WorktreePath(runDir, runName, 1)
	if err := os.RemoveAll(session1WT); err != nil {
		t.Fatalf("remove placeholder session-1 worktree: %v", err)
	}
	if err := CreateWorktree(runWT, session1WT, "goalx/"+runName+"/1", "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree session-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(session1WT, "feature.txt"), []byte("from session 1\n"), 0o644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	runGit(t, session1WT, "add", "feature.txt")
	runGit(t, session1WT, "commit", "-m", "session branch change")
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Mode:         string(goalx.ModeWorker),
		Branch:       "goalx/" + runName + "/1",
		WorktreePath: session1WT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}

	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{
		State: "active",
		Scope: "db race triage",
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	oldIdentity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity session-1: %v", err)
	}
	if oldIdentity == nil {
		t.Fatal("session-1 identity missing")
	}

	if err := Replace(repo, []string{"--run", runName, "session-1", "--engine", "claude-code", "--model", "opus", "--effort", "high"}); err != nil {
		t.Fatalf("Replace: %v", err)
	}

	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	if activity == nil {
		t.Fatal("activity snapshot missing after replace")
	}
	if _, ok := activity.Sessions["session-2"]; !ok {
		t.Fatalf("activity snapshot missing replacement session: %+v", activity.Sessions)
	}
	index, err := LoadContextIndex(ContextIndexPath(runDir))
	if err != nil {
		t.Fatalf("LoadContextIndex: %v", err)
	}
	if index == nil {
		t.Fatal("context index missing after replace")
	}
	found := false
	for _, sess := range index.Sessions {
		if sess.Name == "session-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("context index missing replacement session: %+v", index.Sessions)
	}
	cognition, err := LoadCognitionState(CognitionStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadCognitionState: %v", err)
	}
	found = false
	for _, scope := range cognition.Scopes {
		if scope.Scope == "session-2" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cognition scopes = %#v, want session-2 replacement scope", cognition.Scopes)
	}
	affordances, err := LoadAffordances(AffordancesJSONPath(runDir))
	if err != nil {
		t.Fatalf("LoadAffordances: %v", err)
	}
	if affordances == nil || affordances.RunName != runName {
		t.Fatalf("affordances not written correctly after replace: %+v", affordances)
	}

	oldState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := oldState.Sessions["session-1"].State; got != "parked" {
		t.Fatalf("session-1 state = %q, want parked", got)
	}
	if got := oldState.Sessions["session-2"].State; got != "active" {
		t.Fatalf("session-2 state = %q, want active", got)
	}
	if oldState.Sessions["session-2"].WorktreePath != WorktreePath(runDir, runName, 2) {
		t.Fatalf("session-2 worktree = %q, want %q", oldState.Sessions["session-2"].WorktreePath, WorktreePath(runDir, runName, 2))
	}
	if oldState.Sessions["session-2"].Branch != "goalx/"+runName+"/2" {
		t.Fatalf("session-2 branch = %q, want %q", oldState.Sessions["session-2"].Branch, "goalx/"+runName+"/2")
	}
	data, err := os.ReadFile(filepath.Join(WorktreePath(runDir, runName, 2), "feature.txt"))
	if err != nil {
		t.Fatalf("read session-2 feature.txt: %v", err)
	}
	if string(data) != "from session 1\n" {
		t.Fatalf("session-2 feature.txt = %q, want %q", string(data), "from session 1\n")
	}

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-2"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("replacement identity missing")
	}
	if identity.ReplacesSession != "session-1" {
		t.Fatalf("replaces_session = %q, want session-1", identity.ReplacesSession)
	}
	if identity.Engine != "claude-code" || identity.Model != "opus" || identity.RequestedEffort != goalx.EffortHigh {
		t.Fatalf("identity = %+v, want claude-code/opus/high", identity)
	}
	if identity.BaseBranchSelector != "session-1" {
		t.Fatalf("base_branch_selector = %q, want session-1", identity.BaseBranchSelector)
	}
	if identity.BaseBranch != "goalx/"+runName+"/1" {
		t.Fatalf("base_branch = %q, want %q", identity.BaseBranch, "goalx/"+runName+"/1")
	}
	if identity.BaseExperimentID != oldIdentity.ExperimentID {
		t.Fatalf("base_experiment_id = %q, want %q", identity.BaseExperimentID, oldIdentity.ExperimentID)
	}
	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected experiment log entry for replacement session")
	}
	last := events[len(events)-1]
	if last.Kind != "experiment.created" {
		t.Fatalf("last experiment kind = %q, want experiment.created", last.Kind)
	}
	var created ExperimentCreatedBody
	if err := decodeStrictJSON(last.Body, &created); err != nil {
		t.Fatalf("decode experiment.created: %v", err)
	}
	if created.Session != "session-2" {
		t.Fatalf("created.Session = %q, want session-2", created.Session)
	}
	if created.ExperimentID != identity.ExperimentID {
		t.Fatalf("created.ExperimentID = %q, want %q", created.ExperimentID, identity.ExperimentID)
	}
	if created.BaseExperimentID != oldIdentity.ExperimentID {
		t.Fatalf("created.BaseExperimentID = %q, want %q", created.BaseExperimentID, oldIdentity.ExperimentID)
	}

	coord, err = LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if _, ok := coord.Sessions["session-2"]; ok {
		t.Fatalf("coordination should not publish replacement runtime state: %+v", coord.Sessions["session-2"])
	}
	if got := coord.Sessions["session-1"].Scope; got != "db race triage" {
		t.Fatalf("session-1 scope = %q, want db race triage", got)
	}
	if _, err := os.Stat(filepath.Join(ControlDir(runDir), "handoffs", "session-2.json")); !os.IsNotExist(err) {
		t.Fatalf("replace should not create legacy handoff file, stat err = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	tmuxSession := goalx.TmuxSessionName(repo, runName)
	if !strings.Contains(logText, "kill-window -t "+tmuxSession+":session-1") {
		t.Fatalf("tmux log missing kill-window for session-1:\n%s", logText)
	}
	if !strings.Contains(logText, "new-window -t "+tmuxSession+" -n session-2 -c "+WorktreePath(runDir, runName, 2)+" env ") {
		t.Fatalf("tmux log missing new-window for session-2:\n%s", logText)
	}
}

func TestResumeFailsWhenRunBudgetIsExhausted(t *testing.T) {
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
	cfg.Budget.MaxDuration = time.Second
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(cfg.Mode),
		Active:    true,
		StartedAt: "2000-01-01T00:00:00Z",
		UpdatedAt: "2000-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	err = Resume(repo, []string{"--run", runName, "session-1"})
	if err == nil {
		t.Fatal("Resume error = nil, want budget exhausted")
	}
	for _, want := range []string{"budget exhausted", "max_duration=1s", "exhausted=true"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Resume error = %q, want substring %q", err, want)
		}
	}
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read tmux log: %v", readErr)
	}
	if strings.Contains(string(logData), "new-window") {
		t.Fatalf("resume should not launch tmux window after exhausted budget:\n%s", string(logData))
	}
}

func TestReplaceFailsWhenRunBudgetIsExhausted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakeTmux(t, "master session-1")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	cfg.Budget.MaxDuration = time.Second
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(cfg.Mode),
		Active:    true,
		StartedAt: "2000-01-01T00:00:00Z",
		UpdatedAt: "2000-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	err = Replace(repo, []string{"--run", runName, "session-1"})
	if err == nil {
		t.Fatal("Replace error = nil, want budget exhausted")
	}
	for _, want := range []string{"budget exhausted", "max_duration=1s", "exhausted=true"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Replace error = %q, want substring %q", err, want)
		}
	}
	if _, statErr := os.Stat(SessionIdentityPath(runDir, "session-2")); !os.IsNotExist(statErr) {
		t.Fatalf("replacement session should not be created after exhausted budget, stat err = %v", statErr)
	}
}

func TestReplaceRestoresOldSessionAndCleansPartialStateWhenReplacementLaunchFails(t *testing.T) {
	slowtest.Require(t, "tmux/worktree lifecycle integration test")
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf '%s\n' master
    exit 0
    ;;
  capture-pane)
    printf 'pane output\n'
    exit 0
    ;;
  new-window)
    case " $* " in
      *" -n session-2 "*)
        exit 1
        ;;
    esac
    exit 0
    ;;
  kill-window)
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	session1WT := WorktreePath(runDir, runName, 1)
	if err := os.RemoveAll(session1WT); err != nil {
		t.Fatalf("remove placeholder session-1 worktree: %v", err)
	}
	if err := CreateWorktree(runWT, session1WT, "goalx/"+runName+"/1", "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree session-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(session1WT, "feature.txt"), []byte("from session 1\n"), 0o644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	runGit(t, session1WT, "add", "feature.txt")
	runGit(t, session1WT, "commit", "-m", "session branch change")
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Mode:         string(goalx.ModeWorker),
		Branch:       "goalx/" + runName + "/1",
		WorktreePath: session1WT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, "fix pipeline")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	cfg.Roles.Worker = goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{
		State: "active",
		Scope: "db race triage",
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	err = Replace(repo, []string{"--run", runName, "session-1", "--effort", "high"})
	if err == nil || !strings.Contains(err.Error(), "create tmux window") {
		t.Fatalf("Replace error = %v, want replacement launch failure", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := state.Sessions["session-1"].State; got != "active" {
		t.Fatalf("session-1 state = %q, want active after rollback", got)
	}
	if _, ok := state.Sessions["session-2"]; ok {
		t.Fatalf("session-2 runtime state should be removed after rollback: %#v", state.Sessions["session-2"])
	}
	coord, err = LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if got := coord.Sessions["session-1"].State; got != "active" {
		t.Fatalf("coordination session-1 state = %q, want active", got)
	}
	if _, ok := coord.Sessions["session-2"]; ok {
		t.Fatalf("coordination should not retain session-2 after rollback: %+v", coord.Sessions["session-2"])
	}
	for _, path := range []string{
		SessionIdentityPath(runDir, "session-2"),
		filepath.Dir(SessionIdentityPath(runDir, "session-2")),
		JournalPath(runDir, "session-2"),
		ControlInboxPath(runDir, "session-2"),
		SessionCursorPath(runDir, "session-2"),
		filepath.Join(runDir, "program-2.md"),
		WorktreePath(runDir, runName, 2),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be absent after replacement rollback, stat err = %v", path, statErr)
		}
	}
	events, loadErr := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if loadErr != nil {
		t.Fatalf("LoadDurableLog: %v", loadErr)
	}
	for _, event := range events {
		var body ExperimentCreatedBody
		if event.Kind != "experiment.created" {
			continue
		}
		if err := decodeStrictJSON(event.Body, &body); err == nil && body.Session == "session-2" {
			t.Fatalf("unexpected experiment.created for rolled-back replacement: %+v", body)
		}
	}
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read tmux log: %v", readErr)
	}
	logText := string(logData)
	if !strings.Contains(logText, "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n session-1 -c "+session1WT+" env ") {
		t.Fatalf("tmux log missing rollback resume launch for session-1:\n%s", logText)
	}
}

func TestReplaceRestoresOldSessionWhenReplacementLaunchHandshakeFails(t *testing.T) {
	slowtest.Require(t, "tmux/worktree lifecycle integration test")
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf '%s\n' master
    exit 0
    ;;
  capture-pane)
    case "$3" in
      *:session-2)
        printf 'Quick safety check\nYes, I trust this folder\n'
        exit 0
        ;;
      *)
        printf '❯ ready\n'
        exit 0
        ;;
    esac
    ;;
  new-window)
    exit 0
    ;;
  kill-window)
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	session1WT := WorktreePath(runDir, runName, 1)
	if err := os.RemoveAll(session1WT); err != nil {
		t.Fatalf("remove placeholder session-1 worktree: %v", err)
	}
	if err := CreateWorktree(runWT, session1WT, "goalx/"+runName+"/1", "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree session-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(session1WT, "feature.txt"), []byte("from session 1\n"), 0o644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	runGit(t, session1WT, "add", "feature.txt")
	runGit(t, session1WT, "commit", "-m", "session branch change")
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Mode:         string(goalx.ModeWorker),
		Branch:       "goalx/" + runName + "/1",
		WorktreePath: session1WT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, "fix pipeline")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	cfg.Roles.Worker = goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{
		State: "active",
		Scope: "db race triage",
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	err = Replace(repo, []string{"--run", runName, "session-1", "--effort", "high"})
	if err == nil || !strings.Contains(err.Error(), "launch handshake") {
		t.Fatalf("Replace error = %v, want replacement launch handshake failure", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	if got := state.Sessions["session-1"].State; got != "active" {
		t.Fatalf("session-1 state = %q, want active after rollback", got)
	}
	if _, ok := state.Sessions["session-2"]; ok {
		t.Fatalf("session-2 runtime state should be removed after rollback: %#v", state.Sessions["session-2"])
	}
	coord, err = LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if got := coord.Sessions["session-1"].State; got != "active" {
		t.Fatalf("coordination session-1 state = %q, want active", got)
	}
	if _, ok := coord.Sessions["session-2"]; ok {
		t.Fatalf("coordination should not retain session-2 after rollback: %+v", coord.Sessions["session-2"])
	}
	for _, path := range []string{
		SessionIdentityPath(runDir, "session-2"),
		filepath.Dir(SessionIdentityPath(runDir, "session-2")),
		JournalPath(runDir, "session-2"),
		ControlInboxPath(runDir, "session-2"),
		SessionCursorPath(runDir, "session-2"),
		filepath.Join(runDir, "program-2.md"),
		WorktreePath(runDir, runName, 2),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be absent after replacement handshake rollback, stat err = %v", path, statErr)
		}
	}
	events, loadErr := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if loadErr != nil {
		t.Fatalf("LoadDurableLog: %v", loadErr)
	}
	for _, event := range events {
		var body ExperimentCreatedBody
		if event.Kind != "experiment.created" {
			continue
		}
		if err := decodeStrictJSON(event.Body, &body); err == nil && body.Session == "session-2" {
			t.Fatalf("unexpected experiment.created for rolled-back replacement handshake: %+v", body)
		}
	}
	logData, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("read tmux log: %v", readErr)
	}
	if !strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n session-1 -c "+session1WT+" env ") {
		t.Fatalf("tmux log missing rollback resume launch for session-1:\n%s", string(logData))
	}
}

func TestReplaceRejectsDirtyDedicatedWorktreeTakeover(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakeTmux(t, "master session-1")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	session1WT := WorktreePath(runDir, runName, 1)
	if err := os.RemoveAll(session1WT); err != nil {
		t.Fatalf("remove placeholder session-1 worktree: %v", err)
	}
	if err := CreateWorktree(runWT, session1WT, "goalx/"+runName+"/1", "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree session-1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(session1WT, "feature.txt"), []byte("dirty takeover\n"), 0o644); err != nil {
		t.Fatalf("write feature.txt: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "active",
		Mode:         string(goalx.ModeWorker),
		Branch:       "goalx/" + runName + "/1",
		WorktreePath: session1WT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}

	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{
		State: "active",
		Scope: "db race triage",
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	err = Replace(repo, []string{"--run", runName, "session-1"})
	if err == nil {
		t.Fatal("Replace error = nil, want dirty dedicated worktree rejection")
	}
	for _, want := range []string{"session-1", "dedicated worktree", "uncommitted changes", "cannot hand off", "unsealed worktree boundary"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Replace error = %q, want substring %q", err, want)
		}
	}
}

func TestReplaceRejectsRemovedRouteFlags(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"session-1", "--route-role", "research"},
		{"session-1", "--route-profile", "research_deep"},
	}
	for _, args := range tests {
		if err := Replace(t.TempDir(), args); err == nil {
			t.Fatalf("Replace(%#v) unexpectedly succeeded", args)
		}
	}
}

func TestStatusShowsParkedSessionStateFromRuntime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName, runDir := writeLifecycleRunFixture(t, repo)
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":3,"desc":"awaiting master","status":"idle","owner_scope":"db race triage"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:       "session-1",
		State:      "parked",
		Mode:       string(goalx.ModeWorker),
		OwnerScope: "db race triage",
		LastRound:  3,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
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
	slowtest.Require(t, "tmux lifecycle integration test")

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
		Mode:      goalx.ModeWorker,
		Objective: "fix pipeline",
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Master: goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Sessions: []goalx.SessionConfig{
			{Hint: "db race triage"},
		},
		Target:          goalx.TargetConfig{Files: []string{"."}},
		LocalValidation: goalx.LocalValidationConfig{Command: "go test ./..."},
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
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if _, err := EnsureObligationModel(runDir, nil, nil, "bootstrap-objective", cfg.Objective); err != nil {
		t.Fatalf("EnsureObligationModel: %v", err)
	}
	if err := EnsureObligationLog(runDir); err != nil {
		t.Fatalf("EnsureObligationLog: %v", err)
	}
	if _, err := EnsureAssurancePlan(runDir, NewAcceptanceState(&cfg, 0)); err != nil {
		t.Fatalf("EnsureAssurancePlan: %v", err)
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
	identity, err := NewSessionIdentity(runDir, "session-1", "master-derived-develop", goalx.ModeWorker, "codex", "gpt-5.4", "", "", "", cfg.Target)
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	identity.LocalValidationCommand = goalx.ResolveLocalValidationCommand(&cfg)
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	return runName, runDir
}
