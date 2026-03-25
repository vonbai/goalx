package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/vonbai/goalx/cli"
)

const usage = `goalx — autonomous research CLI

Usage:
  goalx init    "objective" [flags]   Generate an explicit manual draft config from an objective
  goalx start   --config PATH         Create run + tmux + launch the master from an explicit manual draft
  goalx start   "objective" [flags]   Create and start a run directly from CLI flags
  goalx research "objective" [flags]  Start a research run directly from CLI flags
  goalx develop  "objective" [flags]  Start a develop run directly from CLI flags
  goalx list                          List all runs (active / completed / archived)
  goalx status  [--run RUN] [session] Show current run progress and control summary
  goalx context [--run RUN] [--json]    Show the run context index
  goalx afford  [--run RUN] [target] [--json] Show run-scoped command affordances
  goalx attach  [--run RUN] [window]  Attach to tmux session (default: master)
  goalx stop    [--run RUN]           Graceful shutdown
  goalx review  [--run RUN]           Compare all sessions
  goalx diff    [--run RUN] <a> [b]   Diff session code/reports
  goalx keep    [--run NAME] <session> Merge/preserve session
  goalx park    [--run NAME] <session> Park a session for later reuse
  goalx resume  [--run NAME] <session> Resume a parked session
  goalx replace [--run NAME] <session> [flags] Replace a session with a new routed owner
  goalx focus   [--run NAME]           Set the default run for this project
  goalx archive [--run RUN] <session> Git tag + preserve
  goalx save    [--run RUN]           Save run artifacts to user-scoped durable storage
  goalx verify  [--run RUN]           Run the effective acceptance gate, then validate contract and completion provenance
  goalx debate  --from RUN [flags]     Start a debate run from a saved run
  goalx implement --from RUN [flags]   Start a develop run from a saved run
  goalx explore --from RUN [flags]     Start a follow-up research run from a saved run
  goalx drop    [--run RUN]           Cleanup branch + worktree
  goalx report  [--run RUN]           Generate markdown report from journal
  goalx result  [NAME]                 Show saved summary or merged result details
  goalx add     "direction" [--run RUN] Add a session to a running run
  goalx dimension [--run RUN] <session-N|all> Adjust runtime dimension assignments
  goalx tell    [--run RUN] [target] "message" Send a durable instruction to master or a session
  goalx ack-session [--run RUN] <session>      Acknowledge latest processed session inbox entry
  goalx wait    [--run RUN] [target] [--timeout DURATION] Block on unread inbox entries or timeout
  goalx observe [RUN]                  Capture live output from all tmux windows
  goalx auto    "objective" [flags]   Init and start one master-led run, then exit
  goalx serve                         Start the GoalX HTTP control server
  goalx next                           Show next pipeline step

Notes:
  RUN selectors are local-first. Bare NAME stays in the current project; use project-id/run or run_id for cross-project targeting.
  --parallel is optional initial fan-out, not a permanent cap on later dispatch.
  Use --master, --research-role, and --develop-role for role-specific engine/model defaults.
  goalx auto remains the default path; debate/implement/explore require --from RUN unless you choose --write-config.
  .goalx/config.yaml is the shared project config; .goalx/goalx.yaml is an explicit manual draft only.

Run 'goalx <command> --help' for details.`

var (
	errInterrupted       = errors.New("interrupted by signal")
	mainStart            = cli.Start
	mainAuto             = cli.Auto
	mainStop             = cli.Stop
	mainWait             = cli.Wait
	mainSidecar          = cli.Sidecar
	mainLeaseLoop        = cli.LeaseLoop
	mainContext          = cli.Context
	mainAfford           = cli.Afford
	notifySignalsContext = signal.NotifyContext
)

func main() {
	if os.Getenv("HOME") == "" {
		fmt.Fprintln(os.Stderr, "fatal: HOME is not set")
		os.Exit(1)
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: cannot determine working directory: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		fmt.Println(usage)
		os.Exit(0)
	}
	args := os.Args[2:]
	cmd := os.Args[1]

	if cmd == "--help" || cmd == "-h" || cmd == "help" {
		fmt.Println(usage)
		return
	}

	if err = runCommand(cwd, cmd, args); errors.Is(err, errUnknownCommand) {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "goalx %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

var errUnknownCommand = errors.New("unknown command")

func runCommand(cwd, cmd string, args []string) error {
	switch cmd {
	case "start":
		return runWithSignalCleanup(cwd, func() error { return mainStart(cwd, args) })
	case "auto":
		return runWithSignalCleanup(cwd, func() error { return mainAuto(cwd, args) })
	case "research":
		return runWithSignalCleanup(cwd, func() error { return cli.Research(cwd, args) })
	case "develop":
		return runWithSignalCleanup(cwd, func() error { return cli.Develop(cwd, args) })
	case "init":
		return cli.Init(cwd, args)
	case "list":
		return cli.List(cwd, args)
	case "status":
		return cli.Status(cwd, args)
	case "context":
		return mainContext(cwd, args)
	case "afford":
		return mainAfford(cwd, args)
	case "attach":
		return cli.Attach(cwd, args)
	case "stop":
		return cli.Stop(cwd, args)
	case "review":
		return cli.Review(cwd, args)
	case "diff":
		return cli.Diff(cwd, args)
	case "keep":
		return cli.Keep(cwd, args)
	case "park":
		return cli.Park(cwd, args)
	case "resume":
		return cli.Resume(cwd, args)
	case "replace":
		return cli.Replace(cwd, args)
	case "focus":
		return cli.Focus(cwd, args)
	case "archive":
		return cli.Archive(cwd, args)
	case "save":
		return cli.Save(cwd, args)
	case "verify":
		return cli.Verify(cwd, args)
	case "pulse":
		return cli.Pulse(cwd, args)
	case "drop":
		return cli.Drop(cwd, args)
	case "report":
		return cli.Report(cwd, args)
	case "result":
		return cli.Result(cwd, args)
	case "debate":
		return cli.Debate(cwd, args, nil)
	case "implement":
		return cli.Implement(cwd, args, nil)
	case "explore":
		return cli.Explore(cwd, args)
	case "add":
		return cli.Add(cwd, args)
	case "dimension":
		return cli.Dimension(cwd, args)
	case "tell":
		return cli.Tell(cwd, args)
	case "ack-session":
		return cli.AckSession(cwd, args)
	case "wait":
		return mainWait(cwd, args)
	case "observe":
		return cli.Observe(cwd, args)
	case "serve":
		return cli.Serve(cwd, args)
	case "next":
		return cli.Next(cwd, args)
	case "sidecar":
		return mainSidecar(cwd, args)
	case "lease-loop":
		return mainLeaseLoop(cwd, args)
	default:
		return errUnknownCommand
	}
}

func runWithSignalCleanup(cwd string, run func() error) error {
	ctx, stop := notifySignalsContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	done := make(chan error, 1)
	go func() {
		done <- run()
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if err := mainStop(cwd, nil); err != nil {
			fmt.Fprintf(os.Stderr, "warning: cleanup stop failed: %v\n", err)
		}
		return errInterrupted
	}
}
