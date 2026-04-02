package cli

import "strings"

func EvaluateFreshnessState(runDir string) (*FreshnessState, error) {
	state := &FreshnessState{
		Version:   1,
		Cognition: []CognitionFreshnessItem{},
		Evidence:  []EvidenceFreshnessItem{},
	}
	cognition, err := LoadCognitionState(CognitionStatePath(runDir))
	if err != nil {
		return nil, err
	}
	if cognition != nil {
		for _, scope := range cognition.Scopes {
			for _, provider := range scope.Providers {
				item := CognitionFreshnessItem{
					Scope:    scope.Scope,
					Provider: provider.Name,
					State:    freshnessStateFresh,
				}
				switch {
				case !provider.Available:
					item.State = freshnessStateUnknown
					item.Reason = "provider_unavailable"
				case provider.Name == "gitnexus" && provider.IndexProvenance == "seeded" && strings.TrimSpace(provider.AnalyzedInScopeAt) == "":
					item.State = freshnessStateUnknown
					item.Reason = "seeded_cache_unverified"
				case provider.StaleCommits > 0:
					item.State = freshnessStateStale
					item.Reason = "stale_commits"
				case provider.IndexedRevision != "" && provider.HeadRevision != "" && provider.IndexedRevision != provider.HeadRevision:
					item.State = freshnessStateStale
					item.Reason = "indexed_revision_mismatch"
				}
				state.Cognition = append(state.Cognition, item)
			}
		}
	}

	plan, err := LoadAssurancePlan(AssurancePlanPath(runDir))
	if err != nil {
		return nil, err
	}
	impact, err := LoadImpactState(ImpactStatePath(runDir))
	if err != nil {
		return nil, err
	}
	events, err := LoadEvidenceLog(EvidenceLogPath(runDir))
	if err != nil {
		return nil, err
	}
	if plan != nil {
		for _, scenario := range plan.Scenarios {
			item := EvidenceFreshnessItem{
				ScenarioID: scenario.ID,
				State:      freshnessStateUnknown,
				Reason:     "no_evidence",
			}
			body := latestEvidenceForScenario(events, scenario.ID)
			if body.ScenarioID == "" {
				state.Evidence = append(state.Evidence, item)
				continue
			}
			item.LatestRevision = body.Revision
			if impact != nil {
				item.CurrentRevision = impact.HeadRevision
			}
			switch {
			case impact == nil || strings.TrimSpace(impact.HeadRevision) == "":
				item.State = freshnessStateUnknown
				item.Reason = "current_revision_unknown"
			case body.Revision == "" || body.Revision == impact.HeadRevision:
				item.State = freshnessStateFresh
				item.Reason = ""
			case scenarioTouchesImpact(scenario, impact):
				item.State = freshnessStateStale
				item.Reason = "touchpoint_overlap"
			default:
				item.State = freshnessStateFresh
				item.Reason = ""
			}
			state.Evidence = append(state.Evidence, item)
		}
	}
	return state, nil
}

func RefreshFreshnessState(runDir string) error {
	state, err := EvaluateFreshnessState(runDir)
	if err != nil {
		return err
	}
	return SaveFreshnessState(FreshnessStatePath(runDir), state)
}

func latestEvidenceForScenario(events []EvidenceLogEvent, scenarioID string) EvidenceEventBody {
	for i := len(events) - 1; i >= 0; i-- {
		if strings.TrimSpace(events[i].Kind) != "scenario.executed" {
			continue
		}
		body, err := parseEvidenceEventBody(events[i].Body)
		if err != nil {
			continue
		}
		if body.ScenarioID == scenarioID {
			return body
		}
	}
	return EvidenceEventBody{}
}

func scenarioTouchesImpact(scenario AssuranceScenario, impact *ImpactState) bool {
	if impact == nil {
		return false
	}
	for _, file := range scenario.Touchpoints.Files {
		for _, changed := range impact.ChangedFiles {
			if strings.TrimSpace(file) == strings.TrimSpace(changed) {
				return true
			}
		}
	}
	for _, process := range scenario.Touchpoints.Processes {
		for _, changed := range impact.ChangedProcesses {
			if strings.TrimSpace(process) == strings.TrimSpace(changed) {
				return true
			}
		}
	}
	for _, symbol := range scenario.Touchpoints.Symbols {
		for _, changed := range impact.ChangedSymbols {
			if strings.TrimSpace(symbol) == strings.TrimSpace(changed) {
				return true
			}
		}
	}
	return false
}
