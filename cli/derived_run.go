package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type DerivedRunState struct {
	Name            string
	Mode            string
	Objective       string
	RunDir          string
	RunID           string
	Epoch           int
	Charter         string
	Selector        string
	GoalState       string
	ContinuityState string
	Status          string
	StartupPhase    string
	Completed       bool
	HasLease        bool
	HasTmuxSession  bool
}

func loadDerivedRunState(projectRoot, runDir string) (*DerivedRunState, error) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return nil, err
	}
	name := cfg.Name
	if name == "" {
		name = filepath.Base(runDir)
	}
	tmuxSession := resolveRunTmuxSession(projectRoot, runDir, name)
	if err := repairCompletedRunFinalizationForRun(projectRoot, name, runDir, tmuxSession); err != nil {
		return nil, err
	}
	if err := reconcileRunContinuityForRun(projectRoot, name, runDir); err != nil {
		return nil, err
	}

	state := &DerivedRunState{
		Name:           name,
		Mode:           string(cfg.Mode),
		Objective:      cfg.Objective,
		RunDir:         runDir,
		Selector:       goalx.ProjectID(projectRoot) + "/" + name,
		HasLease:       controlLeaseActive(runDir, "runtime-host") || controlLeaseActive(runDir, "master"),
		HasTmuxSession: SessionExistsInRun(runDir, tmuxSession),
	}
	state.RunID, state.Epoch, state.Charter, state.Objective = deriveRunIdentitySurface(runDir, cfg.Objective)

	runtimeState, _ := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	controlState, _ := LoadControlRunState(ControlRunStatePath(runDir))
	startup, err := deriveRunStartupState(runDir, tmuxSession, controlState, runtimeState)
	if err != nil {
		return nil, err
	}
	state.StartupPhase = startup.Phase
	if controlState != nil {
		state.GoalState = strings.TrimSpace(controlState.GoalState)
		state.ContinuityState = strings.TrimSpace(controlState.ContinuityState)
	}
	if state.GoalState == "" && runtimeState != nil && runtimeState.Phase == "complete" {
		state.GoalState = "completed"
	}
	if state.GoalState == "" {
		state.GoalState = "open"
	}
	if state.ContinuityState == "" && runtimeState != nil {
		switch {
		case runtimeState.StoppedAt != "":
			state.ContinuityState = "stopped"
		case runtimeState.Active:
			state.ContinuityState = "running"
		case runtimeState.Phase == "complete":
			state.ContinuityState = "stopped"
		}
	}
	if state.ContinuityState == "" {
		if state.GoalState == "completed" || state.GoalState == "dropped" {
			state.ContinuityState = "stopped"
		} else if state.HasLease || state.HasTmuxSession {
			state.ContinuityState = "running"
		} else {
			state.ContinuityState = "stopped"
		}
	}
	state.Completed = state.GoalState == "completed" || (runtimeState != nil && runtimeState.Phase == "complete")

	switch state.GoalState {
	case "completed":
		state.Status = "completed"
		state.Completed = true
	case "dropped":
		state.Status = "dropped"
	default:
		if startup.Launching() {
			state.Status = "launching"
			return state, nil
		}
		switch state.ContinuityState {
		case "stranded":
			state.Status = "stranded"
		case "stopped":
			state.Status = "stopped"
		default:
			presence, err := BuildTargetPresenceFacts(runDir, tmuxSession)
			if err != nil {
				return nil, err
			}
			sessionMissing := false
			sessionTruthAvailable := false
			for target, facts := range presence {
				if !strings.HasPrefix(target, "session-") {
					continue
				}
				if strings.TrimSpace(facts.State) == TargetPresencePresent || strings.TrimSpace(facts.State) == TargetPresenceParked {
					sessionTruthAvailable = true
				}
				if targetPresenceMissing(facts) {
					sessionMissing = true
				}
			}
			masterMissing := targetPresenceMissing(presence["master"])
			sidecarMissing := targetPresenceMissing(presence["runtime-host"])
			switch {
			case masterMissing || sidecarMissing || sessionMissing:
				state.Status = "degraded"
			case state.HasLease || state.HasTmuxSession || sessionTruthAvailable:
				state.Status = "active"
			default:
				state.Status = "degraded"
			}
		}
	}
	return state, nil
}

func deriveRunIdentitySurface(runDir, fallbackObjective string) (string, int, string, string) {
	runID := ""
	epoch := 0
	objective := strings.TrimSpace(fallbackObjective)

	meta, _ := LoadRunMetadata(RunMetadataPath(runDir))
	if meta != nil {
		if meta.RunID != "" {
			runID = meta.RunID
		}
		if meta.Epoch > 0 {
			epoch = meta.Epoch
		}
	}

	charter, _ := LoadRunCharter(RunCharterPath(runDir))
	if charter != nil {
		if strings.TrimSpace(charter.Objective) != "" {
			objective = strings.TrimSpace(charter.Objective)
		}
		if runID == "" && charter.RunID != "" {
			runID = charter.RunID
		}
	}

	identity, _ := LoadControlRunIdentity(ControlRunIdentityPath(runDir))
	if identity != nil {
		if runID == "" && identity.RunID != "" {
			runID = identity.RunID
		}
		if epoch == 0 && identity.Epoch > 0 {
			epoch = identity.Epoch
		}
	}

	charterStatus := "missing"
	if meta != nil && charter != nil && identity != nil {
		if err := ValidateRunCharterLinkage(meta, charter); err == nil {
			charterStatus = "ok"
		} else {
			charterStatus = "mismatch"
		}
	}
	return runID, epoch, charterStatus, objective
}

func listDerivedRunStates(projectRoot string) ([]DerivedRunState, error) {
	// Collect runs from configured run root and legacy location
	seenDirs := make(map[string]bool)
	states := make([]DerivedRunState, 0)

	// Scan configured run root first
	if layers, err := goalx.LoadConfigLayers(projectRoot); err == nil && layers.Config.RunRoot != "" {
		configuredRoot := goalx.ResolveRunRoot(projectRoot, &layers.Config)
		if configuredStates, err := scanRunDirs(projectRoot, configuredRoot, seenDirs); err == nil {
			states = append(states, configuredStates...)
		}
	}

	// Scan legacy location (user-scoped)
	legacyRoot := ProjectDataDir(projectRoot)
	if legacyStates, err := scanRunDirs(projectRoot, legacyRoot, seenDirs); err == nil {
		states = append(states, legacyStates...)
	}

	// Scan registry-discovered runs for the current project to survive config drift.
	if reg, err := LoadGlobalRunRegistry(); err == nil && reg != nil {
		for _, ref := range reg.Runs {
			if ref.ProjectRoot != projectRoot || strings.TrimSpace(ref.RunDir) == "" {
				continue
			}
			runDir := filepath.Clean(ref.RunDir)
			if seenDirs[runDir] {
				continue
			}
			seenDirs[runDir] = true
			state, err := loadDerivedRunState(projectRoot, runDir)
			if err != nil {
				continue
			}
			states = append(states, *state)
		}
	}

	sort.Slice(states, func(i, j int) bool {
		return states[i].Name < states[j].Name
	})
	return states, nil
}

func scanRunDirs(projectRoot, runsDir string, seenDirs map[string]bool) ([]DerivedRunState, error) {
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	states := make([]DerivedRunState, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "saved" {
			continue
		}
		runDir := filepath.Join(runsDir, entry.Name())
		if seenDirs[runDir] {
			continue
		}
		seenDirs[runDir] = true
		state, err := loadDerivedRunState(projectRoot, runDir)
		if err != nil {
			continue
		}
		states = append(states, *state)
	}
	return states, nil
}

func controlLeaseActive(runDir, holder string) bool {
	lease, err := LoadControlLease(ControlLeasePath(runDir, holder))
	if err != nil || lease == nil || lease.ExpiresAt == "" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, lease.ExpiresAt)
	if err != nil {
		return false
	}
	return expiresAt.After(time.Now().UTC())
}

func derivedRunStatusOpen(status string) bool {
	switch strings.TrimSpace(status) {
	case "active", "degraded", "stranded", "launching":
		return true
	default:
		return false
	}
}

func findSingleRunnableRun(projectRoot string) (*RunContext, error) {
	states, err := listDerivedRunStates(projectRoot)
	if err != nil {
		return nil, err
	}
	runnable := make([]string, 0)
	for _, state := range states {
		if derivedRunStatusOpen(state.Status) {
			runnable = append(runnable, state.Name)
		}
	}
	switch len(runnable) {
	case 0:
		return nil, fmt.Errorf("no runs found")
	case 1:
		return ResolveRun(projectRoot, runnable[0])
	default:
		sort.Strings(runnable)
		return nil, fmt.Errorf("multiple active runs: %s (specify --run)", strings.Join(runnable, ", "))
	}
}
