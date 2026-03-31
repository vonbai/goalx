package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func installStartFakeTmux(t *testing.T) {
	t.Helper()

	binDir := t.TempDir()
	stateDir := t.TempDir()
	tmuxPath := filepath.Join(binDir, "tmux")
	script := `#!/bin/sh
set -eu
state="${GOALX_FAKE_TMUX_STATE:?}"
mkdir -p "$state"
cmd="$1"
shift
case "$cmd" in
  has-session)
    target="$2"
    if [ -f "$state/session_$target" ]; then
      exit 0
    fi
    exit 1
    ;;
  new-session)
    name=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-s" ]; then
        shift
        name="$1"
        break
      fi
      shift
    done
    : > "$state/session_$name"
    exit 0
    ;;
  kill-session)
    target="$2"
    rm -f "$state/session_$target"
    exit 0
    ;;
  list-windows|capture-pane|send-keys)
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
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)
}

func TestStartRequiresExplicitManualConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	writeRootConfigFixture(t, repo, goalx.Config{
		Name:            "manual-draft",
		Mode:            goalx.ModeWorker,
		Objective:       "audit auth flow",
		Target:          goalx.TargetConfig{Files: []string{"report.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
	})

	installStartFakeTmux(t)

	err := Start(repo, nil)
	if err == nil {
		t.Fatal("expected Start to require explicit manual config")
	}
	if !strings.Contains(err.Error(), "--config") {
		t.Fatalf("error = %v, want mention of --config", err)
	}
}

func TestStartWithExplicitManualConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	writeRootConfigFixture(t, repo, goalx.Config{
		Name:            "manual-draft",
		Mode:            goalx.ModeWorker,
		Objective:       "audit auth flow",
		Target:          goalx.TargetConfig{Files: []string{"report.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master:          goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	})

	installStartFakeTmux(t)
	_ = stubRuntimeSupervisor(t)

	draftPath := filepath.Join(repo, ".goalx", "goalx.yaml")
	if err := Start(repo, []string{"--config", draftPath}); err != nil {
		t.Fatalf("Start --config: %v", err)
	}

	runDir := goalx.RunDir(repo, "manual-draft")
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("stat run dir: %v", err)
	}
}

func TestStartHelpIncludesExplicitConfigPath(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Start(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Start --help: %v", err)
		}
	})

	if !strings.Contains(out, "goalx start --config PATH") {
		t.Fatalf("start help missing explicit config usage:\n%s", out)
	}
}

func TestStartWithExplicitManualConfigRequiresExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	installStartFakeTmux(t)

	err := Start(repo, []string{"--config", filepath.Join(repo, ".goalx", "missing.yaml")})
	if err == nil {
		t.Fatal("expected Start --config to fail for missing manual draft")
	}
	if !strings.Contains(err.Error(), "manual draft config not found") {
		t.Fatalf("error = %v, want missing manual draft error", err)
	}
}

func TestLoadManualDraftConfigAllowsUnsetTargetAndLocalValidation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	draftPath := filepath.Join(projectRoot, ".goalx", "goalx.yaml")
	if err := os.MkdirAll(filepath.Dir(draftPath), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	writeRootConfigFixture(t, projectRoot, goalx.Config{
		Name:      "preview-draft",
		Mode:      goalx.ModeWorker,
		Objective: "ship it",
	})

	cfg, _, err := LoadManualDraftConfig(projectRoot, draftPath)
	if err != nil {
		t.Fatalf("LoadManualDraftConfig: %v", err)
	}
	if len(cfg.Target.Files) != 0 {
		t.Fatalf("target.files = %#v, want unset target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "" {
		t.Fatalf("local_validation.command = %q, want unset local_validation", cfg.LocalValidation.Command)
	}
}

func TestLoadManualDraftConfigDraftContextReplacesSharedContextBeforeFiltering(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	sharedExternal := filepath.Join(t.TempDir(), "shared.md")
	if err := os.WriteFile(sharedExternal, []byte("shared"), 0o644); err != nil {
		t.Fatalf("write shared external context: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	projectCfg := fmt.Sprintf("context:\n  files: [%q]\nlocal_validation:\n  command: \"go test ./...\"\ntarget:\n  files: [\".\"]\n", sharedExternal)
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(projectCfg), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "README.md"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("write local README: %v", err)
	}
	writeRootConfigFixture(t, projectRoot, goalx.Config{
		Name:            "draft-context",
		Mode:            goalx.ModeWorker,
		Objective:       "ship it",
		Target:          goalx.TargetConfig{Files: []string{"."}},
		LocalValidation: goalx.LocalValidationConfig{Command: "go test ./..."},
		Context:         goalx.ContextConfig{Files: []string{"README.md"}},
	})

	cfg, _, err := LoadManualDraftConfig(projectRoot, filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadManualDraftConfig: %v", err)
	}
	if len(cfg.Context.Files) != 0 {
		t.Fatalf("context.files = %#v, want local draft context to replace shared external context before filtering", cfg.Context.Files)
	}
}
