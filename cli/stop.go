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
	if err := runtimeSupervisor.Stop(rc.RunDir); err != nil {
		return err
	}
	finalLifecycle := stopLifecycleForRun(rc.RunDir)

	if !SessionExistsInRun(rc.RunDir, rc.TmuxSession) {
		killRunPaneProcessTrees(rc.RunDir, rc.TmuxSession)
		_ = MarkRunInactive(rc.ProjectRoot, rc.Name)
		if state, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir)); err == nil && state != nil {
			state.Active = false
			state.StoppedAt = time.Now().UTC().Format(time.RFC3339)
			if finalLifecycle == "completed" {
				state.Phase = "complete"
			}
			state.UpdatedAt = state.StoppedAt
			_ = UpsertRunRuntimeState(rc.RunDir, *state)
		}
		_ = FinalizeControlRun(rc.RunDir, finalLifecycle)
		if finalLifecycle == "completed" {
			fmt.Printf("Run '%s' completed (no tmux session).\n", rc.Name)
		} else {
			fmt.Printf("Run '%s' is not active (no tmux session).\n", rc.Name)
		}
		return nil
	}

	// Stopping a run preserves the run worktree for inspection or later reuse.
	killRunPaneProcessTrees(rc.RunDir, rc.TmuxSession)
	killAllLeasedProcesses(rc.RunDir)
	if err := KillSessionIfExistsInRun(rc.RunDir, rc.TmuxSession); err != nil {
		return fmt.Errorf("kill tmux session %s: %w", rc.TmuxSession, err)
	}
	if state, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir)); err == nil && state != nil {
		state.Active = false
		state.StoppedAt = time.Now().UTC().Format(time.RFC3339)
		if finalLifecycle == "completed" {
			state.Phase = "complete"
		}
		state.UpdatedAt = state.StoppedAt
		_ = UpsertRunRuntimeState(rc.RunDir, *state)
	}
	_ = MarkRunInactive(rc.ProjectRoot, rc.Name)
	_ = FinalizeControlRun(rc.RunDir, finalLifecycle)
	if finalLifecycle == "completed" {
		fmt.Printf("Run '%s' completed (tmux session %s killed).\n", rc.Name, rc.TmuxSession)
	} else {
		fmt.Printf("Run '%s' stopped (tmux session %s killed).\n", rc.Name, rc.TmuxSession)
	}
	return nil
}
