package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type launchOptions struct {
	Objective      string
	Mode           goalx.Mode
	Parallel       int
	Name           string
	ContextPaths   []string
	Dimensions     []string
	Effort         goalx.EffortLevel
	Master         string
	ResearchRole   string
	DevelopRole    string
	MasterEffort   goalx.EffortLevel
	ResearchEffort goalx.EffortLevel
	DevelopEffort  goalx.EffortLevel
	Subs           []string
	Intent         string
	BudgetSet      bool
	Budget         time.Duration
	NoSnapshot     bool
}

type startInitOptions = launchOptions
type startOptions struct {
	launchOptions
	ConfigPath string
}

func wantsHelp(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch strings.TrimSpace(args[0]) {
	case "--help", "-h", "help":
		return true
	default:
		return false
	}
}

func launchUsage(command string) string {
	switch command {
	case "start":
		return `usage: goalx start "objective" [--research|--develop] [--parallel N] [--name NAME] [--master ENGINE/MODEL] [--research-role ENGINE/MODEL] [--develop-role ENGINE/MODEL] [--context PATHS] [--dimension SPEC]... [--effort LEVEL] [--master-effort LEVEL] [--research-effort LEVEL] [--develop-effort LEVEL] [--budget DURATION] [--sub ENGINE/MODEL[:N]]
       goalx start --config PATH

advanced/manual path:
  goalx start --config .goalx/goalx.yaml

notes:
  selection uses detected candidate pools by default.
  --parallel is optional initial fan-out, not a permanent cap on later dispatch.
  role defaults are separate: --master, --research-role, --develop-role.`
	case "init":
		return `usage: goalx init "objective" [--research|--develop] [--parallel N] [--name NAME] [--master ENGINE/MODEL] [--research-role ENGINE/MODEL] [--develop-role ENGINE/MODEL] [--context PATHS] [--dimension SPEC]... [--effort LEVEL] [--master-effort LEVEL] [--research-effort LEVEL] [--develop-effort LEVEL] [--budget DURATION] [--sub ENGINE/MODEL[:N]]

notes:
  this is the advanced config-first path and writes the explicit manual draft .goalx/goalx.yaml.
  selection uses detected candidate pools by default.
  --parallel is optional initial fan-out, not a permanent cap on later dispatch.
  role defaults are separate: --master, --research-role, --develop-role.`
	default:
		return `usage: goalx <start|init> "objective" [flags]`
	}
}

func parseLaunchOptions(args []string, defaultMode goalx.Mode, allowModeSwitch bool) (launchOptions, error) {
	opts := launchOptions{
		Mode: defaultMode,
	}
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return opts, fmt.Errorf("usage: goalx <run|start|init> \"objective\" [flags]")
	}

	opts.Objective = args[0]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--research":
			if !allowModeSwitch {
				return opts, fmt.Errorf("unexpected --research for this command")
			}
			opts.Mode = goalx.ModeResearch
		case "--develop":
			if !allowModeSwitch {
				return opts, fmt.Errorf("unexpected --develop for this command")
			}
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
		case "--dimension":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --dimension")
			}
			i++
			opts.Dimensions = append(opts.Dimensions, splitListFlag(args[i])...)
		case "--master":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --master")
			}
			i++
			opts.Master = args[i]
		case "--effort":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --effort")
			}
			i++
			level, err := goalx.ParseEffortLevel(args[i])
			if err != nil {
				return opts, err
			}
			opts.Effort = level
		case "--master-effort":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --master-effort")
			}
			i++
			level, err := goalx.ParseEffortLevel(args[i])
			if err != nil {
				return opts, err
			}
			opts.MasterEffort = level
		case "--research-role":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --research-role")
			}
			i++
			opts.ResearchRole = args[i]
		case "--research-effort":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --research-effort")
			}
			i++
			level, err := goalx.ParseEffortLevel(args[i])
			if err != nil {
				return opts, err
			}
			opts.ResearchEffort = level
		case "--develop-role":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --develop-role")
			}
			i++
			opts.DevelopRole = args[i]
		case "--develop-effort":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --develop-effort")
			}
			i++
			level, err := goalx.ParseEffortLevel(args[i])
			if err != nil {
				return opts, err
			}
			opts.DevelopEffort = level
		case "--sub":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --sub")
			}
			i++
			opts.Subs = append(opts.Subs, args[i])
		case "--budget":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --budget")
			}
			i++
			budget, err := parseBudgetOverride(args[i])
			if err != nil {
				return opts, err
			}
			opts.BudgetSet = true
			opts.Budget = budget
		case "--no-snapshot":
			opts.NoSnapshot = true
		case "--engine", "--model":
			return opts, fmt.Errorf("%s is ambiguous; use --master, --research-role, --develop-role, or --sub", args[i])
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return opts, nil
}

func parseBudgetOverride(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("invalid --budget value %q", raw)
	}
	if value == "0" {
		return 0, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < 0 {
		return 0, fmt.Errorf("invalid --budget value %q", raw)
	}
	return duration, nil
}

func splitListFlag(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func parseStartInitArgs(args []string) (startInitOptions, error) {
	return parseLaunchOptions(args, goalx.ModeDevelop, true)
}

func parseStartArgs(args []string) (startOptions, error) {
	opts := startOptions{}
	if len(args) == 0 {
		return opts, nil
	}
	if args[0] == "--config" {
		if len(args) < 2 {
			return opts, fmt.Errorf("usage: goalx start --config PATH")
		}
		opts.ConfigPath = args[1]
		for i := 2; i < len(args); i++ {
			switch args[i] {
			case "--no-snapshot":
				opts.NoSnapshot = true
			default:
				return opts, fmt.Errorf("unknown flag %q", args[i])
			}
		}
		return opts, nil
	}
	launch, err := parseLaunchOptions(args, goalx.ModeDevelop, true)
	if err != nil {
		return opts, err
	}
	opts.launchOptions = launch
	return opts, nil
}
