package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"github.com/vonbai/goalx/cli"
	"gopkg.in/yaml.v3"
)

func buildGoalxBinary(t *testing.T, home string) string {
	t.Helper()

	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "goalx-test")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = pkgDir
	build.Env = append(os.Environ(), "HOME="+home)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, string(out))
	}
	return binPath
}

func runGoalx(t *testing.T, binPath, home, projectRoot string, args ...string) string {
	t.Helper()

	cmd := exec.Command(binPath, args...)
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "HOME="+home)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goalx %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func writeSavedRunFixture(t *testing.T, projectRoot, runName string, cfg goalx.Config, files map[string]string) string {
	t.Helper()

	runDir := cli.SavedRunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal run spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run-spec.yaml"), data, 0o644); err != nil {
		t.Fatalf("write run-spec.yaml: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return runDir
}

func TestMainSupportsResultCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := buildGoalxBinary(t, home)

	projectRoot := t.TempDir()
	runDir := cli.SavedRunDir(projectRoot, "demo-run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name: "demo-run",
		Mode: goalx.ModeResearch,
		Target: goalx.TargetConfig{
			Files: []string{"report.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "run-spec.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("# summary from result\n"), 0o644); err != nil {
		t.Fatalf("write summary.md: %v", err)
	}

	out := runGoalx(t, binPath, home, projectRoot, "result", "demo-run")

	if !strings.Contains(out, "# summary from result") {
		t.Fatalf("result output missing summary:\n%s", out)
	}
}

func TestMainInitWritesPreviewManualDraftOnEmptyProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := buildGoalxBinary(t, home)

	projectRoot := t.TempDir()
	out := runGoalx(t, binPath, home, projectRoot, "init", "ship it", "--develop", "--name", "demo")
	if !strings.Contains(out, "Generated") {
		t.Fatalf("init output missing generated message:\n%s", out)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Name != "demo" {
		t.Fatalf("cfg.Name = %q, want demo", cfg.Name)
	}
	if len(cfg.Target.Files) != 0 {
		t.Fatalf("target.files = %#v, want unset target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "" {
		t.Fatalf("local_validation.command = %q, want unset local_validation", cfg.LocalValidation.Command)
	}
}

func TestMainDebateWriteConfigReResolvesFromSharedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	binPath := buildGoalxBinary(t, home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	projectCfg := `
target:
  files: ["src/"]
local_validation:
  command: "go test ./..."
master:
  engine: claude-code
  model: sonnet
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(projectCfg), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth",
		Preset:    "hybrid",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Research: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
			Develop:  goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target:          goalx.TargetConfig{Files: []string{"report.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "printf old\n"},
	}, map[string]string{
		"summary.md": "# research summary\n",
	})

	out := runGoalx(t, binPath, home, projectRoot, "run", "--from", "research-a", "--intent", "debate", "--write-config", "--preset", "codex", "--name", "debate-a")
	if !strings.Contains(out, "Generated manual draft") {
		t.Fatalf("debate output missing generated message:\n%s", out)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Name != "debate-a" {
		t.Fatalf("cfg.Name = %q, want debate-a", cfg.Name)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "sonnet" {
		t.Fatalf("master = %s/%s, want shared config claude-code/sonnet", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Research.Engine != "codex" || cfg.Roles.Research.Model != "gpt-5.4" {
		t.Fatalf("research = %s/%s, want codex/gpt-5.4", cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	}
	if len(cfg.Target.Files) != 1 || cfg.Target.Files[0] != "src/" {
		t.Fatalf("target.files = %#v, want shared config target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "go test ./..." {
		t.Fatalf("local_validation.command = %q, want shared config local validation", cfg.LocalValidation.Command)
	}
}

func TestRunCommandStopsActiveRunOnSignal(t *testing.T) {
	oldStart := mainStart
	oldStop := mainStop
	oldNotify := notifySignalsContext
	defer func() {
		mainStart = oldStart
		mainStop = oldStop
		notifySignalsContext = oldNotify
	}()

	started := make(chan struct{})
	release := make(chan struct{})
	stopCalls := 0
	mainStart = func(string, []string) error {
		close(started)
		<-release
		return nil
	}
	mainStop = func(string, []string) error {
		stopCalls++
		close(release)
		return nil
	}
	notifySignalsContext = func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(parent)
		go func() {
			<-started
			cancel()
		}()
		return ctx, func() {}
	}

	err := runCommand(t.TempDir(), "start", nil)
	if !errors.Is(err, errInterrupted) {
		t.Fatalf("runCommand error = %v, want errInterrupted", err)
	}
	if stopCalls != 1 {
		t.Fatalf("stop calls = %d, want 1", stopCalls)
	}
}

func TestRunCommandDispatchesRun(t *testing.T) {
	oldRun := mainRun
	defer func() { mainRun = oldRun }()

	called := false
	mainRun = func(_ string, args []string) error {
		called = true
		want := []string{"ship it", "--intent", "research"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("args = %v, want %v", args, want)
			}
		}
		return nil
	}

	if err := runCommand(t.TempDir(), "run", []string{"ship it", "--intent", "research"}); err != nil {
		t.Fatalf("runCommand run: %v", err)
	}
	if !called {
		t.Fatal("run command was not dispatched")
	}
}

func TestRunCommandDispatchesDurable(t *testing.T) {
	oldDurable := mainDurable
	defer func() { mainDurable = oldDurable }()
	called := false
	mainDurable = func(_ string, args []string) error {
		called = true
		want := []string{"replace", "status", "--run", "demo", "--file", "/tmp/status.json"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("args = %v, want %v", args, want)
			}
		}
		return nil
	}
	if err := runCommand(t.TempDir(), "durable", []string{"replace", "status", "--run", "demo", "--file", "/tmp/status.json"}); err != nil {
		t.Fatalf("runCommand durable: %v", err)
	}
	if !called {
		t.Fatal("durable dispatch was not called")
	}
}

func TestRunCommandRejectsLegacyTopLevelAliases(t *testing.T) {
	for _, legacy := range []string{"auto", "research", "develop", "debate", "implement", "explore"} {
		if err := runCommand(t.TempDir(), legacy, []string{"demo"}); !errors.Is(err, errUnknownCommand) {
			t.Fatalf("runCommand %s error = %v, want errUnknownCommand", legacy, err)
		}
	}
}

func TestRunCommandRejectsServe(t *testing.T) {
	if err := runCommand(t.TempDir(), "serve", nil); !errors.Is(err, errUnknownCommand) {
		t.Fatalf("runCommand serve error = %v, want errUnknownCommand", err)
	}
}

func TestRunCommandDispatchesSidecar(t *testing.T) {
	oldSidecar := mainSidecar
	defer func() { mainSidecar = oldSidecar }()

	called := false
	mainSidecar = func(string, []string) error {
		called = true
		return nil
	}

	if err := runCommand(t.TempDir(), "sidecar", []string{"--run", "demo"}); err != nil {
		t.Fatalf("runCommand sidecar: %v", err)
	}
	if !called {
		t.Fatal("sidecar command was not dispatched")
	}
}

func TestRunCommandDispatchesLeaseLoop(t *testing.T) {
	oldLeaseLoop := mainLeaseLoop
	defer func() { mainLeaseLoop = oldLeaseLoop }()

	called := false
	mainLeaseLoop = func(string, []string) error {
		called = true
		return nil
	}

	if err := runCommand(t.TempDir(), "lease-loop", []string{"--run", "demo", "--run-dir", "/tmp/run", "--holder", "master", "--run-id", "run_demo", "--epoch", "1", "--ttl-seconds", "30", "--transport", "tmux", "--pid", "123"}); err != nil {
		t.Fatalf("runCommand lease-loop: %v", err)
	}
	if !called {
		t.Fatal("lease-loop command was not dispatched")
	}
}

func TestRunCommandCanonicalizesRunWorktreeToSourceProjectRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sourceRoot := t.TempDir()
	runDir := goalx.RunDir(sourceRoot, "demo-run")
	worktreeRoot := filepath.Join(runDir, "worktrees", "root")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("mkdir worktree root: %v", err)
	}
	if err := cli.SaveRunMetadata(cli.RunMetadataPath(runDir), &cli.RunMetadata{
		Version:         1,
		ProjectRoot:     sourceRoot,
		ProtocolVersion: 2,
		RunID:           "run_demo",
		RootRunID:       "run_demo",
		Epoch:           1,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	oldContext := mainContext
	defer func() { mainContext = oldContext }()

	gotRoot := ""
	mainContext = func(projectRoot string, args []string) error {
		gotRoot = projectRoot
		return nil
	}

	if err := runCommand(filepath.Join(worktreeRoot, "src"), "context", []string{"--run", "demo-run"}); err != nil {
		t.Fatalf("runCommand context: %v", err)
	}
	if gotRoot != sourceRoot {
		t.Fatalf("project root = %q, want %q", gotRoot, sourceRoot)
	}
}

func TestRunCommandDispatchesWait(t *testing.T) {
	oldWait := mainWait
	defer func() { mainWait = oldWait }()

	called := false
	mainWait = func(string, []string) error {
		called = true
		return nil
	}

	if err := runCommand(t.TempDir(), "wait", []string{"--run", "demo", "--timeout", "30s"}); err != nil {
		t.Fatalf("runCommand wait: %v", err)
	}
	if !called {
		t.Fatal("wait command was not dispatched")
	}
}

func TestRunCommandDispatchesContext(t *testing.T) {
	oldContext := mainContext
	defer func() { mainContext = oldContext }()

	called := false
	mainContext = func(string, []string) error {
		called = true
		return nil
	}

	if err := runCommand(t.TempDir(), "context", []string{"--run", "demo"}); err != nil {
		t.Fatalf("runCommand context: %v", err)
	}
	if !called {
		t.Fatal("context command was not dispatched")
	}
}

func TestRunCommandDispatchesAfford(t *testing.T) {
	oldAfford := mainAfford
	defer func() { mainAfford = oldAfford }()

	called := false
	mainAfford = func(string, []string) error {
		called = true
		return nil
	}

	if err := runCommand(t.TempDir(), "afford", []string{"--run", "demo", "master"}); err != nil {
		t.Fatalf("runCommand afford: %v", err)
	}
	if !called {
		t.Fatal("afford command was not dispatched")
	}
}

func TestRunCommandSupportsDimension(t *testing.T) {
	if err := runCommand(t.TempDir(), "dimension", []string{"--help"}); err != nil {
		t.Fatalf("runCommand dimension --help: %v", err)
	}
}

func TestUsageDocumentsExplicitCrossProjectSelectors(t *testing.T) {
	if !strings.Contains(usage, "project-id/run") {
		t.Fatalf("usage missing project-id/run selector guidance:\n%s", usage)
	}
	if !strings.Contains(usage, "run_id") {
		t.Fatalf("usage missing run_id selector guidance:\n%s", usage)
	}
}

func TestUsageDocumentsDimensionCommand(t *testing.T) {
	if !strings.Contains(usage, "goalx dimension") {
		t.Fatalf("usage missing dimension command:\n%s", usage)
	}
}

func TestUsageDocumentsRunCommand(t *testing.T) {
	if !strings.Contains(usage, "goalx run") {
		t.Fatalf("usage missing run command:\n%s", usage)
	}
	if !strings.Contains(usage, "Primary goal entrypoint") {
		t.Fatalf("usage missing run contract:\n%s", usage)
	}
}
