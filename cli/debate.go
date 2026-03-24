package cli

import (
	"fmt"
	"sort"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// Debate starts a research-mode debate run from an explicit saved run.
func Debate(projectRoot string, args []string, nc *nextConfigJSON) error {
	if wantsHelp(args) {
		fmt.Println(phaseUsage("debate"))
		return nil
	}
	opts, err := parsePhaseOptions("debate", args)
	if err != nil {
		return err
	}
	opts = mergeNextConfigIntoPhaseOptions(opts, nc, goalx.ModeResearch)
	source, err := loadSavedPhaseSource(projectRoot, opts.From)
	if err != nil {
		return err
	}
	if len(source.Context) == 0 {
		return fmt.Errorf("no reports found in %s", source.Dir)
	}

	cfg, engines, err := buildPhaseConfigFromSource(projectRoot, "debate", goalx.ModeResearch, source, opts)
	if err != nil {
		return err
	}
	sort.Strings(source.SessionNames)
	defaultHints := debateDefaultHints(source.SessionNames)
	hints, err := applyPhaseStrategies(defaultHints, cfg.Parallel, opts)
	if err != nil {
		return err
	}
	contextFiles, err := mergePhaseContext(source.Context, opts.ContextPaths)
	if err != nil {
		return err
	}

	cfg.Objective = opts.Objective
	if cfg.Objective == "" {
		cfg.Objective = fmt.Sprintf("基于 %s 的独立调研报告，辩论分歧点并达成共识，输出统一的优先级修复清单。", source.Run)
	}
	cfg.DiversityHints = hints
	cfg.Context = goalx.ContextConfig{Files: contextFiles}
	cfg.Target = goalx.TargetConfig{
		Files:    []string{"report.md"},
		Readonly: []string{"."},
	}
	cfg.Harness = goalx.HarnessConfig{Command: researchReportHarness()}

	if opts.WriteConfig {
		if err := writePhaseConfig(projectRoot, cfg, fmt.Sprintf("# goalx manual draft — debate round based on %s\n", source.Run)); err != nil {
			return err
		}
		fmt.Printf("Generated manual draft %s (debate from %s)\n", ManualDraftConfigPath(projectRoot), source.Run)
		fmt.Println("\n  Next: review .goalx/goalx.yaml, then goalx start --config .goalx/goalx.yaml")
		return nil
	}

	return startWithConfig(projectRoot, cfg, engines, phaseRunMetadataPatch(source, "debate"))
}

func debateDefaultHints(sessionNames []string) []string {
	if len(sessionNames) <= 1 {
		sessionName := "session-1"
		if len(sessionNames) == 1 {
			sessionName = sessionNames[0]
		}
		return []string{
			fmt.Sprintf("你是倡导者。用代码证据支持 %s 报告的结论和方案。", sessionName),
			fmt.Sprintf("你是批评者。用代码证据挑战 %s 报告的每一个结论，寻找遗漏和替代方案。", sessionName),
		}
	}

	hints := make([]string, 0, len(sessionNames))
	for i, sessionName := range sessionNames {
		others := make([]string, 0, len(sessionNames)-1)
		for j, other := range sessionNames {
			if i != j {
				others = append(others, other)
			}
		}
		hints = append(hints, fmt.Sprintf(
			"你支持 %s 的观点。用代码证据辩护 %s 报告中的结论，挑战 %s 的结论。如果对方证据更强，愿意让步。最终输出共识清单。",
			sessionName, sessionName, strings.Join(others, "、"),
		))
	}
	return hints
}
