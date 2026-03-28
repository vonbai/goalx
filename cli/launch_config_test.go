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
preset: hybrid
parallel: 4
master:
  engine: codex
  model: best
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeResearch,
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
		Mode:      goalx.ModeResearch,
		Parallel:  2,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if cfg.Parallel != 2 {
		t.Fatalf("parallel = %d, want 2", cfg.Parallel)
	}
}

func TestBuildLaunchConfigResearchDoesNotHardcodeReportDefaults(t *testing.T) {
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
		Mode:      goalx.ModeResearch,
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if len(cfg.Target.Readonly) != 0 {
		t.Fatalf("research launch should not hardcode readonly target, got %#v", cfg.Target.Readonly)
	}
	if len(cfg.Target.Files) == 1 && cfg.Target.Files[0] == "report.md" {
		t.Fatalf("research launch should not hardcode report.md target: %#v", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command == "test -s report.md && echo 'ok'" {
		t.Fatalf("research launch should not hardcode report local validation")
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
  research:
    effort: high
  develop:
    effort: medium
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
	if cfg.Roles.Research.Effort != goalx.EffortHigh {
		t.Fatalf("research effort = %q, want %q", cfg.Roles.Research.Effort, goalx.EffortHigh)
	}
	if cfg.Roles.Develop.Effort != goalx.EffortMedium {
		t.Fatalf("develop effort = %q, want %q", cfg.Roles.Develop.Effort, goalx.EffortMedium)
	}
}

func TestBuildLaunchConfigPreviewLeavesTargetAndLocalValidationUnsetWhenUnconfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()

	cfg, err := buildLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeDevelop,
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
		Mode:      goalx.ModeDevelop,
	})
	if err != nil {
		t.Fatalf("buildLaunchResolveRequest: %v", err)
	}
	if _, err := goalx.ResolveConfig(layers, req); err != nil {
		t.Fatalf("ResolveConfig: %v", err)
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
		Mode:      goalx.ModeDevelop,
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
		Mode:      goalx.ModeDevelop,
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

func TestResolveLaunchConfigResearchWithClaudePresetAndClaudeOnlyPath(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
preset: claude
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeLaunchPathWithCommands(t, "claude")
	t.Setenv("PATH", pathDir)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeResearch,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolvedCfg.Config.Mode != goalx.ModeResearch {
		t.Fatalf("mode = %q, want %q", resolvedCfg.Config.Mode, goalx.ModeResearch)
	}
}

func TestResolveLaunchConfigDevelopWithClaudePresetAndClaudeOnlyPathFailsOnMissingCodex(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
preset: claude
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	pathDir := makeLaunchPathWithCommands(t, "claude")
	t.Setenv("PATH", pathDir)

	_, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeDevelop,
	})
	if err == nil {
		t.Fatal("resolveLaunchConfig unexpectedly succeeded")
	}
	if !strings.Contains(err.Error(), "roles.develop") || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "required command") {
		t.Fatalf("resolveLaunchConfig error = %v, want missing codex command for develop path", err)
	}
}

func TestResolveLaunchConfigAutoWithHybridPresetAndClaudeOnlyPathFailsOnMissingCodex(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
preset: hybrid
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
	if !strings.Contains(err.Error(), "roles.develop") || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "required command") {
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
		Mode:      goalx.ModeDevelop,
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
		Mode:      goalx.ModeDevelop,
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

func TestBuildLaunchConfigResearchAndDevelopMatchResolverDefaults(t *testing.T) {
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

	tests := []struct {
		name string
		mode goalx.Mode
	}{
		{name: "research", mode: goalx.ModeResearch},
		{name: "develop", mode: goalx.ModeDevelop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
				Objective: "ship it",
				Mode:      tt.mode,
			})
			if err != nil {
				t.Fatalf("resolveLaunchConfig: %v", err)
			}
			resolved, err := goalx.ResolveConfig(layers, goalx.ResolveRequest{
				Objective: "ship it",
				Mode:      tt.mode,
			})
			if err != nil {
				t.Fatalf("ResolveConfig: %v", err)
			}
			cfg := resolvedCfg.Config
			if cfg.Master.Engine != resolved.Config.Master.Engine || cfg.Master.Model != resolved.Config.Master.Model {
				t.Fatalf("master = %s/%s, want %s/%s", cfg.Master.Engine, cfg.Master.Model, resolved.Config.Master.Engine, resolved.Config.Master.Model)
			}
			if cfg.Roles.Research.Engine != resolved.Config.Roles.Research.Engine || cfg.Roles.Research.Model != resolved.Config.Roles.Research.Model {
				t.Fatalf("research = %s/%s, want %s/%s", cfg.Roles.Research.Engine, cfg.Roles.Research.Model, resolved.Config.Roles.Research.Engine, resolved.Config.Roles.Research.Model)
			}
			if cfg.Roles.Develop.Engine != resolved.Config.Roles.Develop.Engine || cfg.Roles.Develop.Model != resolved.Config.Roles.Develop.Model {
				t.Fatalf("develop = %s/%s, want %s/%s", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model, resolved.Config.Roles.Develop.Engine, resolved.Config.Roles.Develop.Model)
			}
		})
	}
}

func TestResolveLaunchConfigIgnoresSharedSessionsForDirectLaunch(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
preset: codex
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
		Mode:      goalx.ModeDevelop,
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
preset: codex
target:
  files: ["."]
local_validation:
  timeout: 30s
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeDevelop,
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
preset: hybrid
parallel: 4
master:
  engine: codex
  model: best
roles:
  research:
    engine: claude-code
    model: opus
  develop:
    engine: codex
    model: fast
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeResearch,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	if resolvedCfg.Config.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", resolvedCfg.Config.Parallel)
	}
}

func TestResolveLaunchConfigResearchDoesNotHardcodeReportDefaults(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "audit auth",
		Mode:      goalx.ModeResearch,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	cfg := resolvedCfg.Config
	if len(cfg.Target.Readonly) != 0 {
		t.Fatalf("research launch should not hardcode readonly target, got %#v", cfg.Target.Readonly)
	}
	if len(cfg.Target.Files) == 1 && cfg.Target.Files[0] == "report.md" {
		t.Fatalf("research launch should not hardcode report.md target: %#v", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command == "test -s report.md && echo 'ok'" {
		t.Fatalf("research launch should not hardcode report local validation")
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
  research:
    effort: high
  develop:
    effort: medium
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
	if cfg.Roles.Research.Effort != goalx.EffortHigh {
		t.Fatalf("research effort = %q, want %q", cfg.Roles.Research.Effort, goalx.EffortHigh)
	}
	if cfg.Roles.Develop.Effort != goalx.EffortMedium {
		t.Fatalf("develop effort = %q, want %q", cfg.Roles.Develop.Effort, goalx.EffortMedium)
	}
}

func TestResolveLaunchConfigDimensionsDoNotIncreaseParallel(t *testing.T) {
	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
preset: codex
parallel: 1
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective:  "audit auth",
		Mode:       goalx.ModeDevelop,
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
preset: codex
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	_, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeDevelop,
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
preset: hybrid
parallel: 2
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

	if buildCfg.Preset != resolvedCfg.Config.Preset {
		t.Fatalf("preset = %q, want %q", buildCfg.Preset, resolvedCfg.Config.Preset)
	}
	if buildCfg.Master.Engine != resolvedCfg.Config.Master.Engine || buildCfg.Master.Model != resolvedCfg.Config.Master.Model {
		t.Fatalf("master = %s/%s, want %s/%s", buildCfg.Master.Engine, buildCfg.Master.Model, resolvedCfg.Config.Master.Engine, resolvedCfg.Config.Master.Model)
	}
	if buildCfg.Roles.Research.Engine != resolvedCfg.Config.Roles.Research.Engine || buildCfg.Roles.Research.Model != resolvedCfg.Config.Roles.Research.Model {
		t.Fatalf("research = %s/%s, want %s/%s", buildCfg.Roles.Research.Engine, buildCfg.Roles.Research.Model, resolvedCfg.Config.Roles.Research.Engine, resolvedCfg.Config.Roles.Research.Model)
	}
	if buildCfg.Roles.Develop.Engine != resolvedCfg.Config.Roles.Develop.Engine || buildCfg.Roles.Develop.Model != resolvedCfg.Config.Roles.Develop.Model {
		t.Fatalf("develop = %s/%s, want %s/%s", buildCfg.Roles.Develop.Engine, buildCfg.Roles.Develop.Model, resolvedCfg.Config.Roles.Develop.Engine, resolvedCfg.Config.Roles.Develop.Model)
	}
}
