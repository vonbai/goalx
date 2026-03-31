package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestDebateUsesSharedMasterConfigInsteadOfSavedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
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
	writeResolvedSavedRunFixture(t, projectRoot, "research-a", launchOptions{
		Objective: "audit auth flow",
		Mode:      goalx.ModeWorker,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeProjectConfigFixture(t, projectRoot, `
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

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Fatalf("master = %s/%s, want claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Worker.Engine != "claude-code" || cfg.Roles.Worker.Model != "sonnet" {
		t.Fatalf("research role = %s/%s, want claude-code/sonnet", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
}

func TestDebateUsesSavedSelectionSnapshotWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
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
	writeResolvedSavedRunFixture(t, projectRoot, "research-a", launchOptions{
		Objective: "audit auth flow",
		Mode:      goalx.ModeWorker,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeSelectionSnapshotFixture(t, SavedRunDir(projectRoot, "research-a"), testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterCandidates: []string{"claude-code/opus"},
			WorkerCandidates: []string{"claude-code/opus", "codex/gpt-5.4"},
		},
		Master: goalx.MasterConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
		Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
	})
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
target:
  files: ["cli/"]
local_validation:
  command: go test ./cli/...
`)

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Fatalf("master = %s/%s, want saved snapshot claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Worker.Engine != "claude-code" || cfg.Roles.Worker.Model != "opus" {
		t.Fatalf("research role = %s/%s, want saved snapshot claude-code/opus", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
	if cfg.Target.Files[0] != "cli/" {
		t.Fatalf("target.files = %#v, want current shared config target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "go test ./cli/..." {
		t.Fatalf("local_validation.command = %q, want current shared config command", cfg.LocalValidation.Command)
	}
}

func TestDebateExplicitPresetOverridesSavedSelectionSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
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
	writeResolvedSavedRunFixture(t, projectRoot, "research-a", launchOptions{
		Objective: "audit auth flow",
		Mode:      goalx.ModeWorker,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeSelectionSnapshotFixture(t, SavedRunDir(projectRoot, "research-a"), testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterCandidates: []string{"claude-code/opus"},
			WorkerCandidates: []string{"claude-code/opus"},
		},
		Master: goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
	})

	if err := Debate(projectRoot, []string{"--from", "research-a", "--master", "codex/gpt-5.4", "--worker", "codex/gpt-5.4", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want explicit preset codex/gpt-5.4", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Worker.Engine != "codex" || cfg.Roles.Worker.Model != "gpt-5.4" {
		t.Fatalf("research role = %s/%s, want codex/gpt-5.4", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
}

func TestDebateAppliesCanonicalPhaseOverridesFromCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
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
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Parallel:  2,
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Debate(projectRoot, []string{
		"--from", "research-a",
		"--parallel", "3",
		"--objective", "custom debate objective",
		"--dimension", "depth,adversarial,evidence",
		"--budget", "15m",
		"--write-config",
	}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Parallel != 3 {
		t.Fatalf("parallel = %d, want 3", cfg.Parallel)
	}
	if cfg.Roles.Worker.Engine != "claude-code" || cfg.Roles.Worker.Model != "sonnet" {
		t.Fatalf("worker role = %s/%s, want claude-code/sonnet", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
	if cfg.Objective != "custom debate objective" {
		t.Fatalf("objective = %q, want custom debate objective", cfg.Objective)
	}
	if cfg.Budget.MaxDuration != 15*60*1_000_000_000 {
		t.Fatalf("budget = %v, want 15m", cfg.Budget.MaxDuration)
	}
	if len(cfg.Sessions) != 3 {
		t.Fatalf("sessions = %#v, want 3 seeded sessions", cfg.Sessions)
	}
	for i, session := range cfg.Sessions {
		if got, want := session.Dimensions, []string{"depth", "adversarial", "evidence"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
			t.Fatalf("session[%d].dimensions = %#v, want %#v", i, got, want)
		}
	}
}

func TestDebateInheritsSavedParallelWithoutOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Parallel:  5,
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Parallel != 5 {
		t.Fatalf("parallel = %d, want 5", cfg.Parallel)
	}
}

func TestDebateInheritsSavedSessionFanoutWithoutParallel(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Sessions:  make([]goalx.SessionConfig, 4),
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", cfg.Parallel)
	}
	if len(cfg.Sessions) != 4 {
		t.Fatalf("sessions = %#v, want 4 inherited fan-out sessions", cfg.Sessions)
	}
}

func TestDebateUsesSavedManifestReportArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Parallel:  1,
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
	}, nil)
	runDir := SavedRunDir(projectRoot, "research-a")
	reportPath := filepath.Join(runDir, "custom-findings.txt")
	if err := os.WriteFile(reportPath, []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write custom report: %v", err)
	}
	if err := SaveArtifacts(filepath.Join(runDir, "artifacts.json"), &ArtifactsManifest{
		Run:     "research-a",
		Version: 1,
		Sessions: []SessionArtifacts{
			{
				Name: "session-1",
				Mode: string(goalx.ModeWorker),
				Artifacts: []ArtifactMeta{
					{Kind: "report", Path: reportPath, RelPath: "custom-findings.txt", DurableName: "session-1-report.md"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveArtifacts: %v", err)
	}

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	found := false
	for _, path := range cfg.Context.Files {
		if path == reportPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("context.files = %#v, want %q from artifacts manifest", cfg.Context.Files, reportPath)
	}
}

func TestDebateReadsLegacyProjectScopedSavedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runDir := LegacySavedRunDir(projectRoot, "research-a")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy saved run: %v", err)
	}
	cfg := goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "session-1-report.md"), []byte("# report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}
}

func TestDebateHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Debate(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Debate --help: %v", err)
		}
	})

	if !strings.Contains(out, "usage: goalx debate --from RUN") {
		t.Fatalf("debate help missing usage:\n%s", out)
	}
	if !strings.Contains(out, "--write-config") {
		t.Fatalf("debate help missing write-config note:\n%s", out)
	}
	if !strings.Contains(out, "--readonly") {
		t.Fatalf("debate help missing readonly note:\n%s", out)
	}
	if !strings.Contains(out, "saved run selection snapshot stays in effect unless you request an explicit CLI selection override") {
		t.Fatalf("debate help missing selection snapshot note:\n%s", out)
	}
}

func TestDebateStartCreatesFreshCharterWithPreservedRootLineage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")
	sourceMeta, sourceCharter := writeSavedPhaseSourceFixture(t, projectRoot, "research-a", "research")
	installPhaseStartFakeTmux(t)
	stubLaunchRunRuntimeHost(t)

	if err := Debate(projectRoot, []string{"--from", "research-a"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	assertPhaseRunLineage(t, projectRoot, derivePhaseRunName("research-a", "debate", ""), "debate", "research-a", sourceMeta, sourceCharter)
}

func TestPhaseWriteConfigUsesManualDraft(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	out := captureStdout(t, func() {
		if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}); err != nil {
			t.Fatalf("Debate --write-config: %v", err)
		}
	})

	if !strings.Contains(out, "manual draft") {
		t.Fatalf("write-config output should describe manual draft, got:\n%s", out)
	}
	if !strings.Contains(out, "goalx start --config .goalx/goalx.yaml") {
		t.Fatalf("write-config output should suggest explicit --config start, got:\n%s", out)
	}
}
