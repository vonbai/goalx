package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestAddExtendsExplicitSessionsSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: fast
parallel: 1
sessions:
  - hint: first
  - hint: second
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	for _, name := range []string{"session-1.jsonl", "session-2.jsonl"} {
		if err := os.WriteFile(filepath.Join(runDir, "journals", name), nil, 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}

	if err := Add(repo, []string{"third direction", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
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

func TestAddUsesBuiltinStrategyAsHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: fast
parallel: 1
sessions:
  - hint: first
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"--strategy", "adversarial", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	want := goalx.BuiltinStrategies["adversarial"]
	state, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("load sessions runtime state: %v", err)
	}
	sess, ok := state.Sessions["session-2"]
	if !ok {
		t.Fatalf("runtime state missing session-2: %#v", state.Sessions)
	}
	if sess.OwnerScope != want {
		t.Fatalf("session-2 owner scope = %q, want %q", sess.OwnerScope, want)
	}
}

func TestAddPropagatesEngineToRenderedProtocol(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
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
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"second direction", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-2.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	if !strings.Contains(string(out), "You are running in Codex CLI with file system access and shell execution.") {
		t.Fatalf("rendered protocol missing codex engine guidance:\n%s", string(out))
	}
}

func TestAddRendersAcceptanceContractAndTeamContext(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
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
target:
  files: ["."]
harness:
  command: "go test ./..."
acceptance:
  command: "go test -run E2E ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "acceptance.md"), []byte("- deploy succeeds\n- e2e passes\n"), 0o644); err != nil {
		t.Fatalf("write acceptance checklist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "acceptance.json"), []byte(`{"status":"pending","command":"go test -run E2E ./..."}`), 0o644); err != nil {
		t.Fatalf("write acceptance state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"second direction", "--run", runName}); err != nil {
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

func TestAddLaunchesSessionWithRuntimeEnv(t *testing.T) {
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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
roles:
  research:
    engine: claude-code
    model: opus
parallel: 1
sessions:
  - hint: first
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	if err := Add(repo, []string{"--run", runName, "--mode", "research", "audit root cause"}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"send-keys -t " + goalx.TmuxSessionName(repo, runName) + ":session-2 env ",
		"HOME='" + home + "'",
		"PATH='" + fakeBin + ":/tmp/goalx-bin:/usr/bin'",
		"ANTHROPIC_API_KEY='anthropic-test'",
		"claude --model claude-opus-4-6 --permission-mode auto --disable-slash-commands",
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("add launch log missing %q:\n%s", want, logText)
		}
	}
}

func TestAddNotifiesMasterViaInbox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
parallel: 1
master:
  engine: codex
  model: gpt-5.4
sessions:
  - hint: first
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"second direction", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
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
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "sent" {
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
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: fast
parallel: 3
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}

	if err := Add(repo, []string{"first direction", "--run", runName}); err != nil {
		t.Fatalf("Add first: %v", err)
	}
	if err := Add(repo, []string{"second direction", "--run", runName}); err != nil {
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
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
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
target:
  files: ["src/"]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"investigate failing auth flow", "--mode", "research", "--run", runName}); err != nil {
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
	for _, want := range []string{
		"## Mode: Research",
		"DO NOT modify any source code.",
		"`report.md`",
		"Agent tool",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
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
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
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
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
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
	if !strings.Contains(text, "You are running in Claude Code with access to:") {
		t.Fatalf("rendered protocol missing claude research engine guidance:\n%s", text)
	}
	if _, err := os.Stat(filepath.Join(runDir, "worktrees", "add-run-2", ".claude", "hooks.json")); err != nil {
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
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

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

	snapshot := []byte(`name: add-run
mode: develop
objective: implement audit fixes
engine: codex
model: codex
parallel: 1
sessions:
  - hint: first
target:
  files: ["."]
harness:
  command: "go test ./..."
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
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
