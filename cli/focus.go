package cli

import "fmt"

const focusUsage = `usage: goalx focus --run NAME`

// Focus marks an active run as the default run for commands that omit --run.
func Focus(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) == 1 && isHelpToken(rest[0]) {
		fmt.Println(focusUsage)
		return nil
	}
	if runName == "" || len(rest) != 0 {
		return fmt.Errorf(focusUsage)
	}

	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		return err
	}
	if _, ok := reg.ActiveRuns[runName]; !ok {
		return fmt.Errorf("run %q is not active", runName)
	}

	reg.FocusedRun = runName
	if err := SaveProjectRegistry(projectRoot, reg); err != nil {
		return err
	}

	fmt.Printf("Focused run set to %s\n", runName)
	return nil
}
