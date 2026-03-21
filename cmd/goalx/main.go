package main

import (
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
  goalx result  [NAME]                 Show saved summary or merged result details
  goalx add     "direction" [--run NAME] Add new subagent to running run
  goalx observe [NAME]                 Capture live output from all tmux windows
  goalx auto    "objective" [flags]   Full pipeline: research → debate → implement
  goalx serve                         Start the GoalX HTTP control server
  goalx next                           Show next pipeline step

Run 'goalx <command> --help' for details.`

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

	switch cmd {
	case "start":
		err = cli.Start(cwd, args)
	case "auto":
		err = cli.Auto(cwd, args)
	case "init":
		err = cli.Init(cwd, args)
	case "list":
		err = cli.List(cwd, args)
	case "status":
		err = cli.Status(cwd, args)
	case "attach":
		err = cli.Attach(cwd, args)
	case "stop":
		err = cli.Stop(cwd, args)
	case "review":
		err = cli.Review(cwd, args)
	case "diff":
		err = cli.Diff(cwd, args)
	case "keep":
		err = cli.Keep(cwd, args)
	case "archive":
		err = cli.Archive(cwd, args)
	case "save":
		err = cli.Save(cwd, args)
	case "drop":
		err = cli.Drop(cwd, args)
	case "report":
		err = cli.Report(cwd, args)
	case "result":
		err = cli.Result(cwd, args)
	case "debate":
		err = cli.Debate(cwd, args)
	case "implement":
		err = cli.Implement(cwd, args)
	case "add":
		err = cli.Add(cwd, args)
	case "observe":
		err = cli.Observe(cwd, args)
	case "serve":
		err = cli.Serve(cwd, args)
	case "next":
		err = cli.Next(cwd, args)
	case "--help", "-h", "help":
		fmt.Println(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "goalx %s: %v\n", cmd, err)
		os.Exit(1)
	}
}
