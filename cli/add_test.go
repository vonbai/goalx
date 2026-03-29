package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestAddExtendsExplicitSessionsSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
  - hint: second
    mode: develop
target:
  files: ["."]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	for _, name := range []string{"session-1.jsonl", "session-2.jsonl"} {
		if err := os.WriteFile(filepath.Join(runDir, "journals", name), nil, 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	if err := Add(repo, []string{"third direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-3"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-3 identity missing")
	}
	if identity.RoleKind != "develop" || identity.Mode != string(goalx.ModeDevelop) {
		t.Fatalf("session-3 identity role/mode = %+v", identity)
	}

	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("load run spec: %v", err)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("len(Sessions) = %d, want immutable 2", len(cfg.Sessions))
	}
	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("load sessions runtime state: %v", err)
	}
	sess, ok := state.Sessions["session-3"]
	if !ok {
		t.Fatalf("runtime state missing session-3: %#v", state.Sessions)
	}
	if sess.OwnerScope != "third direction" {
		t.Fatalf("session-3 owner scope = %q, want %q", sess.OwnerScope, "third direction")
	}
}

func TestAddAttachesDimensionsWithoutReplacingDirection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
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

	if err := Add(repo, []string{"audit root cause", "--mode", "develop", "--dimension", "adversarial", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("load sessions runtime state: %v", err)
	}
	sess, ok := state.Sessions["session-2"]
	if !ok {
		t.Fatalf("runtime state missing session-2: %#v", state.Sessions)
	}
	if sess.OwnerScope != "audit root cause" {
		t.Fatalf("session-2 owner scope = %q, want %q", sess.OwnerScope, "audit root cause")
	}
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-2"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-2 identity missing")
	}
	if got := identity.Dimensions; len(got) != 1 || got[0].Name != "adversarial" || got[0].Guidance != goalx.BuiltinDimensions["adversarial"] {
		t.Fatalf("session-2 dimensions = %#v, want resolved adversarial dimension", got)
	}
}

func TestAddCreatesDimensionsStateEvenWithoutDimensionOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
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
	if _, err := os.Stat(ControlDimensionsPath(runDir)); !os.IsNotExist(err) {
		t.Fatalf("dimensions state should start missing, stat err = %v", err)
	}

	if err := Add(repo, []string{"audit root cause", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	state, err := LoadDimensionsState(ControlDimensionsPath(runDir))
	if err != nil {
		t.Fatalf("LoadDimensionsState: %v", err)
	}
	if state == nil {
		t.Fatal("dimensions state missing after add")
	}
	if state.Version != 1 {
		t.Fatalf("dimensions version = %d, want 1", state.Version)
	}
}

func TestAddDoesNotPublishSessionRuntimeStateToCoordination(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
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
	coord, err := EnsureCoordinationState(runDir, "implement audit fixes")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{Scope: "existing semantic scope"}
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	if err := Add(repo, []string{"audit root cause", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	coord, err = LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		t.Fatalf("LoadCoordinationState: %v", err)
	}
	if _, ok := coord.Sessions["session-2"]; ok {
		t.Fatalf("coordination should not publish session runtime state: %+v", coord.Sessions["session-2"])
	}
	if got := coord.Sessions["session-1"].Scope; got != "existing semantic scope" {
		t.Fatalf("session-1 scope = %q, want existing semantic scope", got)
	}
}

func TestAddFailsWhenRunBudgetIsExhausted(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	cfg.Budget.MaxDuration = time.Second
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
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

	err = Add(repo, []string{"second direction", "--mode", "develop", "--run", runName})
	if err == nil {
		t.Fatal("Add error = nil, want budget exhausted")
	}
	for _, want := range []string{"budget exhausted", "max_duration=1s", "exhausted=true"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Add error = %q, want substring %q", err, want)
		}
	}
	if _, statErr := os.Stat(SessionIdentityPath(runDir, "session-2")); !os.IsNotExist(statErr) {
		t.Fatalf("session-2 identity should not be created after exhausted budget, stat err = %v", statErr)
	}
}

func TestAddPropagatesEngineToRenderedProtocol(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
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

	if err := Add(repo, []string{"second direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-2.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	if !strings.Contains(string(out), "You are running in Codex CLI.") {
		t.Fatalf("rendered protocol missing codex engine guidance:\n%s", string(out))
	}
}

func TestAddRendersSessionLocalValidationOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
  - hint: second
    mode: develop
    local_validation:
      command: "test -s report.md"
target:
  files: ["."]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.RemoveAll(filepath.Join(runDir, "sessions", "session-2")); err != nil {
		t.Fatalf("remove seeded session-2 identity: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"second direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-2.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "test -s report.md") {
		t.Fatalf("rendered protocol missing session local validation override:\n%s", text)
	}
	if strings.Contains(text, "go test ./...") {
		t.Fatalf("rendered protocol should not fall back to run local validation when session override exists:\n%s", text)
	}
}

func TestAddWorktreeUsesSessionBaseBranch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: gpt-5.4
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	session1WT := WorktreePath(runDir, runName, 1)
	if err := CreateWorktree(runWT, session1WT, "goalx/"+runName+"/1"); err != nil {
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
		Mode:         string(goalx.ModeDevelop),
		Branch:       "goalx/" + runName + "/1",
		WorktreePath: session1WT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	if err := Add(repo, []string{"follow up slice", "--mode", "develop", "--worktree", "--base-branch", "session-1", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(WorktreePath(runDir, runName, 2), "feature.txt"))
	if err != nil {
		t.Fatalf("read session-2 feature.txt: %v", err)
	}
	if string(data) != "from session 1\n" {
		t.Fatalf("session-2 feature.txt = %q, want session-1 branch contents", string(data))
	}
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-2"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-2 identity missing")
	}
	if identity.BaseBranchSelector != "session-1" {
		t.Fatalf("BaseBranchSelector = %q, want session-1", identity.BaseBranchSelector)
	}
	if identity.BaseBranch != "goalx/"+runName+"/1" {
		t.Fatalf("BaseBranch = %q, want %q", identity.BaseBranch, "goalx/"+runName+"/1")
	}
	parentIdentity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity session-1: %v", err)
	}
	if parentIdentity == nil {
		t.Fatal("session-1 identity missing")
	}
	if identity.BaseExperimentID != parentIdentity.ExperimentID {
		t.Fatalf("BaseExperimentID = %q, want %q", identity.BaseExperimentID, parentIdentity.ExperimentID)
	}
	if strings.TrimSpace(identity.ExperimentID) == "" {
		t.Fatal("session-2 ExperimentID empty")
	}
	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected experiment.created event for session-2")
	}
}

func TestAddWorktreeBaseBranchFailsForSharedSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: gpt-5.4
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "active",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	err := Add(repo, []string{"follow up slice", "--mode", "develop", "--worktree", "--base-branch", "session-1", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), "has no dedicated branch") {
		t.Fatalf("Add error = %v, want dedicated branch error", err)
	}
}

func TestAddWorktreeWithoutExplicitBaseRecordsRunRootParent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: gpt-5.4
parallel: 0
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}

	if err := Add(repo, []string{"new slice", "--mode", "develop", "--worktree", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	if identity.BaseBranchSelector != "run-root" {
		t.Fatalf("BaseBranchSelector = %q, want run-root", identity.BaseBranchSelector)
	}
	if identity.BaseBranch != "goalx/"+runName+"/root" {
		t.Fatalf("BaseBranch = %q, want %q", identity.BaseBranch, "goalx/"+runName+"/root")
	}
}

func TestAddRendersAcceptanceContractAndTeamContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
  command: "go test ./..."
acceptance:
  command: "go test -run E2E ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "acceptance.md"), []byte("- deploy succeeds\n- e2e passes\n"), 0o644); err != nil {
		t.Fatalf("write acceptance checklist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "acceptance.json"), []byte(`{"version":1,"goal_version":1,"default_command":"go test -run E2E ./...","effective_command":"go test -run E2E ./...","last_result":{}}`), 0o644); err != nil {
		t.Fatalf("write acceptance state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"second direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-2.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Team Context",
		"session-1",
		"session-2",
		"of 2 sessions",
		"acceptance.md",
		"acceptance.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestAddLaunchesSessionWithCurrentProcessEnvInExplicitWorktree(t *testing.T) {
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
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-test")
	t.Setenv("FOO_TOOLCHAIN_ROOT", "/opt/add-before")

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
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
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-after")
	t.Setenv("FOO_TOOLCHAIN_ROOT", "/opt/add-after")

	if err := Add(repo, []string{"--run", runName, "--worktree", "--mode", "research", "audit root cause"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"new-window -t " + goalx.TmuxSessionName(repo, runName) + " -n session-2 -c " + WorktreePath(runDir, runName, 2) + " env ",
		"/bin/bash -c ",
		"lease-loop --run",
		"--holder",
		"session-2",
		"FOO_TOOLCHAIN_ROOT='/opt/add-after'",
		"HOME='" + home + "'",
		"PATH='" + fakeBin + ":/tmp/goalx-bin:/usr/bin'",
		"ANTHROPIC_API_KEY='anthropic-after'",
		"claude --model claude-opus-4-6 --permission-mode auto",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("add launch log missing %q:\n%s", want, logText)
		}
	}
	if strings.Contains(logText, "--disable-slash-commands") {
		t.Fatalf("claude sessions should keep provider-native skills enabled:\n%s", logText)
	}
	if strings.Contains(logText, "send-keys -t "+goalx.TmuxSessionName(repo, runName)+":session-2") {
		t.Fatalf("add should launch session directly, not via send-keys:\n%s", logText)
	}
	if strings.Contains(logText, "ANTHROPIC_API_KEY='anthropic-test'") || strings.Contains(logText, "FOO_TOOLCHAIN_ROOT='/opt/add-before'") {
		t.Fatalf("add should use current process env, not a stored snapshot:\n%s", logText)
	}
}

func TestAddLaunchesSessionInRunWorktreeByDefault(t *testing.T) {
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
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
local_validation:
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

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", err)
	}
	sess, ok := state.Sessions["session-2"]
	if !ok {
		t.Fatalf("runtime state missing session-2: %#v", state.Sessions)
	}
	if sess.WorktreePath != "" {
		t.Fatalf("session-2 worktree path = %q, want empty", sess.WorktreePath)
	}
	if _, err := os.Stat(WorktreePath(runDir, runName, 2)); !os.IsNotExist(err) {
		t.Fatalf("session worktree should not exist, stat err = %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	want := "new-window -t " + goalx.TmuxSessionName(repo, runName) + " -n session-2 -c " + runWT + " env "
	if !strings.Contains(logText, want) {
		t.Fatalf("tmux log missing run worktree launch:\n%s", logText)
	}
}

func TestAddWithWorktreeCopiesGitignoredFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	writeAndCommit(t, repo, ".gitignore", "CLAUDE.md\n", "add ignore rules")
	writeTestFile(t, repo, "CLAUDE.md", "source root instructions\n")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	writeTestFile(t, runWT, "CLAUDE.md", "master customized instructions\n")
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"--run", runName, "--worktree", "--mode", "develop", "second direction"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(WorktreePath(runDir, runName, 2), "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read session CLAUDE.md: %v", err)
	}
	if string(data) != "master customized instructions\n" {
		t.Fatalf("session CLAUDE.md = %q, want run worktree customization", string(data))
	}
}

func TestAddNotifiesMasterViaInbox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: codex
parallel: 1
master:
  engine: codex
  model: gpt-5.4
sessions:
  - hint: first
    mode: develop
target:
  files: ["."]
local_validation:
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

	if err := Add(repo, []string{"second direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	sessionInbox, err := os.ReadFile(ControlInboxPath(runDir, "session-2"))
	if err != nil {
		t.Fatalf("read session inbox: %v", err)
	}
	sessionText := string(sessionInbox)
	for _, want := range []string{`"type":"develop"`, `"source":"master"`, `"body":"second direction"`} {
		if !strings.Contains(sessionText, want) {
			t.Fatalf("session inbox missing %q:\n%s", want, sessionText)
		}
	}

	inbox, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	text := string(inbox)
	for _, want := range []string{`"type":"session_added"`, `"source":"goalx add"`, `session-2`, `second direction`} {
		if !strings.Contains(text, want) {
			t.Fatalf("master inbox missing %q:\n%s", want, text)
		}
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "idle_prompt" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
	}
}

func TestAddStartsNumberingFromExistingRunArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: fast
parallel: 3
target:
  files: ["."]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))

	if err := Add(repo, []string{"first direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := Add(repo, []string{"second direction", "--mode", "develop", "--run", runName}); err != nil {
		t.Fatalf("Add second: %v", err)
	}

	for _, path := range []string{
		filepath.Join(runDir, "program-1.md"),
		filepath.Join(runDir, "program-2.md"),
		filepath.Join(runDir, "journals", "session-1.jsonl"),
		filepath.Join(runDir, "journals", "session-2.jsonl"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
	for _, path := range []string{
		filepath.Join(runDir, "program-4.md"),
		filepath.Join(runDir, "journals", "session-4.jsonl"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be absent, stat err = %v", path, err)
		}
	}
}

func TestAddSupportsResearchModeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"investigate failing auth flow", "--mode", "research", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-2"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-2 identity missing")
	}
	if identity.RoleKind != "research" || identity.Engine != "claude-code" || identity.Model != "opus" {
		t.Fatalf("session-2 identity = %+v", identity)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("load sessions runtime state: %v", err)
	}
	sess, ok := state.Sessions["session-2"]
	if !ok {
		t.Fatalf("runtime state missing session-2: %#v", state.Sessions)
	}
	if sess.Mode != string(goalx.ModeResearch) {
		t.Fatalf("session-2 mode = %q, want %q", sess.Mode, goalx.ModeResearch)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-2.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Mode: Research",
		"Research mode typically focuses on producing reports; code modification controlled by target config.",
		"## Native Helpers",
		"You are running in Claude Code.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestAddRoutesByModeAndEffortWhenEngineModelNotExplicit(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: auto
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
    effort: high
  develop:
    engine: codex
    model: gpt-5.4
parallel: 1
sessions:
  - hint: first
    mode: develop
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"investigate auth regression", "--mode", "research", "--effort", "high", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-2"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-2 identity missing")
	}
	if identity.Mode != string(goalx.ModeResearch) {
		t.Fatalf("mode = %q, want %q", identity.Mode, goalx.ModeResearch)
	}
	if identity.Engine != "claude-code" || identity.Model != "opus" {
		t.Fatalf("engine/model = %q/%q, want claude-code/opus", identity.Engine, identity.Model)
	}
	if identity.RequestedEffort != goalx.EffortHigh {
		t.Fatalf("requested_effort = %q, want high", identity.RequestedEffort)
	}
	if identity.EffectiveEffort == "" {
		t.Fatalf("effective_effort empty in %+v", identity)
	}
}

func TestAddExplicitEngineModelBypassesRoleDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: auto
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
parallel: 0
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))

	if err := Add(repo, []string{"investigate auth regression", "--mode", "research", "--engine", "codex", "--model", "gpt-5.4", "--effort", "high", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	if identity.Mode != string(goalx.ModeResearch) {
		t.Fatalf("mode = %q, want %q", identity.Mode, goalx.ModeResearch)
	}
	if identity.Engine != "codex" || identity.Model != "gpt-5.4" {
		t.Fatalf("engine/model = %q/%q, want codex/gpt-5.4", identity.Engine, identity.Model)
	}
	if identity.RequestedEffort != goalx.EffortHigh {
		t.Fatalf("requested_effort = %q, want high", identity.RequestedEffort)
	}
	if identity.EffectiveEffort == "" {
		t.Fatalf("effective_effort empty in %+v", identity)
	}
}

func TestAddRejectsUnknownFlag(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: gpt-5.4
parallel: 0
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))

	err := Add(repo, []string{"second direction", "--mode", "develop", "--unknown", "value", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), `unknown flag "--unknown"`) {
		t.Fatalf("Add error = %v, want unknown flag", err)
	}
	if _, statErr := os.Stat(SessionIdentityPath(runDir, "session-1")); !os.IsNotExist(statErr) {
		t.Fatalf("unexpected session identity created, stat err = %v", statErr)
	}
}

func TestAddRejectsRemovedRouteFlags(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"second direction", "--mode", "develop", "--route-role", "develop"},
		{"second direction", "--mode", "develop", "--route-profile", "missing-profile"},
	}
	for _, args := range tests {
		if err := Add(t.TempDir(), args); err == nil {
			t.Fatalf("Add(%#v) unexpectedly succeeded", args)
		}
	}
}

func TestAddRejectsExplicitEngineModelWithoutMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: auto
objective: implement audit fixes
roles:
  develop:
    engine: codex
    model: gpt-5.4
parallel: 0
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, _ := writeAddRunFixture(t, repo, string(snapshot))

	err := Add(repo, []string{"second direction", "--engine", "codex", "--model", "gpt-5.4", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), "--mode is required") {
		t.Fatalf("Add error = %v, want missing --mode", err)
	}
}

func TestAddRequiresDurableIdentityForExistingSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity: %v", err)
	}

	err := Add(repo, []string{"second direction", "--mode", "develop", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), "session identity") {
		t.Fatalf("Add error = %v, want missing session identity", err)
	}
}

func TestAddDoesNotLeaveSessionIdentityWhenLaunchFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 0
    ;;
  new-window)
    exit 1
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	err := Add(repo, []string{"first direction", "--mode", "develop", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), "create tmux window") {
		t.Fatalf("Add error = %v, want tmux window failure", err)
	}
	for _, path := range []string{
		SessionIdentityPath(runDir, "session-2"),
		filepath.Dir(SessionIdentityPath(runDir, "session-2")),
		JournalPath(runDir, "session-2"),
		ControlInboxPath(runDir, "session-2"),
		SessionCursorPath(runDir, "session-2"),
		filepath.Join(runDir, "program-2.md"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be absent after failed add, stat err = %v", path, statErr)
		}
	}
	state, stateErr := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if stateErr != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", stateErr)
	}
	if _, ok := state.Sessions["session-2"]; ok {
		t.Fatalf("session-2 runtime state should be removed after failed add: %#v", state.Sessions["session-2"])
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
			t.Fatalf("unexpected experiment.created for failed add: %+v", body)
		}
	}
}

func TestAddRollsBackWhenLaunchHandshakeStaysBlank(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 0
    ;;
  new-window)
    exit 0
    ;;
  capture-pane)
    case "$3" in
      *:session-2)
        exit 0
        ;;
      *)
        printf '❯ ready\n'
        exit 0
        ;;
    esac
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
  files: ["src/"]
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	err := Add(repo, []string{"blank launch", "--mode", "develop", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), "launch handshake") {
		t.Fatalf("Add error = %v, want launch handshake failure", err)
	}
	for _, path := range []string{
		SessionIdentityPath(runDir, "session-2"),
		filepath.Dir(SessionIdentityPath(runDir, "session-2")),
		JournalPath(runDir, "session-2"),
		ControlInboxPath(runDir, "session-2"),
		SessionCursorPath(runDir, "session-2"),
		filepath.Join(runDir, "program-2.md"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be absent after failed add handshake, stat err = %v", path, statErr)
		}
	}
	state, stateErr := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if stateErr != nil {
		t.Fatalf("LoadSessionsRuntimeState: %v", stateErr)
	}
	if _, ok := state.Sessions["session-2"]; ok {
		t.Fatalf("session-2 runtime state should be removed after failed add handshake: %#v", state.Sessions["session-2"])
	}
	coord, coordErr := LoadCoordinationState(CoordinationPath(runDir))
	if coordErr != nil {
		t.Fatalf("LoadCoordinationState: %v", coordErr)
	}
	if _, ok := coord.Sessions["session-2"]; ok {
		t.Fatalf("coordination should not retain session-2 after failed add handshake: %+v", coord.Sessions["session-2"])
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
			t.Fatalf("unexpected experiment.created for failed add handshake: %+v", body)
		}
	}
}

func TestAddResearchModeOverrideUsesResearchRoleWithoutExplicitSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
parallel: 1
target:
  files: ["src/"]
local_validation:
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
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	effective := goalx.EffectiveSessionConfig(cfg, 0)
	if effective.Mode == "" {
		effective.Mode = cfg.Mode
	}
	if effective.Engine == "" || effective.Model == "" {
		roleDefaults := goalx.SessionConfig{}
		switch effective.Mode {
		case goalx.ModeResearch:
			roleDefaults = cfg.Roles.Research
		case goalx.ModeDevelop:
			roleDefaults = cfg.Roles.Develop
		}
		if effective.Engine == "" {
			effective.Engine = roleDefaults.Engine
		}
		if effective.Model == "" {
			effective.Model = roleDefaults.Model
		}
		if effective.Effort == "" {
			effective.Effort = roleDefaults.Effort
		}
	}
	identity, err := NewSessionIdentity(
		runDir,
		"session-1",
		sessionRoleKind(effective.Mode),
		effective.Mode,
		effective.Engine,
		effective.Model,
		effective.Effort,
		"",
		"",
		*effective.Target,
	)
	if err != nil {
		t.Fatalf("NewSessionIdentity session-1: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity session-1: %v", err)
	}

	if err := Add(repo, []string{"audit root cause", "--mode", "research", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("load sessions runtime state: %v", err)
	}
	sess, ok := state.Sessions["session-2"]
	if !ok {
		t.Fatalf("runtime state missing session-2: %#v", state.Sessions)
	}
	if sess.Mode != string(goalx.ModeResearch) {
		t.Fatalf("session-2 mode = %q, want %q", sess.Mode, goalx.ModeResearch)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-2.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "You are running in Claude Code.") {
		t.Fatalf("rendered protocol missing claude research engine guidance:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(runWT, ".claude", "hooks.json")); err != nil {
		t.Fatalf("expected claude adapter hook for session-2: %v", err)
	}
}

func TestAddHelpDoesNotCreateSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	writeReadyTmuxScript(t, tmuxPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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
local_validation:
  command: "go test ./..."
`)
	runName, runDir := writeAddRunFixture(t, repo, string(snapshot))
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Add(repo, []string{"--help", "--run", runName}); err != nil {
			t.Fatalf("Add --help: %v", err)
		}
	})
	if !strings.Contains(out, addUsage) {
		t.Fatalf("Add --help output = %q, want usage %q", out, addUsage)
	}

	for _, path := range []string{
		filepath.Join(runDir, "program-2.md"),
		filepath.Join(runDir, "journals", "session-2.jsonl"),
		ControlInboxPath(runDir, "session-2"),
		SessionCursorPath(runDir, "session-2"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("expected %s to be absent after --help, stat err = %v", path, statErr)
		}
	}
}

func writeAddRunFixture(t *testing.T, repo, snapshot string) (string, string) {
	t.Helper()

	runName := "add-run"
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
	if err := os.WriteFile(RunSpecPath(runDir), []byte(snapshot), 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if cfg.Master.Engine == "" {
		cfg.Master.Engine = "codex"
	}
	if cfg.Master.Model == "" {
		cfg.Master.Model = "gpt-5.4"
	}
	if cfg.Roles.Research.Engine == "" {
		cfg.Roles.Research = goalx.SessionConfig{Engine: "claude-code", Model: "opus"}
	}
	if cfg.Roles.Develop.Engine == "" {
		cfg.Roles.Develop = goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"}
	}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
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
	if _, err := EnsureAcceptanceState(runDir, cfg, goalState.Version); err != nil {
		t.Fatalf("EnsureAcceptanceState: %v", err)
	}
	charter, err := NewRunCharter(runDir, cfg.Name, cfg.Objective, meta)
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
	if _, err := EnsureCoordinationState(runDir, cfg.Objective); err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	fence, err := NewIdentityFence(runDir, meta)
	if err != nil {
		t.Fatalf("NewIdentityFence: %v", err)
	}
	if err := SaveIdentityFence(IdentityFencePath(runDir), fence); err != nil {
		t.Fatalf("SaveIdentityFence: %v", err)
	}
	for i := range cfg.Sessions {
		effective := goalx.EffectiveSessionConfig(cfg, i)
		if effective.Mode == "" {
			effective.Mode = cfg.Mode
		}
		if effective.Engine == "" || effective.Model == "" {
			roleDefaults := goalx.SessionConfig{}
			switch effective.Mode {
			case goalx.ModeResearch:
				roleDefaults = cfg.Roles.Research
			case goalx.ModeDevelop:
				roleDefaults = cfg.Roles.Develop
			}
			if effective.Engine == "" {
				effective.Engine = roleDefaults.Engine
			}
			if effective.Model == "" {
				effective.Model = roleDefaults.Model
			}
			if effective.Effort == "" {
				effective.Effort = roleDefaults.Effort
			}
		}
		identity, err := NewSessionIdentity(
			runDir,
			SessionName(i+1),
			sessionRoleKind(effective.Mode),
			effective.Mode,
			effective.Engine,
			effective.Model,
			effective.Effort,
			"",
			"",
			*effective.Target,
		)
		if err != nil {
			t.Fatalf("NewSessionIdentity %s: %v", SessionName(i+1), err)
		}
		identity.LocalValidationCommand = resolveSessionLocalValidationCommand(effective)
		if err := SaveSessionIdentity(SessionIdentityPath(runDir, SessionName(i+1)), identity); err != nil {
			t.Fatalf("SaveSessionIdentity %s: %v", SessionName(i+1), err)
		}
	}
	return runName, runDir
}

func writeReadyTmuxScript(t *testing.T, tmuxPath string) {
	t.Helper()

	script := `#!/bin/sh
case "$1" in
  has-session)
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
}
