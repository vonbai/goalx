package cli

import (
	"encoding/json"
	"fmt"
)

const affordUsage = "usage: goalx afford [--run NAME] [master|session-N] [--json]"

func Afford(projectRoot string, args []string) error {
	if printUsageIfHelp(args, affordUsage) {
		return nil
	}
	runName, target, jsonOut, err := parseAffordArgs(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	doc, err := BuildAffordances(projectRoot, rc.Name, rc.RunDir, target)
	if err != nil {
		return err
	}
	if jsonOut {
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(RenderAffordancesMarkdown(doc))
	return nil
}

func parseAffordArgs(args []string) (runName, target string, jsonOut bool, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", "", false, err
	}
	positional := make([]string, 0, len(rest))
	for _, arg := range rest {
		switch arg {
		case "--json":
			jsonOut = true
		case "--help", "-h", "help":
			return "", "", false, fmt.Errorf(affordUsage)
		default:
			positional = append(positional, arg)
		}
	}
	switch len(positional) {
	case 0:
	case 1:
		if isAffordTarget(positional[0]) {
			target = positional[0]
		} else {
			if runName != "" {
				return "", "", false, fmt.Errorf(affordUsage)
			}
			runName = positional[0]
		}
	case 2:
		if runName != "" {
			return "", "", false, fmt.Errorf(affordUsage)
		}
		runName = positional[0]
		target = positional[1]
	default:
		return "", "", false, fmt.Errorf(affordUsage)
	}
	if target != "" && !isAffordTarget(target) {
		// Reuse the existing session-target grammar: master or session-N.
		return "", "", false, fmt.Errorf("invalid target %q (expected master or session-N)", target)
	}
	return runName, target, jsonOut, nil
}

func isAffordTarget(arg string) bool {
	return isWaitTarget(arg)
}
