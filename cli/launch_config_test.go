package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func writeLaunchConfigProjectFile(t *testing.T, projectRoot, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
}

func makeDetectedPresetPath(t *testing.T) string {
	t.Helper()
	binDir := t.TempDir()
	for _, name := range []string{"claude", "codex"} {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write %s shim: %v", name, err)
		}
	}
	return binDir
}

func makeLaunchPathWithCommands(t *testing.T, names ...string) string {
	t.Helper()
	binDir := t.TempDir()
	for _, name := range names {
		path := filepath.Join(binDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatalf("write %s shim: %v", name, err)
		}
	}
	return binDir
}

func TestBuildLaunchConfigPreservesConfiguredParallelWhenFlagOmitted(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfgYAML := `
parallel: 4
master:
  engine: codex
  model: best
roles:
  worker:
    engine: claude-code
    model: opus
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", cfg.Parallel)
	}
}

func TestBuildLaunchConfigOverridesParallelWhenFlagProvided(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("parallel: 4\n"), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
		Parallel:  2,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Parallel != 2 {
		t.Fatalf("parallel = %d, want 2", cfg.Parallel)
	}
}

func TestBuildLaunchConfigWorkerDoesNotHardcodeReportDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if len(cfg.Target.Readonly) != 0 {
		t.Fatalf("worker launch should not hardcode readonly target, got %#v", cfg.Target.Readonly)
	}
	if len(cfg.Target.Files) == 1 && cfg.Target.Files[0] == "report.md" {
		t.Fatalf("worker launch should not hardcode report.md target: %#v", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command == "test -s report.md && echo 'ok'" {
		t.Fatalf("worker launch should not hardcode report local validation")
	}
}

func TestBuildLaunchConfigAutoLeavesPreconfiguredSessionsModeUnset(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
		Subs:      []string{"codex/codex:2"},
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 preconfigured sessions", cfg.Sessions)
	}
	for i, sess := range cfg.Sessions {
		if sess.Mode != "" {
			t.Fatalf("session[%d].Mode = %q, want unset mode", i, sess.Mode)
		}
	}
}

func TestBuildLaunchConfigAutoPreservesConfiguredEffortDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfgYAML := `
master:
  effort: high
roles:
  worker:
    effort: high
local_validation:
  command: go test ./...
target:
  files: ["."]
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Mode != goalx.ModeAuto {
		t.Fatalf("cfg.Mode = %q, want %q", cfg.Mode, goalx.ModeAuto)
	}
	if cfg.Master.Effort != goalx.EffortHigh {
		t.Fatalf("master effort = %q, want %q", cfg.Master.Effort, goalx.EffortHigh)
	}
	if cfg.Roles.Worker.Effort != goalx.EffortHigh {
		t.Fatalf("worker effort = %q, want %q", cfg.Roles.Worker.Effort, goalx.EffortHigh)
	}
}

func TestBuildLaunchConfigSplitsContextFilesAndRefs(t *testing.T) {
	projectRoot := t.TempDir()
	readmePath := filepath.Join(projectRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("# demo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective:    "audit auth",
		Mode:         goalx.ModeWorker,
		ContextPaths: []string{"README.md", "https://example.com/spec", "ref:ticket-123", "note:follow the rejection path"},
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}

	if len(cfg.Context.Files) != 1 || cfg.Context.Files[0] != readmePath {
		t.Fatalf("context.files = %#v, want %#v", cfg.Context.Files, []string{readmePath})
	}
	if got, want := cfg.Context.Refs, []string{"https://example.com/spec", "ticket-123", "follow the rejection path"}; len(got) != len(want) || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("context.refs = %#v, want %#v", got, want)
	}
}

func TestBuildLaunchConfigPreservesContextRefWithCommas(t *testing.T) {
	projectRoot := t.TempDir()

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective:    "audit auth",
		Mode:         goalx.ModeWorker,
		ContextPaths: []string{"note:program centric, owner scoped, no demo drift"},
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}

	if len(cfg.Context.Files) != 0 {
		t.Fatalf("context.files = %#v, want none", cfg.Context.Files)
	}
	if got, want := cfg.Context.Refs, []string{"program centric, owner scoped, no demo drift"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("context.refs = %#v, want %#v", got, want)
	}
}

func TestBuildLaunchConfigAutoSuffixesConflictingGeneratedRunName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	existingRunDir := goalx.RunDir(projectRoot, goalx.Slugify("ship it"))
	if err := os.MkdirAll(existingRunDir, 0o755); err != nil {
		t.Fatalf("mkdir existing run dir: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Name != "ship-it-2" {
		t.Fatalf("cfg.Name = %q, want ship-it-2", cfg.Name)
	}
}

func TestBuildLaunchConfigSkipsToNextAvailableGeneratedRunName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	for _, name := range []string{"ship-it", "ship-it-2"} {
		if err := os.MkdirAll(goalx.RunDir(projectRoot, name), 0o755); err != nil {
			t.Fatalf("mkdir existing run dir %s: %v", name, err)
		}
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Name != "ship-it-3" {
		t.Fatalf("cfg.Name = %q, want ship-it-3", cfg.Name)
	}
}

func TestBuildLaunchConfigKeepsExplicitNameEvenWhenConflictExists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	existingRunDir := goalx.RunDir(projectRoot, "chosen-name")
	if err := os.MkdirAll(existingRunDir, 0o755); err != nil {
		t.Fatalf("mkdir existing run dir: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
		Name:      "chosen-name",
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Name != "chosen-name" {
		t.Fatalf("cfg.Name = %q, want chosen-name", cfg.Name)
	}
}

func TestBuildLaunchConfigPreviewLeavesTargetAndLocalValidationUnsetWhenUnconfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if len(cfg.Target.Files) != 0 {
		t.Fatalf("target.files = %#v, want unset target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "" {
		t.Fatalf("local_validation.command = %q, want unset local_validation", cfg.LocalValidation.Command)
	}

	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	req, err := buildLaunchResolveRequest(projectRoot, layers.Config, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("buildLaunchResolveRequest: %v", err)
	}
	if _, err := goalx.ResolveConfig(layers, req); err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
}

func TestBuildLaunchResolveRequestAppliesReadonlyTargetOverride(t *testing.T) {
	projectRoot := t.TempDir()

	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	req, err := buildLaunchResolveRequest(projectRoot, layers.Config, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
		Readonly:  true,
	})
	if err != nil {
		t.Fatalf("buildLaunchResolveRequest: %v", err)
	}
	if req.TargetOverride == nil {
		t.Fatal("TargetOverride = nil, want readonly override")
	}
	if got, want := req.TargetOverride.Readonly, []string{"."}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("target.readonly override = %#v, want %#v", got, want)
	}
}

func TestResolveLaunchConfigAppliesReadonlyOverride(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
target:
  files: ["cli/"]
local_validation:
  command: go test ./cli/...
`)

	resolved, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
		Readonly:  true,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if got, want := resolved.Config.Target.Readonly, []string{"."}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("target.readonly = %#v, want %#v", got, want)
	}
	if got, want := resolved.Config.Target.Files, []string{"cli/"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("target.files = %#v, want %#v", got, want)
	}
}

func TestResolveLaunchConfigPreservesConfiguredBudgetWhenFlagOmitted(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
budget:
  max_duration: 45m
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolved, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolved.Config.Budget.MaxDuration != 45*time.Minute {
		t.Fatalf("budget = %v, want 45m", resolved.Config.Budget.MaxDuration)
	}
}

func TestResolveLaunchConfigPreservesMemoryLLMExtractOff(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
memory:
  llm_extract: off
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolved, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolved.Config.Memory.LLMExtract != "off" {
		t.Fatalf("memory.llm_extract = %q, want off", resolved.Config.Memory.LLMExtract)
	}
	if err := goalx.ValidateConfig(&resolved.Config, resolved.Engines); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestResolveLaunchConfigWorkerWithClaudeOnlyPath(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: claude-code
    model: sonnet
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeLaunchPathWithCommands(t, "claude")
	t.Setenv("PATH", pathDir)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolvedCfg.Config.Mode != goalx.ModeWorker {
		t.Fatalf("mode = %q, want %q", resolvedCfg.Config.Mode, goalx.ModeWorker)
	}
}

func TestResolveLaunchConfigWorkerWithMissingConfiguredEngineFails(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: codex
    model: gpt-5.4
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeLaunchPathWithCommands(t, "claude")
	t.Setenv("PATH", pathDir)

	_, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
	})
	if err == nil {
		t.Fatal("resolveLaunchConfig unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "roles.worker") || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "required command") {
		t.Fatalf("resolveLaunchConfig error = %v, want missing codex command for worker path", err)
	}
}

func TestResolveLaunchConfigAutoWithHybridPresetAndClaudeOnlyPathFailsOnMissingCodex(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: codex
    model: gpt-5.4
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeLaunchPathWithCommands(t, "claude")
	t.Setenv("PATH", pathDir)

	_, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeAuto,
	})
	if err == nil {
		t.Fatal("resolveLaunchConfig unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "roles.worker") || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "required command") {
		t.Fatalf("resolveLaunchConfig error = %v, want missing codex command for auto path", err)
	}
}

func TestResolveLaunchConfigRejectsUnknownMemoryLLMExtractMode(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
memory:
  llm_extract: auto
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	_, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err == nil {
		t.Fatal("resolveLaunchConfig accepted unknown memory.llm_extract mode")
	}
	if got := err.Error(); got != `memory.llm_extract must be "off" when set, got "auto"` {
		t.Fatalf("error = %q, want strict memory.llm_extract validation", got)
	}
}

func TestResolveLaunchConfigLeavesBudgetUnlimitedByDefaultForNonEvolve(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolved, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolved.Config.Budget.MaxDuration != 0 {
		t.Fatalf("budget = %v, want unlimited (0)", resolved.Config.Budget.MaxDuration)
	}
}

func TestResolveLaunchConfigHonorsExplicitZeroBudgetForEvolveIntent(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolved, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
		Intent:    runIntentEvolve,
		BudgetSet: true,
		Budget:    0,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolved.Config.Budget.MaxDuration != 0 {
		t.Fatalf("budget = %v, want 0", resolved.Config.Budget.MaxDuration)
	}
}

func TestBuildLaunchConfigWorkerMatchesResolverDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
name: shared
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeDetectedPresetPath(t)
	t.Setenv("PATH", pathDir+":"+os.Getenv("PATH"))

	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	resolved, err := goalx.ResolveConfig(layers, goalx.ResolveRequest{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}
	cfg := resolvedCfg.Config
	if cfg.Master.Engine != resolved.Config.Master.Engine || cfg.Master.Model != resolved.Config.Master.Model {
		t.Fatalf("master = %s/%s, want %s/%s", cfg.Master.Engine, cfg.Master.Model, resolved.Config.Master.Engine, resolved.Config.Master.Model)
	}
	if cfg.Roles.Worker.Engine != resolved.Config.Roles.Worker.Engine || cfg.Roles.Worker.Model != resolved.Config.Roles.Worker.Model {
		t.Fatalf("worker = %s/%s, want %s/%s", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model, resolved.Config.Roles.Worker.Engine, resolved.Config.Roles.Worker.Model)
	}
}

func TestResolveLaunchConfigIgnoresSharedSessionsForDirectLaunch(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
roles:
  worker:
    engine: codex
    model: gpt-5.4
target:
  files: ["."]
local_validation:
  command: go test ./...
sessions:
  - engine: nope
    model: bad
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if len(resolvedCfg.Config.Sessions) != 0 {
		t.Fatalf("sessions = %#v, want direct launch to clear shared sessions", resolvedCfg.Config.Sessions)
	}
	if err := goalx.ValidateConfig(&resolvedCfg.Config, resolvedCfg.Engines); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestResolveLaunchConfigPreservesLocalValidationTimeoutWithoutInferringCommand(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
target:
  files: ["."]
local_validation:
  timeout: 30s
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolvedCfg.Config.LocalValidation.Command != "" {
		t.Fatalf("local_validation.command = %q, want empty", resolvedCfg.Config.LocalValidation.Command)
	}
	if resolvedCfg.Config.LocalValidation.Timeout != 30*time.Second {
		t.Fatalf("local_validation.timeout = %v, want %v", resolvedCfg.Config.LocalValidation.Timeout, 30*time.Second)
	}
}

func TestResolveLaunchConfigPreservesConfiguredParallelWhenFlagOmitted(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
parallel: 4
master:
  engine: codex
  model: best
roles:
  worker:
    engine: claude-code
    model: opus
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolvedCfg.Config.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", resolvedCfg.Config.Parallel)
	}
}

func TestResolveLaunchConfigWorkerDoesNotHardcodeReportDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeWorker,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	cfg := resolvedCfg.Config
	if len(cfg.Target.Readonly) != 0 {
		t.Fatalf("worker launch should not hardcode readonly target, got %#v", cfg.Target.Readonly)
	}
	if len(cfg.Target.Files) == 1 && cfg.Target.Files[0] == "report.md" {
		t.Fatalf("worker launch should not hardcode report.md target: %#v", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command == "test -s report.md && echo 'ok'" {
		t.Fatalf("worker launch should not hardcode report local validation")
	}
}

func TestResolveLaunchConfigAutoLeavesPreconfiguredSessionsModeUnset(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
		Subs:      []string{"codex/codex:2"},
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if len(resolvedCfg.Config.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 preconfigured sessions", resolvedCfg.Config.Sessions)
	}
	for i, sess := range resolvedCfg.Config.Sessions {
		if sess.Mode != "" {
			t.Fatalf("session[%d].Mode = %q, want unset mode", i, sess.Mode)
		}
	}
}

func TestResolveLaunchConfigAutoPreservesConfiguredEffortDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  effort: high
roles:
  worker:
    effort: high
local_validation:
  command: go test ./...
target:
  files: ["."]
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	cfg := resolvedCfg.Config
	if cfg.Mode != goalx.ModeAuto {
		t.Fatalf("cfg.Mode = %q, want %q", cfg.Mode, goalx.ModeAuto)
	}
	if cfg.Master.Effort != goalx.EffortHigh {
		t.Fatalf("master effort = %q, want %q", cfg.Master.Effort, goalx.EffortHigh)
	}
	if cfg.Roles.Worker.Effort != goalx.EffortHigh {
		t.Fatalf("worker effort = %q, want %q", cfg.Roles.Worker.Effort, goalx.EffortHigh)
	}
}

func TestResolveLaunchConfigDimensionsDoNotIncreaseParallel(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
parallel: 1
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective:  "audit auth",
		Mode:       goalx.ModeWorker,
		Dimensions: []string{"audit", "adversarial", "evidence"},
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolvedCfg.Config.Parallel != 1 {
		t.Fatalf("parallel = %d, want 1", resolvedCfg.Config.Parallel)
	}
}

func TestResolveLaunchConfigFailsWhenSelectedEngineUnavailable(t *testing.T) {
	projectRoot := t.TempDir()
	t.Setenv("PATH", t.TempDir())
	writeLaunchConfigProjectFile(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
roles:
  worker:
    engine: codex
    model: gpt-5.4
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	_, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeWorker,
	})
	if err == nil {
		t.Fatal("resolveLaunchConfig unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "required command") || !strings.Contains(err.Error(), "codex") {
		t.Fatalf("resolveLaunchConfig error = %v, want missing codex command", err)
	}
}

func TestBuildLaunchConfigMatchesResolveLaunchConfig(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
parallel: 2
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: codex
    model: gpt-5.4
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeDetectedPresetPath(t)
	t.Setenv("PATH", pathDir+":"+os.Getenv("PATH"))

	buildCfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeAuto,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}

	if buildCfg.Master.Engine != resolvedCfg.Config.Master.Engine || buildCfg.Master.Model != resolvedCfg.Config.Master.Model {
		t.Fatalf("master = %s/%s, want %s/%s", buildCfg.Master.Engine, buildCfg.Master.Model, resolvedCfg.Config.Master.Engine, resolvedCfg.Config.Master.Model)
	}
	if buildCfg.Roles.Worker.Engine != resolvedCfg.Config.Roles.Worker.Engine || buildCfg.Roles.Worker.Model != resolvedCfg.Config.Roles.Worker.Model {
		t.Fatalf("worker = %s/%s, want %s/%s", buildCfg.Roles.Worker.Engine, buildCfg.Roles.Worker.Model, resolvedCfg.Config.Roles.Worker.Engine, resolvedCfg.Config.Roles.Worker.Model)
	}
}
