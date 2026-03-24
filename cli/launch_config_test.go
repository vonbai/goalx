package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

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
