package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	EvolveFrontierActive  = "active"
	EvolveFrontierStopped = "stopped"
)

type EvolveFacts struct {
	Version               int      `json:"version"`
	FrontierState         string   `json:"frontier_state,omitempty"`
	BestExperimentID      string   `json:"best_experiment_id,omitempty"`
	OpenCandidateIDs      []string `json:"open_candidate_ids,omitempty"`
	OpenCandidateCount    int      `json:"open_candidate_count,omitempty"`
	LastStopReasonCode    string   `json:"last_stop_reason_code,omitempty"`
	LastStopAt            string   `json:"last_stop_at,omitempty"`
	LastManagementEventAt string   `json:"last_management_event_at,omitempty"`
	ManagementGap         string   `json:"management_gap,omitempty"`
	UpdatedAt             string   `json:"updated_at,omitempty"`
}

func EvolveFactsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "evolve-facts.json")
}

func BuildEvolveFacts(runDir string) (*EvolveFacts, error) {
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	if meta == nil || strings.TrimSpace(meta.Intent) != runIntentEvolve {
		return nil, nil
	}
	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		return nil, err
	}

	facts := &EvolveFacts{Version: 1}
	if state, err := LoadIntegrationState(IntegrationStatePath(runDir)); err != nil {
		return nil, err
	} else if state != nil {
		facts.BestExperimentID = strings.TrimSpace(state.CurrentExperimentID)
	}

	created := map[string]time.Time{}
	closed := map[string]time.Time{}
	latestOpenAt := time.Time{}
	latestMgmtAt := time.Time{}
	latestStopAt := time.Time{}

	for _, event := range events {
		at, err := time.Parse(time.RFC3339, strings.TrimSpace(event.At))
		if err != nil {
			return nil, err
		}
		if at.After(latestMgmtAt) {
			latestMgmtAt = at
		}
		switch strings.TrimSpace(event.Kind) {
		case "experiment.created":
			var body ExperimentCreatedBody
			if err := json.Unmarshal(event.Body, &body); err != nil {
				return nil, err
			}
			id := strings.TrimSpace(body.ExperimentID)
			if id == "" {
				continue
			}
			created[id] = at
			if at.After(latestOpenAt) {
				latestOpenAt = at
			}
		case "experiment.integrated":
			if at.After(latestOpenAt) {
				latestOpenAt = at
			}
		case "experiment.closed":
			var body ExperimentClosedBody
			if err := json.Unmarshal(event.Body, &body); err != nil {
				return nil, err
			}
			id := strings.TrimSpace(body.ExperimentID)
			if id == "" {
				continue
			}
			closed[id] = at
		case "evolve.stopped":
			var body EvolveStoppedBody
			if err := json.Unmarshal(event.Body, &body); err != nil {
				return nil, err
			}
			if at.After(latestStopAt) {
				latestStopAt = at
				facts.LastStopReasonCode = strings.TrimSpace(body.ReasonCode)
				facts.LastStopAt = strings.TrimSpace(body.StoppedAt)
			}
		}
	}

	openIDs := make([]string, 0, len(created))
	for id, createdAt := range created {
		if closedAt, ok := closed[id]; ok && (closedAt.Equal(createdAt) || closedAt.After(createdAt)) {
			continue
		}
		openIDs = append(openIDs, id)
	}
	sort.Strings(openIDs)
	facts.OpenCandidateIDs = openIDs
	facts.OpenCandidateCount = len(openIDs)

	switch {
	case !latestStopAt.IsZero() && (latestOpenAt.IsZero() || latestStopAt.After(latestOpenAt) || latestStopAt.Equal(latestOpenAt)):
		facts.FrontierState = EvolveFrontierStopped
	case !latestOpenAt.IsZero():
		facts.FrontierState = EvolveFrontierActive
	}

	if !latestMgmtAt.IsZero() {
		facts.LastManagementEventAt = latestMgmtAt.UTC().Format(time.RFC3339)
	}
	facts.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return facts, nil
}

func LoadEvolveFacts(path string) (*EvolveFacts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var facts EvolveFacts
	if err := json.Unmarshal(data, &facts); err != nil {
		return nil, err
	}
	if facts.Version <= 0 {
		facts.Version = 1
	}
	return &facts, nil
}

func SaveEvolveFacts(runDir string, facts *EvolveFacts) error {
	if facts == nil {
		return os.Remove(EvolveFactsPath(runDir))
	}
	if facts.Version <= 0 {
		facts.Version = 1
	}
	if strings.TrimSpace(facts.UpdatedAt) == "" {
		facts.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(EvolveFactsPath(runDir), facts)
}

func RefreshEvolveFacts(runDir string) error {
	facts, err := BuildEvolveFacts(runDir)
	if err != nil {
		return err
	}
	if facts == nil {
		if err := os.Remove(EvolveFactsPath(runDir)); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return SaveEvolveFacts(runDir, facts)
}
