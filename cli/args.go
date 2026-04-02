package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

func extractRunFlag(args []string) (string, []string, error) {
	var runName string
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		if args[i] == "--run" {
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for --run")
			}
			runName = args[i+1]
			i++
			continue
		}
		rest = append(rest, args[i])
	}

	return runName, rest, nil
}

func hasHelpArg(args []string) bool {
	if len(args) != 1 {
		return false
	}
	return isHelpToken(args[0])
}

func printUsageIfHelp(args []string, usage string) bool {
	if !hasHelpArg(args) {
		return false
	}
	fmt.Println(usage)
	return true
}

func parseStatusArgs(args []string) (runName, sessionName string, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", "", err
	}
	if runName != "" && isHelpToken(runName) {
		return "", "", fmt.Errorf("usage: goalx status [NAME] [session-N]")
	}
	if len(rest) > 0 && isHelpToken(rest[0]) {
		return "", "", fmt.Errorf("usage: goalx status [NAME] [session-N]")
	}
	// Allow positional run name: "goalx status myrun" without --run flag
	if runName == "" && len(rest) >= 1 {
		runName = rest[0]
		rest = rest[1:]
	}
	if len(rest) > 1 {
		return "", "", fmt.Errorf("usage: goalx status [NAME] [session-N]")
	}
	if len(rest) == 1 {
		sessionName = rest[0]
	}
	return runName, sessionName, nil
}

func sessionCount(cfg *goalx.Config) int {
	if cfg != nil && len(cfg.Sessions) > 0 {
		return len(cfg.Sessions)
	}
	return 1
}

func sessionWindowName(runName string, idx int) string {
	return fmt.Sprintf("session-%d", idx)
}

func resolveWindowName(runName, name string) (string, error) {
	if name == "" || name == "master" {
		if name == "" {
			return "master", nil
		}
		return name, nil
	}

	idx, err := parseSessionIndex(name)
	if err != nil {
		return "", fmt.Errorf("invalid window %q (expected master or session-N)", name)
	}
	return sessionWindowName(runName, idx), nil
}
