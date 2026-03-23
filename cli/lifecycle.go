package cli

import (
	"fmt"
	"os"
	"path/filepath"
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

	coord, err := EnsureCoordinationState(rc.RunDir, rc.Config.Objective)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	digest := inferSessionCoordination(JournalPath(rc.RunDir, sessionName))
	current := coord.Sessions[sessionName]
	if digest.Scope == "" {
		digest.Scope = current.Scope
	}
	if digest.BlockedBy == "" {
		digest.BlockedBy = current.BlockedBy
	}
	if digest.LastRound == 0 {
		digest.LastRound = current.LastRound
	}
	digest.State = "parked"
	digest.UpdatedAt = now
	coord.Sessions[sessionName] = digest
	coord.Version++
	coord.UpdatedAt = now
	snapshot, err := SnapshotSessionRuntime(rc.RunDir, sessionName, WorktreePath(rc.RunDir, rc.Config.Name, idx))
	if err != nil {
		return fmt.Errorf("snapshot session runtime: %w", err)
	}
	snapshot.State = "parked"
	snapshot.Mode = string(goalx.EffectiveSessionConfig(rc.Config, idx-1).Mode)
	snapshot.Branch = fmt.Sprintf("goalx/%s/%d", rc.Config.Name, idx)
	snapshot.OwnerScope = digest.Scope
	snapshot.BlockedBy = digest.BlockedBy
	if err := SaveCoordinationState(CoordinationPath(rc.RunDir), coord); err != nil {
		return err
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
			_, _ = DeliverControlNudge(rc.RunDir, "session-parked:"+sessionName, "session-parked:"+sessionName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudge)
		}
	}
	if err := UpsertSessionRuntimeState(rc.RunDir, snapshot); err != nil {
		return fmt.Errorf("update session runtime state: %w", err)
	}

	fmt.Printf("Parked %s in run '%s'\n", sessionName, rc.Name)
	return nil
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

	wtPath := WorktreePath(rc.RunDir, rc.Config.Name, idx)
	if info, err := os.Stat(wtPath); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("%s is not a directory", wtPath)
		}
		return fmt.Errorf("resume %s requires existing worktree: %w", sessionName, err)
	}
	if _, err := EnsureSessionGuidanceState(rc.RunDir, sessionName); err != nil {
		return fmt.Errorf("init guidance state: %w", err)
	}

	_, engines, err := goalx.LoadConfig(rc.ProjectRoot)
	if err != nil {
		_, engines, err = goalx.LoadRawBaseConfig(rc.ProjectRoot)
		if err != nil {
			return fmt.Errorf("load config for engine resolution: %w", err)
		}
	}
	effective := goalx.EffectiveSessionConfig(rc.Config, idx-1)
	engineCmd, err := goalx.ResolveEngineCommand(engines, effective.Engine, effective.Model)
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	if effective.Engine == "claude-code" {
		engineCmd += " --disable-slash-commands"
	}

	sessionDataList, err := buildSessionDataList(rc.RunDir, rc.Config, engines)
	if err != nil {
		return fmt.Errorf("build session roster: %w", err)
	}
	subData := ProtocolData{
		RunName:             rc.Config.Name,
		Objective:           rc.Config.Objective,
		Mode:                effective.Mode,
		Engine:              effective.Engine,
		Sessions:            sessionDataList,
		Target:              *effective.Target,
		Harness:             *effective.Harness,
		Context:             rc.Config.Context,
		Budget:              rc.Config.Budget,
		SessionName:         sessionName,
		SessionIndex:        idx - 1,
		JournalPath:         JournalPath(rc.RunDir, sessionName),
		GuidancePath:        GuidancePath(rc.RunDir, sessionName),
		GuidanceStatePath:   SessionGuidanceStatePath(rc.RunDir, sessionName),
		WorktreePath:        wtPath,
		GoalContractPath:    GoalContractPath(rc.RunDir),
		AcceptancePath:      AcceptanceChecklistPath(rc.RunDir),
		AcceptanceStatePath: AcceptanceStatePath(rc.RunDir),
		RunStatePath:        RunRuntimeStatePath(rc.RunDir),
		SessionsStatePath:   SessionsRuntimeStatePath(rc.RunDir),
		ProjectRegistryPath: ProjectRegistryPath(rc.ProjectRoot),
		ProjectRoot:         absProjectRoot,
		DiversityHint:       effective.Hint,
	}
	if err := RenderSubagentProtocol(subData, rc.RunDir, idx-1); err != nil {
		return fmt.Errorf("render protocol: %w", err)
	}
	protocolPath := filepath.Join(rc.RunDir, sessionNameToProgramFile(idx))
	prompt := goalx.ResolvePrompt(engines, effective.Engine, protocolPath)

	if err := NewWindow(rc.TmuxSession, windowName, wtPath); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}
	launchCmd := fmt.Sprintf("%s %q", engineCmd, prompt)
	if err := SendKeys(rc.TmuxSession+":"+windowName, launchCmd); err != nil {
		return fmt.Errorf("launch subagent: %w", err)
	}

	coord, err := EnsureCoordinationState(rc.RunDir, rc.Config.Objective)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	current := coord.Sessions[sessionName]
	current.State = "active"
	current.BlockedBy = ""
	if current.Scope == "" {
		current.Scope = effective.Hint
	}
	if current.LastRound == 0 {
		current.LastRound = inferSessionCoordination(JournalPath(rc.RunDir, sessionName)).LastRound
	}
	current.UpdatedAt = now
	coord.Sessions[sessionName] = current
	coord.Version++
	coord.UpdatedAt = now
	if err := SaveCoordinationState(CoordinationPath(rc.RunDir), coord); err != nil {
		return err
	}
	if err := UpsertSessionRuntimeState(rc.RunDir, SessionRuntimeState{
		Name:         sessionName,
		State:        "active",
		Mode:         string(effective.Mode),
		Branch:       fmt.Sprintf("goalx/%s/%d", rc.Config.Name, idx),
		WorktreePath: wtPath,
		OwnerScope:   current.Scope,
	}); err != nil {
		return fmt.Errorf("update session runtime state: %w", err)
	}
	if _, err := AppendMasterInboxMessage(rc.RunDir, "session_resumed", "goalx resume", fmt.Sprintf("%s was resumed for reuse.", sessionName)); err == nil {
		_, _ = DeliverControlNudge(rc.RunDir, "session-resumed:"+sessionName, "session-resumed:"+sessionName, rc.TmuxSession+":master", rc.Config.Master.Engine, sendAgentNudge)
	}

	fmt.Printf("Resumed %s in run '%s'\n", sessionName, rc.Name)
	return nil
}

func inferSessionCoordination(journalPath string) CoordinationSession {
	entries, err := goalx.LoadJournal(journalPath)
	if err != nil || len(entries) == 0 {
		return CoordinationSession{}
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
	return CoordinationSession{
		State:     state,
		Scope:     last.OwnerScope,
		BlockedBy: last.BlockedBy,
		LastRound: last.Round,
	}
}

func sessionNameToProgramFile(idx int) string {
	return fmt.Sprintf("program-%d.md", idx)
}
