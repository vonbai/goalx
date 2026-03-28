package cli

import (
	"fmt"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const (
	runIntentDeliver   = "deliver"
	runIntentEvolve    = "evolve"
	runIntentResearch  = "research"
	runIntentDevelop   = "develop"
	runIntentDebate    = "debate"
	runIntentImplement = "implement"
	runIntentExplore   = "explore"
)

var (
	runEntrypoint           = Run
	runAutoWithOptions      = startResolvedLaunch
	runLaunchWithOptions    = startResolvedLaunch
	runDebateWithNextConfig = runDebate
	runImplementWithNextCfg = runImplement
	runExploreIntent        = runExplore
)

type runRequest struct {
	Intent string
	Args   []string
}

func Run(projectRoot string, args []string, nc *nextConfigJSON) error {
	if wantsHelp(args) {
		fmt.Println(runUsage())
		return nil
	}

	req, err := parseRunRequest(args)
	if err != nil {
		return err
	}

	switch req.Intent {
	case runIntentDeliver:
		fallthrough
	case runIntentEvolve:
		opts, err := parseLaunchOptions(req.Args, goalx.ModeAuto, true)
		if err != nil {
			return err
		}
		opts.Intent = req.Intent
		if err := runAutoWithOptions(projectRoot, opts); err != nil {
			return fmt.Errorf("start: %w", err)
		}
		printAutoStarted()
		return nil
	case runIntentResearch:
		opts, err := parseLaunchOptions(req.Args, goalx.ModeResearch, false)
		if err != nil {
			return err
		}
		opts.Intent = req.Intent
		return runLaunchWithOptions(projectRoot, opts)
	case runIntentDevelop:
		opts, err := parseLaunchOptions(req.Args, goalx.ModeDevelop, false)
		if err != nil {
			return err
		}
		opts.Intent = req.Intent
		return runLaunchWithOptions(projectRoot, opts)
	case runIntentDebate:
		return runDebateWithNextConfig(projectRoot, req.Args, nc)
	case runIntentImplement:
		return runImplementWithNextCfg(projectRoot, req.Args, nc)
	case runIntentExplore:
		return runExploreIntent(projectRoot, req.Args)
	default:
		return fmt.Errorf("unsupported run intent %q", req.Intent)
	}
}

func runUsage() string {
	return `usage: goalx run "objective" [--intent deliver|evolve|research|develop] [flags]
       goalx run --from RUN --intent debate [flags]

notes:
  run is the primary entrypoint.
  omit --intent for the default deliver path.`
}

func parseRunRequest(args []string) (runRequest, error) {
	req := runRequest{Intent: runIntentDeliver}
	req.Args = make([]string, 0, len(args))

	rawIntent := ""
	for i := 0; i < len(args); i++ {
		if args[i] != "--intent" {
			req.Args = append(req.Args, args[i])
			continue
		}
		if i+1 >= len(args) {
			return runRequest{}, fmt.Errorf("missing value for --intent")
		}
		i++
		rawIntent = strings.TrimSpace(args[i])
	}

	if rawIntent == "" {
		return req, nil
	}
	intent, err := normalizeRunIntent(rawIntent)
	if err != nil {
		return runRequest{}, err
	}
	req.Intent = intent
	return req, nil
}

func normalizeRunIntent(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "", "auto", runIntentDeliver:
		return runIntentDeliver, nil
	case runIntentEvolve, runIntentResearch, runIntentDevelop, runIntentDebate, runIntentImplement, runIntentExplore:
		return strings.TrimSpace(raw), nil
	default:
		return "", fmt.Errorf("unknown --intent %q", raw)
	}
}

func prependRunIntent(args []string, intent string) []string {
	next := make([]string, 0, len(args)+2)
	next = append(next, "--intent", intent)
	next = append(next, args...)
	return next
}

func printAutoStarted() {
	fmt.Println("Run started.")
	fmt.Println("Use `goalx status`, `goalx observe`, or `goalx attach` to monitor progress.")
}
