package cli

import (
	"fmt"
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

func reconcileRunContinuityForRun(projectRoot, runName, runDir string) error {
	if strings.TrimSpace(runDir) == "" {
		return nil
	}
	if _, err := os.Stat(RunHostStatePath(runDir)); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	host, err := runtimeSupervisor.Inspect(runDir)
	if err != nil {
		return err
	}
	if host == nil {
		return nil
	}
	if err := SaveRunHostState(RunHostStatePath(runDir), host); err != nil {
		return err
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		return err
	}
	if controlState == nil || strings.TrimSpace(controlState.ContinuityState) != "running" || host.Running {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	switch strings.TrimSpace(controlState.GoalState) {
	case "completed", "dropped":
		controlState.ContinuityState = "stopped"
	default:
		controlState.GoalState = "open"
		controlState.ContinuityState = "stranded"
	}
	controlState.UpdatedAt = now
	if err := SaveControlRunState(ControlRunStatePath(runDir), controlState); err != nil {
		return err
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return err
	}
	if runtimeState != nil {
		runtimeState.Active = false
		if runtimeState.StoppedAt == "" {
			runtimeState.StoppedAt = now
		}
		runtimeState.UpdatedAt = now
		if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), runtimeState); err != nil {
			return err
		}
	}
	if strings.TrimSpace(projectRoot) != "" && strings.TrimSpace(runName) != "" {
		if err := MarkRunInactive(projectRoot, runName); err != nil {
			return err
		}
	}
	return nil
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
	if rc == nil {
		return nil
	}
	return repairCompletedRunFinalizationForRun(rc.ProjectRoot, rc.Name, rc.RunDir, rc.TmuxSession)
}

func repairCompletedRunFinalizationByRunDir(runDir string) error {
	if strings.TrimSpace(runDir) == "" || !completedCloseoutReady(runDir) {
		return nil
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return err
	}
	projectRoot := ""
	if meta != nil {
		projectRoot = strings.TrimSpace(meta.ProjectRoot)
	}
	if projectRoot == "" {
		return fmt.Errorf("completed run repair requires run metadata project_root at %s", RunMetadataPath(runDir))
	}
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return err
	}
	runName := strings.TrimSpace(cfg.Name)
	if runName == "" {
		runName = filepath.Base(runDir)
	}
	return repairCompletedRunFinalizationForRun(projectRoot, runName, runDir, resolveRunTmuxSession(projectRoot, runDir, runName))
}

func repairCompletedRunFinalizationForRun(projectRoot, runName, runDir, tmuxSession string) error {
	if strings.TrimSpace(runDir) == "" || !completedCloseoutReady(runDir) {
		return nil
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err == nil && controlState != nil && controlState.GoalState == "completed" {
		return nil
	}
	if strings.TrimSpace(projectRoot) == "" {
		return fmt.Errorf("completed run repair requires project root for %s", runDir)
	}
	if strings.TrimSpace(runName) == "" {
		runName = filepath.Base(runDir)
	}
	if strings.TrimSpace(tmuxSession) == "" {
		tmuxSession = resolveRunTmuxSession(projectRoot, runDir, runName)
	}
	return finalizeCompletedRun(projectRoot, runName, runDir, tmuxSession, finalizeControlRunOptions{killLeasedProcesses: true})
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
	if err := expire("runtime-host"); err != nil {
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
		if holder == "" || holder == "master" || holder == "runtime-host" || shouldSkipControlHolder(skipHolders, holder) {
			continue
		}
		if err := ExpireControlLease(runDir, holder); err != nil {
			return err
		}
	}
	return nil
}

func finalizeCompletedRunFromRuntimeHost(projectRoot, runName, runDir, tmuxSession string) error {
	return finalizeCompletedRun(projectRoot, runName, runDir, tmuxSession, finalizeControlRunOptions{
		killLeasedProcesses: true,
		skipKillHolders:     map[string]bool{"runtime-host": true},
		skipExpireHolders:   map[string]bool{"runtime-host": true},
	})
}

func finalizeCompletedRun(projectRoot, runName, runDir, tmuxSession string, opts finalizeControlRunOptions) error {
	now := time.Now().UTC().Format(time.RFC3339)

	runState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return err
	}
	if runState != nil {
		runState.Active = false
		runState.Phase = "complete"
		if runState.StoppedAt == "" {
			runState.StoppedAt = now
		}
		runState.UpdatedAt = now
		if err := UpsertRunRuntimeState(runDir, *runState); err != nil {
			return err
		}
	}

	killRunPaneProcessTrees(runDir, tmuxSession)
	if err := KillSessionIfExistsInRun(runDir, tmuxSession); err != nil {
		return err
	}

	if err := MarkRunInactive(projectRoot, runName); err != nil {
		return err
	}

	return finalizeControlRun(runDir, "completed", opts)
}

func shouldSkipControlHolder(skipHolders map[string]bool, holder string) bool {
	return skipHolders != nil && skipHolders[holder]
}
