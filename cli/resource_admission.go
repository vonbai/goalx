package cli

import (
	"errors"
	"fmt"
	"strings"
)

var errResourceAdmissionBlocked = errors.New("resource admission blocked")

type ResourceAdmissionDecision struct {
	Allowed                bool
	State                  string
	HeadroomBytes          int64
	ExpectedRSSBytes       int64
	ProjectedHeadroomBytes int64
	Reasons                []string
}

func evaluateResourceAdmission(runDir, engine, model string) (*ResourceAdmissionDecision, error) {
	if err := RefreshResourceState(runDir); err != nil {
		return nil, fmt.Errorf("refresh resource state: %w", err)
	}
	state, err := LoadResourceState(ResourceStatePath(runDir))
	if err != nil {
		return nil, fmt.Errorf("load resource state: %w", err)
	}
	if state == nil {
		return nil, fmt.Errorf("resource state missing")
	}
	profile, ok := lookupEngineMemoryProfile(engine, model)
	if !ok {
		return nil, fmt.Errorf("resource admission has no built-in memory profile for %s/%s", strings.TrimSpace(engine), strings.TrimSpace(model))
	}

	projectedHeadroom := state.HeadroomBytes - profile.ExpectedRSSBytes
	reasons := append([]string(nil), state.Reasons...)
	decision := &ResourceAdmissionDecision{
		Allowed:                true,
		State:                  strings.TrimSpace(state.State),
		HeadroomBytes:          state.HeadroomBytes,
		ExpectedRSSBytes:       profile.ExpectedRSSBytes,
		ProjectedHeadroomBytes: projectedHeadroom,
		Reasons:                compactStrings(reasons),
	}

	switch {
	case decision.State == resourceStateUnknown:
		decision.Allowed = false
		decision.Reasons = compactStrings(append(decision.Reasons, "resource_state_unknown"))
	case decision.State == resourceStateCritical:
		decision.Allowed = false
		decision.Reasons = compactStrings(append(decision.Reasons, "resource_state_critical"))
	case projectedHeadroom <= resourceCriticalReserveBytes:
		decision.Allowed = false
		decision.Reasons = compactStrings(append(decision.Reasons, "projected_headroom_below_critical_reserve"))
	case decision.State == resourceStateTight && projectedHeadroom <= resourceHostReserveBytes:
		decision.Allowed = false
		decision.Reasons = compactStrings(append(decision.Reasons, "projected_headroom_below_reserve"))
	}
	return decision, nil
}

func requireResourceAdmission(runDir, engine, model, action string) error {
	decision, err := evaluateResourceAdmission(runDir, engine, model)
	if err != nil {
		return err
	}
	if decision.Allowed {
		return nil
	}
	return fmt.Errorf(
		"%w %s: engine=%s model=%s state=%s headroom_bytes=%d expected_rss_bytes=%d projected_headroom_bytes=%d reasons=%s",
		errResourceAdmissionBlocked,
		strings.TrimSpace(action),
		strings.TrimSpace(engine),
		strings.TrimSpace(model),
		blankAsUnknown(decision.State),
		decision.HeadroomBytes,
		decision.ExpectedRSSBytes,
		decision.ProjectedHeadroomBytes,
		strings.Join(decision.Reasons, ","),
	)
}
