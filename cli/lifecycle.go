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
	sessionIdentity, err := RequireSessionIdentity(rc.RunDir, sessionName)
	if err != nil {
		return fmt.Errorf("load session identity: %w", err)
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
	sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load session runtime state: %w", err)
	}
	worktreePath := resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, sessionName, sessionState)
	snapshot, err := SnapshotSessionRuntime(rc.RunDir, sessionName, worktreePath)
	if err != nil {
		return fmt.Errorf("snapshot session runtime: %w", err)
	}
	snapshot.State = "parked"
	snapshot.Mode = sessionIdentity.Mode
	snapshot.Branch = resolvedSessionBranch(rc.RunDir, rc.Config.Name, sessionName, sessionState)
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
	_ = ExpireControlLease(rc.RunDir, sessionName)
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

	_, engines, err := goalx.LoadConfig(rc.ProjectRoot)
	if err != nil {
		_, engines, err = goalx.LoadRawBaseConfig(rc.ProjectRoot)
		if err != nil {
			return fmt.Errorf("load config for engine resolution: %w", err)
		}
	}
	engineCmd, err := goalx.ResolveEngineCommand(engines, sessionIdentity.Engine, sessionIdentity.Model)
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	if sessionIdentity.Engine == "claude-code" {
		engineCmd += " --disable-slash-commands"
	}
	coord, err := EnsureCoordinationState(rc.RunDir, rc.Config.Objective)
	if err != nil {
		return err
	}
	current := coord.Sessions[sessionName]

	sessionDataList, err := buildSessionDataList(rc.RunDir, rc.Config, engines)
	if err != nil {
		return fmt.Errorf("build session roster: %w", err)
	}
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
		SessionName:         sessionName,
		SessionIndex:        idx - 1,
		JournalPath:         JournalPath(rc.RunDir, sessionName),
		CharterPath:         RunCharterPath(rc.RunDir),
		SessionIdentityPath: SessionIdentityPath(rc.RunDir, sessionName),
		SessionInboxPath:    ControlInboxPath(rc.RunDir, sessionName),
		SessionCursorPath:   SessionCursorPath(rc.RunDir, sessionName),
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
		DiversityHint:       current.Scope,
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
	launchEnv, err := RequireLaunchEnvSnapshot(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load run launch env: %w", err)
	}
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	checkSec, _ := normalizeSidecarInterval(rc.Config.Master.CheckInterval)
	sessionLeaseTTL := time.Duration(checkSec) * time.Second * 2

	launchCmd := buildLeaseWrappedLaunchCommandWithEnv(launchEnv.Env, goalxBin, rc.Name, rc.RunDir, sessionName, meta.RunID, meta.Epoch, sessionLeaseTTL, engineCmd, prompt)
	if err := NewWindowWithCommand(rc.TmuxSession, windowName, workdir, launchCmd); err != nil {
		return fmt.Errorf("create tmux window: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	current.State = "active"
	current.BlockedBy = ""
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
		Mode:         sessionIdentity.Mode,
		Branch:       resolvedSessionBranch(rc.RunDir, rc.Config.Name, sessionName, sessionState),
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
