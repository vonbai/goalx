package cli

import "fmt"

// Attach attaches to a tmux session window for the current run.
func Attach(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx attach [--run NAME] [window]") {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) > 1 {
		return fmt.Errorf("usage: goalx attach [--run NAME] [window]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	window, err := resolveWindowName(rc.Config.Name, "")
	if err != nil {
		return err
	}
	if len(rest) == 1 {
		window, err = resolveWindowName(rc.Config.Name, rest[0])
		if err != nil {
			return err
		}
	}

	if !SessionExists(rc.TmuxSession) {
		if state, err := loadDerivedRunState(projectRoot, rc.RunDir); err == nil && state != nil && (state.Status == "active" || state.Status == "degraded") {
			return fmt.Errorf("tmux transport unavailable for run '%s' (state=%s); use `goalx observe --run %s` or `goalx status --run %s`", rc.Name, state.Status, rc.Name, rc.Name)
		}
		return fmt.Errorf("tmux session %s not found (run may have stopped)", rc.TmuxSession)
	}
	return AttachSession(rc.TmuxSession, window)
}
