package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

// Add creates a new subagent session in a running run.
func Add(projectRoot string, args []string) error {
	// Parse: goalx add "hint/direction" [--run NAME] [--engine ENGINE] [--model MODEL] [--strategy NAME]
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}

	// Extract flags from rest args
	var flagEngine, flagModel string
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
		return fmt.Errorf("usage: goalx add \"research direction\" [--run NAME] [--engine ENGINE] [--model MODEL] [--strategy NAME]")
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

	// Determine next session number
	existingSessions := sessionCount(rc.Config)
	newNum := existingSessions + 1
	sName := SessionName(newNum)
	wtPath := WorktreePath(rc.RunDir, rc.Config.Name, newNum)
	journalPath := JournalPath(rc.RunDir, sName)
	guidancePath := GuidancePath(rc.RunDir, sName)
	branch := fmt.Sprintf("goalx/%s/%d", rc.Config.Name, newNum)
	windowName := sessionWindowName(rc.Config.Name, newNum)

	// Resolve engine
	_, engines, err := goalx.LoadConfig(projectRoot)
	if err != nil {
		// Fallback: try base config
		_, engines, err = goalx.LoadRawBaseConfig(projectRoot)
		if err != nil {
			return fmt.Errorf("load config for engine resolution: %w", err)
		}
	}

	engine := rc.Config.Engine
	model := rc.Config.Model
	if flagEngine != "" {
		engine = flagEngine
	}
	if flagModel != "" {
		model = flagModel
	}
	engineCmd, err := goalx.ResolveEngineCommand(engines, engine, model)
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	if engine == "claude-code" {
		engineCmd += " --disable-slash-commands"
	}

	// Create worktree
	absProjectRoot, _ := filepath.Abs(projectRoot)
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

	// Render protocol
	protocolPath := filepath.Join(rc.RunDir, fmt.Sprintf("program-%d.md", newNum))
	subData := ProtocolData{
		Objective:     rc.Config.Objective,
		Mode:          rc.Config.Mode,
		Target:        rc.Config.Target,
		Harness:       rc.Config.Harness,
		Context:       rc.Config.Context,
		Budget:        rc.Config.Budget,
		SessionName:   sName,
		JournalPath:   journalPath,
		GuidancePath:  guidancePath,
		WorktreePath:  wtPath,
		DiversityHint: hint,
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

	// Notify master
	masterMsg := fmt.Sprintf(
		"New %s added to your run. Window: %s, Worktree: %s, Journal: %s, Guidance: %s. Direction: %s. Add it to your check cycle.",
		sName, windowName, wtPath, journalPath, guidancePath, hint,
	)
	SendKeys(rc.TmuxSession+":master", masterMsg)

	// Update config snapshot with new session count
	newSess := goalx.SessionConfig{Hint: hint}
	if flagEngine != "" {
		newSess.Engine = flagEngine
	}
	if flagModel != "" {
		newSess.Model = flagModel
	}
	if len(rc.Config.Sessions) > 0 {
		rc.Config.Sessions = append(rc.Config.Sessions, newSess)
	} else {
		rc.Config.Parallel = newNum
	}
	cfgYAML, err := yaml.Marshal(&rc.Config)
	if err != nil {
		return fmt.Errorf("marshal config snapshot: %w", err)
	}
	if err := os.WriteFile(filepath.Join(rc.RunDir, "goalx.yaml"), cfgYAML, 0644); err != nil {
		return fmt.Errorf("write config snapshot: %w", err)
	}

	fmt.Printf("Added %s to run '%s'\n", sName, rc.Name)
	fmt.Printf("  window: %s\n", windowName)
	fmt.Printf("  direction: %s\n", hint)
	fmt.Printf("  master notified\n")
	return nil
}
