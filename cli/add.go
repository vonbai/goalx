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

const addUsage = `usage: goalx add "research direction" [--run NAME] --mode MODE [--engine ENGINE] [--model MODEL] [--effort LEVEL] [--dimension SPEC]... [--worktree] [--base-branch BRANCH|session-N]`

// Add creates a new subagent session in a running run.
func Add(projectRoot string, args []string) (err error) {
	// Parse: goalx add "hint/direction" [--run NAME] [--engine ENGINE] [--model MODEL] [--mode MODE] [--dimension NAME]
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	for _, arg := range rest {
		switch arg {
		case "--help", "-h", "help":
			fmt.Println(addUsage)
			return nil
		}
	}

	// Extract flags from rest args
	var flagEngine, flagModel string
	var explicitEngine, explicitModel bool
	var flagEffort goalx.EffortLevel
	var flagMode goalx.Mode
	var flagDimensions []string
	useWorktree := false
	baseBranchSelector := ""
	var hintParts []string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--engine":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --engine")
			}
			i++
			flagEngine = rest[i]
			explicitEngine = true
		case "--model":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --model")
			}
			i++
			flagModel = rest[i]
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
			flagEffort = level
		case "--mode":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --mode")
			}
			i++
			switch goalx.Mode(rest[i]) {
			case goalx.ModeResearch, goalx.ModeDevelop:
				flagMode = goalx.Mode(rest[i])
			default:
				return fmt.Errorf("invalid --mode %q (expected research or develop)", rest[i])
			}
		case "--dimension":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --dimension")
			}
			i++
			flagDimensions = append(flagDimensions, splitListFlag(rest[i])...)
		case "--worktree":
			useWorktree = true
		case "--base-branch":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --base-branch")
			}
			i++
			baseBranchSelector = strings.TrimSpace(rest[i])
		default:
			if strings.HasPrefix(rest[i], "-") {
				return fmt.Errorf("unknown flag %q", rest[i])
			}
			hintParts = append(hintParts, rest[i])
		}
	}
	if len(hintParts) == 0 {
		return fmt.Errorf(addUsage)
	}
	if explicitEngine != explicitModel {
		return fmt.Errorf("--engine and --model must be provided together")
	}
	if baseBranchSelector != "" && !useWorktree {
		return fmt.Errorf("--base-branch requires --worktree")
	}
	if flagMode == "" {
		return fmt.Errorf("--mode is required")
	}
	hint := strings.Join(hintParts, " ")

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	// Check run is active
	if !SessionExists(rc.TmuxSession) {
		return fmt.Errorf("run '%s' is not active (no tmux session)", rc.Name)
	}
	if err := requireRunBudgetAvailable(rc.RunDir, rc.Config); err != nil {
		return err
	}

	// Determine next session number from the run's existing session artifacts.
	newNum, err := nextAvailableSessionIndex(rc.ProjectRoot, rc.RunDir, rc.Config.Name)
	if err != nil {
		return err
	}
	sName := SessionName(newNum)
	sessionIdentityPath := SessionIdentityPath(rc.RunDir, sName)
	journalPath := JournalPath(rc.RunDir, sName)
	windowName := sessionWindowName(rc.Config.Name, newNum)
	cleanup := &cleanupStack{}
	sessionCommitted := false
	defer func() {
		if err == nil || sessionCommitted {
			return
		}
		if cleanupErr := cleanup.Run(); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("rollback add %s: %w", sName, cleanupErr))
		}
	}()

	renderCfg := *rc.Config
	renderCfg.Sessions = append([]goalx.SessionConfig(nil), goalx.ExpandSessions(rc.Config)...)
	for len(renderCfg.Sessions) < newNum {
		renderCfg.Sessions = append(renderCfg.Sessions, goalx.SessionConfig{})
	}
	sessionCfg := renderCfg.Sessions[newNum-1]
	sessionCfg.Hint = hint
	if flagEngine != "" {
		sessionCfg.Engine = flagEngine
	}
	if flagModel != "" {
		sessionCfg.Model = flagModel
	}
	if flagMode != "" {
		sessionCfg.Mode = flagMode
	}
	if flagEffort != "" {
		sessionCfg.Effort = flagEffort
	}
	if len(flagDimensions) > 0 {
		if _, err := goalx.ResolveDimensionSpecs(flagDimensions); err != nil {
			return err
		}
		sessionCfg.Dimensions = append([]string(nil), flagDimensions...)
	}
	renderCfg.Sessions[newNum-1] = sessionCfg
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
		sName,
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
	sessionIdentity.LocalValidationCommand = resolveSessionLocalValidationCommand(effectiveSession)
	dimensionsCatalog, err := loadDimensionCatalog(rc.ProjectRoot)
	if err != nil {
		return fmt.Errorf("load dimension catalog: %w", err)
	}
	if len(effectiveSession.Dimensions) > 0 {
		resolvedDimensions, err := goalx.ResolveDimensionSpecs(effectiveSession.Dimensions, dimensionsCatalog)
		if err != nil {
			return fmt.Errorf("resolve session dimensions: %w", err)
		}
		sessionIdentity.Dimensions = resolvedDimensions
	}
	engine := sessionIdentity.Engine
	model := sessionIdentity.Model
	meta, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective)
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	runWT := RunWorktreePath(rc.RunDir)
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	checkSec, _ := normalizeSidecarInterval(rc.Config.Master.CheckInterval)
	sessionLeaseTTL := time.Duration(checkSec) * time.Second * 2

	// Resolve engine
	engines, err := loadEngineCatalog(rc.ProjectRoot)
	if err != nil {
		return fmt.Errorf("load config for engine resolution: %w", err)
	}
	launchSpec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: engine,
		Model:  model,
		Effort: effectiveSession.Effort,
	})
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	engineCmd := launchSpec.Command
	sessionIdentity.EffectiveEffort = launchSpec.EffectiveEffort
	workdir := runWT
	wtPath := ""
	branch := ""
	baseBranch := ""
	baseBranchHint := ""
	baseExperimentID := ""
	if useWorktree {
		wtPath = WorktreePath(rc.RunDir, rc.Config.Name, newNum)
		branch = fmt.Sprintf("goalx/%s/%d", rc.Config.Name, newNum)
		if baseBranchSelector != "" {
			resolvedBaseBranch, err := resolveAddBaseBranchSelector(rc.RunDir, rc.Config.Name, baseBranchSelector)
			if err != nil {
				return err
			}
			baseBranch = resolvedBaseBranch
			baseBranchHint = baseBranchSelector
			if _, err := parseSessionIndex(baseBranchSelector); err == nil {
				parentIdentity, err := RequireSessionIdentity(rc.RunDir, baseBranchSelector)
				if err != nil {
					return fmt.Errorf("load %s identity: %w", baseBranchSelector, err)
				}
				if strings.TrimSpace(parentIdentity.ExperimentID) == "" {
					return fmt.Errorf("session %s has no experiment_id", baseBranchSelector)
				}
				baseExperimentID = parentIdentity.ExperimentID
			}
		} else {
			baseBranch = fmt.Sprintf("goalx/%s/root", rc.Config.Name)
			baseBranchHint = "run-root"
		}
		sessionIdentity.BaseExperimentID = baseExperimentID
		sessionIdentity.BaseBranchSelector = baseBranchHint
		sessionIdentity.BaseBranch = baseBranch
	}
	if err := SaveSessionIdentity(sessionIdentityPath, sessionIdentity); err != nil {
		return fmt.Errorf("write session identity: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionIdentitySurface(rc.RunDir, sName) })

	if useWorktree {
		if err := CreateWorktree(runWT, wtPath, branch, baseBranch); err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
		cleanup.Add(func() error { return cleanupSessionWorktreeBoundary(runWT, wtPath, branch) })
		if err := CopyGitignoredFiles(runWT, wtPath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: copy gitignored files to session worktree: %v\n", err)
		}
		workdir = wtPath
	}
	absProjectRoot, _ := filepath.Abs(rc.ProjectRoot)

	// Create journal + session control files
	if err := os.WriteFile(journalPath, nil, 0644); err != nil {
		return fmt.Errorf("init journal: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionJournal(rc.RunDir, sName) })
	if err := EnsureSessionControl(rc.RunDir, sName); err != nil {
		return fmt.Errorf("init session control: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionControlSurface(rc.RunDir, sName) })
	assignmentType := string(sessionIdentity.Mode)
	if assignmentType == "" {
		assignmentType = "tell"
	}
	if _, err := AppendControlInboxMessage(rc.RunDir, sName, assignmentType, "master", hint); err != nil {
		return fmt.Errorf("seed session inbox: %w", err)
	}

	// Generate adapter
	if err := GenerateAdapter(engine, workdir, ControlInboxPath(rc.RunDir, sName), SessionCursorPath(rc.RunDir, sName)); err != nil {
		return fmt.Errorf("generate adapter: %w", err)
	}
	if err := EnsureEngineTrusted(engine, workdir); err != nil {
		return fmt.Errorf("trust bootstrap: %w", err)
	}
	if _, err := EnsureDimensionsState(rc.RunDir); err != nil {
		return fmt.Errorf("init dimensions state: %w", err)
	}
	sessionDataList, err := buildSessionDataList(rc.RunDir, &renderCfg, engines, dimensionsCatalog)
	if err != nil {
		return fmt.Errorf("build session roster: %w", err)
	}

	// Render protocol
	protocolPath := filepath.Join(rc.RunDir, fmt.Sprintf("program-%d.md", newNum))
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
		SessionName:               sName,
		SessionIndex:              newNum - 1,
		CurrentDimensions:         CurrentSessionDimensions(rc.RunDir, sName, sessionIdentity.Dimensions),
		JournalPath:               journalPath,
		CharterPath:               RunCharterPath(rc.RunDir),
		SessionIdentityPath:       sessionIdentityPath,
		SessionInboxPath:          ControlInboxPath(rc.RunDir, sName),
		SessionCursorPath:         SessionCursorPath(rc.RunDir, sName),
		WorktreePath:              wtPath,
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
		RunWorktreePath:           runWT,
		SessionBaseBranchSelector: sessionIdentity.BaseBranchSelector,
		SessionBaseBranch:         sessionIdentity.BaseBranch,
	}
	if err := RenderSubagentProtocol(subData, rc.RunDir, newNum-1); err != nil {
		return fmt.Errorf("render protocol: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionProgram(rc.RunDir, newNum) })

	// Launch in tmux
	prompt := goalx.ResolvePrompt(engines, engine, protocolPath)
	launchCmd := buildLeaseWrappedLaunchCommand(goalxBin, rc.Name, rc.RunDir, sName, meta.RunID, meta.Epoch, sessionLeaseTTL, engineCmd, prompt)
	if err := NewWindowWithCommand(rc.TmuxSession, windowName, workdir, launchCmd); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionWindow(rc.TmuxSession, windowName) })
	if err := waitForSessionLaunchReady(rc.TmuxSession, sName, windowName, sessionIdentity.Engine); err != nil {
		return err
	}

	if err := UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
		Name:         sName,
		State:        "active",
		Mode:         sessionIdentity.Mode,
		Branch:       branch,
		WorktreePath: wtPath,
		OwnerScope:   hint,
	}); err != nil {
		return fmt.Errorf("prime session runtime state: %w", err)
	}
	cleanup.Add(func() error { return cleanupSessionRuntimeEntry(rc.RunDir, sName) })
	if err := appendExperimentCreated(rc.RunDir, ExperimentCreatedBody{
		ExperimentID:     sessionIdentity.ExperimentID,
		Session:          sName,
		Branch:           branch,
		Worktree:         workdir,
		Intent:           sessionIdentity.Mode,
		BaseRef:          sessionIdentity.BaseBranch,
		BaseExperimentID: sessionIdentity.BaseExperimentID,
		CreatedAt:        sessionIdentity.CreatedAt,
	}); err != nil {
		return fmt.Errorf("append experiment.created for %s: %w", sName, err)
	}
	sessionCommitted = true
	cleanup.Commit()
	// Notify master through durable inbox, then best-effort tmux nudge.
	masterMsg := fmt.Sprintf(
		"New %s added to your run. Window: %s, Workdir: %s, Journal: %s, Inbox: %s. Direction: %s. Add it to your check cycle.",
		sName, windowName, workdir, journalPath, ControlInboxPath(rc.RunDir, sName), hint,
	)
	masterNotified := false
	if err := PersistPanePIDsFromTmux(rc.RunDir, sName, rc.TmuxSession+":"+windowName); err != nil {
		fmt.Fprintf(os.Stderr, "warning: persist %s pane pid: %v\n", sName, err)
	}
	if _, err := AppendMasterInboxMessage(rc.RunDir, "session_added", "goalx add", masterMsg); err != nil {
		fmt.Fprintf(os.Stderr, "warning: notify master inbox: %v\n", err)
	} else if _, err := DeliverControlNudge(rc.RunDir, "session-added:"+sName, "session-added:"+sName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudgeDetailed); err != nil {
		fmt.Fprintf(os.Stderr, "warning: nudge master: %v\n", err)
		masterNotified = true
	} else {
		masterNotified = true
	}
	if err := RefreshRunGuidance(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: refresh run guidance: %v\n", err)
	}

	fmt.Printf("Added %s to run '%s'\n", sName, rc.Name)
	fmt.Printf("  window: %s\n", windowName)
	fmt.Printf("  direction: %s\n", hint)
	if masterNotified {
		fmt.Printf("  master notified\n")
	}
	return nil
}

func buildSessionDataList(runDir string, cfg *goalx.Config, engines map[string]goalx.EngineConfig, dimensionsCatalog map[string]string) ([]SessionData, error) {
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	sessionState, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return nil, fmt.Errorf("load session runtime state: %w", err)
	}

	list := make([]SessionData, 0, len(indexes))
	for _, num := range indexes {
		sName := SessionName(num)
		effective := goalx.EffectiveSessionConfig(cfg, num-1)
		engine := effective.Engine
		model := effective.Model
		mode := effective.Mode
		identity, err := RequireSessionIdentity(runDir, sName)
		if err != nil {
			return nil, fmt.Errorf("load %s identity: %w", sName, err)
		}
		if identity.Engine != "" {
			engine = identity.Engine
		}
		if identity.Model != "" {
			model = identity.Model
		}
		if identity.Mode != "" {
			mode = goalx.Mode(identity.Mode)
		}
		dimensions := append([]goalx.ResolvedDimension(nil), identity.Dimensions...)
		if len(dimensions) == 0 && len(effective.Dimensions) > 0 {
			resolvedDimensions, err := goalx.ResolveDimensionSpecs(effective.Dimensions, dimensionsCatalog)
			if err != nil {
				return nil, fmt.Errorf("resolve %s dimensions: %w", sName, err)
			}
			dimensions = resolvedDimensions
		}
		dimensions = CurrentSessionDimensions(runDir, sName, dimensions)
		engineCommand := ""
		if strings.TrimSpace(engine) != "" && strings.TrimSpace(model) != "" {
			spec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
				Engine: engine,
				Model:  model,
				Effort: effective.Effort,
			})
			if err != nil {
				return nil, fmt.Errorf("resolve session-%d engine: %w", num, err)
			}
			engineCommand = spec.Command
		}
		worktreePath := resolvedSessionWorktreePath(runDir, cfg.Name, sName, sessionState)
		list = append(list, SessionData{
			Name:              sName,
			WindowName:        sessionWindowName(cfg.Name, num),
			WorktreePath:      worktreePath,
			JournalPath:       JournalPath(runDir, sName),
			SessionInboxPath:  ControlInboxPath(runDir, sName),
			SessionCursorPath: SessionCursorPath(runDir, sName),
			Engine:            engine,
			Model:             model,
			Mode:              mode,
			Hint:              effective.Hint,
			Dimensions:        dimensions,
			EngineCommand:     engineCommand,
		})
	}
	return list, nil
}

func resolveAddBaseBranchSelector(runDir, runName, selector string) (string, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return "", fmt.Errorf("base branch selector is required")
	}
	if _, err := parseSessionIndex(selector); err == nil {
		sessionState, err := EnsureSessionsRuntimeState(runDir)
		if err != nil {
			return "", fmt.Errorf("load session runtime state: %w", err)
		}
		branch := resolvedSessionBranch(runDir, runName, selector, sessionState)
		if branch == "" {
			return "", fmt.Errorf("session %q has no dedicated branch", selector)
		}
		return branch, nil
	}
	return selector, nil
}
