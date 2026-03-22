package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
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

	if err := Debate(projectRoot, nil, nil); err != nil {
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
	if err := Debate(projectRoot, nil, nc); err != nil {
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

	if err := Debate(projectRoot, nil, nil); err != nil {
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

	if err := Debate(projectRoot, nil, &nextConfigJSON{Preset: "claude"}); err != nil {
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
	runDir := filepath.Join(projectRoot, ".goalx", "runs", "research-a")
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

	if err := Debate(projectRoot, nil, nil); err != nil {
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
