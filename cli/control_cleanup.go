package cli

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

type finalizeControlRunOptions struct {
	killLeasedProcesses bool
	skipKillHolders     map[string]bool
	skipExpireHolders   map[string]bool
}

func completedCloseoutReady(runDir string) bool {
	facts, err := BuildRunCloseoutFacts(runDir)
	return err == nil && facts.ReadyToFinalize()
}

func stopLifecycleForRun(runDir string) string {
	if completedCloseoutReady(runDir) {
		return "completed"
	}
	return "stopped"
}

func repairCompletedRunFinalization(rc *RunContext) error {
	if rc == nil || SessionExists(rc.TmuxSession) || !completedCloseoutReady(rc.RunDir) {
		return nil
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(rc.RunDir))
	if err == nil && controlState != nil && controlState.LifecycleState == "completed" {
		return nil
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if state, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir)); err == nil && state != nil {
		state.Active = false
		state.Phase = "complete"
		if state.StoppedAt == "" {
			state.StoppedAt = now
		}
		state.UpdatedAt = now
		if err := UpsertRunRuntimeState(rc.RunDir, *state); err != nil {
			return err
		}
	}

	killRunPaneProcessTrees(rc.RunDir, rc.TmuxSession)
	if err := KillSessionIfExists(rc.TmuxSession); err != nil {
		return err
	}
	if err := MarkRunInactive(rc.ProjectRoot, rc.Name); err != nil {
		return err
	}
	return FinalizeControlRun(rc.RunDir, "completed")
}

func FinalizeControlRun(runDir, lifecycle string) error {
	return finalizeControlRun(runDir, lifecycle, finalizeControlRunOptions{killLeasedProcesses: true})
}

func finalizeControlRun(runDir, lifecycle string, opts finalizeControlRunOptions) error {
	if err := EnsureControlState(runDir); err != nil {
		return err
	}
	if opts.killLeasedProcesses {
		killLeasedProcesses(runDir, opts.skipKillHolders)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if err := expireControlLeases(runDir, opts.skipExpireHolders); err != nil {
		return err
	}
	if err := finalizeSessionRuntimeStates(runDir, lifecycle, now); err != nil {
		return err
	}
	return submitAndApplyControlOp(runDir, controlOpFinalizeControlSurfaces, controlFinalizeControlSurfacesBody{
		Lifecycle: lifecycle,
		UpdatedAt: now,
	})
}

func killAllLeasedProcesses(runDir string) {
	killLeasedProcesses(runDir, nil)
}

func killLeasedProcesses(runDir string, skipHolders map[string]bool) {
	entries, err := os.ReadDir(ControlLeasesDir(runDir))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		holder := strings.TrimSuffix(entry.Name(), ".json")
		if shouldSkipControlHolder(skipHolders, holder) {
			continue
		}
		lease, err := LoadControlLease(filepath.Join(ControlLeasesDir(runDir), entry.Name()))
		if err == nil && lease.PID > 0 {
			KillProcessTree(lease.PID)
		}
	}
}

func finalizeSessionRuntimeStates(runDir, lifecycle, now string) error {
	return submitAndApplyControlOp(runDir, controlOpSessionsRuntimeFinalize, controlSessionsRuntimeFinalizeBody{
		Lifecycle: lifecycle,
		UpdatedAt: now,
	})
}

func expireAllControlLeases(runDir string) error {
	return expireControlLeases(runDir, nil)
}

func expireControlLeases(runDir string, skipHolders map[string]bool) error {
	expire := func(holder string) error {
		if shouldSkipControlHolder(skipHolders, holder) {
			return nil
		}
		return ExpireControlLease(runDir, holder)
	}
	if err := expire("master"); err != nil {
		return err
	}
	if err := expire("sidecar"); err != nil {
		return err
	}
	entries, err := os.ReadDir(ControlLeasesDir(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		holder := strings.TrimSuffix(entry.Name(), ".json")
		if holder == "" || holder == "master" || holder == "sidecar" || shouldSkipControlHolder(skipHolders, holder) {
			continue
		}
		if err := ExpireControlLease(runDir, holder); err != nil {
			return err
		}
	}
	return nil
}

func finalizeCompletedRunFromSidecar(projectRoot, runName, runDir, tmuxSession string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return err
	}
	if runState != nil {
		runState.Active = false
		runState.Phase = "complete"
		runState.UpdatedAt = now
		if err := UpsertRunRuntimeState(runDir, *runState); err != nil {
			return err
		}
	}

	killRunPaneProcessTrees(runDir, tmuxSession)
	if err := KillSessionIfExists(tmuxSession); err != nil {
		return err
	}

	if err := MarkRunInactive(projectRoot, runName); err != nil {
		return err
	}

	return finalizeControlRun(runDir, "completed", finalizeControlRunOptions{
		killLeasedProcesses: true,
		skipKillHolders:     map[string]bool{"sidecar": true},
		skipExpireHolders:   map[string]bool{"sidecar": true},
	})
}

func shouldSkipControlHolder(skipHolders map[string]bool, holder string) bool {
	return skipHolders != nil && skipHolders[holder]
}
