package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	goalx "github.com/vonbai/goalx"
)

// Start executes the full goalx start workflow.
func Start(projectRoot string, args []string) (err error) {
	var cfg *goalx.Config
	var engines map[string]goalx.EngineConfig
	if len(args) > 0 {
		if wantsHelp(args) {
			fmt.Println(launchUsage("start"))
			return nil
		}
		opts, err := parseStartArgs(args)
		if err != nil {
			return err
		}
		if opts.ConfigPath != "" {
			cfg, engines, err = LoadManualDraftConfig(projectRoot, opts.ConfigPath)
			if err != nil {
				return fmt.Errorf("load manual draft config: %w", err)
			}
		} else {
			cfg, err = buildLaunchConfig(projectRoot, opts.launchOptions)
			if err != nil {
				return err
			}
			_, engines, err = goalx.LoadRawBaseConfig(projectRoot)
			if err != nil {
				return fmt.Errorf("load base config: %w", err)
			}
		}
	} else {
		return fmt.Errorf("goalx start requires either an objective with flags or --config PATH for an explicit manual draft")
	}
	return startWithConfig(projectRoot, cfg, engines, nil)
}

func startWithConfig(projectRoot string, cfg *goalx.Config, engines map[string]goalx.EngineConfig, metaPatch *RunMetadata) (err error) {
	// 2. Auto-generate name if missing
	if cfg.Name == "" {
		cfg.Name = goalx.Slugify(cfg.Objective)
	}

	// 3. Validate before any side effects
	if err := goalx.ValidateConfig(cfg, engines); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 4. Compute paths
	runDir := goalx.RunDir(projectRoot, cfg.Name)
	tmuxSess := goalx.TmuxSessionName(projectRoot, cfg.Name)
	absProjectRoot, _ := filepath.Abs(projectRoot)
	tmuxCreated := false
	defer func() {
		if err == nil {
			return
		}
		if tmuxCreated {
			if killErr := KillSession(tmuxSess); killErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup tmux session %s: %v\n", tmuxSess, killErr)
			}
		}
		if rmErr := os.RemoveAll(runDir); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(os.Stderr, "warning: cleanup run dir %s: %v\n", runDir, rmErr)
		}
	}()

	// 5. Check conflicts
	if _, err := os.Stat(runDir); err == nil {
		return fmt.Errorf("run directory already exists: %s (use a different name or goalx drop first)", runDir)
	}
	if SessionExists(tmuxSess) {
		return fmt.Errorf("tmux session already exists: %s (goalx stop first)", tmuxSess)
	}

	// 5b. Warn if working tree has uncommitted changes (future session worktrees are created from HEAD)
	if dirty, err := hasDirtyWorktree(projectRoot); err == nil && dirty {
		fmt.Fprintln(os.Stderr, "⚠ Working tree has uncommitted changes that won't be visible to future sessions.")
		fmt.Fprintln(os.Stderr, "  goalx add creates worktrees from the latest commit (HEAD).")
		fmt.Fprintln(os.Stderr, "  Consider: git add -A && git commit -m 'wip' before letting the master spawn workers.")
		fmt.Fprintln(os.Stderr, "")
	}

	// 6. Create run directory structure
	dirs := []string{
		runDir,
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "guidance"),
		filepath.Join(runDir, "worktrees"),
		filepath.Join(projectRoot, ".goalx"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	if err := EnsureProjectGoalxIgnored(projectRoot); err != nil {
		return fmt.Errorf("bootstrap .goalx ignore: %w", err)
	}

	// 8. Persist immutable run spec
	if err := SaveRunSpec(runDir, cfg); err != nil {
		return fmt.Errorf("write run spec: %w", err)
	}

	// 9. Resolve master engine command
	masterCmd, err := goalx.ResolveEngineCommand(engines, cfg.Master.Engine, cfg.Master.Model)
	if err != nil {
		return fmt.Errorf("master engine: %w", err)
	}
	masterProtocolPath := filepath.Join(runDir, "master.md")
	masterPrompt := goalx.ResolvePrompt(engines, cfg.Master.Engine, masterProtocolPath)
	goalContractPath := GoalContractPath(runDir)
	acceptancePath := AcceptanceChecklistPath(runDir)
	acceptanceStatePath := AcceptanceStatePath(runDir)
	statusPath := ProjectStatusCachePath(projectRoot)

	if err := EnsureEngineTrusted(cfg.Master.Engine, absProjectRoot); err != nil {
		return fmt.Errorf("trust bootstrap master: %w", err)
	}
	if err := GenerateMasterAdapter(cfg.Master.Engine, absProjectRoot, RunRuntimeStatePath(runDir), CompletionStatePath(runDir)); err != nil {
		return fmt.Errorf("generate master adapter: %w", err)
	}

	// 11. Render protocols
	masterData := ProtocolData{
		RunName:                cfg.Name,
		Objective:              cfg.Objective,
		Description:            cfg.Description,
		Mode:                   cfg.Mode,
		Engines:                engines,
		Master:                 cfg.Master,
		Harness:                cfg.Harness,
		Budget:                 cfg.Budget,
		Target:                 cfg.Target,
		Context:                cfg.Context,
		Preferences:            cfg.Preferences,
		TmuxSession:            tmuxSess,
		ProjectRoot:            absProjectRoot,
		SummaryPath:            filepath.Join(runDir, "summary.md"),
		GoalContractPath:       goalContractPath,
		AcceptancePath:         acceptancePath,
		AcceptanceStatePath:    acceptanceStatePath,
		CompletionProofPath:    CompletionStatePath(runDir),
		RunStatePath:           RunRuntimeStatePath(runDir),
		SessionsStatePath:      SessionsRuntimeStatePath(runDir),
		ProjectRegistryPath:    ProjectRegistryPath(projectRoot),
		RunMetadataPath:        RunMetadataPath(runDir),
		CoordinationPath:       CoordinationPath(runDir),
		MasterInboxPath:        MasterInboxPath(runDir),
		MasterCursorPath:       MasterCursorPath(runDir),
		ControlRunIdentityPath: ControlRunIdentityPath(runDir),
		ControlRunStatePath:    ControlRunStatePath(runDir),
		ControlEventsPath:      ControlEventsPath(runDir),
		MasterLeasePath:        ControlLeasePath(runDir, "master"),
		SidecarLeasePath:       ControlLeasePath(runDir, "sidecar"),
		ControlRemindersPath:   ControlRemindersPath(runDir),
		ControlDeliveriesPath:  ControlDeliveriesPath(runDir),
		MasterJournalPath:      filepath.Join(runDir, "master.jsonl"),
		StatusPath:             statusPath,
		EngineCommand:          masterCmd,
	}
	if err := RenderMasterProtocol(masterData, runDir); err != nil {
		return fmt.Errorf("render master protocol: %w", err)
	}

	// 12. Initialize master journal
	if err := os.WriteFile(filepath.Join(runDir, "master.jsonl"), nil, 0644); err != nil {
		return fmt.Errorf("init master journal: %w", err)
	}
	if err := os.WriteFile(acceptancePath, nil, 0644); err != nil {
		return fmt.Errorf("init acceptance checklist: %w", err)
	}
	if _, err := EnsureGoalContractState(runDir, cfg.Objective); err != nil {
		return fmt.Errorf("init goal contract: %w", err)
	}
	if _, err := EnsureAcceptanceState(runDir, cfg); err != nil {
		return fmt.Errorf("init acceptance state: %w", err)
	}
	meta, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective)
	if err != nil {
		return fmt.Errorf("init run metadata: %w", err)
	}
	if metaPatch != nil {
		if metaPatch.PhaseKind != "" {
			meta.PhaseKind = metaPatch.PhaseKind
		}
		if metaPatch.SourceRun != "" {
			meta.SourceRun = metaPatch.SourceRun
		}
		if metaPatch.SourcePhase != "" {
			meta.SourcePhase = metaPatch.SourcePhase
		}
		if metaPatch.ParentRun != "" {
			meta.ParentRun = metaPatch.ParentRun
		}
		if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
			return fmt.Errorf("write run metadata: %w", err)
		}
	}
	if _, err := EnsureArtifactsManifest(runDir); err != nil {
		return fmt.Errorf("init artifacts manifest: %w", err)
	}
	if _, err := EnsureCoordinationState(runDir, cfg.Objective); err != nil {
		return fmt.Errorf("init coordination state: %w", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		return fmt.Errorf("init master control: %w", err)
	}
	runState, err := EnsureRuntimeState(runDir, cfg)
	if err != nil {
		return fmt.Errorf("init runtime state: %w", err)
	}
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		return fmt.Errorf("init session runtime state: %w", err)
	}
	if err := RegisterActiveRun(projectRoot, cfg); err != nil {
		return fmt.Errorf("register active run: %w", err)
	}
	if err := syncProjectStatusCache(projectRoot, runState); err != nil {
		return fmt.Errorf("init project status cache: %w", err)
	}

	// 13. Create tmux session (first window = "master")
	if err := NewSession(tmuxSess, "master"); err != nil {
		return fmt.Errorf("tmux new-session: %w", err)
	}
	tmuxCreated = true

	// Set master working directory to project root
	if err := SendKeys(tmuxSess+":master", "cd "+absProjectRoot); err != nil {
		return fmt.Errorf("set master cwd: %w", err)
	}

	// Launch master
	masterLaunch := fmt.Sprintf("%s %q", masterCmd, masterPrompt)
	if err := SendKeys(tmuxSess+":master", masterLaunch); err != nil {
		return fmt.Errorf("launch master: %w", err)
	}

	// Launch the run-scoped sidecar for lease renewal and supervision.
	checkSec, warning := normalizeSidecarInterval(cfg.Master.CheckInterval)
	if warning != "" {
		fmt.Fprint(os.Stderr, warning)
	}
	if err := launchRunSidecar(projectRoot, cfg.Name, time.Duration(checkSec)*time.Second); err != nil {
		return fmt.Errorf("launch sidecar: %w", err)
	}

	// 14. Print status
	fmt.Printf("✓ Run '%s' started\n", cfg.Name)
	fmt.Printf("  tmux session: %s\n", tmuxSess)
	fmt.Printf("  master: %s/%s\n", cfg.Master.Engine, cfg.Master.Model)
	fmt.Printf("  run dir: %s\n", runDir)
	fmt.Printf("  attach: goalx attach [--run %s] [master|session-N]\n", cfg.Name)
	return nil
}
