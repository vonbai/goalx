package goalx

import "strings"

func ResolveRouteProfile(cfg *Config, routeRole string, dimensions []ResolvedDimension, effort EffortLevel) (string, ExecutionProfile, bool) {
	if cfg == nil {
		return "", ExecutionProfile{}, false
	}
	dimensionSet := make(map[string]bool, len(dimensions))
	for _, dimension := range dimensions {
		name := strings.TrimSpace(dimension.Name)
		if name == "" {
			continue
		}
		dimensionSet[name] = true
	}

	role := strings.TrimSpace(routeRole)
	for _, rule := range cfg.Routing.Rules {
		if role != "" && rule.Role != role {
			continue
		}
		if role == "" && strings.TrimSpace(rule.Role) != "" {
			continue
		}
		if !ruleMatches(rule, dimensionSet, effort) {
			continue
		}
		profile, ok := cfg.Routing.Profiles[rule.Profile]
		if !ok {
			continue
		}
		return rule.Profile, profile, true
	}
	return "", ExecutionProfile{}, false
}

func ResolveSessionRoute(cfg *Config, session SessionConfig) (SessionConfig, error) {
	if cfg == nil {
		return session, nil
	}

	resolved := session
	roleDefault := explicitRoleSession(cfg, resolved.Mode, resolved.RouteRole)
	resolvedDimensions, err := resolveDimensionSpecsWithCatalog(resolveDimensionCatalog(cfg), resolved.Dimensions)
	if err != nil {
		return SessionConfig{}, err
	}

	if resolved.Engine != "" && resolved.Model != "" {
		if resolved.Effort == "" {
			resolved.Effort = roleDefault.Effort
		}
		return resolved, nil
	}

	if strings.TrimSpace(resolved.RouteProfile) != "" {
		profile := cfg.Routing.Profiles[resolved.RouteProfile]
		applyProfile(&resolved, profile)
		return fillRoleDefaults(resolved, roleDefault), nil
	}

	requestedEffort := resolved.Effort
	if requestedEffort == "" {
		requestedEffort = roleDefault.Effort
	}
	if profileName, profile, ok := ResolveRouteProfile(cfg, resolved.RouteRole, resolvedDimensions, requestedEffort); ok {
		resolved.RouteProfile = profileName
		applyProfile(&resolved, profile)
		return fillRoleDefaults(resolved, roleDefault), nil
	}

	return fillRoleDefaults(resolved, roleDefault), nil
}

func ruleMatches(rule RoutingRule, dimensions map[string]bool, effort EffortLevel) bool {
	if len(rule.Efforts) > 0 {
		matched := false
		for _, candidate := range rule.Efforts {
			if candidate == effort {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, name := range rule.AllDimensions {
		if !dimensions[name] {
			return false
		}
	}
	if len(rule.AnyDimensions) > 0 {
		matched := false
		for _, name := range rule.AnyDimensions {
			if dimensions[name] {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func applyProfile(session *SessionConfig, profile ExecutionProfile) {
	if session == nil {
		return
	}
	if profile.Engine != "" {
		session.Engine = profile.Engine
	}
	if profile.Model != "" {
		session.Model = profile.Model
	}
	if profile.Effort != "" {
		session.Effort = profile.Effort
	}
}

func fillRoleDefaults(session SessionConfig, roleDefault SessionConfig) SessionConfig {
	if session.Engine == "" {
		session.Engine = roleDefault.Engine
	}
	if session.Model == "" {
		session.Model = roleDefault.Model
	}
	if session.Effort == "" {
		session.Effort = roleDefault.Effort
	}
	return session
}
func resolveDimensionCatalog(cfg *Config) map[string]string {
	if cfg == nil || len(cfg.dimensionCatalog) == 0 {
		return BuiltinDimensions
	}
	return cfg.dimensionCatalog
}
