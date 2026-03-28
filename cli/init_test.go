package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestInitDevelopUsesProjectConfigWhenAvailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte("target:\n  files: [\"README.md\"]\nlocal_validation:\n  command: \"test -f README.md\"\n")
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	if err := Init(projectRoot, []string{"ship it", "--develop", "--name", "demo"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if len(cfg.Target.Files) != 1 || cfg.Target.Files[0] != "README.md" {
		t.Fatalf("target.files = %#v, want [README.md]", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "test -f README.md" {
		t.Fatalf("local_validation.command = %q, want %q", cfg.LocalValidation.Command, "test -f README.md")
	}
}

func TestInitDevelopLeavesLocalValidationAndTargetUnsetWithoutProjectConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectRoot, "go.mod"), []byte("module example.com/demo\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	if err := Init(projectRoot, []string{"ship it", "--develop", "--name", "demo"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if len(cfg.Target.Files) != 0 {
		t.Fatalf("target.files = %#v, want empty", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "" {
		t.Fatalf("local_validation.command = %q, want empty", cfg.LocalValidation.Command)
	}
}

func TestInitAllowsDraftGenerationWithoutSupportedEngines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	projectRoot := t.TempDir()
	if err := Init(projectRoot, []string{"ship it", "--develop", "--name", "demo"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Name != "demo" {
		t.Fatalf("name = %q, want demo", cfg.Name)
	}
}

func TestInitResearchUsesResearchPresetDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte("master:\n  engine: claude-code\n  model: sonnet\nroles:\n  research:\n    engine: codex\n    model: gpt-5.4\n")
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	if err := Init(projectRoot, []string{"investigate it", "--research", "--name", "demo"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Roles.Research.Engine != "codex" || cfg.Roles.Research.Model != "gpt-5.4" {
		t.Fatalf("research role = %s/%s, want codex/gpt-5.4", cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	}
}

func TestInitManualDraftOmitsUserScopedSelectionBlock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
selection:
  master_candidates:
    - codex/gpt-5.4
  research_candidates:
    - claude-code/opus
  develop_candidates:
    - codex/gpt-5.4-mini
`) + "\n")
	if err := os.WriteFile(filepath.Join(userGoalxDir, "config.yaml"), userCfg, 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	pathDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pathDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write claude shim: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pathDir, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write codex shim: %v", err)
	}
	t.Setenv("PATH", pathDir)

	projectRoot := t.TempDir()
	if err := Init(projectRoot, []string{"ship it", "--develop", "--name", "demo"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("read goalx.yaml: %v", err)
	}
	if strings.Contains(string(raw), "selection:") {
		t.Fatalf("goalx.yaml should not persist user-scoped selection:\n%s", string(raw))
	}
}

func TestInitDoesNotHardcodeResearchLocalValidationToReportDotMD(t *testing.T) {
	data, err := os.ReadFile("init.go")
	if err != nil {
		t.Fatalf("read init.go: %v", err)
	}
	if strings.Contains(string(data), `test -s report.md && echo 'ok'`) {
		t.Fatalf("init.go still hardcodes the research local validation to report.md")
	}
}
