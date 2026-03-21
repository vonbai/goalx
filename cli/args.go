package cli

import (
	"fmt"
	"strconv"
	"strings"

	goalx "github.com/vonbai/goalx"
)

type startInitOptions struct {
	Objective    string
	Mode         goalx.Mode
	Parallel     int
	Name         string
	ContextPaths []string
	Strategies   []string
	Master       string   // "engine/model" format
	Auditor      string   // "engine/model" format
	Subs         []string // repeatable "engine/model:N" for explicit session list
	Preset       string
}

func parseStartInitArgs(args []string) (startInitOptions, error) {
	opts := startInitOptions{
		Mode:     goalx.ModeDevelop,
		Parallel: 1,
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return opts, fmt.Errorf("usage: goalx start \"objective\" [--research|--develop] [--parallel N] [--name NAME]")
	}

	opts.Objective = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--research":
			opts.Mode = goalx.ModeResearch
		case "--develop":
			opts.Mode = goalx.ModeDevelop
		case "--parallel":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --parallel")
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 1 {
				return opts, fmt.Errorf("invalid --parallel value %q", args[i])
			}
			opts.Parallel = n
		case "--name":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --name")
			}
			i++
			opts.Name = args[i]
		case "--context":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --context")
			}
			i++
			opts.ContextPaths = strings.Split(args[i], ",")
		case "--strategy":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --strategy")
			}
			i++
			opts.Strategies = strings.Split(args[i], ",")
		case "--master":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --master")
			}
			i++
			opts.Master = args[i]
		case "--auditor":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --auditor")
			}
			i++
			opts.Auditor = args[i]
		case "--sub":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --sub")
			}
			i++
			opts.Subs = append(opts.Subs, args[i])
		case "--preset":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --preset")
			}
			i++
			opts.Preset = args[i]
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	return opts, nil
}

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

func parseStatusArgs(args []string) (runName, sessionName string, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", "", err
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
	if len(cfg.Sessions) > 0 {
		return len(cfg.Sessions)
	}
	if cfg.Parallel > 0 {
		return cfg.Parallel
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
