package cli

import (
	"sort"
	"strings"
)

type RequiredCoverage struct {
	OwnersPresent          bool     `json:"owners_present"`
	OpenRequiredIDs        []string `json:"open_required_ids,omitempty"`
	OwnedOpenIDs           []string `json:"owned_open_ids,omitempty"`
	UnmappedOpenIDs        []string `json:"unmapped_open_ids,omitempty"`
	OwnerSessionMissingIDs []string `json:"owner_session_missing_ids,omitempty"`
	OwnerAttentionIDs      []string `json:"owner_attention_ids,omitempty"`
	OwnerBlockedIDs        []string `json:"owner_blocked_ids,omitempty"`
	OwnerRiskyIDs          []string `json:"owner_risky_ids,omitempty"`
	IdleReusableSessions   []string `json:"idle_reusable_sessions,omitempty"`
	ParkedReusableSessions []string `json:"parked_reusable_sessions,omitempty"`
}

func BuildRequiredCoverage(runDir string) (RequiredCoverage, error) {
	goal, err := LoadGoalState(GoalPath(runDir))
	if err != nil {
		return RequiredCoverage{}, err
	}
	coord, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		return RequiredCoverage{}, err
	}
	sessionState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		return RequiredCoverage{}, err
	}
	attention, err := loadTargetAttentionFacts(runDir)
	if err != nil {
		return RequiredCoverage{}, err
	}
	return buildRequiredCoverage(goal, coord, sessionState, coverageSessionRoster(runDir, sessionState), attention), nil
}

func buildRequiredCoverage(goal *GoalState, coord *CoordinationState, sessionState *SessionsRuntimeState, sessionRoster map[string]struct{}, attention map[string]TargetAttentionFacts) RequiredCoverage {
	coverage := RequiredCoverage{
		OpenRequiredIDs:        []string{},
		OwnedOpenIDs:           []string{},
		UnmappedOpenIDs:        []string{},
		OwnerSessionMissingIDs: []string{},
		OwnerAttentionIDs:      []string{},
		OwnerBlockedIDs:        []string{},
		OwnerRiskyIDs:          []string{},
		IdleReusableSessions:   []string{},
		ParkedReusableSessions: []string{},
	}
	if goal != nil {
		normalizeGoalState(goal)
	}
	if coord != nil {
		normalizeCoordinationState(coord)
	}
	coverage.OwnersPresent = coord != nil && len(coord.Owners) > 0
	openOwnerSessions := openRequiredOwnerSessions(goal, coord)

	for _, sessionName := range sortedCoverageSessionNames(sessionRoster) {
		if _, owned := openOwnerSessions[sessionName]; owned {
			continue
		}
		switch coverageSessionLifecycleState(sessionName, sessionState, coord) {
		case "idle":
			coverage.IdleReusableSessions = append(coverage.IdleReusableSessions, sessionName)
		case "parked":
			coverage.ParkedReusableSessions = append(coverage.ParkedReusableSessions, sessionName)
		}
	}

	if goal == nil {
		return coverage
	}
	for _, item := range goal.Required {
		if normalizeGoalItemState(item.State) != goalItemStateOpen {
			continue
		}
		coverage.OpenRequiredIDs = append(coverage.OpenRequiredIDs, item.ID)
		if !coverage.OwnersPresent {
			continue
		}
		owner := ""
		if coord != nil && coord.Owners != nil {
			owner = strings.TrimSpace(coord.Owners[item.ID])
		}
		if owner == "" {
			coverage.UnmappedOpenIDs = append(coverage.UnmappedOpenIDs, item.ID)
			continue
		}
		coverage.OwnedOpenIDs = append(coverage.OwnedOpenIDs, item.ID)
		if isSessionOwnerToken(owner) {
			if _, ok := sessionRoster[owner]; !ok {
				coverage.OwnerSessionMissingIDs = append(coverage.OwnerSessionMissingIDs, item.ID)
				coverage.OwnerRiskyIDs = append(coverage.OwnerRiskyIDs, item.ID)
				continue
			}
			switch attentionStateForOwner(attention, owner, sessionState, coord) {
			case TargetAttentionNeedsAttention, TargetAttentionActiveIdle:
				coverage.OwnerAttentionIDs = append(coverage.OwnerAttentionIDs, item.ID)
			case TargetAttentionTransportBlocked, TargetAttentionProgressBlocked:
				coverage.OwnerBlockedIDs = append(coverage.OwnerBlockedIDs, item.ID)
			case TargetAttentionOwnershipRisky:
				coverage.OwnerRiskyIDs = append(coverage.OwnerRiskyIDs, item.ID)
			}
		}
	}

	return coverage
}

func coverageSessionRoster(runDir string, sessionState *SessionsRuntimeState) map[string]struct{} {
	roster := map[string]struct{}{}
	for _, idx := range discoverSessionIndexesFromFS(runDir) {
		roster[SessionName(idx)] = struct{}{}
	}
	if sessionState != nil {
		for name := range sessionState.Sessions {
			if isSessionOwnerToken(name) {
				roster[name] = struct{}{}
			}
		}
	}
	return roster
}

func coverageSessionLifecycleState(sessionName string, sessionState *SessionsRuntimeState, coord *CoordinationState) string {
	if sessionState != nil {
		if sess, ok := sessionState.Sessions[sessionName]; ok {
			if state := strings.TrimSpace(sess.State); state != "" {
				return state
			}
		}
	}
	if coord != nil && coord.Sessions != nil {
		if state := strings.TrimSpace(coord.Sessions[sessionName].State); state != "" {
			return state
		}
	}
	return ""
}

func sortedCoverageSessionNames(roster map[string]struct{}) []string {
	if len(roster) == 0 {
		return nil
	}
	names := make([]string, 0, len(roster))
	for name := range roster {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isSessionOwnerToken(owner string) bool {
	_, err := parseSessionIndex(strings.TrimSpace(owner))
	return err == nil
}

func openRequiredOwnerSessions(goal *GoalState, coord *CoordinationState) map[string]struct{} {
	owners := map[string]struct{}{}
	if goal == nil || coord == nil || coord.Owners == nil {
		return owners
	}
	for _, item := range goal.Required {
		if normalizeGoalItemState(item.State) != goalItemStateOpen {
			continue
		}
		owner := strings.TrimSpace(coord.Owners[item.ID])
		if !isSessionOwnerToken(owner) {
			continue
		}
		owners[owner] = struct{}{}
	}
	return owners
}

func attentionStateForOwner(attention map[string]TargetAttentionFacts, owner string, sessionState *SessionsRuntimeState, coord *CoordinationState) string {
	if owner = strings.TrimSpace(owner); owner == "" {
		return ""
	}
	if attention != nil {
		if facts, ok := attention[owner]; ok && strings.TrimSpace(facts.AttentionState) != "" {
			return facts.AttentionState
		}
	}
	runtimeState := coverageSessionLifecycleState(owner, sessionState, coord)
	switch {
	case runtimeState == "parked" || runtimeState == "done" || runtimeState == "blocked" || runtimeState == "stopped" || runtimeState == "completed":
		return TargetAttentionOwnershipRisky
	case runtimeState == "idle":
		return TargetAttentionActiveIdle
	}
	return ""
}
