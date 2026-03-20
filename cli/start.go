package cli

import (
	"fmt"
	"os"
	"path/filepath"

	ar "github.com/vonbai/autoresearch"
	"gopkg.in/yaml.v3"
)

// Start executes the full goalx start workflow.
func Start(projectRoot string, args []string) error {
	if len(args) > 0 {
		if _, err := parseStartInitArgs(args); err != nil {
			return err
		}
		if err := Init(projectRoot, args); err != nil {
			return err
		}
	}

	// 1. Load config
	cfg, engines, err := ar.LoadConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// 2. Auto-generate name if missing
	if cfg.Name == "" {
		cfg.Name = ar.Slugify(cfg.Objective)
	}

	// 3. Validate before any side effects
	if err := ar.ValidateConfig(cfg, engines); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// 4. Compute paths
	runDir := ar.RunDir(projectRoot, cfg.Name)
	tmuxSess := ar.TmuxSessionName(projectRoot, cfg.Name)

	// 5. Check conflicts
	if _, err := os.Stat(runDir); err == nil {
		return fmt.Errorf("run directory already exists: %s (use a different name or goalx drop first)", runDir)
	}
	if SessionExists(tmuxSess) {
		return fmt.Errorf("tmux session already exists: %s (goalx stop first)", tmuxSess)
	}

	// 5b. Warn if working tree has uncommitted changes (worktrees are created from HEAD)
	if dirty, err := hasDirtyWorktree(projectRoot); err == nil && dirty {
		fmt.Fprintln(os.Stderr, "⚠ Working tree has uncommitted changes that won't be visible to subagents.")
		fmt.Fprintln(os.Stderr, "  Subagents work on worktrees created from the latest commit (HEAD).")
		fmt.Fprintln(os.Stderr, "  Consider: git add -A && git commit -m 'wip' before starting.")
		fmt.Fprintln(os.Stderr, "")
	}

	// 6. Expand sessions
	sessions := ar.ExpandSessions(cfg)

	// 7. Create run directory structure
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

	// Clear stale status.json from previous runs
	os.Remove(filepath.Join(projectRoot, ".goalx", "status.json"))

	// 8. Snapshot config (YAML for consistency with input format)
	cfgYAML, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config snapshot: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), cfgYAML, 0644); err != nil {
		return fmt.Errorf("write run snapshot: %w", err)
	}

	// 9. Build session data
	absProjectRoot, _ := filepath.Abs(projectRoot)
	var sessionDataList []SessionData

	for i, sess := range sessions {
		num := i + 1
		sName := SessionName(num)
		wtPath := WorktreePath(runDir, cfg.Name, num)
		journalPath := JournalPath(runDir, sName)
		guidancePath := GuidancePath(runDir, sName)
		branch := fmt.Sprintf("goalx/%s/%d", cfg.Name, num)

		// Resolve session engine/model (inherit from config if not set)
		sEngine := sess.Engine
		if sEngine == "" {
			sEngine = cfg.Engine
		}
		sModel := sess.Model
		if sModel == "" {
			sModel = cfg.Model
		}
		engineCmd, err := ar.ResolveEngineCommand(engines, sEngine, sModel)
		if err != nil {
			return fmt.Errorf("session-%d engine: %w", num, err)
		}
		// Subagents don't need skills — disable slash commands AND the Skill
		// tool itself (plugins inject skills via system prompt, bypassing
		// --disable-slash-commands; --disallowedTools Skill blocks invocation).
		if sEngine == "claude-code" {
			engineCmd += " --disable-slash-commands --disallowedTools Skill"
		}

		protocolPath := filepath.Join(runDir, fmt.Sprintf("program-%d.md", num))
		prompt := ar.ResolvePrompt(engines, sEngine, protocolPath)

		sessionDataList = append(sessionDataList, SessionData{
			Name:          sName,
			WindowName:    sessionWindowName(cfg.Name, num),
			WorktreePath:  wtPath,
			JournalPath:   journalPath,
			GuidancePath:  guidancePath,
			EngineCommand: engineCmd,
			Prompt:        prompt,
		})

		// Create worktree
		if err := CreateWorktree(absProjectRoot, wtPath, branch); err != nil {
			return fmt.Errorf("create worktree %s: %w", sName, err)
		}

		// Initialize empty journal
		if err := os.WriteFile(journalPath, nil, 0644); err != nil {
			return fmt.Errorf("init journal %s: %w", sName, err)
		}

		// Initialize empty guidance file
		if err := os.WriteFile(guidancePath, nil, 0644); err != nil {
			return fmt.Errorf("init guidance %s: %w", sName, err)
		}

		// Generate adapter
		if err := GenerateAdapter(sEngine, wtPath, guidancePath); err != nil {
			return fmt.Errorf("generate adapter %s: %w", sName, err)
		}

		if err := EnsureEngineTrusted(sEngine, wtPath); err != nil {
			return fmt.Errorf("trust bootstrap %s: %w", sName, err)
		}
	}

	// 10. Resolve master engine command
	masterCmd, err := ar.ResolveEngineCommand(engines, cfg.Master.Engine, cfg.Master.Model)
	if err != nil {
		return fmt.Errorf("master engine: %w", err)
	}
	masterProtocolPath := filepath.Join(runDir, "master.md")
	masterPrompt := ar.ResolvePrompt(engines, cfg.Master.Engine, masterProtocolPath)
	acceptancePath := filepath.Join(runDir, "acceptance.md")

	if err := EnsureEngineTrusted(cfg.Master.Engine, absProjectRoot); err != nil {
		return fmt.Errorf("trust bootstrap master: %w", err)
	}

	// 11. Render protocols
	masterData := ProtocolData{
		Objective:      cfg.Objective,
		Description:    cfg.Description,
		Mode:           cfg.Mode,
		Sessions:       sessionDataList,
		Master:         cfg.Master,
		Harness:        cfg.Harness,
		Budget:         cfg.Budget,
		Target:         cfg.Target,
		TmuxSession:    tmuxSess,
		SummaryPath:       filepath.Join(runDir, "summary.md"),
		AcceptancePath:    acceptancePath,
		MasterJournalPath: filepath.Join(runDir, "master.jsonl"),
		StatusPath:        filepath.Join(projectRoot, ".goalx", "status.json"),
		EngineCommand:     masterCmd,
	}
	if err := RenderMasterProtocol(masterData, runDir); err != nil {
		return fmt.Errorf("render master protocol: %w", err)
	}

	for i, sd := range sessionDataList {
		subData := ProtocolData{
			Objective:    cfg.Objective,
			Description:  cfg.Description,
			Mode:         cfg.Mode,
			Target:       cfg.Target,
			Harness:      cfg.Harness,
			Context:      cfg.Context,
			Budget:       cfg.Budget,
			SessionName:  sd.Name,
			JournalPath:  sd.JournalPath,
			GuidancePath: sd.GuidancePath,
			WorktreePath: sd.WorktreePath,
		}
		if i < len(sessions) && sessions[i].Hint != "" {
			subData.DiversityHint = sessions[i].Hint
		}
		if err := RenderSubagentProtocol(subData, runDir, i); err != nil {
			return fmt.Errorf("render protocol session-%d: %w", i+1, err)
		}
	}

	// 12. Initialize master journal
	if err := os.WriteFile(filepath.Join(runDir, "master.jsonl"), nil, 0644); err != nil {
		return fmt.Errorf("init master journal: %w", err)
	}
	if err := os.WriteFile(acceptancePath, nil, 0644); err != nil {
		return fmt.Errorf("init acceptance checklist: %w", err)
	}

	// 13. Create tmux session (first window = "master")
	if err := NewSession(tmuxSess, "master"); err != nil {
		return fmt.Errorf("tmux new-session: %w", err)
	}

	// Launch master
	masterLaunch := fmt.Sprintf("%s %q", masterCmd, masterPrompt)
	if err := SendKeys(tmuxSess+":master", masterLaunch); err != nil {
		return fmt.Errorf("launch master: %w", err)
	}

	// Launch subagents
	for _, sd := range sessionDataList {
		windowName := sd.WindowName
		if err := NewWindow(tmuxSess, windowName, sd.WorktreePath); err != nil {
			return fmt.Errorf("tmux new-window %s: %w", windowName, err)
		}
		subLaunch := fmt.Sprintf("%s %q", sd.EngineCommand, sd.Prompt)
		if err := SendKeys(tmuxSess+":"+windowName, subLaunch); err != nil {
			return fmt.Errorf("launch %s: %w", windowName, err)
		}
	}

	// Launch heartbeat window
	checkSec, warning := normalizeHeartbeatInterval(cfg.Master.CheckInterval)
	if warning != "" {
		fmt.Fprint(os.Stderr, warning)
	}
	hbCmd := HeartbeatCommand(tmuxSess, checkSec)
	if err := NewWindow(tmuxSess, "heartbeat", "/tmp"); err != nil {
		return fmt.Errorf("tmux heartbeat window: %w", err)
	}
	if err := SendKeys(tmuxSess+":heartbeat", hbCmd); err != nil {
		return fmt.Errorf("launch heartbeat: %w", err)
	}

	// 14. Print status
	fmt.Printf("✓ Run '%s' started\n", cfg.Name)
	fmt.Printf("  tmux session: %s\n", tmuxSess)
	fmt.Printf("  master: %s/%s\n", cfg.Master.Engine, cfg.Master.Model)
	fmt.Printf("  sessions: %d\n", len(sessions))
	for i, sd := range sessionDataList {
		engine := cfg.Engine
		model := cfg.Model
		if i < len(sessions) && sessions[i].Engine != "" {
			engine = sessions[i].Engine
		}
		if i < len(sessions) && sessions[i].Model != "" {
			model = sessions[i].Model
		}
		hint := ""
		if i < len(sessions) && sessions[i].Hint != "" {
			hint = " (" + sessions[i].Hint + ")"
		}
		fmt.Printf("    %s: %s/%s%s → %s\n", sd.Name, engine, model, hint, sd.WorktreePath)
	}
	fmt.Printf("  run dir: %s\n", runDir)
	fmt.Printf("  attach: goalx attach [--run %s] [session-N]\n", cfg.Name)
	return nil
}
