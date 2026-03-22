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
		filepath.Join(runDir, "guidance"),
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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
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

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(runDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("load updated snapshot: %v", err)
	}
	if len(cfg.Sessions) != 3 {
		t.Fatalf("len(Sessions) = %d, want 3", len(cfg.Sessions))
	}
	if cfg.Sessions[2].Hint != "third direction" {
		t.Fatalf("Sessions[2].Hint = %q, want %q", cfg.Sessions[2].Hint, "third direction")
	}
	if cfg.Parallel != 1 {
		t.Fatalf("Parallel = %d, want 1 for explicit sessions config", cfg.Parallel)
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
		filepath.Join(runDir, "guidance"),
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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	if err := Add(repo, []string{"--strategy", "adversarial", "--run", runName}); err != nil {
		t.Fatalf("Add: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(runDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("load updated snapshot: %v", err)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("len(Sessions) = %d, want 2", len(cfg.Sessions))
	}
	want := goalx.BuiltinStrategies["adversarial"]
	if cfg.Sessions[1].Hint != want {
		t.Fatalf("Sessions[1].Hint = %q, want %q", cfg.Sessions[1].Hint, want)
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
		filepath.Join(runDir, "guidance"),
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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
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
		filepath.Join(runDir, "guidance"),
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
acceptance:
  command: "go test -run E2E ./..."
`)
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
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
		filepath.Join(runDir, "guidance"),
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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
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
