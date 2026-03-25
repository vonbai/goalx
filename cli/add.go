package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const addUsage = `usage: goalx add "research direction" [--run NAME] [--engine ENGINE] [--model MODEL] [--mode MODE] [--dimension NAME] [--worktree]`

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
	var flagMode goalx.Mode
	useWorktree := false
	var hintParts []string
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--engine":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --engine")
			}
			i++
			flagEngine = rest[i]
		case "--model":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --model")
			}
			i++
			flagModel = rest[i]
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
			hints, err := goalx.ResolveDimensions(strings.Split(rest[i], ","))
			if err != nil {
				return err
			}
			hintParts = append(hintParts, hints...)
		case "--worktree":
			useWorktree = true
		default:
			hintParts = append(hintParts, rest[i])
		}
	}
	if len(hintParts) == 0 {
		return fmt.Errorf(addUsage)
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

	// Determine next session number from the run's existing session artifacts.
	newNum, err := nextAvailableSessionIndex(rc.ProjectRoot, rc.RunDir, rc.Config.Name)
	if err != nil {
		return err
	}
	sName := SessionName(newNum)
	sessionIdentityPath := SessionIdentityPath(rc.RunDir, sName)
	journalPath := JournalPath(rc.RunDir, sName)
	windowName := sessionWindowName(rc.Config.Name, newNum)
	sessionIdentityWritten := false
	defer func() {
		if err == nil || !sessionIdentityWritten {
			return
		}
		if removeErr := os.Remove(sessionIdentityPath); removeErr != nil && !os.IsNotExist(removeErr) {
			fmt.Fprintf(os.Stderr, "warning: cleanup session identity %s: %v\n", sessionIdentityPath, removeErr)
		}
	}()

	newSess := goalx.SessionConfig{Hint: hint}
	if flagEngine != "" {
		newSess.Engine = flagEngine
	}
	if flagModel != "" {
		newSess.Model = flagModel
	}
	if flagMode != "" {
		newSess.Mode = flagMode
	}

	renderCfg := *rc.Config
	renderCfg.Sessions = append([]goalx.SessionConfig(nil), goalx.ExpandSessions(rc.Config)...)
	for len(renderCfg.Sessions) < newNum {
		renderCfg.Sessions = append(renderCfg.Sessions, goalx.SessionConfig{})
	}
	renderCfg.Sessions[newNum-1] = newSess
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
		"",
		target,
	)
	if err != nil {
		return fmt.Errorf("create session identity: %w", err)
	}
	engine := sessionIdentity.Engine
	model := sessionIdentity.Model
	meta, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective)
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	launchEnv, err := RequireLaunchEnvSnapshot(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load run launch env: %w", err)
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
	if err := SaveSessionIdentity(sessionIdentityPath, sessionIdentity); err != nil {
		return fmt.Errorf("write session identity: %w", err)
	}
	sessionIdentityWritten = true
	if engine == "claude-code" {
		engineCmd += " --disable-slash-commands"
	}

	workdir := runWT
	wtPath := ""
	branch := ""
	if useWorktree {
		wtPath = WorktreePath(rc.RunDir, rc.Config.Name, newNum)
		branch = fmt.Sprintf("goalx/%s/%d", rc.Config.Name, newNum)
		if err := CreateWorktree(runWT, wtPath, branch); err != nil {
			return fmt.Errorf("create worktree: %w", err)
		}
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
	if err := EnsureSessionControl(rc.RunDir, sName); err != nil {
		return fmt.Errorf("init session control: %w", err)
	}

	// Generate adapter
	if err := GenerateAdapter(engine, workdir, ControlInboxPath(rc.RunDir, sName), SessionCursorPath(rc.RunDir, sName)); err != nil {
		return fmt.Errorf("generate adapter: %w", err)
	}
	if err := EnsureEngineTrusted(engine, workdir); err != nil {
		return fmt.Errorf("trust bootstrap: %w", err)
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
	sessionDataList, err := buildSessionDataList(rc.RunDir, &renderCfg, engines)
	if err != nil {
		return fmt.Errorf("build session roster: %w", err)
	}

	// Render protocol
	protocolPath := filepath.Join(rc.RunDir, fmt.Sprintf("program-%d.md", newNum))
	subData := ProtocolData{
		RunName:             rc.Config.Name,
		Objective:           rc.Config.Objective,
		Mode:                goalx.Mode(sessionIdentity.Mode),
		Engine:              sessionIdentity.Engine,
		Sessions:            sessionDataList,
		Target:              sessionIdentity.Target,
		Harness:             rc.Config.Harness,
		Context:             rc.Config.Context,
		Budget:              rc.Config.Budget,
		SessionName:         sName,
		SessionIndex:        newNum - 1,
		JournalPath:         journalPath,
		CharterPath:         RunCharterPath(rc.RunDir),
		SessionIdentityPath: sessionIdentityPath,
		SessionInboxPath:    ControlInboxPath(rc.RunDir, sName),
		SessionCursorPath:   SessionCursorPath(rc.RunDir, sName),
		WorktreePath:        wtPath,
		GoalPath:            GoalPath(rc.RunDir),
		GoalLogPath:         GoalLogPath(rc.RunDir),
		IdentityFencePath:   IdentityFencePath(rc.RunDir),
		AcceptanceNotesPath: existingProtocolPath(AcceptanceNotesPath(rc.RunDir)),
		AcceptanceStatePath: AcceptanceStatePath(rc.RunDir),
		CompletionProofPath: CompletionStatePath(rc.RunDir),
		RunStatePath:        RunRuntimeStatePath(rc.RunDir),
		SessionsStatePath:   SessionsRuntimeStatePath(rc.RunDir),
		ProjectRegistryPath: ProjectRegistryPath(rc.ProjectRoot),
		ProjectRoot:         absProjectRoot,
	}
	if err := RenderSubagentProtocol(subData, rc.RunDir, newNum-1); err != nil {
		return fmt.Errorf("render protocol: %w", err)
	}

	// Launch in tmux
	prompt := goalx.ResolvePrompt(engines, engine, protocolPath)
	launchCmd := buildLeaseWrappedLaunchCommandWithEnv(launchEnv.Env, goalxBin, rc.Name, rc.RunDir, sName, meta.RunID, meta.Epoch, sessionLeaseTTL, engineCmd, prompt)
	if err := NewWindowWithCommand(rc.TmuxSession, windowName, workdir, launchCmd); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}
	if err := PersistPanePIDsFromTmux(rc.RunDir, sName, rc.TmuxSession+":"+windowName); err != nil {
		return fmt.Errorf("persist %s pane pid: %w", sName, err)
	}

	if coord, err := EnsureCoordinationState(rc.RunDir, rc.Config.Objective); err == nil {
		now := time.Now().UTC().Format(time.RFC3339)
		coord.Sessions[sName] = CoordinationSession{
			State:     "active",
			Scope:     hint,
			UpdatedAt: now,
		}
		coord.Version++
		coord.UpdatedAt = now
		if err := SaveCoordinationState(CoordinationPath(rc.RunDir), coord); err != nil {
			return fmt.Errorf("update coordination state: %w", err)
		}
	} else {
		return fmt.Errorf("load coordination state: %w", err)
	}
	// Notify master through durable inbox, then best-effort tmux nudge.
	masterMsg := fmt.Sprintf(
		"New %s added to your run. Window: %s, Workdir: %s, Journal: %s, Inbox: %s. Direction: %s. Add it to your check cycle.",
		sName, windowName, workdir, journalPath, ControlInboxPath(rc.RunDir, sName), hint,
	)
	if _, err := AppendMasterInboxMessage(rc.RunDir, "session_added", "goalx add", masterMsg); err != nil {
		return fmt.Errorf("notify master inbox: %w", err)
	}
	if _, err := DeliverControlNudge(rc.RunDir, "session-added:"+sName, "session-added:"+sName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudge); err != nil {
		return fmt.Errorf("nudge master: %w", err)
	}

	fmt.Printf("Added %s to run '%s'\n", sName, rc.Name)
	fmt.Printf("  window: %s\n", windowName)
	fmt.Printf("  direction: %s\n", hint)
	fmt.Printf("  master notified\n")
	return nil
}

func buildSessionDataList(runDir string, cfg *goalx.Config, engines map[string]goalx.EngineConfig) ([]SessionData, error) {
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
		spec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
			Engine: engine,
			Model:  model,
			Effort: effective.Effort,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve session-%d engine: %w", num, err)
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
			EngineCommand:     spec.Command,
		})
	}
	return list, nil
}
