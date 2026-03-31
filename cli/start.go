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
	if len(args) > 0 {
		if wantsHelp(args) {
			fmt.Println(launchUsage("start"))
			return nil
		}
		opts, err := parseStartArgs(args)
		if err != nil {
			return err
		}
		return startWithOptions(projectRoot, opts)
	}
	return fmt.Errorf("goalx start requires either an objective with flags or --config PATH for an explicit manual draft")
}

func startWithOptions(projectRoot string, opts startOptions) error {
	var cfg *goalx.Config
	var engines map[string]goalx.EngineConfig
	var selectionSnapshot *SelectionSnapshot

	if opts.ConfigPath != "" {
		resolved, err := ResolveManualDraftConfig(projectRoot, opts.ConfigPath)
		if err != nil {
			return fmt.Errorf("load manual draft config: %w", err)
		}
		cfg = &resolved.Config
		engines = resolved.Engines
		selectionSnapshot = BuildSelectionSnapshot(cfg, resolved.SelectionPolicy, resolved.ExplicitSelection)
	} else {
		resolved, err := resolveLaunchConfig(projectRoot, opts.launchOptions)
		if err != nil {
			return err
		}
		cfg = &resolved.Config
		engines = resolved.Engines
		selectionSnapshot = BuildSelectionSnapshot(cfg, resolved.SelectionPolicy, resolved.ExplicitSelection)
	}

	return startWithConfig(projectRoot, cfg, engines, selectionSnapshot, launchRunMetadataPatch(opts.launchOptions), opts.NoSnapshot)
}

func startResolvedLaunch(projectRoot string, opts launchOptions) error {
	resolved, err := resolveLaunchConfig(projectRoot, opts)
	if err != nil {
		return err
	}
	selectionSnapshot := BuildSelectionSnapshot(&resolved.Config, resolved.SelectionPolicy, resolved.ExplicitSelection)
	return startWithConfig(projectRoot, &resolved.Config, resolved.Engines, selectionSnapshot, launchRunMetadataPatch(opts), opts.NoSnapshot)
}

type startRunState struct {
	projectRoot        string
	runDir             string
	tmuxSession        string
	absProjectRoot     string
	runWorktree        string
	runBranch          string
	runDirCreated      bool
	tmuxCreated        bool
	runWorktreeCreated bool
	runtimeStarted     bool
}

func newStartRunState(projectRoot, runName string) *startRunState {
	runDir := goalx.RunDir(projectRoot, runName)
	tmuxSession := resolveRunTmuxSession(projectRoot, runDir, runName)
	return &startRunState{
		projectRoot:    projectRoot,
		runDir:         runDir,
		tmuxSession:    tmuxSession,
		absProjectRoot: mustAbsPath(projectRoot),
		runWorktree:    RunWorktreePath(runDir),
		runBranch:      fmt.Sprintf("goalx/%s/root", runName),
	}
}

func mustAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func (s *startRunState) cleanup(errp *error) {
	if errp == nil || *errp == nil {
		return
	}
	if s.runtimeStarted {
		if stopErr := runtimeSupervisor.Stop(s.runDir); stopErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cleanup runtime supervisor for %s: %v\n", s.runDir, stopErr)
		}
	}
	if s.tmuxCreated {
		if killErr := KillSessionIfExistsInRun(s.runDir, s.tmuxSession); killErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cleanup tmux session %s: %v\n", s.tmuxSession, killErr)
		}
	}
	if s.runWorktreeCreated {
		if rmErr := RemoveWorktree(s.absProjectRoot, s.runWorktree); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(os.Stderr, "warning: cleanup run worktree %s: %v\n", s.runWorktree, rmErr)
		}
	}
	if s.runWorktreeCreated {
		if exists, branchErr := branchExists(s.absProjectRoot, s.runBranch); branchErr == nil && exists {
			if delErr := DeleteBranch(s.absProjectRoot, s.runBranch); delErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup run branch %s: %v\n", s.runBranch, delErr)
			}
		}
	}
	if s.runDirCreated {
		if rmErr := os.RemoveAll(s.runDir); rmErr != nil && !os.IsNotExist(rmErr) {
			fmt.Fprintf(os.Stderr, "warning: cleanup run dir %s: %v\n", s.runDir, rmErr)
		}
	}
}

func startWithConfig(projectRoot string, cfg *goalx.Config, engines map[string]goalx.EngineConfig, selectionSnapshot *SelectionSnapshot, metaPatch *RunMetadata, noSnapshot bool) (err error) {
	if err := goalx.ValidateConfig(cfg, engines); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	state := newStartRunState(projectRoot, cfg.Name)
	defer state.cleanup(&err)

	if err := ensureStartAvailable(state); err != nil {
		return err
	}
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

	if err := bootstrapStartWorkspace(state, cfg, selectionSnapshot, metaPatch); err != nil {
		return err
	}

	masterCmd, masterPrompt, meta, checkSec, warning, err := bootstrapStartDurables(projectRoot, state, cfg, engines, metaPatch, snapshotCommit)
	if err != nil {
		return err
	}
	if warning != "" {
		fmt.Fprint(os.Stderr, warning)
	}
	if err := submitControlOperationTarget(state.runDir, RunBootstrapOperationKey(), ControlOperationTarget{
		Kind:              ControlOperationKindRunBootstrap,
		State:             ControlOperationStatePreparing,
		Summary:           "launching master runtime",
		PendingConditions: []string{"master_window_ready", "master_pane_pid_persisted"},
	}); err != nil {
		return err
	}
	if err := refreshBoundaryEstablishmentOperation(state.runDir); err != nil {
		return err
	}

	if err := launchStartRuntime(state, cfg, meta, masterCmd, masterPrompt, checkSec); err != nil {
		return err
	}
	return submitControlOperationTarget(state.runDir, RunBootstrapOperationKey(), ControlOperationTarget{
		Kind:    ControlOperationKindRunBootstrap,
		State:   ControlOperationStateCommitted,
		Summary: "run bootstrap committed",
	})
}

func ensureStartAvailable(state *startRunState) error {
	if _, err := os.Stat(state.runDir); err == nil {
		return fmt.Errorf("run directory already exists: %s (use a different name or goalx drop first)", state.runDir)
	}
	if SessionExistsInRun(state.runDir, state.tmuxSession) {
		return fmt.Errorf("tmux session already exists: %s (goalx stop first)", state.tmuxSession)
	}
	return nil
}

func bootstrapStartWorkspace(state *startRunState, cfg *goalx.Config, selectionSnapshot *SelectionSnapshot, metaPatch *RunMetadata) error {
	if err := os.MkdirAll(state.runDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", state.runDir, err)
	}
	state.runDirCreated = true
	dirs := []string{
		filepath.Join(state.runDir, "journals"),
		ReportsDir(state.runDir),
		filepath.Join(state.runDir, "worktrees"),
		filepath.Join(state.projectRoot, ".goalx"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
	}
	if err := EnsureProjectGoalxIgnored(state.projectRoot); err != nil {
		return fmt.Errorf("bootstrap .goalx ignore: %w", err)
	}
	if _, err := ensureRunTmuxLocator(state.projectRoot, state.runDir, cfg.Name); err != nil {
		return fmt.Errorf("write tmux locator: %w", err)
	}
	if err := SaveRunSpec(state.runDir, cfg); err != nil {
		return fmt.Errorf("write run spec: %w", err)
	}
	if selectionSnapshot != nil {
		if err := SaveSelectionSnapshot(state.runDir, selectionSnapshot); err != nil {
			return fmt.Errorf("write selection snapshot: %w", err)
		}
	}

	sourceBaseBranch := ""
	if metaPatch != nil && metaPatch.SourceRun != "" {
		candidate := fmt.Sprintf("goalx/%s/root", goalx.Slugify(metaPatch.SourceRun))
		if ok, _ := branchExists(state.absProjectRoot, candidate); ok {
			sourceBaseBranch = candidate
			fmt.Fprintf(os.Stderr, "✓ Forking from source run worktree: %s\n", candidate)
		}
	}
	if err := CreateWorktree(state.absProjectRoot, state.runWorktree, state.runBranch, sourceBaseBranch); err != nil {
		return fmt.Errorf("create run worktree: %w", err)
	}
	state.runWorktreeCreated = true

	copySource := state.absProjectRoot
	if sourceBaseBranch != "" && metaPatch != nil && metaPatch.SourceRun != "" {
		srcRunDir := goalx.RunDir(state.projectRoot, goalx.Slugify(metaPatch.SourceRun))
		srcWT := RunWorktreePath(srcRunDir)
		if info, statErr := os.Stat(srcWT); statErr == nil && info.IsDir() {
			copySource = srcWT
		}
	}
	if err := CopyGitignoredFiles(copySource, state.runWorktree); err != nil {
		fmt.Fprintf(os.Stderr, "warning: copy gitignored files: %v\n", err)
	}
	return nil
}

func bootstrapStartDurables(projectRoot string, state *startRunState, cfg *goalx.Config, engines map[string]goalx.EngineConfig, metaPatch *RunMetadata, snapshotCommit string) (string, string, *RunMetadata, int, string, error) {
	masterSpec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: cfg.Master.Engine,
		Model:  cfg.Master.Model,
		Effort: cfg.Master.Effort,
	})
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("master engine: %w", err)
	}
	masterCmd := masterSpec.Command
	masterProtocolPath := filepath.Join(state.runDir, "master.md")
	masterPrompt := goalx.ResolvePrompt(engines, cfg.Master.Engine, masterProtocolPath)

	if err := EnsureEngineTrusted(cfg.Master.Engine, state.runWorktree); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("trust bootstrap master: %w", err)
	}

	goalState, err := EnsureGoalState(state.runDir)
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init goal state: %w", err)
	}
	if _, err := EnsureObjectiveContract(state.runDir, cfg.Objective); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init objective contract: %w", err)
	}
	if err := EnsureGoalLog(state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init goal log: %w", err)
	}
	if _, err := EnsureAcceptanceState(state.runDir, cfg, goalState.Version); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init acceptance state: %w", err)
	}
	meta, err := EnsureRunMetadata(state.runDir, projectRoot, cfg.Objective)
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init run metadata: %w", err)
	}
	applyRunMetadataPatch(meta, metaPatch)
	if snapshotCommit != "" {
		meta.SnapshotCommit = snapshotCommit
	}
	charter, err := NewRunCharter(state.runDir, cfg.Name, cfg.Objective, meta)
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init run charter: %w", err)
	}
	if err := SaveRunCharter(RunCharterPath(state.runDir), charter); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("write run charter: %w", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("hash run charter: %w", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(state.runDir), meta); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("write run metadata: %w", err)
	}
	if err := EnsureSuccessCompilation(projectRoot, state.runDir, cfg, meta); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("compile success plane: %w", err)
	}
	if err := ensureExperimentsSurface(state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init experiments surface: %w", err)
	}
	baseRef := strings.TrimSpace(meta.BaseRevision)
	baseExperimentID := ""
	if metaPatch != nil && strings.TrimSpace(metaPatch.SourceRun) != "" {
		sourceState, err := ResolveIntegrationState(projectRoot, goalx.Slugify(metaPatch.SourceRun))
		if err != nil {
			return "", "", nil, 0, "", fmt.Errorf("load source integration state: %w", err)
		}
		if sourceState != nil {
			baseExperimentID = sourceState.CurrentExperimentID
			if strings.TrimSpace(sourceState.CurrentBranch) != "" {
				baseRef = sourceState.CurrentBranch
			}
		}
	}
	if err := initializeRootExperimentLineageWithBase(state.runDir, state.runWorktree, cfg.Name, meta.Intent, baseRef, baseExperimentID); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init root experiment lineage: %w", err)
	}
	if _, err := EnsureArtifactsManifest(state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init artifacts manifest: %w", err)
	}
	if _, err := EnsureCoordinationState(state.runDir, cfg.Objective); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init coordination state: %w", err)
	}
	if err := EnsureMasterControl(state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init master control: %w", err)
	}
	if _, err := EnsureDimensionsState(state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init dimensions state: %w", err)
	}
	if err := GenerateAdapter(cfg.Master.Engine, state.runWorktree, MasterInboxPath(state.runDir), MasterCursorPath(state.runDir)); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("generate master adapter: %w", err)
	}
	fence, err := NewIdentityFence(state.runDir, meta)
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init identity fence: %w", err)
	}
	if err := SaveIdentityFence(IdentityFencePath(state.runDir), fence); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("write identity fence: %w", err)
	}

	masterData, err := buildMasterProtocolData(projectRoot, state.runDir, state.tmuxSession, cfg, engines, masterCmd, meta)
	if err != nil {
		return "", "", nil, 0, "", fmt.Errorf("build master protocol data: %w", err)
	}
	if err := RenderMasterProtocol(masterData, state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("render master protocol: %w", err)
	}
	if err := os.WriteFile(filepath.Join(state.runDir, "master.jsonl"), nil, 0o644); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init master journal: %w", err)
	}
	if _, err := EnsureRuntimeState(state.runDir, cfg); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init runtime state: %w", err)
	}
	if _, err := EnsureSessionsRuntimeState(state.runDir); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("init session runtime state: %w", err)
	}
	if err := RegisterActiveRun(projectRoot, cfg); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("register active run: %w", err)
	}
	if err := setFocusedRun(projectRoot, cfg.Name); err != nil {
		return "", "", nil, 0, "", fmt.Errorf("focus active run: %w", err)
	}

	checkSec, warning := normalizeRuntimeHostInterval(cfg.Master.CheckInterval)
	return masterCmd, masterPrompt, meta, checkSec, warning, nil
}

func launchStartRuntime(state *startRunState, cfg *goalx.Config, meta *RunMetadata, masterCmd, masterPrompt string, checkSec int) error {
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	masterLeaseTTL := time.Duration(checkSec) * time.Second * 2
	masterLaunch := buildMasterLaunchCommand(goalxBin, cfg.Name, state.runDir, meta.RunID, meta.Epoch, masterLeaseTTL, masterCmd, masterPrompt)
	if err := NewSessionWithCommandInRun(state.runDir, state.tmuxSession, "master", state.runWorktree, masterLaunch); err != nil {
		return fmt.Errorf("tmux new-session: %w", err)
	}
	state.tmuxCreated = true
	if err := PersistPanePIDsFromTmux(state.runDir, "master", state.tmuxSession+":master"); err != nil {
		return fmt.Errorf("persist master pane pid: %w", err)
	}

	if _, err := runtimeSupervisor.Start(RuntimeSupervisorStartSpec{
		ProjectRoot: state.projectRoot,
		RunName:     cfg.Name,
		RunDir:      state.runDir,
		Interval:    time.Duration(checkSec) * time.Second,
	}); err != nil {
		return fmt.Errorf("launch runtime supervisor: %w", err)
	}
	state.runtimeStarted = true
	if _, err := RefreshRunGuidance(state.projectRoot, cfg.Name, state.runDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
	}

	fmt.Printf("✓ Run '%s' started\n", cfg.Name)
	fmt.Printf("  tmux session: %s\n", state.tmuxSession)
	fmt.Printf("  master: %s/%s\n", cfg.Master.Engine, cfg.Master.Model)
	fmt.Printf("  run dir: %s\n", state.runDir)
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
	if patch.Intent != "" {
		meta.Intent = patch.Intent
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

func launchRunMetadataPatch(opts launchOptions) *RunMetadata {
	if strings.TrimSpace(opts.Intent) == "" {
		return nil
	}
	return &RunMetadata{Intent: strings.TrimSpace(opts.Intent)}
}

func buildMasterProtocolData(projectRoot, runDir, tmuxSession string, cfg *goalx.Config, engines map[string]goalx.EngineConfig, masterCmd string, meta *RunMetadata) (ProtocolData, error) {
	if cfg == nil {
		return ProtocolData{}, fmt.Errorf("run config is nil")
	}
	selectionSnapshotPath := ""
	selectionPolicy := goalx.DeriveSelectionPolicy(cfg)
	if selectionSnapshot, err := LoadSelectionSnapshot(SelectionSnapshotPath(runDir)); err != nil {
		return ProtocolData{}, fmt.Errorf("load selection snapshot: %w", err)
	} else if selectionSnapshot != nil {
		selectionSnapshotPath = SelectionSnapshotPath(runDir)
		if !selectionPolicyEmpty(selectionSnapshot.Policy) {
			selectionPolicy = copySelectionPolicy(selectionSnapshot.Policy)
		}
	}
	dimensionsCatalog := cfg.DimensionCatalog()
	sessionDataList, err := buildSessionDataList(runDir, cfg, engines, dimensionsCatalog)
	if err != nil {
		return ProtocolData{}, err
	}
	intent := ""
	runStartedAt := ""
	if meta != nil {
		intent = meta.Intent
		runStartedAt = meta.StartedAt
	}
	return ProtocolData{
		RunName:                cfg.Name,
		Objective:              cfg.Objective,
		Description:            cfg.Description,
		Intent:                 intent,
		CurrentTime:            time.Now().UTC().Format(time.RFC3339),
		RunStartedAt:           runStartedAt,
		ExperimentsLogPath:     ExperimentsLogPath(runDir),
		Mode:                   cfg.Mode,
		Engines:                engines,
		Sessions:               sessionDataList,
		Master:                 cfg.Master,
		Roles:                  cfg.Roles,
		Budget:                 cfg.Budget,
		Target:                 cfg.Target,
		Context:                cfg.Context,
		Preferences:            cfg.Preferences,
		DimensionsCatalog:      dimensionsCatalog,
		TmuxSession:            tmuxSession,
		ProjectRoot:            RunWorktreePath(runDir),
		SummaryPath:            SummaryPath(runDir),
		CharterPath:            RunCharterPath(runDir),
		ObjectiveContractPath:  ObjectiveContractPath(runDir),
		GoalPath:               GoalPath(runDir),
		GoalLogPath:            GoalLogPath(runDir),
		IntegrationStatePath:   IntegrationStatePath(runDir),
		IdentityFencePath:      IdentityFencePath(runDir),
		AcceptanceNotesPath:    existingProtocolPath(AcceptanceNotesPath(runDir)),
		AcceptanceStatePath:    AcceptanceStatePath(runDir),
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
			LivenessPath:           LivenessPath(runDir),
		WorktreeSnapshotPath:   WorktreeSnapshotPath(runDir),
		ControlRemindersPath:   ControlRemindersPath(runDir),
		ControlDeliveriesPath:  ControlDeliveriesPath(runDir),
		SelectionSnapshotPath:  selectionSnapshotPath,
		SelectionPolicy:        selectionPolicy,
		MasterJournalPath:      filepath.Join(runDir, "master.jsonl"),
		StatusPath:             RunStatusPath(runDir),
		EngineCommand:          masterCmd,
	}, nil
}

func ensureExperimentsSurface(runDir string) error {
	return ensureEmptyFile(ExperimentsLogPath(runDir))
}
