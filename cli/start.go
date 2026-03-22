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

	// 9. Resolve master engine command
	masterCmd, err := goalx.ResolveEngineCommand(engines, cfg.Master.Engine, cfg.Master.Model)
	if err != nil {
		return fmt.Errorf("master engine: %w", err)
	}
	masterProtocolPath := filepath.Join(runDir, "master.md")
	masterPrompt := goalx.ResolvePrompt(engines, cfg.Master.Engine, masterProtocolPath)
	acceptancePath := AcceptanceChecklistPath(runDir)
	acceptanceStatePath := AcceptanceStatePath(runDir)
	statusPath := filepath.Join(projectRoot, ".goalx", "status.json")

	if err := EnsureEngineTrusted(cfg.Master.Engine, absProjectRoot); err != nil {
		return fmt.Errorf("trust bootstrap master: %w", err)
	}
	if err := GenerateMasterAdapter(cfg.Master.Engine, absProjectRoot, statusPath); err != nil {
		return fmt.Errorf("generate master adapter: %w", err)
	}

	// 11. Render protocols
	masterData := ProtocolData{
		RunName:             cfg.Name,
		Objective:           cfg.Objective,
		Description:         cfg.Description,
		Mode:                cfg.Mode,
		Engines:             engines,
		Master:              cfg.Master,
		Harness:             cfg.Harness,
		Budget:              cfg.Budget,
		Target:              cfg.Target,
		Context:             cfg.Context,
		Preferences:         cfg.Preferences,
		TmuxSession:         tmuxSess,
		ProjectRoot:         absProjectRoot,
		SummaryPath:         filepath.Join(runDir, "summary.md"),
		AcceptancePath:      acceptancePath,
		AcceptanceStatePath: acceptanceStatePath,
		MasterJournalPath:   filepath.Join(runDir, "master.jsonl"),
		StatusPath:          statusPath,
		EngineCommand:       masterCmd,
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
	if _, err := EnsureAcceptanceState(runDir, cfg); err != nil {
		return fmt.Errorf("init acceptance state: %w", err)
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

	// Launch heartbeat window (pure timer — wakes master periodically)
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
	fmt.Printf("  run dir: %s\n", runDir)
	fmt.Printf("  attach: goalx attach [--run %s] [master|session-N]\n", cfg.Name)
	return nil
}
