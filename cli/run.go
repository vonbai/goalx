package cli

import (
	"fmt"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const (
	runIntentDeliver   = "deliver"
	runIntentEvolve    = "evolve"
	runIntentDebate    = "debate"
	runIntentImplement = "implement"
	runIntentExplore   = "explore"
)

var (
	runEntrypoint        = Run
	runAutoWithOptions   = startResolvedLaunch
	runLaunchWithOptions = startResolvedLaunch
	runDebateIntent      = runDebate
	runImplementIntent   = runImplement
	runExploreIntent     = runExplore
)

type runRequest struct {
	Intent string
	Args   []string
}

func Run(projectRoot string, args []string) error {
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
		fallthrough
	case runIntentExplore:
		if req.Intent == runIntentExplore && hasLiteralFlag(req.Args, "--from") {
			return runExploreIntent(projectRoot, req.Args)
		}
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
	case runIntentDebate:
		if err := requirePhaseSource(req.Intent, req.Args); err != nil {
			return err
		}
		return runDebateIntent(projectRoot, req.Args)
	case runIntentImplement:
		if err := requirePhaseSource(req.Intent, req.Args); err != nil {
			return err
		}
		return runImplementIntent(projectRoot, req.Args)
	default:
		return fmt.Errorf("unsupported run intent %q", req.Intent)
	}
}

func runUsage() string {
	return `usage: goalx run "objective" [--intent deliver|evolve|explore] [flags]
       goalx run --from RUN --intent debate|implement|explore [flags]

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
	case runIntentEvolve, runIntentDebate, runIntentImplement, runIntentExplore:
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

func hasLiteralFlag(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func requirePhaseSource(intent string, args []string) error {
	if hasLiteralFlag(args, "--from") {
		return nil
	}
	return fmt.Errorf("--intent %s requires --from RUN", intent)
}

func printAutoStarted() {
	fmt.Println("Run started.")
	fmt.Println("Use `goalx status`, `goalx observe`, or `goalx attach` to monitor progress.")
}
