package cli

import (
	"fmt"
	"os"
	"time"
)

// Recover relaunches an existing run in place after tmux/master loss or an explicit stop.
func Recover(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx recover [--run NAME]") {
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
		return fmt.Errorf("usage: goalx recover [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	controlState, err := LoadControlRunState(ControlRunStatePath(rc.RunDir))
	if err != nil {
		return err
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(rc.RunDir))
	if err != nil {
		return err
	}
	startup, err := deriveRunStartupState(rc.RunDir, rc.TmuxSession, controlState, runtimeState)
	if err != nil {
		return err
	}
	if startup.Phase == "bootstrapping" {
		return fmt.Errorf("run %q bootstrap is still in progress; wait for start to finish before recovering it", rc.Name)
	}
	if startup.Phase == "settling" {
		return fmt.Errorf("run %q is still settling; use `goalx wait --run %s master --timeout 30s` or recheck `goalx status/observe` before recovering it", rc.Name, rc.Name)
	}
	if SessionExistsInRun(rc.RunDir, rc.TmuxSession) {
		return fmt.Errorf("run %q is already active (tmux session %s exists); use `goalx wait --run %s master --timeout 30s` or `goalx status/observe` instead of recovering it", rc.Name, rc.TmuxSession, rc.Name)
	}
	if err := requireRunBudgetAvailable(rc.RunDir, rc.Config); err != nil {
		return err
	}
	effectiveCfg, err := configWithSelectionSnapshot(rc.RunDir, rc.Config)
	if err != nil {
		return err
	}
	if err := requireResourceAdmission(rc.RunDir, effectiveCfg.Master.Engine, effectiveCfg.Master.Model, "master recovery"); err != nil {
		return err
	}
	if stopped, lifecycle, err := waitRunStopped(rc.RunDir); err != nil {
		return err
	} else if stopped && lifecycle == "completed" {
		return fmt.Errorf("run %q is completed; start a next phase instead of recovering it", rc.Name)
	}
	if err := EnsureMasterControl(rc.RunDir); err != nil {
		return fmt.Errorf("init master control: %w", err)
	}
	if err := runtimeSupervisor.Stop(rc.RunDir); err != nil {
		return err
	}
	killRunPaneProcessTrees(rc.RunDir, rc.TmuxSession)
	killAllLeasedProcesses(rc.RunDir)

	if err := RefreshRunMemorySeeds(rc.RunDir); err != nil {
		return fmt.Errorf("refresh run memory seeds: %w", err)
	}
	if err := AppendExtractedMemoryProposals(rc.RunDir, time.Now().UTC()); err != nil {
		return fmt.Errorf("append extracted memory proposals: %w", err)
	}
	if err := PromoteMemoryProposals(); err != nil {
		return fmt.Errorf("promote memory proposals: %w", err)
	}
	if tmuxSession, err := ensureRunTmuxLocator(rc.ProjectRoot, rc.RunDir, rc.Name); err != nil {
		return fmt.Errorf("rewrite tmux locator: %w", err)
	} else if tmuxSession != "" {
		rc.TmuxSession = tmuxSession
	}

	if err := relaunchMaster(rc.ProjectRoot, rc.RunDir, rc.TmuxSession, rc.Config); err != nil {
		return err
	}
	if err := PersistPanePIDsFromTmux(rc.RunDir, "master", rc.TmuxSession+":master"); err != nil {
		return fmt.Errorf("persist master pane pid: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if runtimeState == nil {
		runtimeState = &RunRuntimeState{
			Version:   1,
			Run:       rc.Name,
			Mode:      string(rc.Config.Mode),
			StartedAt: now,
		}
	}
	runtimeState.Run = rc.Name
	if runtimeState.Mode == "" {
		runtimeState.Mode = string(rc.Config.Mode)
	}
	if runtimeState.StartedAt == "" {
		runtimeState.StartedAt = now
	}
	runtimeState.Active = true
	runtimeState.StoppedAt = ""
	runtimeState.UpdatedAt = now
	if err := SaveRunRuntimeState(RunRuntimeStatePath(rc.RunDir), runtimeState); err != nil {
		return err
	}

	if controlState == nil {
		controlState = &ControlRunState{Version: 1}
	}
	controlState.GoalState = "open"
	controlState.ContinuityState = "running"
	controlState.UpdatedAt = now
	if err := SaveControlRunState(ControlRunStatePath(rc.RunDir), controlState); err != nil {
		return err
	}

	if err := RegisterActiveRun(rc.ProjectRoot, rc.Config); err != nil {
		return fmt.Errorf("register active run: %w", err)
	}
	if err := setFocusedRun(rc.ProjectRoot, rc.Name); err != nil {
		return fmt.Errorf("focus active run: %w", err)
	}

	checkSec, _ := normalizeRuntimeHostInterval(rc.Config.Master.CheckInterval)
	if _, err := runtimeSupervisor.Start(RuntimeSupervisorStartSpec{
		ProjectRoot: rc.ProjectRoot,
		RunName:     rc.Name,
		RunDir:      rc.RunDir,
		Interval:    time.Duration(checkSec) * time.Second,
	}); err != nil {
		return fmt.Errorf("launch runtime supervisor: %w", err)
	}
	if _, err := RefreshRunGuidance(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
	}

	fmt.Printf("✓ Run '%s' recovered\n", rc.Name)
	fmt.Printf("  tmux session: %s\n", rc.TmuxSession)
	fmt.Printf("  master: %s/%s\n", rc.Config.Master.Engine, rc.Config.Master.Model)
	fmt.Printf("  run dir: %s\n", rc.RunDir)
	fmt.Printf("  attach: goalx attach [--run %s] [master|session-N]\n", rc.Name)
	return nil
}
