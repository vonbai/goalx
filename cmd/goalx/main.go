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

const usage = `goalx — autonomous goal execution CLI

Usage:
  goalx run     "objective" [--readonly] [flags]   Primary goal entrypoint; defaults to deliver semantics
  goalx run     --from RUN --intent debate|implement|explore [--readonly] [flags] Continue a saved run with an explicit next-step intent
  goalx init    "objective" [flags]   Generate an explicit manual draft config from an objective
  goalx start   --config PATH         Create run + tmux + launch the master from an explicit manual draft
  goalx start   "objective" [flags]   Create and start a run directly from CLI flags
  goalx list                          List all runs (active / completed / archived)
  goalx status  [--run RUN] [session] Show current run progress and control summary
  goalx context [--run RUN] [--json]    Show the run context index
  goalx afford  [--run RUN] [target] [--json] Show run-scoped command affordances
  goalx attach  [--run RUN] [window]  Attach to tmux session (default: master)
  goalx stop    [--run RUN]           Graceful shutdown
  goalx recover [--run RUN]           Recover and relaunch an existing run in place
  goalx review  [--run RUN]           Compare all sessions
  goalx diff    [--run RUN] <a> [b]   Diff session code/reports
  goalx keep    [--run NAME] <session> Merge/preserve session
  goalx integrate [--run NAME] --method METHOD --from SOURCE[,SOURCE...] Record a master-owned run-root integration
  goalx park    [--run NAME] <session> Park a session for later reuse
  goalx resume  [--run NAME] <session> Resume a parked session
  goalx replace [--run NAME] <session> [flags] Replace a session with a new routed owner
  goalx focus   [--run NAME]           Set the default run for this project
  goalx archive [--run RUN] <session> Git tag + preserve
  goalx save    [--run RUN]           Save run artifacts to user-scoped durable storage
  goalx verify  [--run RUN]           Run the effective acceptance command and record exit code + output
  goalx durable write <surface> ...   Apply structured durable authoring payloads
  goalx schema  <surface> [--json]    Show the authoring contract for a durable surface
  goalx drop    [--run RUN]           Cleanup branch + worktree
  goalx report  [--run RUN]           Generate markdown report from journal
  goalx result  [NAME]                 Show saved summary or merged result details
  goalx add     "direction" [--run RUN] [flags] Add a session to a running run
  goalx dimension [--run RUN] <session-N|all> Adjust runtime dimension assignments
  goalx tell    [--run RUN] [target] "message" Send a durable instruction to master or a session
  goalx ack-session [--run RUN] <session>      Acknowledge latest processed session inbox entry
  goalx wait    [--run RUN] [target] [--timeout DURATION] Block on unread inbox entries or timeout
  goalx observe [RUN]                  Capture live output from all tmux windows

Notes:
  RUN selectors are local-first. Bare NAME stays in the current project; use project-id/run or run_id for cross-project targeting.
  --parallel is optional initial fan-out, not a permanent cap on later dispatch.
  Use --master and --worker for role-specific engine/model defaults.
  goalx run --intent debate|implement requires --from RUN.
  .goalx/config.yaml is the shared project config; .goalx/goalx.yaml is an explicit manual draft only.

Run 'goalx <command> --help' for details.`

var (
	errInterrupted       = errors.New("interrupted by signal")
	mainStart            = cli.Start
	mainRun              = cli.Run
	mainStop             = cli.Stop
	mainRecover          = cli.Recover
	mainWait             = cli.Wait
	mainRuntimeHost      = cli.RuntimeHostCommand
	mainLeaseLoop        = cli.LeaseLoop
	mainTargetRunner     = cli.TargetRunnerCommand
	mainContext          = cli.Context
	mainAfford           = cli.Afford
	mainDurable          = cli.Durable
	mainSchema           = cli.Schema
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
	cwd = cli.CanonicalProjectRoot(cwd)

	switch cmd {
	case "run":
		if runNeedsSignalCleanup(args) {
			return runWithSignalCleanup(cwd, func() error { return mainRun(cwd, args) })
		}
		return mainRun(cwd, args)
	case "start":
		return runWithSignalCleanup(cwd, func() error { return mainStart(cwd, args) })
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
	case "recover":
		return mainRecover(cwd, args)
	case "review":
		return cli.Review(cwd, args)
	case "diff":
		return cli.Diff(cwd, args)
	case "keep":
		return cli.Keep(cwd, args)
	case "integrate":
		return cli.Integrate(cwd, args)
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
	case "durable":
		return mainDurable(cwd, args)
	case "schema":
		return mainSchema(cwd, args)
	case "pulse":
		return cli.Pulse(cwd, args)
	case "drop":
		return cli.Drop(cwd, args)
	case "report":
		return cli.Report(cwd, args)
	case "result":
		return cli.Result(cwd, args)
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
	case "claude-hook":
		return cli.ClaudeHook(cwd, args)
	case "runtime-host":
		return mainRuntimeHost(cwd, args)
	case "lease-loop":
		return mainLeaseLoop(cwd, args)
	case "target-runner":
		return mainTargetRunner(cwd, args)
	default:
		return errUnknownCommand
	}
}

func runNeedsSignalCleanup(args []string) bool {
	intent := "deliver"
	hasFrom := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--intent":
			if i+1 < len(args) {
				intent = args[i+1]
				i++
			}
		case "--from":
			hasFrom = true
		}
	}

	if hasFrom {
		return false
	}

	switch intent {
	case "debate", "implement":
		return false
	default:
		return true
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
