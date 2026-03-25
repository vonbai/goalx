package cli

import (
	"fmt"

	goalx "github.com/vonbai/goalx"
)

func Develop(projectRoot string, args []string) error {
	if wantsHelp(args) {
		fmt.Println(launchUsage("develop"))
		return nil
	}
	opts, err := parseLaunchOptions(args, goalx.ModeDevelop, false)
	if err != nil {
		return err
	}
	return startResolvedLaunch(projectRoot, opts)
}
