package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

// Implement starts a develop-mode implementation run from an explicit saved run.
func Implement(projectRoot string, args []string, nc *nextConfigJSON) error {
	if wantsHelp(args) {
		fmt.Println(phaseUsage("implement"))
		return nil
	}
	opts, err := parsePhaseOptions("implement", args)
	if err != nil {
		return err
	}
	opts = mergeNextConfigIntoPhaseOptions(opts, nc, goalx.ModeDevelop)
	source, err := loadSavedPhaseSource(projectRoot, opts.From)
	if err != nil {
		return err
	}
	if len(source.Context) == 0 {
		return fmt.Errorf("no reports/summary found in %s", source.Dir)
	}

	cfg, engines, err := buildPhaseConfigFromSource(projectRoot, "implement", goalx.ModeDevelop, source, opts)
	if err != nil {
		return err
	}
	baseCfg, _, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load base config: %w", err)
	}
	targetFiles := baseCfg.Target.Files
	if len(targetFiles) == 0 {
		targetFiles = InferTarget(projectRoot)
	}
	harness := baseCfg.Harness.Command
	if harness == "" {
		harness = InferHarness(projectRoot)
	}
	if harness == "" {
		harness = "echo 'no harness inferred - configure harness.command in .goalx/config.yaml'"
	}
	contextFiles, err := mergePhaseContext(source.Context, opts.ContextPaths)
	if err != nil {
		return err
	}
	defaultHints := []string{
		"你负责优先级最高的修复项（P0 + P1 中不依赖其他文件的项）。逐个修复，每个修完跑一次 gate 验证。",
		"你负责剩余修复项（P2 + 重构类 P1）。先做独立的删除/清理，再做涉及多文件的重构。每步跑 gate。",
	}
	hints, err := applyPhaseStrategies(defaultHints, cfg.Parallel, opts)
	if err != nil {
		return err
	}

	cfg.Objective = opts.Objective
	if cfg.Objective == "" {
		cfg.Objective = fmt.Sprintf("实施 %s 的共识修复清单。严格按照 context 中的文档执行，不做额外改动。", source.Run)
	}
	cfg.DiversityHints = hints
	cfg.Context = goalx.ContextConfig{Files: contextFiles}
	cfg.Target = goalx.TargetConfig{Files: targetFiles}
	cfg.Harness = goalx.HarnessConfig{Command: harness}

	if opts.WriteConfig {
		if err := writePhaseConfig(projectRoot, cfg, fmt.Sprintf("# goalx manual draft — implement fixes from %s\n", source.Run)); err != nil {
			return err
		}
		fmt.Printf("Generated manual draft %s (implement from %s)\n", ManualDraftConfigPath(projectRoot), source.Run)
		fmt.Println("\n  Next: review .goalx/goalx.yaml, then goalx start --config .goalx/goalx.yaml")
		return nil
	}

	return startWithConfig(projectRoot, cfg, engines, phaseRunMetadataPatch(source, "implement"))
}
