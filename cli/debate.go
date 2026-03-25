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
	return runPhaseAction(projectRoot, phaseActionSpec{
		Kind:         "debate",
		Mode:         goalx.ModeResearch,
		NoContextErr: "no reports found in %s",
		DraftHeader:  "# goalx manual draft — debate round based on %s\n",
		DefaultHints: debatePhaseHints,
	}, opts, nc)
}

func debatePhaseHints(source *savedPhaseSource) []string {
	if source == nil {
		return debateDefaultHints(nil)
	}
	sessionNames := append([]string(nil), source.SessionNames...)
	sort.Strings(sessionNames)
	return debateDefaultHints(sessionNames)
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
