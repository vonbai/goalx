package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type phaseOptions struct {
	From           string
	Name           string
	Objective      string
	Parallel       int
	ContextPaths   []string
	Dimensions     []string
	Effort         goalx.EffortLevel
	Master         string
	ResearchRole   string
	DevelopRole    string
	MasterEffort   goalx.EffortLevel
	ResearchEffort goalx.EffortLevel
	DevelopEffort  goalx.EffortLevel
	BudgetSet      bool
	Budget         time.Duration
	WriteConfig    bool
}

func phaseUsage(command string) string {
	return fmt.Sprintf(`usage: goalx %s --from RUN [--name NAME] [--objective TEXT] [--parallel N] [--master ENGINE/MODEL] [--research-role ENGINE/MODEL] [--develop-role ENGINE/MODEL] [--context PATHS] [--dimension SPEC]... [--effort LEVEL] [--master-effort LEVEL] [--research-effort LEVEL] [--develop-effort LEVEL] [--budget DURATION] [--write-config]

notes:
  --from RUN is required and must reference a saved run.
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
		case "--write-config":
			opts.WriteConfig = true
		case "--engine", "--model":
			return opts, fmt.Errorf("%s is ambiguous; use --master, --research-role, or --develop-role", args[i])
		default:
			return opts, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if strings.TrimSpace(opts.From) == "" {
		return opts, fmt.Errorf("usage: goalx %s --from RUN [flags]", command)
	}
	return opts, nil
}

func mergeNextConfigIntoPhaseOptions(opts phaseOptions, nc *nextConfigJSON, phaseMode goalx.Mode) phaseOptions {
	_ = phaseMode
	if nc == nil {
		return opts
	}
	if opts.Parallel == 0 && nc.Parallel > 0 {
		opts.Parallel = nc.Parallel
	}
	if opts.Objective == "" && nc.Objective != "" {
		opts.Objective = nc.Objective
	}
	if len(opts.ContextPaths) == 0 && len(nc.Context) > 0 {
		opts.ContextPaths = append([]string(nil), nc.Context...)
	}
	if len(opts.Dimensions) == 0 && len(nc.Dimensions) > 0 {
		opts.Dimensions = append([]string(nil), nc.Dimensions...)
	}
	return opts
}
