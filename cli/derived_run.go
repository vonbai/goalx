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
	Name           string
	Mode           string
	Objective      string
	RunDir         string
	RunID          string
	Epoch          int
	Charter        string
	Selector       string
	LifecycleState string
	Status         string
	Completed      bool
	HasLease       bool
	HasTmuxSession bool
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
	tmuxSession := goalx.TmuxSessionName(projectRoot, name)
	if err := repairCompletedRunFinalizationForRun(projectRoot, name, runDir, tmuxSession); err != nil {
		return nil, err
	}

	state := &DerivedRunState{
		Name:           name,
		Mode:           string(cfg.Mode),
		Objective:      cfg.Objective,
		RunDir:         runDir,
		Selector:       goalx.ProjectID(projectRoot) + "/" + name,
		HasLease:       controlLeaseActive(runDir, "sidecar") || controlLeaseActive(runDir, "master"),
		HasTmuxSession: SessionExists(tmuxSession),
	}
	state.RunID, state.Epoch, state.Charter, state.Objective = deriveRunIdentitySurface(runDir, cfg.Objective)

	runtimeState, _ := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	controlState, _ := LoadControlRunState(ControlRunStatePath(runDir))
	if controlState != nil && controlState.LifecycleState != "" {
		state.LifecycleState = controlState.LifecycleState
	} else if runtimeState != nil {
		switch {
		case runtimeState.StoppedAt != "":
			state.LifecycleState = "stopped"
		case runtimeState.Active:
			state.LifecycleState = "active"
		case runtimeState.Phase == "complete":
			state.LifecycleState = "completed"
		default:
			state.LifecycleState = "inactive"
		}
	}
	state.Completed = runtimeState != nil && runtimeState.Phase == "complete"

	switch state.LifecycleState {
	case "active":
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
		sidecarMissing := targetPresenceMissing(presence["sidecar"])
		switch {
		case masterMissing || sidecarMissing || sessionMissing:
			if !state.HasLease && !state.HasTmuxSession && !sessionTruthAvailable {
				state.Status = "stranded"
				break
			}
			state.Status = "degraded"
		case state.HasLease || state.HasTmuxSession || sessionTruthAvailable:
			state.Status = "active"
		default:
			state.Status = "degraded"
		}
	case "completed":
		state.Status = "completed"
		state.Completed = true
	case "stopped", "inactive", "dropped":
		state.Status = state.LifecycleState
	default:
		if state.Completed {
			state.Status = "completed"
		} else if state.HasLease {
			state.Status = "active"
		} else {
			state.Status = "completed"
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
	case "active", "degraded", "stranded":
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
