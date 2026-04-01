package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type launchOptions struct {
	Objective    string
	Mode         goalx.Mode
	Parallel     int
	Name         string
	Readonly     bool
	ContextPaths []string
	Dimensions   []string
	Effort       goalx.EffortLevel
	Master       string
	Worker       string
	MasterEffort goalx.EffortLevel
	WorkerEffort goalx.EffortLevel
	Subs         []string
	Intent       string
	BudgetSet    bool
	Budget       time.Duration
	NoSnapshot   bool
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
		return `usage: goalx start "objective" [--parallel N] [--name NAME] [--master ENGINE/MODEL] [--worker ENGINE/MODEL] [--context ITEMS] [--dimension SPEC]... [--effort LEVEL] [--master-effort LEVEL] [--worker-effort LEVEL] [--budget DURATION] [--readonly] [--sub ENGINE/MODEL[:N]]
       goalx start --objective TEXT [flags]
       goalx start --objective-file PATH [flags]
       goalx start --config PATH

advanced/manual path:
  goalx start --config .goalx/goalx.yaml

notes:
  use one comma-delimited --context value for multiple items; escape literal commas inside one item as \\,.
  selection uses detected candidate pools by default.
  --parallel is optional initial fan-out, not a permanent cap on later dispatch.
 role defaults are separate: --master and --worker.`
	case "init":
		return `usage: goalx init "objective" [--parallel N] [--name NAME] [--master ENGINE/MODEL] [--worker ENGINE/MODEL] [--context ITEMS] [--dimension SPEC]... [--effort LEVEL] [--master-effort LEVEL] [--worker-effort LEVEL] [--budget DURATION] [--readonly] [--sub ENGINE/MODEL[:N]]
       goalx init --objective TEXT [flags]
       goalx init --objective-file PATH [flags]

notes:
  this is the advanced config-first path and writes the explicit manual draft .goalx/goalx.yaml.
  use one comma-delimited --context value for multiple items; escape literal commas inside one item as \\,.
  selection uses detected candidate pools by default.
  --parallel is optional initial fan-out, not a permanent cap on later dispatch.
  role defaults are separate: --master and --worker.`
	default:
		return `usage: goalx <start|init> "objective" [flags]`
	}
}

func parseLaunchOptions(args []string, defaultMode goalx.Mode, allowModeSwitch bool) (launchOptions, error) {
	opts := launchOptions{
		Mode: defaultMode,
	}
	positionalObjective := ""
	explicitObjective := ""
	objectiveFile := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--objective":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --objective")
			}
			i++
			explicitObjective = args[i]
		case "--objective-file":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --objective-file")
			}
			i++
			objectiveFile = args[i]
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
			items, err := splitContextFlagValue(args[i])
			if err != nil {
				return opts, err
			}
			opts.ContextPaths = append(opts.ContextPaths, items...)
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
		case "--worker":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --worker")
			}
			i++
			opts.Worker = args[i]
		case "--worker-effort":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --worker-effort")
			}
			i++
			level, err := goalx.ParseEffortLevel(args[i])
			if err != nil {
				return opts, err
			}
			opts.WorkerEffort = level
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
		case "--readonly":
			opts.Readonly = true
		case "--no-snapshot":
			opts.NoSnapshot = true
		case "--engine", "--model":
			return opts, fmt.Errorf("%s is ambiguous; use --master, --worker, or --sub", args[i])
		default:
			if strings.HasPrefix(args[i], "-") {
				return opts, fmt.Errorf("unknown flag %q", args[i])
			}
			if positionalObjective != "" {
				return opts, fmt.Errorf("objective must be provided once")
			}
			positionalObjective = args[i]
		}
	}
	objectiveSources := 0
	if strings.TrimSpace(positionalObjective) != "" {
		objectiveSources++
	}
	if strings.TrimSpace(explicitObjective) != "" {
		objectiveSources++
	}
	if strings.TrimSpace(objectiveFile) != "" {
		objectiveSources++
	}
	if objectiveSources == 0 {
		return opts, fmt.Errorf("usage: goalx <run|start|init> \"objective\" [flags]")
	}
	if objectiveSources > 1 {
		return opts, fmt.Errorf("objective must be provided by exactly one of positional objective, --objective, or --objective-file")
	}
	switch {
	case strings.TrimSpace(objectiveFile) != "":
		data, err := os.ReadFile(objectiveFile)
		if err != nil {
			return opts, fmt.Errorf("read --objective-file %q: %w", objectiveFile, err)
		}
		opts.Objective = strings.TrimSpace(string(data))
	case strings.TrimSpace(explicitObjective) != "":
		opts.Objective = strings.TrimSpace(explicitObjective)
	default:
		opts.Objective = strings.TrimSpace(positionalObjective)
	}
	if opts.Objective == "" {
		return opts, fmt.Errorf("objective cannot be empty")
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
	return parseLaunchOptions(args, goalx.ModeWorker, true)
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
	launch, err := parseLaunchOptions(args, goalx.ModeWorker, true)
	if err != nil {
		return opts, err
	}
	opts.launchOptions = launch
	return opts, nil
}
