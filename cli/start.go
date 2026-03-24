package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

// Start executes the full goalx start workflow.
func Start(projectRoot string, args []string) (err error) {
	var cfg *goalx.Config
	var engines map[string]goalx.EngineConfig
	var opts startOptions
	if len(args) > 0 {
		if wantsHelp(args) {
			fmt.Println(launchUsage("start"))
			return nil
		}
		opts, err = parseStartArgs(args)
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
	return startWithConfig(projectRoot, cfg, engines, nil, opts.NoSnapshot)
}

func startWithConfig(projectRoot string, cfg *goalx.Config, engines map[string]goalx.EngineConfig, metaPatch *RunMetadata, noSnapshot bool) (err error) {
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
	runWT := RunWorktreePath(runDir)
	runBranch := fmt.Sprintf("goalx/%s/root", cfg.Name)
	tmuxCreated := false
	runWorktreeCreated := false
	defer func() {
		if err == nil {
			return
		}
		if tmuxCreated {
			if killErr := KillSession(tmuxSess); killErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup tmux session %s: %v\n", tmuxSess, killErr)
			}
		}
		if runWorktreeCreated {
			if rmErr := RemoveWorktree(absProjectRoot, runWT); rmErr != nil && !os.IsNotExist(rmErr) {
				fmt.Fprintf(os.Stderr, "warning: cleanup run worktree %s: %v\n", runWT, rmErr)
			}
		}
		if exists, branchErr := branchExists(absProjectRoot, runBranch); branchErr == nil && exists {
			if delErr := DeleteBranch(absProjectRoot, runBranch); delErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup run branch %s: %v\n", runBranch, delErr)
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

	// 5b. Snapshot tracked changes so the run worktree starts from the current source-root state.
	snapshotCommit := ""
	if dirty, err := hasDirtyWorktree(projectRoot); err == nil && dirty {
		if noSnapshot {
			fmt.Fprintln(os.Stderr, "⚠ Working tree has uncommitted changes (--no-snapshot: skipping auto-commit)")
		} else {
			snapshotCommit, err = autoSnapshotTrackedChanges(projectRoot, cfg.Name)
			if err != nil {
				return err
			}
		}
	}

	// 6. Create run directory structure
	dirs := []string{
		runDir,
		filepath.Join(runDir, "journals"),
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
	// If this run continues from a previous run (--from), fork from its worktree branch
	// instead of HEAD. This inherits the source run's code changes.
	var sourceBaseBranch string
	if metaPatch != nil && metaPatch.SourceRun != "" {
		candidate := fmt.Sprintf("goalx/%s/root", goalx.Slugify(metaPatch.SourceRun))
		if ok, _ := branchExists(absProjectRoot, candidate); ok {
			sourceBaseBranch = candidate
			fmt.Fprintf(os.Stderr, "✓ Forking from source run worktree: %s\n", candidate)
		}
	}
	if err := CreateWorktree(absProjectRoot, runWT, runBranch, sourceBaseBranch); err != nil {
		return fmt.Errorf("create run worktree: %w", err)
	}
	runWorktreeCreated = true
	// Copy gitignored files from source run worktree if forking, otherwise from sourceRoot
	copySource := absProjectRoot
	if sourceBaseBranch != "" {
		// Source run's worktree might still exist — use it for gitignored files
		if metaPatch != nil && metaPatch.SourceRun != "" {
			srcRunDir := goalx.RunDir(projectRoot, goalx.Slugify(metaPatch.SourceRun))
			srcWT := RunWorktreePath(srcRunDir)
			if info, statErr := os.Stat(srcWT); statErr == nil && info.IsDir() {
				copySource = srcWT
			}
		}
	}
	if err := CopyGitignoredFiles(copySource, runWT); err != nil {
		fmt.Fprintf(os.Stderr, "warning: copy gitignored files: %v\n", err)
	}

	// 9. Resolve master engine command
	masterSpec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: cfg.Master.Engine,
		Model:  cfg.Master.Model,
		Effort: cfg.Master.Effort,
	})
	if err != nil {
		return fmt.Errorf("master engine: %w", err)
	}
	masterCmd := masterSpec.Command
	masterProtocolPath := filepath.Join(runDir, "master.md")
	masterPrompt := goalx.ResolvePrompt(engines, cfg.Master.Engine, masterProtocolPath)
	goalPath := GoalPath(runDir)
	goalLogPath := GoalLogPath(runDir)
	acceptanceStatePath := AcceptanceStatePath(runDir)
	statusPath := ProjectStatusCachePath(projectRoot)

	if err := EnsureEngineTrusted(cfg.Master.Engine, runWT); err != nil {
		return fmt.Errorf("trust bootstrap master: %w", err)
	}

	// Initialize durable goal boundary and immutable run identity before rendering protocols.
	goalState, err := EnsureGoalState(runDir)
	if err != nil {
		return fmt.Errorf("init goal state: %w", err)
	}
	if err := EnsureGoalLog(runDir); err != nil {
		return fmt.Errorf("init goal log: %w", err)
	}
	if _, err := EnsureAcceptanceState(runDir, cfg, goalState.Version); err != nil {
		return fmt.Errorf("init acceptance state: %w", err)
	}
	meta, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective)
	if err != nil {
		return fmt.Errorf("init run metadata: %w", err)
	}
	applyRunMetadataPatch(meta, metaPatch)
	if snapshotCommit != "" {
		meta.SnapshotCommit = snapshotCommit
	}
	charter, err := NewRunCharter(runDir, cfg.Name, cfg.Objective, meta)
	if err != nil {
		return fmt.Errorf("init run charter: %w", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		return fmt.Errorf("write run charter: %w", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		return fmt.Errorf("hash run charter: %w", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		return fmt.Errorf("write run metadata: %w", err)
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
	if err := GenerateAdapter(cfg.Master.Engine, runWT, MasterInboxPath(runDir), MasterCursorPath(runDir)); err != nil {
		return fmt.Errorf("generate master adapter: %w", err)
	}
	if err := SaveLaunchEnvSnapshot(ControlLaunchEnvPath(runDir), CaptureCurrentLaunchEnvSnapshot()); err != nil {
		return fmt.Errorf("write launch env snapshot: %w", err)
	}
	fence, err := NewIdentityFence(runDir, meta)
	if err != nil {
		return fmt.Errorf("init identity fence: %w", err)
	}
	if err := SaveIdentityFence(IdentityFencePath(runDir), fence); err != nil {
		return fmt.Errorf("write identity fence: %w", err)
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
		ProjectRoot:            runWT,
		SummaryPath:            filepath.Join(runDir, "summary.md"),
		CharterPath:            RunCharterPath(runDir),
		GoalPath:               goalPath,
		GoalLogPath:            goalLogPath,
		IdentityFencePath:      IdentityFencePath(runDir),
		AcceptanceNotesPath:    existingProtocolPath(AcceptanceNotesPath(runDir)),
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
		MasterLeasePath:        ControlLeasePath(runDir, "master"),
		SidecarLeasePath:       ControlLeasePath(runDir, "sidecar"),
		LivenessPath:           LivenessPath(runDir),
		WorktreeSnapshotPath:   WorktreeSnapshotPath(runDir),
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
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		return fmt.Errorf("init runtime state: %w", err)
	}
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		return fmt.Errorf("init session runtime state: %w", err)
	}
	if err := RegisterActiveRun(projectRoot, cfg); err != nil {
		return fmt.Errorf("register active run: %w", err)
	}

	checkSec, warning := normalizeSidecarInterval(cfg.Master.CheckInterval)
	if warning != "" {
		fmt.Fprint(os.Stderr, warning)
	}

	// Launch master
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	launchEnv, err := RequireLaunchEnvSnapshot(runDir)
	if err != nil {
		return fmt.Errorf("load launch env snapshot: %w", err)
	}
	masterLeaseTTL := time.Duration(checkSec) * time.Second * 2
	masterLaunch := buildMasterLaunchCommandWithEnv(launchEnv.Env, goalxBin, cfg.Name, runDir, meta.RunID, meta.Epoch, masterLeaseTTL, masterCmd, masterPrompt)
	if err := NewSessionWithCommand(tmuxSess, "master", runWT, masterLaunch); err != nil {
		return fmt.Errorf("tmux new-session: %w", err)
	}
	tmuxCreated = true
	if err := PersistPanePIDsFromTmux(runDir, "master", tmuxSess+":master"); err != nil {
		return fmt.Errorf("persist master pane pid: %w", err)
	}

	// Launch the run-scoped sidecar for lease renewal and supervision.
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

func autoSnapshotTrackedChanges(projectRoot, runName string) (string, error) {
	addOut, err := exec.Command("git", "-C", projectRoot, "add", "-u").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git add -u: %w: %s", err, strings.TrimSpace(string(addOut)))
	}
	if err := exec.Command("git", "-C", projectRoot, "diff", "--cached", "--quiet").Run(); err == nil {
		fmt.Fprintln(os.Stderr, "ℹ Auto-snapshot: no tracked changes to commit")
		return "", nil
	} else {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return "", fmt.Errorf("git diff --cached --quiet: %w", err)
		}
	}

	commitOut, err := exec.Command("git", "-C", projectRoot, "commit", "-m", fmt.Sprintf("goalx: snapshot before %s", runName)).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git commit auto-snapshot: %w: %s", err, strings.TrimSpace(string(commitOut)))
	}
	snapshotCommit, err := gitHeadRevision(projectRoot)
	if err != nil {
		return "", err
	}
	shortCommit := snapshotCommit
	if len(shortCommit) > 8 {
		shortCommit = shortCommit[:8]
	}
	fmt.Fprintf(os.Stderr, "✓ Auto-snapshot: %s\n", shortCommit)
	return snapshotCommit, nil
}

func applyRunMetadataPatch(meta *RunMetadata, patch *RunMetadata) {
	if meta == nil || patch == nil {
		return
	}
	if patch.RootRunID != "" {
		meta.RootRunID = patch.RootRunID
	}
	if patch.PhaseKind != "" {
		meta.PhaseKind = patch.PhaseKind
	}
	if patch.SourceRun != "" {
		meta.SourceRun = patch.SourceRun
	}
	if patch.SourcePhase != "" {
		meta.SourcePhase = patch.SourcePhase
	}
	if patch.ParentRun != "" {
		meta.ParentRun = patch.ParentRun
	}
}
