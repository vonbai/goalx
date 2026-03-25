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
	return runPhaseAction(projectRoot, phaseActionSpec{
		Kind:         "explore",
		Mode:         goalx.ModeResearch,
		NoContextErr: "no reports found in %s",
		DraftHeader:  "# goalx manual draft — explore based on %s\n",
		DefaultHints: explorePhaseHints,
	}, opts, nil)
}

func explorePhaseHints(*savedPhaseSource) []string {
	return []string{
		"继续扩大证据覆盖范围，优先验证原结论的盲点、缺失案例和失败模式。",
		"从替代架构路径、反例和更高 ROI 方案入手，补充可派发的新切片。",
	}
}
