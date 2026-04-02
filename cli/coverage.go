package cli

import (
	"sort"
	"strings"
)

type RequiredCoverage struct {
	RequiredPresent             bool     `json:"required_present"`
	OpenRequiredIDs             []string `json:"open_required_ids,omitempty"`
	MappedRequiredIDs           []string `json:"mapped_required_ids,omitempty"`
	UnmappedRequiredIDs         []string `json:"unmapped_required_ids,omitempty"`
	SessionOwnerMissingIDs      []string `json:"session_owner_missing_ids,omitempty"`
	MasterOwnedRequiredIDs      []string `json:"master_owned_required_ids,omitempty"`
	MasterOrphanedRequiredIDs   []string `json:"master_orphaned_required_ids,omitempty"`
	ProbingRequiredIDs          []string `json:"probing_required_ids,omitempty"`
	WaitingRequiredIDs          []string `json:"waiting_required_ids,omitempty"`
	BlockedRequiredIDs          []string `json:"blocked_required_ids,omitempty"`
	PrematureBlockedRequiredIDs []string `json:"premature_blocked_required_ids,omitempty"`
	IdleReusableSessions        []string `json:"idle_reusable_sessions,omitempty"`
	ParkedReusableSessions      []string `json:"parked_reusable_sessions,omitempty"`
}

func BuildRequiredCoverage(runDir string) (RequiredCoverage, error) {
	goal, err := LoadCanonicalGoalState(runDir)
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
		OpenRequiredIDs:             []string{},
		MappedRequiredIDs:           []string{},
		UnmappedRequiredIDs:         []string{},
		SessionOwnerMissingIDs:      []string{},
		MasterOwnedRequiredIDs:      []string{},
		MasterOrphanedRequiredIDs:   []string{},
		ProbingRequiredIDs:          []string{},
		WaitingRequiredIDs:          []string{},
		BlockedRequiredIDs:          []string{},
		PrematureBlockedRequiredIDs: []string{},
		IdleReusableSessions:        []string{},
		ParkedReusableSessions:      []string{},
	}
	if goal != nil {
		normalizeGoalState(goal)
	}
	if coord != nil {
		normalizeCoordinationState(coord)
	}
	coverage.RequiredPresent = coord != nil && len(coord.Required) > 0
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
		if !coverage.RequiredPresent {
			continue
		}
		required := CoordinationRequiredItem{}
		found := false
		if coord != nil && coord.Required != nil {
			required, found = coord.Required[item.ID]
		}
		if !found {
			coverage.UnmappedRequiredIDs = append(coverage.UnmappedRequiredIDs, item.ID)
			continue
		}
		coverage.MappedRequiredIDs = append(coverage.MappedRequiredIDs, item.ID)
		owner := strings.TrimSpace(required.Owner)
		switch owner {
		case "master":
			coverage.MasterOwnedRequiredIDs = append(coverage.MasterOwnedRequiredIDs, item.ID)
			if required.ExecutionState == coordinationRequiredExecutionStateProbing || required.ExecutionState == coordinationRequiredExecutionStateWaiting {
				if !coverageHasActiveExecutionLane(sessionRoster, sessionState, coord) && (len(coverage.IdleReusableSessions) > 0 || len(coverage.ParkedReusableSessions) > 0) {
					coverage.MasterOrphanedRequiredIDs = append(coverage.MasterOrphanedRequiredIDs, item.ID)
				}
			}
		}
		switch required.ExecutionState {
		case coordinationRequiredExecutionStateProbing:
			coverage.ProbingRequiredIDs = append(coverage.ProbingRequiredIDs, item.ID)
		case coordinationRequiredExecutionStateWaiting:
			coverage.WaitingRequiredIDs = append(coverage.WaitingRequiredIDs, item.ID)
		case coordinationRequiredExecutionStateBlocked:
			if coordinationRequiredSurfacesExhausted(required.Surfaces) {
				coverage.BlockedRequiredIDs = append(coverage.BlockedRequiredIDs, item.ID)
			} else {
				coverage.PrematureBlockedRequiredIDs = append(coverage.PrematureBlockedRequiredIDs, item.ID)
			}
		}
		if isSessionOwnerToken(owner) {
			if _, ok := sessionRoster[owner]; !ok {
				coverage.SessionOwnerMissingIDs = append(coverage.SessionOwnerMissingIDs, item.ID)
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
	if goal == nil || coord == nil || coord.Required == nil {
		return owners
	}
	for _, item := range goal.Required {
		if normalizeGoalItemState(item.State) != goalItemStateOpen {
			continue
		}
		required, ok := coord.Required[item.ID]
		if !ok {
			continue
		}
		owner := strings.TrimSpace(required.Owner)
		if !isSessionOwnerToken(owner) {
			continue
		}
		owners[owner] = struct{}{}
	}
	return owners
}

func coverageHasActiveExecutionLane(sessionRoster map[string]struct{}, sessionState *SessionsRuntimeState, coord *CoordinationState) bool {
	for _, sessionName := range sortedCoverageSessionNames(sessionRoster) {
		switch coverageSessionLifecycleState(sessionName, sessionState, coord) {
		case "active", "progress", "working":
			return true
		}
	}
	return false
}
