package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

var (
	autoStart = startAuto
)

// Auto initializes a run, starts the master, and exits.
// The master continues orchestrating in tmux.
func Auto(projectRoot string, args []string) error {
	if wantsHelp(args) {
		fmt.Println(launchUsage("auto"))
		return nil
	}

	if err := autoStart(projectRoot, args); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	printAutoStarted()
	return nil
}

func startAuto(projectRoot string, args []string) error {
	opts, err := parseLaunchOptions(args, goalx.ModeAuto, true)
	if err != nil {
		return err
	}
	return autoWithOptions(projectRoot, opts)
}

func autoWithOptions(projectRoot string, opts launchOptions) error {
	return startResolvedLaunch(projectRoot, opts)
}

func printAutoStarted() {
	fmt.Println("Run started.")
	fmt.Println("Use `goalx status`, `goalx observe`, or `goalx attach` to monitor progress.")
}
