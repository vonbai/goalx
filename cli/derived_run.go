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

	state := &DerivedRunState{
		Name:           name,
		Mode:           string(cfg.Mode),
		Objective:      cfg.Objective,
		RunDir:         runDir,
		Selector:       goalx.ProjectID(projectRoot) + "/" + name,
		HasLease:       controlLeaseActive(runDir, "sidecar") || controlLeaseActive(runDir, "master"),
		HasTmuxSession: SessionExists(goalx.TmuxSessionName(projectRoot, name)),
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
		if state.HasLease {
			state.Status = "active"
		} else {
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
		if strings.TrimSpace(meta.Objective) != "" {
			objective = strings.TrimSpace(meta.Objective)
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

	return runID, epoch, deriveRunCharterStatus(runDir, meta, charter, identity), objective
}

func deriveRunCharterStatus(runDir string, meta *RunMetadata, charter *RunCharter, identity *ControlRunIdentity) string {
	if meta == nil || charter == nil || identity == nil {
		return "missing"
	}
	if err := ValidateRunCharterLinkage(meta, charter); err != nil {
		return "mismatch"
	}
	if strings.TrimSpace(identity.CharterPath) == "" || identity.CharterPath != RunCharterPath(runDir) {
		return "mismatch"
	}
	if strings.TrimSpace(identity.RunID) == "" || identity.Epoch <= 0 || strings.TrimSpace(identity.CharterID) == "" || strings.TrimSpace(identity.CharterDigest) == "" {
		return "missing"
	}
	charterHash, err := hashRunCharter(charter)
	if err != nil {
		return "mismatch"
	}
	switch {
	case identity.RunID != meta.RunID:
		return "mismatch"
	case identity.Epoch != meta.Epoch:
		return "mismatch"
	case identity.CharterID != charter.CharterID:
		return "mismatch"
	case identity.CharterDigest != charterHash:
		return "mismatch"
	default:
		return "ok"
	}
}

func listDerivedRunStates(projectRoot string) ([]DerivedRunState, error) {
	runsDir := ProjectDataDir(projectRoot)
	entries, err := os.ReadDir(runsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	states := make([]DerivedRunState, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "saved" {
			continue
		}
		runDir := filepath.Join(runsDir, entry.Name())
		state, err := loadDerivedRunState(projectRoot, runDir)
		if err != nil {
			continue
		}
		states = append(states, *state)
	}
	sort.Slice(states, func(i, j int) bool {
		return states[i].Name < states[j].Name
	})
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

func findSingleRunnableRun(projectRoot string) (*RunContext, error) {
	states, err := listDerivedRunStates(projectRoot)
	if err != nil {
		return nil, err
	}
	runnable := make([]string, 0)
	for _, state := range states {
		if state.Status == "active" || state.Status == "degraded" {
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
