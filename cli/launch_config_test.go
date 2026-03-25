package cli

import (
	"os"
	"path/filepath"
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
	if cfg.Harness.Command == "test -s report.md && echo 'ok'" {
		t.Fatalf("research launch should not hardcode report harness")
	}
}

func TestBuildLaunchConfigAutoDefaultsPreconfiguredSessionsToDevelopMode(t *testing.T) {
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
		Subs:      []string{"codex/codex"},
		Auditor:   "codex/codex",
	})
	if err != nil {
		t.Fatalf("buildLaunchConfig: %v", err)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 preconfigured sessions", cfg.Sessions)
	}
	for i, sess := range cfg.Sessions {
		if sess.Mode != goalx.ModeDevelop {
			t.Fatalf("session[%d].Mode = %q, want %q", i, sess.Mode, goalx.ModeDevelop)
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
harness:
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

func TestBuildLaunchConfigResearchAndDevelopMatchResolverDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeLaunchConfigProjectFile(t, projectRoot, `
name: shared
target:
  files: ["."]
harness:
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
harness:
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

func TestResolveLaunchConfigPreservesHarnessTimeoutWhenInferringCommand(t *testing.T) {
	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	writeLaunchConfigProjectFile(t, projectRoot, `
preset: codex
target:
  files: ["."]
harness:
  timeout: 30s
`)

	resolvedCfg, err := resolveLaunchConfig(projectRoot, launchOptions{
		Objective: "ship it",
		Mode:      goalx.ModeDevelop,
	})
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	wantCommand := "go build ./... && go test ./... -count=1 && go vet ./..."
	if resolvedCfg.Config.Harness.Command != wantCommand {
		t.Fatalf("harness.command = %q, want %q", resolvedCfg.Config.Harness.Command, wantCommand)
	}
	if resolvedCfg.Config.Harness.Timeout != 30*time.Second {
		t.Fatalf("harness.timeout = %v, want %v", resolvedCfg.Config.Harness.Timeout, 30*time.Second)
	}
}
