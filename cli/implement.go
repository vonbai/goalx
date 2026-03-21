package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func buildPhaseConfig(mode goalx.Mode, savedCfg goalx.Config, engines map[string]goalx.EngineConfig, nc *nextConfigJSON) goalx.Config {
	preset := savedCfg.Preset
	if preset == "" {
		preset = "claude"
	}

	defaultEngine := savedCfg.Engine
	defaultModel := savedCfg.Model
	if nc != nil && nc.Preset != "" {
		preset = nc.Preset
		defaults := goalx.Config{Preset: preset, Mode: mode}
		goalx.ApplyPreset(&defaults)
		defaultEngine = defaults.Engine
		defaultModel = defaults.Model
	} else {
		defaults := goalx.Config{
			Preset: preset,
			Mode:   mode,
			Engine: defaultEngine,
			Model:  defaultModel,
		}
		goalx.ApplyPreset(&defaults)
		defaultEngine = defaults.Engine
		defaultModel = defaults.Model
	}

	parallel := savedCfg.Parallel
	if parallel < 1 {
		parallel = 2
	}

	budget := savedCfg.Budget
	if budget.MaxDuration == 0 {
		budget.MaxDuration = 2 * time.Hour
	}

	engine, model := resolveNextEngineModel(engines, defaultEngine, defaultModel, nc)
	return goalx.Config{
		Mode:     mode,
		Preset:   preset,
		Engine:   engine,
		Model:    model,
		Parallel: nextConfigParallel(parallel, nc),
		Master:   savedCfg.Master,
		Budget: goalx.BudgetConfig{
			MaxDuration: nextConfigBudget(budget.MaxDuration, nc),
		},
	}
}

// Implement generates a goalx.yaml for a develop round based on prior research/debate consensus.
func Implement(projectRoot string, args []string, nc *nextConfigJSON) error {
	savesDir := filepath.Join(projectRoot, ".goalx", "runs")

	// Prefer debate run (saved as mode=research, name=debate), then any research run
	var run, runDir string
	debateDir := filepath.Join(savesDir, "debate")
	if debateCfg, err2 := goalx.LoadYAML[goalx.Config](filepath.Join(debateDir, "goalx.yaml")); err2 == nil && debateCfg.Mode == goalx.ModeResearch {
		run, runDir = "debate", debateDir
	} else {
		var err error
		run, runDir, err = findLatestSavedRun(savesDir, goalx.ModeResearch)
		if err != nil {
			return fmt.Errorf("no saved research or debate run found in .goalx/runs/: %w", err)
		}
	}

	// Collect context files (summary + reports, absolute paths)
	var contextFiles []string
	absRunDir, _ := filepath.Abs(runDir)
	entries, _ := os.ReadDir(runDir)
	for _, e := range entries {
		name := e.Name()
		if name == "summary.md" || strings.HasSuffix(name, "-report.md") {
			contextFiles = append(contextFiles, filepath.Join(absRunDir, name))
		}
	}
	if len(contextFiles) == 0 {
		return fmt.Errorf("no reports/summary found in %s", runDir)
	}

	// Load the base config to get harness from project config
	baseCfg, engines, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load base config: %w", err)
	}

	harness := baseCfg.Harness.Command
	if harness == "" {
		harness = InferHarness(projectRoot)
	}
	if harness == "" {
		harness = "echo 'no harness inferred - configure harness.command in .goalx/config.yaml'"
	}
	targetFiles := baseCfg.Target.Files
	if len(targetFiles) == 0 {
		targetFiles = InferTarget(projectRoot)
	}

	// Read saved config for objective context
	savedCfg, _ := goalx.LoadYAML[goalx.Config](filepath.Join(runDir, "goalx.yaml"))
	objContext := savedCfg.Objective
	if objContext == "" {
		objContext = run
	}
	defaultHints := []string{
		"你负责优先级最高的修复项（P0 + P1 中不依赖其他文件的项）。逐个修复，每个修完跑一次 gate 验证。",
		"你负责剩余修复项（P2 + 重构类 P1）。先做独立的删除/清理，再做涉及多文件的重构。每步跑 gate。",
	}

	cfg := buildPhaseConfig(goalx.ModeDevelop, savedCfg, engines, nc)
	cfg.Name = "implement"
	cfg.Objective = nextConfigObjective(fmt.Sprintf("实施 %s 的共识修复清单。严格按照 context 中的文档执行，不做额外改动。", run), nc)
	cfg.DiversityHints = nextConfigHints(defaultHints, cfg.Parallel, nc)
	cfg.Context = goalx.ContextConfig{Files: contextFiles}
	cfg.Target = goalx.TargetConfig{
		Files: targetFiles,
	}
	cfg.Harness = goalx.HarnessConfig{Command: harness}
	goalx.ApplyPreset(&cfg)

	goalxDir := filepath.Join(projectRoot, ".goalx")
	os.MkdirAll(goalxDir, 0755)
	outPath := filepath.Join(goalxDir, "goalx.yaml")
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("# goalx.yaml — implement fixes from %s\n", run)
	if err := os.WriteFile(outPath, append([]byte(header), data...), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated %s (implement from %s)\n", outPath, run)
	fmt.Printf("  context: %d files from .goalx/runs/%s/\n", len(contextFiles), run)
	fmt.Printf("  harness: %s\n", harness)
	fmt.Println("\n  Next: review goalx.yaml (check target.files + objective), then goalx start")
	return nil
}
