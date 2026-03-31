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
	EvolveFrontierActive                        = "active"
	EvolveFrontierStopped                       = "stopped"
	EvolveManagementGapMissingStopOrDispatch    = "missing_stop_or_dispatch"
	EvolveManagementGapReviewWithoutManagedStop = "review_without_managed_stop"
)

type EvolveFacts struct {
	Version               int      `json:"version"`
	FrontierState         string   `json:"frontier_state,omitempty"`
	BestExperimentID      string   `json:"best_experiment_id,omitempty"`
	OpenCandidateIDs      []string `json:"open_candidate_ids,omitempty"`
	OpenCandidateCount    int      `json:"open_candidate_count,omitempty"`
	ActiveSessionCount    int      `json:"active_session_count,omitempty"`
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
	if sessionsState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir)); err != nil {
		return nil, err
	} else {
		facts.ActiveSessionCount = countRuntimeActiveSessions(sessionsState)
	}
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
			var body ExperimentIntegratedBody
			if err := json.Unmarshal(event.Body, &body); err != nil {
				return nil, err
			}
			if facts.BestExperimentID == "" {
				facts.BestExperimentID = strings.TrimSpace(body.ResultExperimentID)
			}
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
				if facts.BestExperimentID == "" {
					facts.BestExperimentID = strings.TrimSpace(body.BestExperimentID)
				}
			}
		}
	}

	openIDs := make([]string, 0, len(created))
	for id, createdAt := range created {
		if id != "" && id == facts.BestExperimentID {
			continue
		}
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
	status, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		return nil, err
	}
	facts.ManagementGap = detectEvolveManagementGap(facts, status)
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

func LoadCurrentEvolveFacts(runDir string) (*EvolveFacts, error) {
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	if meta == nil || strings.TrimSpace(meta.Intent) != runIntentEvolve {
		return nil, nil
	}
	facts, err := LoadEvolveFacts(EvolveFactsPath(runDir))
	if err != nil {
		return nil, err
	}
	if facts != nil {
		return facts, nil
	}
	return BuildEvolveFacts(runDir)
}

func detectEvolveManagementGap(facts *EvolveFacts, status *RunStatusRecord) string {
	if facts == nil || status == nil {
		return ""
	}
	if strings.TrimSpace(status.Phase) == runStatusPhaseReview && facts.ActiveSessionCount == 0 && facts.FrontierState != EvolveFrontierStopped {
		return EvolveManagementGapReviewWithoutManagedStop
	}
	if facts.FrontierState != EvolveFrontierActive || facts.ActiveSessionCount > 0 {
		return ""
	}
	statusUpdatedAt, statusOK := parseRFC3339Time(status.UpdatedAt)
	managementAt, managementOK := parseRFC3339Time(facts.LastManagementEventAt)
	switch {
	case statusOK && managementOK && statusUpdatedAt.After(managementAt):
		return EvolveManagementGapMissingStopOrDispatch
	case statusOK && !managementOK:
		return EvolveManagementGapMissingStopOrDispatch
	default:
		return ""
	}
}

func activeRunStatusSessionCount(names []string) int {
	count := 0
	for _, name := range names {
		if strings.TrimSpace(name) != "" {
			count++
		}
	}
	return count
}

func countRuntimeActiveSessions(state *SessionsRuntimeState) int {
	if state == nil || state.Sessions == nil {
		return 0
	}
	count := 0
	for _, session := range state.Sessions {
		switch strings.TrimSpace(session.State) {
		case "active", "progress", "working", "idle":
			count++
		}
	}
	return count
}

func parseRFC3339Time(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
