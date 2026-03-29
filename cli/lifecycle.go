package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

func Park(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx park [--run NAME] <session-name>") {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: goalx park [--run NAME] <session-name>")
	}
	sessionName := rest[0]

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	idx, err := parseSessionIndex(sessionName)
	if err != nil {
		return err
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session %q out of range for run %q", sessionName, rc.Name)
	}
	sessionIdentity, err := RequireSessionIdentity(rc.RunDir, sessionName)
	if err != nil {
		return fmt.Errorf("load session identity: %w", err)
	}

	sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load session runtime state: %w", err)
	}
	current := sessionState.Sessions[sessionName]
	digest := inferSessionCoordination(JournalPath(rc.RunDir, sessionName))
	scope := scopeOrFallback(digest.Scope, current.OwnerScope)
	blockedBy := scopeOrFallback(digest.BlockedBy, current.BlockedBy)
	lastRound := digest.LastRound
	if lastRound == 0 {
		lastRound = current.LastRound
	}
	worktreePath := resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, sessionName, sessionState)
	snapshot, err := SnapshotSessionRuntime(rc.RunDir, sessionName, worktreePath)
	if err != nil {
		return fmt.Errorf("snapshot session runtime: %w", err)
	}
	snapshot.State = "parked"
	snapshot.Mode = sessionIdentity.Mode
	snapshot.Branch = resolvedSessionBranch(rc.RunDir, rc.Config.Name, sessionName, sessionState)
	snapshot.OwnerScope = scope
	snapshot.BlockedBy = blockedBy
	if lastRound > 0 {
		snapshot.LastRound = lastRound
	}

	if SessionExists(rc.TmuxSession) {
		windowName, err := resolveWindowName(rc.Name, sessionName)
		if err != nil {
			return err
		}
		if WindowExists(rc.TmuxSession, windowName) {
			if err := KillWindow(rc.TmuxSession, windowName); err != nil {
				return fmt.Errorf("kill window %s: %w", windowName, err)
			}
		}
		if _, err := AppendMasterInboxMessage(rc.RunDir, "session_parked", "goalx park", fmt.Sprintf("%s was parked for reuse.", sessionName)); err == nil {
			_, _ = DeliverControlNudge(rc.RunDir, "session-parked:"+sessionName, "session-parked:"+sessionName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudgeDetailed)
		}
	}
	_ = ExpireControlLease(rc.RunDir, sessionName)
	if err := UpsertSessionRuntimeState(rc.RunDir, snapshot); err != nil {
		return fmt.Errorf("update session runtime state: %w", err)
	}
	if err := RefreshRunGuidance(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
	}

	fmt.Printf("Parked %s in run '%s'\n", sessionName, rc.Name)
	return nil
}

type sessionCoordinationDigest struct {
	State     string
	Scope     string
	BlockedBy string
	LastRound int
}

func Resume(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx resume [--run NAME] <session-name>") {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: goalx resume [--run NAME] <session-name>")
	}
	sessionName := rest[0]

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if !SessionExists(rc.TmuxSession) {
		return fmt.Errorf("run '%s' is not active (no tmux session)", rc.Name)
	}
	if err := requireRunBudgetAvailable(rc.RunDir, rc.Config); err != nil {
		return err
	}
	absProjectRoot, _ := filepath.Abs(rc.ProjectRoot)

	idx, err := parseSessionIndex(sessionName)
	if err != nil {
		return err
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session %q out of range for run %q", sessionName, rc.Name)
	}
	windowName, err := resolveWindowName(rc.Name, sessionName)
	if err != nil {
		return err
	}
	if WindowExists(rc.TmuxSession, windowName) {
		return fmt.Errorf("%s is already active", sessionName)
	}

	sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load session runtime state: %w", err)
	}
	wtPath := resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, sessionName, sessionState)
	workdir := sessionWorkdir(rc.RunDir, rc.Config.Name, sessionName, sessionState)
	if info, err := os.Stat(workdir); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("%s is not a directory", workdir)
		}
		return fmt.Errorf("resume %s requires existing workdir: %w", sessionName, err)
	}
	if err := EnsureSessionControl(rc.RunDir, sessionName); err != nil {
		return fmt.Errorf("init session control: %w", err)
	}
	sessionIdentity, err := RequireSessionIdentity(rc.RunDir, sessionName)
	if err != nil {
		return fmt.Errorf("load session identity: %w", err)
	}

	engines, err := loadEngineCatalog(rc.ProjectRoot)
	if err != nil {
		return fmt.Errorf("load config for engine resolution: %w", err)
	}
	dimensionsCatalog, err := loadDimensionCatalog(rc.ProjectRoot)
	if err != nil {
		return fmt.Errorf("load dimension catalog: %w", err)
	}
	spec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: sessionIdentity.Engine,
		Model:  sessionIdentity.Model,
		Effort: sessionIdentity.RequestedEffort,
	})
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	engineCmd := spec.Command
	if _, err := EnsureDimensionsState(rc.RunDir); err != nil {
		return fmt.Errorf("init dimensions state: %w", err)
	}

	sessionDataList, err := buildSessionDataList(rc.RunDir, rc.Config, engines, dimensionsCatalog)
	if err != nil {
		return fmt.Errorf("build session roster: %w", err)
	}
	subData := ProtocolData{
		RunName:                   rc.Config.Name,
		Objective:                 rc.Config.Objective,
		Mode:                      goalx.Mode(sessionIdentity.Mode),
		Engine:                    sessionIdentity.Engine,
		Sessions:                  sessionDataList,
		Target:                    sessionIdentity.Target,
		LocalValidationCommand:    sessionIdentity.LocalValidationCommand,
		Context:                   rc.Config.Context,
		Budget:                    rc.Config.Budget,
		SessionName:               sessionName,
		SessionIndex:              idx - 1,
		CurrentDimensions:         CurrentSessionDimensions(rc.RunDir, sessionName, sessionIdentity.Dimensions),
		JournalPath:               JournalPath(rc.RunDir, sessionName),
		CharterPath:               RunCharterPath(rc.RunDir),
		SessionIdentityPath:       SessionIdentityPath(rc.RunDir, sessionName),
		SessionInboxPath:          ControlInboxPath(rc.RunDir, sessionName),
		SessionCursorPath:         SessionCursorPath(rc.RunDir, sessionName),
		WorktreePath:              wtPath,
		ObjectiveContractPath:     ObjectiveContractPath(rc.RunDir),
		GoalPath:                  GoalPath(rc.RunDir),
		GoalLogPath:               GoalLogPath(rc.RunDir),
		IdentityFencePath:         IdentityFencePath(rc.RunDir),
		AcceptanceNotesPath:       existingProtocolPath(AcceptanceNotesPath(rc.RunDir)),
		AcceptanceStatePath:       AcceptanceStatePath(rc.RunDir),
		CompletionProofPath:       CompletionStatePath(rc.RunDir),
		RunStatePath:              RunRuntimeStatePath(rc.RunDir),
		SessionsStatePath:         SessionsRuntimeStatePath(rc.RunDir),
		ProjectRegistryPath:       ProjectRegistryPath(rc.ProjectRoot),
		ProjectRoot:               absProjectRoot,
		RunWorktreePath:           RunWorktreePath(rc.RunDir),
		SessionBaseBranchSelector: sessionIdentity.BaseBranchSelector,
		SessionBaseBranch:         sessionIdentity.BaseBranch,
	}
	if err := RenderSubagentProtocol(subData, rc.RunDir, idx-1); err != nil {
		return fmt.Errorf("render protocol: %w", err)
	}
	protocolPath := filepath.Join(rc.RunDir, sessionNameToProgramFile(idx))
	prompt := goalx.ResolvePrompt(engines, sessionIdentity.Engine, protocolPath)
	meta, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective)
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	checkSec, _ := normalizeSidecarInterval(rc.Config.Master.CheckInterval)
	sessionLeaseTTL := time.Duration(checkSec) * time.Second * 2

	launchCmd := buildLeaseWrappedLaunchCommand(goalxBin, rc.Name, rc.RunDir, sessionName, meta.RunID, meta.Epoch, sessionLeaseTTL, engineCmd, prompt)
	if err := NewWindowWithCommand(rc.TmuxSession, windowName, workdir, launchCmd); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}
	if err := waitForSessionLaunchReady(rc.TmuxSession, sessionName, windowName, sessionIdentity.Engine); err != nil {
		_ = cleanupSessionWindow(rc.TmuxSession, windowName)
		return err
	}

	current := sessionState.Sessions[sessionName]
	ownerScope := scopeOrFallback(current.OwnerScope, inferSessionCoordination(JournalPath(rc.RunDir, sessionName)).Scope, sessionName)
	if err := UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
		Name:         sessionName,
		State:        "active",
		Mode:         sessionIdentity.Mode,
		Branch:       resolvedSessionBranch(rc.RunDir, rc.Config.Name, sessionName, sessionState),
		WorktreePath: wtPath,
		OwnerScope:   ownerScope,
	}); err != nil {
		return fmt.Errorf("update session runtime state: %w", err)
	}
	if _, err := AppendMasterInboxMessage(rc.RunDir, "session_resumed", "goalx resume", fmt.Sprintf("%s was resumed for reuse.", sessionName)); err == nil {
		_, _ = DeliverControlNudge(rc.RunDir, "session-resumed:"+sessionName, "session-resumed:"+sessionName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudgeDetailed)
	}
	if err := RefreshRunGuidance(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
	}

	fmt.Printf("Resumed %s in run '%s'\n", sessionName, rc.Name)
	return nil
}

func Replace(projectRoot string, args []string) (err error) {
	const usage = "usage: goalx replace [--run NAME] <session-name> [--mode MODE] [--engine ENGINE] [--model MODEL] [--effort LEVEL] [--dimension SPEC]..."
	if printUsageIfHelp(args, usage) {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		return fmt.Errorf(usage)
	}

	oldSessionName := strings.TrimSpace(rest[0])
	if oldSessionName == "" {
		return fmt.Errorf(usage)
	}
	opts := goalx.SessionConfig{}
	var explicitEngine, explicitModel bool
	for i := 1; i < len(rest); i++ {
		switch rest[i] {
		case "--mode":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --mode")
			}
			i++
			switch goalx.Mode(rest[i]) {
			case goalx.ModeResearch, goalx.ModeDevelop:
				opts.Mode = goalx.Mode(rest[i])
			default:
				return fmt.Errorf("invalid --mode %q (expected research or develop)", rest[i])
			}
		case "--engine":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --engine")
			}
			i++
			opts.Engine = strings.TrimSpace(rest[i])
			explicitEngine = true
		case "--model":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --model")
			}
			i++
			opts.Model = strings.TrimSpace(rest[i])
			explicitModel = true
		case "--effort":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --effort")
			}
			i++
			level, err := goalx.ParseEffortLevel(rest[i])
			if err != nil {
				return err
			}
			opts.Effort = level
		case "--dimension":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --dimension")
			}
			i++
			opts.Dimensions = append(opts.Dimensions, splitListFlag(rest[i])...)
		default:
			return fmt.Errorf("unknown flag %q", rest[i])
		}
	}
	if explicitEngine != explicitModel {
		return fmt.Errorf("--engine and --model must be provided together")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if !SessionExists(rc.TmuxSession) {
		return fmt.Errorf("run '%s' is not active (no tmux session)", rc.Name)
	}
	if _, err := parseSessionIndex(oldSessionName); err != nil {
		return err
	}
	if err := requireRunBudgetAvailable(rc.RunDir, rc.Config); err != nil {
		return err
	}
	oldIdentity, err := RequireSessionIdentity(rc.RunDir, oldSessionName)
	if err != nil {
		return fmt.Errorf("load session identity: %w", err)
	}

	digest := inferSessionCoordination(JournalPath(rc.RunDir, oldSessionName))
	sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load session runtime state: %w", err)
	}
	current := sessionState.Sessions[oldSessionName]
	scope := scopeOrFallback(digest.Scope, current.OwnerScope, oldSessionName)
	oldWorktreePath := resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, oldSessionName, sessionState)
	oldBranch := resolvedSessionBranch(rc.RunDir, rc.Config.Name, oldSessionName, sessionState)
	workdir := sessionWorkdir(rc.RunDir, rc.Config.Name, oldSessionName, sessionState)
	if info, err := os.Stat(workdir); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("%s is not a directory", workdir)
		}
		return fmt.Errorf("replacement requires existing workdir: %w", err)
	}
	dedicatedWorktree := strings.TrimSpace(oldWorktreePath) != ""
	if dedicatedWorktree {
		if strings.TrimSpace(oldBranch) == "" {
			return fmt.Errorf("session %s has no recorded dedicated branch; replace requires a sealed worktree boundary", oldSessionName)
		}
		dirtyPaths, err := dirtyWorktreePaths(oldWorktreePath)
		if err != nil {
			return fmt.Errorf("inspect %s worktree: %w", oldSessionName, err)
		}
		if len(dirtyPaths) > 0 {
			return fmt.Errorf("session %s dedicated worktree has uncommitted changes (%s); replace cannot hand off an unsealed worktree boundary", oldSessionName, summarizeDirtyPaths(dirtyPaths))
		}
	}

	cleanup := &cleanupStack{}
	replacementCommitted := false
	oldParked := false
	defer func() {
		if err == nil || replacementCommitted {
			return
		}
		rollbackErr := cleanup.Run()
		if oldParked {
			if restoreErr := restoreParkedSession(projectRoot, rc.Name, rc.RunDir, oldSessionName, scope); restoreErr != nil {
				rollbackErr = errors.Join(rollbackErr, fmt.Errorf("restore %s: %w", oldSessionName, restoreErr))
			}
		}
		if rollbackErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback replace %s: %w", oldSessionName, rollbackErr))
		}
	}()
	if err := Park(projectRoot, []string{"--run", rc.Name, oldSessionName}); err != nil {
		return err
	}
	oldParked = true

	newNum, err := nextAvailableSessionIndex(rc.ProjectRoot, rc.RunDir, rc.Config.Name)
	if err != nil {
		return err
	}
	newSessionName := SessionName(newNum)
	journalPath := JournalPath(rc.RunDir, newSessionName)
	sessionIdentityPath := SessionIdentityPath(rc.RunDir, newSessionName)
	windowName := sessionWindowName(rc.Config.Name, newNum)
	replacementWorktreePath := oldWorktreePath
	replacementBranch := oldBranch
	replacementWorkdir := workdir
	replacementBaseSelector := oldIdentity.BaseBranchSelector
	replacementBaseBranch := oldIdentity.BaseBranch
	if dedicatedWorktree {
		replacementWorktreePath = WorktreePath(rc.RunDir, rc.Config.Name, newNum)
		replacementBranch = fmt.Sprintf("goalx/%s/%d", rc.Config.Name, newNum)
		replacementWorkdir = replacementWorktreePath
		replacementBaseSelector = oldSessionName
		replacementBaseBranch = oldBranch
	}

	oldDimensions := goalx.ResolveDimensionNames(oldIdentity.Dimensions)
	newSession := goalx.SessionConfig{
		Hint:       scope,
		Mode:       goalx.Mode(oldIdentity.Mode),
		Effort:     oldIdentity.RequestedEffort,
		Dimensions: append([]string(nil), oldDimensions...),
		Target:     &oldIdentity.Target,
	}
	routeChanged := opts.Mode != "" || opts.Effort != "" || len(opts.Dimensions) > 0
	if !routeChanged && !(explicitEngine || explicitModel) {
		newSession.Engine = oldIdentity.Engine
		newSession.Model = oldIdentity.Model
	}
	if opts.Mode != "" {
		newSession.Mode = opts.Mode
	}
	if opts.Effort != "" {
		newSession.Effort = opts.Effort
	}
	if len(opts.Dimensions) > 0 {
		newSession.Dimensions = append([]string(nil), opts.Dimensions...)
	}
	if explicitEngine {
		newSession.Engine = opts.Engine
	}
	if explicitModel {
		newSession.Model = opts.Model
	}
	if routeChanged && !(explicitEngine || explicitModel) {
		newSession.Engine = ""
		newSession.Model = ""
	}

	renderCfg := *rc.Config
	renderCfg.Sessions = append([]goalx.SessionConfig(nil), goalx.ExpandSessions(rc.Config)...)
	for len(renderCfg.Sessions) < newNum {
		renderCfg.Sessions = append(renderCfg.Sessions, goalx.SessionConfig{})
	}
	renderCfg.Sessions[newNum-1] = newSession
	if renderCfg.Parallel < len(renderCfg.Sessions) {
		renderCfg.Parallel = len(renderCfg.Sessions)
	}
	effectiveSession := goalx.EffectiveSessionConfig(&renderCfg, newNum-1)
	var target goalx.TargetConfig
	if effectiveSession.Target != nil {
		target = *effectiveSession.Target
	}
	sessionIdentity, err := NewSessionIdentity(
		rc.RunDir,
		newSessionName,
		sessionRoleKind(effectiveSession.Mode),
		effectiveSession.Mode,
		effectiveSession.Engine,
		effectiveSession.Model,
		effectiveSession.Effort,
		"",
		"",
		target,
	)
	if err != nil {
		return fmt.Errorf("create session identity: %w", err)
	}
	sessionIdentity.ReplacesSession = oldSessionName
	sessionIdentity.BaseExperimentID = oldIdentity.ExperimentID
	sessionIdentity.BaseBranchSelector = replacementBaseSelector
	sessionIdentity.BaseBranch = replacementBaseBranch
	if len(effectiveSession.Dimensions) > 0 {
		dimensionsCatalog, err := loadDimensionCatalog(rc.ProjectRoot)
		if err != nil {
			return fmt.Errorf("load dimension catalog: %w", err)
		}
		resolvedDimensions, err := goalx.ResolveDimensionSpecs(effectiveSession.Dimensions, dimensionsCatalog)
		if err != nil {
			return fmt.Errorf("resolve replacement dimensions: %w", err)
		}
		sessionIdentity.Dimensions = resolvedDimensions
	}

	engines, err := loadEngineCatalog(rc.ProjectRoot)
	if err != nil {
		return fmt.Errorf("load config for engine resolution: %w", err)
	}
	launchSpec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: sessionIdentity.Engine,
		Model:  sessionIdentity.Model,
		Effort: effectiveSession.Effort,
	})
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	sessionIdentity.EffectiveEffort = launchSpec.EffectiveEffort
	sessionIdentity.LocalValidationCommand = resolveSessionLocalValidationCommand(effectiveSession)
	if err := SaveSessionIdentity(sessionIdentityPath, sessionIdentity); err != nil {
		return fmt.Errorf("write session identity: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionIdentitySurface(rc.RunDir, newSessionName) })
	if err := os.WriteFile(journalPath, nil, 0o644); err != nil {
		return fmt.Errorf("init journal: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionJournal(rc.RunDir, newSessionName) })
	if err := EnsureSessionControl(rc.RunDir, newSessionName); err != nil {
		return fmt.Errorf("init session control: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionControlSurface(rc.RunDir, newSessionName) })
	if dedicatedWorktree {
		runWT := RunWorktreePath(rc.RunDir)
		if err := CreateWorktree(runWT, replacementWorktreePath, replacementBranch, replacementBaseBranch); err != nil {
			return fmt.Errorf("create replacement worktree: %w", err)
		}
		cleanup.Add(func() error { return cleanupSessionWorktreeBoundary(runWT, replacementWorktreePath, replacementBranch) })
		if err := CopyGitignoredFiles(runWT, replacementWorktreePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: copy gitignored files to replacement worktree: %v\n", err)
		}
	}
	if err := GenerateAdapter(sessionIdentity.Engine, replacementWorkdir, ControlInboxPath(rc.RunDir, newSessionName), SessionCursorPath(rc.RunDir, newSessionName)); err != nil {
		return fmt.Errorf("generate adapter: %w", err)
	}
	if err := EnsureEngineTrusted(sessionIdentity.Engine, replacementWorkdir); err != nil {
		return fmt.Errorf("trust bootstrap: %w", err)
	}
	if _, err := EnsureDimensionsState(rc.RunDir); err != nil {
		return fmt.Errorf("init dimensions state: %w", err)
	}

	dimensionsCatalog, err := loadDimensionCatalog(rc.ProjectRoot)
	if err != nil {
		return fmt.Errorf("load dimension catalog: %w", err)
	}
	sessionDataList, err := buildSessionDataList(rc.RunDir, &renderCfg, engines, dimensionsCatalog)
	if err != nil {
		return fmt.Errorf("build session roster: %w", err)
	}
	absProjectRoot, _ := filepath.Abs(rc.ProjectRoot)
	subData := ProtocolData{
		RunName:                rc.Config.Name,
		Objective:              rc.Config.Objective,
		Mode:                   effectiveSession.Mode,
		Engine:                 sessionIdentity.Engine,
		Sessions:               sessionDataList,
		Target:                 target,
		LocalValidationCommand: sessionIdentity.LocalValidationCommand,
		Context:                rc.Config.Context,
		Budget:                 rc.Config.Budget,
		SessionName:            newSessionName,
		SessionIndex:           newNum - 1,
		CurrentDimensions:      CurrentSessionDimensions(rc.RunDir, newSessionName, sessionIdentity.Dimensions),
		JournalPath:            journalPath,
		CharterPath:            RunCharterPath(rc.RunDir),
		SessionIdentityPath:    sessionIdentityPath,
		SessionInboxPath:       ControlInboxPath(rc.RunDir, newSessionName),
		SessionCursorPath:      SessionCursorPath(rc.RunDir, newSessionName),
		WorktreePath:           replacementWorktreePath,
		GoalPath:               GoalPath(rc.RunDir),
		GoalLogPath:            GoalLogPath(rc.RunDir),
		IdentityFencePath:      IdentityFencePath(rc.RunDir),
		AcceptanceNotesPath:    existingProtocolPath(AcceptanceNotesPath(rc.RunDir)),
		AcceptanceStatePath:    AcceptanceStatePath(rc.RunDir),
		CompletionProofPath:    CompletionStatePath(rc.RunDir),
		RunStatePath:           RunRuntimeStatePath(rc.RunDir),
		SessionsStatePath:      SessionsRuntimeStatePath(rc.RunDir),
		ProjectRegistryPath:    ProjectRegistryPath(rc.ProjectRoot),
		ProjectRoot:            absProjectRoot,
	}
	if err := RenderSubagentProtocol(subData, rc.RunDir, newNum-1); err != nil {
		return fmt.Errorf("render protocol: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionProgram(rc.RunDir, newNum) })

	meta, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective)
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	checkSec, _ := normalizeSidecarInterval(rc.Config.Master.CheckInterval)
	sessionLeaseTTL := time.Duration(checkSec) * time.Second * 2
	engineCmd := launchSpec.Command
	protocolPath := filepath.Join(rc.RunDir, sessionNameToProgramFile(newNum))
	prompt := goalx.ResolvePrompt(engines, sessionIdentity.Engine, protocolPath)
	launchCmd := buildLeaseWrappedLaunchCommand(goalxBin, rc.Name, rc.RunDir, newSessionName, meta.RunID, meta.Epoch, sessionLeaseTTL, engineCmd, prompt)
	if err := NewWindowWithCommand(rc.TmuxSession, windowName, replacementWorkdir, launchCmd); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionWindow(rc.TmuxSession, windowName) })
	if err := waitForSessionLaunchReady(rc.TmuxSession, newSessionName, windowName, sessionIdentity.Engine); err != nil {
		return err
	}

	if err := UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
		Name:         newSessionName,
		State:        "active",
		Mode:         sessionIdentity.Mode,
		Branch:       replacementBranch,
		WorktreePath: replacementWorktreePath,
		OwnerScope:   scope,
	}); err != nil {
		return fmt.Errorf("update session runtime state: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionRuntimeEntry(rc.RunDir, newSessionName) })
	if err := appendExperimentCreated(rc.RunDir, ExperimentCreatedBody{
		ExperimentID:     sessionIdentity.ExperimentID,
		Session:          newSessionName,
		Branch:           replacementBranch,
		Worktree:         replacementWorkdir,
		Intent:           sessionIdentity.Mode,
		BaseRef:          sessionIdentity.BaseBranch,
		BaseExperimentID: sessionIdentity.BaseExperimentID,
		CreatedAt:        sessionIdentity.CreatedAt,
	}); err != nil {
		return fmt.Errorf("append experiment.created for %s: %w", newSessionName, err)
	}
	replacementCommitted = true
	cleanup.Commit()
	if err := PersistPanePIDsFromTmux(rc.RunDir, newSessionName, rc.TmuxSession+":"+windowName); err != nil {
		fmt.Fprintf(os.Stderr, "warning: persist %s pane pid: %v\n", newSessionName, err)
	}
	if _, err := AppendMasterInboxMessage(rc.RunDir, "session_replaced", "goalx replace", fmt.Sprintf("%s was replaced by %s.", oldSessionName, newSessionName)); err == nil {
		if _, err := DeliverControlNudge(rc.RunDir, "session-replaced:"+oldSessionName, "session-replaced:"+newSessionName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudgeDetailed); err != nil {
			fmt.Fprintf(os.Stderr, "warning: nudge master: %v\n", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "warning: notify master inbox: %v\n", err)
	}
	if err := RefreshRunGuidance(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
	}

	fmt.Printf("Replaced %s with %s in run '%s'\n", oldSessionName, newSessionName, rc.Name)
	return nil
}

func inferSessionCoordination(journalPath string) sessionCoordinationDigest {
	entries, err := goalx.LoadJournal(journalPath)
	if err != nil || len(entries) == 0 {
		return sessionCoordinationDigest{}
	}
	last := entries[len(entries)-1]
	state := "active"
	switch last.Status {
	case "idle":
		state = "idle"
	case "stuck":
		state = "blocked"
	case "done":
		state = "done"
	}
	return sessionCoordinationDigest{
		State:     state,
		Scope:     last.OwnerScope,
		BlockedBy: last.BlockedBy,
		LastRound: last.Round,
	}
}

func sessionNameToProgramFile(idx int) string {
	return fmt.Sprintf("program-%d.md", idx)
}
