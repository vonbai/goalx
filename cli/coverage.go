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
	return buildRequiredCoverage(goal, coord, sessionState, coverageSessionRoster(runDir, sessionState)), nil
}

func buildRequiredCoverage(goal *GoalState, coord *CoordinationState, sessionState *SessionsRuntimeState, sessionRoster map[string]struct{}) RequiredCoverage {
	coverage := RequiredCoverage{
		OpenRequiredIDs:        []string{},
		OwnedOpenIDs:           []string{},
		UnmappedOpenIDs:        []string{},
		OwnerSessionMissingIDs: []string{},
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

	for _, sessionName := range sortedCoverageSessionNames(sessionRoster) {
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
