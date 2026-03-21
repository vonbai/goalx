package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestImplementPreservesSavedMasterConfig(t *testing.T) {
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

	if err := Implement(projectRoot, nil, nil); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "codex" {
		t.Fatalf("master = %s/%s, want codex/codex", cfg.Master.Engine, cfg.Master.Model)
	}
}

func TestImplementAppliesNextConfigOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("target:\n  files: [cli/]\nharness:\n  command: go test ./...\n"), 0o644); err != nil {
		t.Fatalf("write base config: %v", err)
	}
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Preset:    "claude",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	nc := &nextConfigJSON{
		Parallel:       4,
		Engine:         "codex",
		Model:          "fast",
		DiversityHints: []string{"P0", "P1", "P2", "verification"},
		BudgetSeconds:  1200,
		Objective:      "custom implement objective",
	}
	if err := Implement(projectRoot, nil, nc); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", cfg.Parallel)
	}
	if cfg.Engine != "codex" || cfg.Model != "fast" {
		t.Fatalf("engine/model = %s/%s, want codex/fast", cfg.Engine, cfg.Model)
	}
	if cfg.Objective != "custom implement objective" {
		t.Fatalf("objective = %q, want custom implement objective", cfg.Objective)
	}
	if cfg.Budget.MaxDuration != 20*60*1_000_000_000 {
		t.Fatalf("budget = %v, want 20m", cfg.Budget.MaxDuration)
	}
	if len(cfg.DiversityHints) != 4 || cfg.DiversityHints[3] != "verification" {
		t.Fatalf("diversity_hints = %#v, want next_config values", cfg.DiversityHints)
	}
}

func TestImplementResolvesNextConfigStrategiesIntoHints(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Preset:    "claude",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	nc := &nextConfigJSON{
		Parallel:       3,
		Strategies:     []string{"depth", "adversarial"},
		DiversityHints: []string{"verification"},
	}
	if err := Implement(projectRoot, nil, nc); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	wantHints := []string{
		goalx.BuiltinStrategies["depth"],
		goalx.BuiltinStrategies["adversarial"],
		"verification",
	}
	if len(cfg.DiversityHints) != len(wantHints) {
		t.Fatalf("diversity_hints = %#v, want %#v", cfg.DiversityHints, wantHints)
	}
	for i := range wantHints {
		if cfg.DiversityHints[i] != wantHints[i] {
			t.Fatalf("diversity_hints[%d] = %q, want %q", i, cfg.DiversityHints[i], wantHints[i])
		}
	}
}

func TestImplementAppliesNextConfigPreset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Preset:    "claude",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Implement(projectRoot, nil, &nextConfigJSON{Preset: "claude-h"}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Preset != "claude-h" {
		t.Fatalf("preset = %q, want claude-h", cfg.Preset)
	}
	if cfg.Engine != "claude-code" || cfg.Model != "opus" {
		t.Fatalf("engine/model = %s/%s, want claude-code/opus", cfg.Engine, cfg.Model)
	}
}
