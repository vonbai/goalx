package cli

import (
	"fmt"
	"time"
)

// Stop kills the tmux session for the current run.
func Stop(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx stop [--run NAME]") {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx stop [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if err := stopRunSidecar(rc.RunDir); err != nil {
		return err
	}

	if !SessionExists(rc.TmuxSession) {
		_ = MarkRunInactive(rc.ProjectRoot, rc.Name)
		if state, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir)); err == nil && state != nil {
			state.Active = false
			state.StoppedAt = time.Now().UTC().Format(time.RFC3339)
			state.UpdatedAt = state.StoppedAt
			_ = SaveRunRuntimeState(RunRuntimeStatePath(rc.RunDir), state)
		}
		_ = FinalizeControlRun(rc.RunDir, "stopped")
		fmt.Printf("Run '%s' is not active (no tmux session).\n", rc.Name)
		return nil
	}

	// Stopping a run preserves the run worktree for inspection or later reuse.
	if err := KillSession(rc.TmuxSession); err != nil {
		return fmt.Errorf("kill tmux session %s: %w", rc.TmuxSession, err)
	}
	if state, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir)); err == nil && state != nil {
		state.Active = false
		state.StoppedAt = time.Now().UTC().Format(time.RFC3339)
		state.UpdatedAt = state.StoppedAt
		_ = SaveRunRuntimeState(RunRuntimeStatePath(rc.RunDir), state)
	}
	_ = MarkRunInactive(rc.ProjectRoot, rc.Name)
	_ = FinalizeControlRun(rc.RunDir, "stopped")
	fmt.Printf("Run '%s' stopped (tmux session %s killed).\n", rc.Name, rc.TmuxSession)
	return nil
}
