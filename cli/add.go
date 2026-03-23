package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const addUsage = `usage: goalx add "research direction" [--run NAME] [--engine ENGINE] [--model MODEL] [--mode MODE] [--strategy NAME]`

// Add creates a new subagent session in a running run.
func Add(projectRoot string, args []string) error {
	// Parse: goalx add "hint/direction" [--run NAME] [--engine ENGINE] [--model MODEL] [--mode MODE] [--strategy NAME]
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
		case "--strategy":
			if i+1 >= len(rest) {
				return fmt.Errorf("missing value for --strategy")
			}
			i++
			hints, err := goalx.ResolveStrategies(strings.Split(rest[i], ","))
			if err != nil {
				return err
			}
			hintParts = append(hintParts, hints...)
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
	wtPath := WorktreePath(rc.RunDir, rc.Config.Name, newNum)
	journalPath := JournalPath(rc.RunDir, sName)
	guidancePath := GuidancePath(rc.RunDir, sName)
	branch := fmt.Sprintf("goalx/%s/%d", rc.Config.Name, newNum)
	windowName := sessionWindowName(rc.Config.Name, newNum)

	newSess := goalx.SessionConfig{Hint: hint}
	if flagEngine != "" {
		newSess.Engine = flagEngine
	}
	if flagModel != "" {
		newSess.Model = flagModel
	}
	if flagMode != "" {
		newSess.Mode = flagMode
		target, harness, err := defaultSessionExecution(rc.ProjectRoot, rc.Config, flagMode)
		if err != nil {
			return err
		}
		newSess.Target = &target
		newSess.Harness = &harness
	}

	renderCfg := *rc.Config
	renderCfg.Sessions = append([]goalx.SessionConfig(nil), rc.Config.Sessions...)
	if len(renderCfg.Sessions) > 0 {
		renderCfg.Sessions = append(renderCfg.Sessions, newSess)
	} else {
		renderCfg.Parallel = newNum
	}
	effectiveSession := goalx.EffectiveSessionConfig(&renderCfg, newNum-1)
	engine := effectiveSession.Engine
	model := effectiveSession.Model

	// Resolve engine
	_, engines, err := goalx.LoadConfig(rc.ProjectRoot)
	if err != nil {
		_, engines, err = goalx.LoadRawBaseConfig(rc.ProjectRoot)
		if err != nil {
			return fmt.Errorf("load config for engine resolution: %w", err)
		}
	}
	engineCmd, err := goalx.ResolveEngineCommand(engines, engine, model)
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	if engine == "claude-code" {
		engineCmd += " --disable-slash-commands"
	}

	// Create worktree
	absProjectRoot, _ := filepath.Abs(rc.ProjectRoot)
	if err := CreateWorktree(absProjectRoot, wtPath, branch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	// Create journal + guidance files
	if err := os.WriteFile(journalPath, nil, 0644); err != nil {
		return fmt.Errorf("init journal: %w", err)
	}
	if err := os.WriteFile(guidancePath, nil, 0644); err != nil {
		return fmt.Errorf("init guidance: %w", err)
	}
	if _, err := EnsureSessionGuidanceState(rc.RunDir, sName); err != nil {
		return fmt.Errorf("init guidance state: %w", err)
	}

	// Generate adapter
	if err := GenerateAdapter(engine, wtPath, guidancePath); err != nil {
		return fmt.Errorf("generate adapter: %w", err)
	}
	if err := EnsureEngineTrusted(engine, wtPath); err != nil {
		return fmt.Errorf("trust bootstrap: %w", err)
	}

	// Deny Skill tool via project-level settings.json
	if engine == "claude-code" {
		if err := ensureSubagentRestrictions(wtPath); err != nil {
			return fmt.Errorf("subagent restrictions: %w", err)
		}
	}

	if err := UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
		Name:         sName,
		State:        "active",
		Mode:         string(effectiveSession.Mode),
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
		Mode:                effectiveSession.Mode,
		Engine:              effectiveSession.Engine,
		Sessions:            sessionDataList,
		Target:              *effectiveSession.Target,
		Harness:             *effectiveSession.Harness,
		Context:             rc.Config.Context,
		Budget:              rc.Config.Budget,
		SessionName:         sName,
		SessionIndex:        newNum - 1,
		JournalPath:         journalPath,
		GuidancePath:        guidancePath,
		GuidanceStatePath:   SessionGuidanceStatePath(rc.RunDir, sName),
		WorktreePath:        wtPath,
		GoalContractPath:    GoalContractPath(rc.RunDir),
		AcceptancePath:      AcceptanceChecklistPath(rc.RunDir),
		AcceptanceStatePath: AcceptanceStatePath(rc.RunDir),
		RunStatePath:        RunRuntimeStatePath(rc.RunDir),
		SessionsStatePath:   SessionsRuntimeStatePath(rc.RunDir),
		ProjectRegistryPath: ProjectRegistryPath(rc.ProjectRoot),
		ProjectRoot:         absProjectRoot,
		DiversityHint:       hint,
	}
	if err := RenderSubagentProtocol(subData, rc.RunDir, newNum-1); err != nil {
		return fmt.Errorf("render protocol: %w", err)
	}

	// Launch in tmux
	prompt := goalx.ResolvePrompt(engines, engine, protocolPath)
	if err := NewWindow(rc.TmuxSession, windowName, wtPath); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}
	launchCmd := fmt.Sprintf("%s %q", engineCmd, prompt)
	if err := SendKeys(rc.TmuxSession+":"+windowName, launchCmd); err != nil {
		return fmt.Errorf("launch subagent: %w", err)
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
		"New %s added to your run. Window: %s, Worktree: %s, Journal: %s, Guidance: %s. Direction: %s. Add it to your check cycle.",
		sName, windowName, wtPath, journalPath, guidancePath, hint,
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

	list := make([]SessionData, 0, len(indexes))
	for _, num := range indexes {
		sName := SessionName(num)
		effective := goalx.EffectiveSessionConfig(cfg, num-1)
		engine := effective.Engine
		model := effective.Model
		engineCmd, err := goalx.ResolveEngineCommand(engines, engine, model)
		if err != nil {
			return nil, fmt.Errorf("resolve session-%d engine: %w", num, err)
		}
		list = append(list, SessionData{
			Name:              sName,
			WindowName:        sessionWindowName(cfg.Name, num),
			WorktreePath:      WorktreePath(runDir, cfg.Name, num),
			JournalPath:       JournalPath(runDir, sName),
			GuidancePath:      GuidancePath(runDir, sName),
			GuidanceStatePath: SessionGuidanceStatePath(runDir, sName),
			Engine:            engine,
			Model:             model,
			Mode:              effective.Mode,
			EngineCommand:     engineCmd,
		})
	}
	return list, nil
}

func defaultSessionExecution(projectRoot string, cfg *goalx.Config, mode goalx.Mode) (goalx.TargetConfig, goalx.HarnessConfig, error) {
	switch mode {
	case goalx.ModeResearch:
		return goalx.TargetConfig{
				Files:    []string{"report.md"},
				Readonly: []string{"."},
			},
			goalx.HarnessConfig{Command: "test -s report.md && echo 'ok'"},
			nil
	case goalx.ModeDevelop:
		target := cfg.Target
		harness := cfg.Harness
		if cfg.Mode == goalx.ModeResearch {
			baseCfg, _, err := goalx.LoadRawBaseConfig(projectRoot)
			if err != nil {
				return goalx.TargetConfig{}, goalx.HarnessConfig{}, fmt.Errorf("load base config for develop session: %w", err)
			}
			if len(baseCfg.Target.Files) > 0 {
				target = baseCfg.Target
			} else if inferred := InferTarget(projectRoot); len(inferred) > 0 {
				target = goalx.TargetConfig{Files: inferred}
			}
			if baseCfg.Harness.Command != "" {
				harness = baseCfg.Harness
			} else if inferred := InferHarness(projectRoot); inferred != "" {
				harness = goalx.HarnessConfig{Command: inferred}
			}
		}
		return target, harness, nil
	default:
		return goalx.TargetConfig{}, goalx.HarnessConfig{}, fmt.Errorf("unsupported session mode %q", mode)
	}
}
