package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type phaseOptions struct {
	From         string
	Name         string
	Objective    string
	Parallel     int
	Readonly     bool
	ContextPaths []string
	Dimensions   []string
	Effort       goalx.EffortLevel
	Master       string
	Worker       string
	MasterEffort goalx.EffortLevel
	WorkerEffort goalx.EffortLevel
	BudgetSet    bool
	Budget       time.Duration
	WriteConfig  bool
}

func phaseUsage(command string) string {
	return fmt.Sprintf(`usage: goalx %s --from RUN [--name NAME] [--objective TEXT] [--parallel N] [--master ENGINE/MODEL] [--worker ENGINE/MODEL] [--context ITEMS] [--dimension SPEC]... [--effort LEVEL] [--master-effort LEVEL] [--worker-effort LEVEL] [--budget DURATION] [--readonly] [--write-config]

notes:
  --from RUN is required and must reference a saved run.
  use one comma-delimited --context value for multiple items; escape literal commas inside one item as \\,.
  --parallel is optional initial fan-out for the new phase run.
  saved run selection snapshot stays in effect unless you request an explicit CLI selection override.
  direct start is the default; use --write-config only for advanced config-first control.`, command)
}

func parsePhaseOptions(command string, args []string) (phaseOptions, error) {
	opts := phaseOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--from":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --from")
			}
			i++
			opts.From = args[i]
		case "--name":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --name")
			}
			i++
			opts.Name = args[i]
		case "--objective":
			if i+1 >= len(args) {
				return opts, fmt.Errorf("missing value for --objective")
			}
			i++
			opts.Objective = args[i]
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
		case "--write-config":
			opts.WriteConfig = true
		case "--engine", "--model":
			return opts, fmt.Errorf("%s is ambiguous; use --master or --worker", args[i])
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if strings.TrimSpace(opts.From) == "" {
		return opts, fmt.Errorf("usage: goalx %s --from RUN [flags]", command)
	}
	return opts, nil
}
