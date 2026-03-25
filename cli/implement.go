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
	return runPhaseAction(projectRoot, phaseActionSpec{
		Kind:         "implement",
		Mode:         goalx.ModeDevelop,
		NoContextErr: "no reports/summary found in %s",
		DraftHeader:  "# goalx manual draft — implement fixes from %s\n",
		DefaultHints: implementPhaseHints,
	}, opts, nc)
}

func implementPhaseHints(*savedPhaseSource) []string {
	return []string{
		"你负责优先级最高的修复项（P0 + P1 中不依赖其他文件的项）。逐个修复，每个修完跑一次 gate 验证。",
		"你负责剩余修复项（P2 + 重构类 P1）。先做独立的删除/清理，再做涉及多文件的重构。每步跑 gate。",
	}
}
