package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestDebatePreservesSavedMasterConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
		Parallel:  2,
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "codex",
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, nil); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "codex" {
		t.Fatalf("master = %s/%s, want codex/codex", cfg.Master.Engine, cfg.Master.Model)
	}
}

func TestDebateAppliesNextConfigOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	nc := &nextConfigJSON{
		Parallel:       3,
		Engine:         "codex",
		Model:          "fast",
		DiversityHints: []string{"angle A", "angle B", "angle C"},
		BudgetSeconds:  900,
		Objective:      "custom debate objective",
	}
	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, nc); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Parallel != 3 {
		t.Fatalf("parallel = %d, want 3", cfg.Parallel)
	}
	if cfg.Engine != "codex" || cfg.Model != "fast" {
		t.Fatalf("engine/model = %s/%s, want codex/fast", cfg.Engine, cfg.Model)
	}
	if cfg.Objective != "custom debate objective" {
		t.Fatalf("objective = %q, want custom debate objective", cfg.Objective)
	}
	if cfg.Budget.MaxDuration != 15*60*1_000_000_000 {
		t.Fatalf("budget = %v, want 15m", cfg.Budget.MaxDuration)
	}
	if len(cfg.DiversityHints) != 3 || cfg.DiversityHints[2] != "angle C" {
		t.Fatalf("diversity_hints = %#v, want next_config values", cfg.DiversityHints)
	}
}

func TestDebateInheritsSavedParallelWithoutOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
		Parallel:  5,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, nil); err != nil {
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

func TestDebateAppliesNextConfigPreset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "codex",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, &nextConfigJSON{Preset: "claude"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Preset != "claude" {
		t.Fatalf("preset = %q, want claude", cfg.Preset)
	}
	if cfg.Engine != "claude-code" || cfg.Model != "sonnet" {
		t.Fatalf("engine/model = %s/%s, want claude-code/sonnet", cfg.Engine, cfg.Model)
	}
}

func TestDebateUsesSavedManifestReportArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
		Parallel:  1,
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
				Mode: string(goalx.ModeResearch),
				Artifacts: []ArtifactMeta{
					{Kind: "report", Path: reportPath, RelPath: "custom-findings.txt", DurableName: "session-1-report.md"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveArtifacts: %v", err)
	}

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, nil); err != nil {
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
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
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

	if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, nil); err != nil {
		t.Fatalf("Debate: %v", err)
	}
}

func TestDebateHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Debate(t.TempDir(), []string{"--help"}, nil); err != nil {
			t.Fatalf("Debate --help: %v", err)
		}
	})

	if !strings.Contains(out, "usage: goalx debate --from RUN") {
		t.Fatalf("debate help missing usage:\n%s", out)
	}
	if !strings.Contains(out, "--write-config") {
		t.Fatalf("debate help missing write-config note:\n%s", out)
	}
}

func TestDebateStartCreatesFreshCharterWithPreservedRootLineage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")
	sourceMeta, sourceCharter := writeSavedPhaseSourceFixture(t, projectRoot, "research-a", "research")
	installPhaseStartFakeTmux(t)
	stubLaunchRunSidecar(t)

	if err := Debate(projectRoot, []string{"--from", "research-a"}, nil); err != nil {
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
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	out := captureStdout(t, func() {
		if err := Debate(projectRoot, []string{"--from", "research-a", "--write-config"}, nil); err != nil {
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
