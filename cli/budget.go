package cli

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const budgetUsage = "usage: goalx budget [--run NAME] [--extend DURATION | --set-total DURATION | --clear]"

func Budget(projectRoot string, args []string) error {
	if printUsageIfHelp(args, budgetUsage) {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	action, amount, err := parseBudgetMutation(rest)
	if err != nil {
		return err
	}
	var before time.Duration
	if action != "" {
		if stopped, lifecycle, err := waitRunStopped(rc.RunDir); err != nil {
			return err
		} else if stopped && (lifecycle == "completed" || lifecycle == "dropped") {
			return fmt.Errorf("run %q is %s; budget can only change on non-final runs", rc.Name, lifecycle)
		}
		cfg := *rc.Config
		before = cfg.Budget.MaxDuration
		switch action {
		case "extend":
			cfg.Budget.MaxDuration += amount
		case "set-total":
			cfg.Budget.MaxDuration = amount
		case "clear":
			cfg.Budget.MaxDuration = 0
		default:
			return fmt.Errorf(budgetUsage)
		}
		if err := SaveRunSpec(rc.RunDir, &cfg); err != nil {
			return err
		}
		rc.Config = &cfg
		beforeState, err := captureInterventionBeforeState(rc.RunDir)
		if err != nil {
			return err
		}
		kind := "budget_" + strings.ReplaceAll(action, "-", "_")
		message := fmt.Sprintf("budget %s: %s -> %s", action, before.String(), rc.Config.Budget.MaxDuration.String())
		if err := AppendInterventionEvent(rc.RunDir, kind, "user", InterventionEventBody{
			Run:                 rc.Name,
			Message:             message,
			AffectedTargets:     []string{"master"},
			BudgetAction:        action,
			BudgetBeforeSeconds: int64(before / time.Second),
			BudgetAfterSeconds:  int64(rc.Config.Budget.MaxDuration / time.Second),
			Before:              beforeState,
		}); err != nil {
			return err
		}
		if _, err := RefreshRunGuidance(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
			fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
		}
		if facts, err := LoadTargetPresenceFact(rc.RunDir, rc.TmuxSession, "master"); err == nil && targetPresenceAvailableForTransport(facts) {
			dedupeKey := fmt.Sprintf("budget-change:%s", action)
			if _, err := DeliverControlNudge(rc.RunDir, dedupeKey, dedupeKey, rc.TmuxSession+":master", rc.Config.Master.Engine, func(target, engine string) (TransportDeliveryOutcome, error) {
				return sendAgentNudgeDetailedInRunFunc(rc.RunDir, target, engine)
			}); err != nil {
				fmt.Fprintf(os.Stderr, "warning: nudge master after budget change: %v\n", err)
			}
		}
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir))
	if err != nil {
		return err
	}
	meta, err := LoadRunMetadata(RunMetadataPath(rc.RunDir))
	if err != nil {
		return err
	}

	budget := buildActivityBudget(rc.Config, runtimeState, meta, "")
	if summary := formatBudgetSummary(budget); summary != "" {
		fmt.Printf("Budget: %s\n", summary)
		return nil
	}
	fmt.Println("Budget: none")
	return nil
}

func parseBudgetMutation(args []string) (string, time.Duration, error) {
	action := ""
	var amount time.Duration
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--extend":
			if action != "" || i+1 >= len(args) {
				return "", 0, fmt.Errorf(budgetUsage)
			}
			i++
			duration, err := parsePositiveBudgetDuration(args[i])
			if err != nil {
				return "", 0, err
			}
			action = "extend"
			amount = duration
		case "--set-total":
			if action != "" || i+1 >= len(args) {
				return "", 0, fmt.Errorf(budgetUsage)
			}
			i++
			duration, err := parsePositiveBudgetDuration(args[i])
			if err != nil {
				return "", 0, err
			}
			action = "set-total"
			amount = duration
		case "--clear":
			if action != "" {
				return "", 0, fmt.Errorf(budgetUsage)
			}
			action = "clear"
		default:
			return "", 0, fmt.Errorf(budgetUsage)
		}
	}
	return action, amount, nil
}

func parsePositiveBudgetDuration(raw string) (time.Duration, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, fmt.Errorf("invalid budget duration %q", raw)
	}
	if value == "0" || value == "0s" {
		return 0, fmt.Errorf("invalid budget duration %q: use --clear to remove the limit", raw)
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid budget duration %q", raw)
	}
	return duration, nil
}
