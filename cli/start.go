package cli

import (
	"fmt"
	"os"
	"path/filepath"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

// Start executes the full goalx start workflow.
func Start(projectRoot string, args []string) (err error) {
	if len(args) > 0 {
		if _, err := parseStartInitArgs(args); err != nil {
			return err
		}
		if err := Init(projectRoot, args); err != nil {
			return err
		}
	}

	// 1. Load config
	cfg, engines, err := goalx.LoadConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

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
	type cleanupWorktree struct {
		path   string
		branch string
	}
	var createdWorktrees []cleanupWorktree
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
		for i := len(createdWorktrees) - 1; i >= 0; i-- {
			wt := createdWorktrees[i]
			if rmErr := RemoveWorktree(absProjectRoot, wt.path); rmErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup worktree %s: %v\n", wt.path, rmErr)
			}
			if delErr := DeleteBranch(absProjectRoot, wt.branch); delErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup branch %s: %v\n", wt.branch, delErr)
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

	// 5b. Warn if working tree has uncommitted changes (worktrees are created from HEAD)
	if dirty, err := hasDirtyWorktree(projectRoot); err == nil && dirty {
		fmt.Fprintln(os.Stderr, "⚠ Working tree has uncommitted changes that won't be visible to subagents.")
		fmt.Fprintln(os.Stderr, "  Subagents work on worktrees created from the latest commit (HEAD).")
		fmt.Fprintln(os.Stderr, "  Consider: git add -A && git commit -m 'wip' before starting.")
		fmt.Fprintln(os.Stderr, "")
	}

	// 6. Expand sessions
	sessions := goalx.ExpandSessions(cfg)

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
		engineCmd, err := goalx.ResolveEngineCommand(engines, sEngine, sModel)
		if err != nil {
			return fmt.Errorf("session-%d engine: %w", num, err)
		}
		// Subagents don't need skills — disable slash commands. The Skill
		// tool is denied via project-level .claude/settings.json (written by
		// ensureSubagentRestrictions) to avoid --disallowedTools eating the
		// next positional argument.
		if sEngine == "claude-code" {
			engineCmd += " --disable-slash-commands"
		}

		protocolPath := filepath.Join(runDir, fmt.Sprintf("program-%d.md", num))
		prompt := goalx.ResolvePrompt(engines, sEngine, protocolPath)

		sessionDataList = append(sessionDataList, SessionData{
			Name:          sName,
			WindowName:    sessionWindowName(cfg.Name, num),
			WorktreePath:  wtPath,
			JournalPath:   journalPath,
			GuidancePath:  guidancePath,
			Engine:        sEngine,
			EngineCommand: engineCmd,
			Prompt:        prompt,
		})

		// Create worktree
		if err := CreateWorktree(absProjectRoot, wtPath, branch); err != nil {
			return fmt.Errorf("create worktree %s: %w", sName, err)
		}
		createdWorktrees = append(createdWorktrees, cleanupWorktree{path: wtPath, branch: branch})

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

		// Deny Skill tool via project-level settings.json
		if sEngine == "claude-code" {
			if err := ensureSubagentRestrictions(wtPath); err != nil {
				return fmt.Errorf("subagent restrictions %s: %w", sName, err)
			}
		}
	}

	// 10. Resolve master engine command
	masterCmd, err := goalx.ResolveEngineCommand(engines, cfg.Master.Engine, cfg.Master.Model)
	if err != nil {
		return fmt.Errorf("master engine: %w", err)
	}
	masterProtocolPath := filepath.Join(runDir, "master.md")
	masterPrompt := goalx.ResolvePrompt(engines, cfg.Master.Engine, masterProtocolPath)
	acceptancePath := filepath.Join(runDir, "acceptance.md")
	statusPath := filepath.Join(projectRoot, ".goalx", "status.json")

	if err := EnsureEngineTrusted(cfg.Master.Engine, absProjectRoot); err != nil {
		return fmt.Errorf("trust bootstrap master: %w", err)
	}
	if err := GenerateMasterAdapter(cfg.Master.Engine, absProjectRoot, statusPath); err != nil {
		return fmt.Errorf("generate master adapter: %w", err)
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
		Context:        cfg.Context,
		TmuxSession:    tmuxSess,
		ProjectRoot:       absProjectRoot,
		SummaryPath:       filepath.Join(runDir, "summary.md"),
		AcceptancePath:    acceptancePath,
		MasterJournalPath: filepath.Join(runDir, "master.jsonl"),
		StatusPath:        statusPath,
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
			Engine:       sd.Engine,
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
