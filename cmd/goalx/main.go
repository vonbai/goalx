package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/vonbai/goalx/cli"
)

const usage = `goalx — autonomous research CLI

Usage:
  goalx init    "objective" [flags]   Generate goalx.yaml from objective
  goalx start                         Create run + worktree + tmux + launch agents from goalx.yaml
  goalx start   "objective" [flags]   Init + start in one step (zero-config)
  goalx list                          List all runs (active / completed / archived)
  goalx status  [--run NAME] [session] Show current run progress from journal
  goalx attach  [--run NAME] [window]  Attach to tmux session (default: master)
  goalx stop    [--run NAME]           Graceful shutdown
  goalx review  [--run NAME]           Compare all sessions
  goalx diff    [--run NAME] <a> [b]   Diff session code/reports
  goalx keep    [--run NAME] <session> Merge/preserve session
  goalx archive [--run NAME] <session> Git tag + preserve
  goalx save    [--run NAME]           Save run artifacts to .goalx/runs/<name>/
  goalx debate                         Generate debate config from latest research
  goalx implement                      Generate develop config from consensus
  goalx drop    [--run NAME]           Cleanup branch + worktree
  goalx report  [--run NAME]           Generate markdown report from journal
  goalx add     "direction" [--run NAME] Add new subagent to running run
  goalx observe [NAME]                 Capture live output from all tmux windows
  goalx auto    "objective" [flags]   Full pipeline: research → debate → implement
  goalx serve                         Start the GoalX HTTP control server
  goalx next                           Show next pipeline step

Run 'goalx <command> --help' for details.`

var serveCommand = cli.Serve

type unknownCommandError struct {
	name string
}

func (e unknownCommandError) Error() string {
	return fmt.Sprintf("unknown command: %s", e.name)
}

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

	err = dispatch(cwd, cmd, args)
	if err != nil {
		var unknownErr unknownCommandError
		if errors.As(err, &unknownErr) {
			fmt.Fprintln(os.Stderr, err)
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "goalx %s: %v\n", cmd, err)
		os.Exit(1)
	}
}

func dispatch(cwd, cmd string, args []string) error {
	switch cmd {
	case "start":
		return cli.Start(cwd, args)
	case "auto":
		return cli.Auto(cwd, args)
	case "init":
		return cli.Init(cwd, args)
	case "list":
		return cli.List(cwd, args)
	case "status":
		return cli.Status(cwd, args)
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
	case "archive":
		return cli.Archive(cwd, args)
	case "save":
		return cli.Save(cwd, args)
	case "drop":
		return cli.Drop(cwd, args)
	case "report":
		return cli.Report(cwd, args)
	case "debate":
		return cli.Debate(cwd, args)
	case "implement":
		return cli.Implement(cwd, args)
	case "add":
		return cli.Add(cwd, args)
	case "observe":
		return cli.Observe(cwd, args)
	case "serve":
		return serveCommand(cwd, args)
	case "next":
		return cli.Next(cwd, args)
	case "--help", "-h", "help":
		fmt.Println(usage)
		return nil
	default:
		return unknownCommandError{name: cmd}
	}
}
