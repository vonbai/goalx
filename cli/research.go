package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

func Research(projectRoot string, args []string) error {
	if wantsHelp(args) {
		fmt.Println(launchUsage("research"))
		return nil
	}
	opts, err := parseLaunchOptions(args, goalx.ModeResearch, false)
	if err != nil {
		return err
	}
	return startResolvedLaunch(projectRoot, opts)
}
