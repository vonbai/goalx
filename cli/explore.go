package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

// Explore starts a follow-up research run from an explicit saved run.
func Explore(projectRoot string, args []string) error {
	if wantsHelp(args) {
		fmt.Println(phaseUsage("explore"))
		return nil
	}
	opts, err := parsePhaseOptions("explore", args)
	if err != nil {
		return err
	}
	source, err := loadSavedPhaseSource(projectRoot, opts.From)
	if err != nil {
		return err
	}
	if len(source.Context) == 0 {
		return fmt.Errorf("no reports found in %s", source.Dir)
	}

	cfg, engines, err := buildPhaseConfigFromSource(projectRoot, "explore", goalx.ModeResearch, source, opts)
	if err != nil {
		return err
	}
	contextFiles, err := mergePhaseContext(source.Context, opts.ContextPaths)
	if err != nil {
		return err
	}
	defaultHints := []string{
		"继续扩大证据覆盖范围，优先验证原结论的盲点、缺失案例和失败模式。",
		"从替代架构路径、反例和更高 ROI 方案入手，补充可派发的新切片。",
	}
	hints, err := applyPhaseStrategies(defaultHints, cfg.Parallel, opts)
	if err != nil {
		return err
	}
	cfg.Objective = opts.Objective
	if cfg.Objective == "" {
		cfg.Objective = fmt.Sprintf("基于 %s 的已有研究结果，继续扩展探索、验证盲点、寻找更优路径，并产出新的可执行切片。", source.Run)
	}
	cfg.DiversityHints = hints
	cfg.Context = goalx.ContextConfig{Files: contextFiles}
	cfg.Target = goalx.TargetConfig{
		Files:    []string{"report.md"},
		Readonly: []string{"."},
	}
	cfg.Harness = goalx.HarnessConfig{Command: researchReportHarness()}

	if opts.WriteConfig {
		if err := writePhaseConfig(projectRoot, cfg, fmt.Sprintf("# goalx manual draft — explore based on %s\n", source.Run)); err != nil {
			return err
		}
		fmt.Printf("Generated manual draft %s (explore from %s)\n", ManualDraftConfigPath(projectRoot), source.Run)
		fmt.Println("\n  Next: review .goalx/goalx.yaml, then goalx start --config .goalx/goalx.yaml")
		return nil
	}

	return startWithConfig(projectRoot, cfg, engines, phaseRunMetadataPatch(source, "explore"))
}
