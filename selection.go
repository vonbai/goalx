package goalx

import (
	"fmt"
	"strings"
)

type EffectiveSelectionPolicy struct {
	DisabledEngines    []string
	DisabledTargets    []string
	MasterCandidates   []string
	ResearchCandidates []string
	DevelopCandidates  []string
	MasterEffort       EffortLevel
	ResearchEffort     EffortLevel
	DevelopEffort      EffortLevel
}

type SelectionTarget struct {
	Engine string
	Model  string
}

func hasSelectionConfig(cfg SelectionConfig) bool {
	return len(cfg.DisabledEngines) > 0 ||
		len(cfg.DisabledTargets) > 0 ||
		len(cfg.MasterCandidates) > 0 ||
		len(cfg.ResearchCandidates) > 0 ||
		len(cfg.DevelopCandidates) > 0 ||
		cfg.MasterEffort != "" ||
		cfg.ResearchEffort != "" ||
		cfg.DevelopEffort != ""
}

func copySelectionConfig(src SelectionConfig) SelectionConfig {
	return SelectionConfig{
		DisabledEngines:    append([]string(nil), src.DisabledEngines...),
		DisabledTargets:    append([]string(nil), src.DisabledTargets...),
		MasterCandidates:   append([]string(nil), src.MasterCandidates...),
		ResearchCandidates: append([]string(nil), src.ResearchCandidates...),
		DevelopCandidates:  append([]string(nil), src.DevelopCandidates...),
		MasterEffort:       src.MasterEffort,
		ResearchEffort:     src.ResearchEffort,
		DevelopEffort:      src.DevelopEffort,
	}
}

func normalizeSelectionConfig(cfg SelectionConfig, engines map[string]EngineConfig) (SelectionConfig, error) {
	var err error
	out := copySelectionConfig(cfg)
	if out.DisabledEngines, err = normalizeSelectionEngines(out.DisabledEngines, "selection.disabled_engines", engines); err != nil {
		return SelectionConfig{}, err
	}
	if out.DisabledTargets, err = normalizeSelectionTargets(out.DisabledTargets, "selection.disabled_targets", engines); err != nil {
		return SelectionConfig{}, err
	}
	if out.MasterCandidates, err = normalizeSelectionTargets(out.MasterCandidates, "selection.master_candidates", engines); err != nil {
		return SelectionConfig{}, err
	}
	if out.ResearchCandidates, err = normalizeSelectionTargets(out.ResearchCandidates, "selection.research_candidates", engines); err != nil {
		return SelectionConfig{}, err
	}
	if out.DevelopCandidates, err = normalizeSelectionTargets(out.DevelopCandidates, "selection.develop_candidates", engines); err != nil {
		return SelectionConfig{}, err
	}
	if err := validateEffortLevel(out.MasterEffort, "selection.master_effort"); err != nil {
		return SelectionConfig{}, err
	}
	if err := validateEffortLevel(out.ResearchEffort, "selection.research_effort"); err != nil {
		return SelectionConfig{}, err
	}
	if err := validateEffortLevel(out.DevelopEffort, "selection.develop_effort"); err != nil {
		return SelectionConfig{}, err
	}
	return out, nil
}

func normalizeSelectionEngines(values []string, field string, engines map[string]EngineConfig) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(values))
	for _, raw := range values {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if _, ok := engines[name]; !ok {
			return nil, fmt.Errorf("%s contains unknown engine %q", field, name)
		}
		normalized = append(normalized, name)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	if err := validateUniqueNames(normalized, field); err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeSelectionTargets(values []string, field string, engines map[string]EngineConfig) ([]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(values))
	for i, raw := range values {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		target, err := parseSelectionTarget(raw, fmt.Sprintf("%s[%d]", field, i))
		if err != nil {
			return nil, err
		}
		formatted := formatSelectionTarget(target)
		if err := validateLaunchRequest(engines, LaunchRequest{Engine: target.Engine, Model: target.Model}, fmt.Sprintf("%s[%d]", field, i)); err != nil {
			return nil, err
		}
		normalized = append(normalized, formatted)
	}
	if len(normalized) == 0 {
		return nil, nil
	}
	if err := validateUniqueNames(normalized, field); err != nil {
		return nil, err
	}
	return normalized, nil
}

func parseSelectionTarget(raw string, field string) (SelectionTarget, error) {
	parts := strings.Split(raw, "/")
	if len(parts) != 2 {
		return SelectionTarget{}, fmt.Errorf("%s must be ENGINE/MODEL, got %q", field, raw)
	}
	engine := strings.TrimSpace(parts[0])
	model := strings.TrimSpace(parts[1])
	if engine == "" || model == "" {
		return SelectionTarget{}, fmt.Errorf("%s must be ENGINE/MODEL, got %q", field, raw)
	}
	return SelectionTarget{Engine: engine, Model: model}, nil
}

func formatSelectionTarget(target SelectionTarget) string {
	return strings.TrimSpace(target.Engine) + "/" + strings.TrimSpace(target.Model)
}

func DetectAvailableEngines(engines map[string]EngineConfig) map[string]bool {
	if len(engines) == 0 {
		engines = BuiltinEngines
	}
	available := make(map[string]bool, len(engines))
	for name, engine := range engines {
		binary := launchBinaryName(strings.ReplaceAll(engine.Command, "{model_id}", "model"))
		if binary == "" {
			continue
		}
		if commandExists(binary) {
			available[name] = true
		}
	}
	return available
}

func resolveEffectiveSelectionPolicy(cfg *Config, engines map[string]EngineConfig) (EffectiveSelectionPolicy, bool, error) {
	if cfg == nil {
		return EffectiveSelectionPolicy{}, false, fmt.Errorf("config is nil")
	}
	if hasSelectionConfig(cfg.Selection) {
		policy, err := compileExplicitSelectionPolicy(cfg.Selection, engines, DetectAvailableEngines(engines))
		return policy, true, err
	}
	return compileConfigSelectionPolicy(cfg), false, nil
}

func DeriveSelectionPolicy(cfg *Config) EffectiveSelectionPolicy {
	return compileConfigSelectionPolicy(cfg)
}

func compileExplicitSelectionPolicy(selection SelectionConfig, engines map[string]EngineConfig, availability map[string]bool) (EffectiveSelectionPolicy, error) {
	defaults, defaultsErr := builtinSelectionDefaults(availability)
	policy := EffectiveSelectionPolicy{
		DisabledEngines: append([]string(nil), selection.DisabledEngines...),
		DisabledTargets: append([]string(nil), selection.DisabledTargets...),
		MasterEffort:    selection.MasterEffort,
		ResearchEffort:  selection.ResearchEffort,
		DevelopEffort:   selection.DevelopEffort,
	}

	var err error
	if policy.MasterCandidates, err = resolveSelectionCandidates(selection.MasterCandidates, defaults.MasterCandidates, defaultsErr); err != nil {
		return EffectiveSelectionPolicy{}, err
	}
	if policy.ResearchCandidates, err = resolveSelectionCandidates(selection.ResearchCandidates, defaults.ResearchCandidates, defaultsErr); err != nil {
		return EffectiveSelectionPolicy{}, err
	}
	if policy.DevelopCandidates, err = resolveSelectionCandidates(selection.DevelopCandidates, defaults.DevelopCandidates, defaultsErr); err != nil {
		return EffectiveSelectionPolicy{}, err
	}
	if policy.MasterEffort == "" && defaultsErr == nil {
		policy.MasterEffort = defaults.MasterEffort
	}
	if policy.ResearchEffort == "" && defaultsErr == nil {
		policy.ResearchEffort = defaults.ResearchEffort
	}
	if policy.DevelopEffort == "" && defaultsErr == nil {
		policy.DevelopEffort = defaults.DevelopEffort
	}

	disabledEngines := make(map[string]bool, len(policy.DisabledEngines))
	for _, engine := range policy.DisabledEngines {
		disabledEngines[engine] = true
	}
	disabledTargets := make(map[string]bool, len(policy.DisabledTargets))
	for _, target := range policy.DisabledTargets {
		disabledTargets[target] = true
	}

	policy.MasterCandidates = filterUsableSelectionCandidates(policy.MasterCandidates, disabledEngines, disabledTargets, availability)
	policy.ResearchCandidates = filterUsableSelectionCandidates(policy.ResearchCandidates, disabledEngines, disabledTargets, availability)
	policy.DevelopCandidates = filterUsableSelectionCandidates(policy.DevelopCandidates, disabledEngines, disabledTargets, availability)

	if len(policy.MasterCandidates) == 0 {
		return EffectiveSelectionPolicy{}, fmt.Errorf("selection.master_candidates has no usable candidates after availability and disabled-target filtering")
	}
	if len(policy.ResearchCandidates) == 0 {
		return EffectiveSelectionPolicy{}, fmt.Errorf("selection.research_candidates has no usable candidates after availability and disabled-target filtering")
	}
	if len(policy.DevelopCandidates) == 0 {
		return EffectiveSelectionPolicy{}, fmt.Errorf("selection.develop_candidates has no usable candidates after availability and disabled-target filtering")
	}
	return policy, nil
}

func resolveSelectionCandidates(explicit []string, defaults []string, defaultsErr error) ([]string, error) {
	if len(explicit) > 0 {
		return append([]string(nil), explicit...), nil
	}
	if defaultsErr != nil {
		return nil, defaultsErr
	}
	return append([]string(nil), defaults...), nil
}

func builtinSelectionDefaults(availability map[string]bool) (SelectionConfig, error) {
	hasCodex := availability["codex"]
	hasClaude := availability["claude-code"]
	switch {
	case hasCodex && hasClaude:
		return SelectionConfig{
			MasterCandidates:   []string{"codex/gpt-5.4", "claude-code/opus"},
			ResearchCandidates: []string{"claude-code/opus", "codex/gpt-5.4"},
			DevelopCandidates:  []string{"codex/gpt-5.4", "codex/gpt-5.4-mini"},
			MasterEffort:       EffortHigh,
			ResearchEffort:     EffortHigh,
			DevelopEffort:      EffortMedium,
		}, nil
	case hasCodex:
		return SelectionConfig{
			MasterCandidates:   []string{"codex/gpt-5.4"},
			ResearchCandidates: []string{"codex/gpt-5.4"},
			DevelopCandidates:  []string{"codex/gpt-5.4", "codex/gpt-5.4-mini"},
			MasterEffort:       EffortHigh,
			ResearchEffort:     EffortHigh,
			DevelopEffort:      EffortMedium,
		}, nil
	case hasClaude:
		return SelectionConfig{
			MasterCandidates:   []string{"claude-code/opus"},
			ResearchCandidates: []string{"claude-code/opus"},
			DevelopCandidates:  []string{"claude-code/opus"},
			MasterEffort:       EffortHigh,
			ResearchEffort:     EffortHigh,
			DevelopEffort:      EffortMedium,
		}, nil
	default:
		return SelectionConfig{}, fmt.Errorf("no supported engines found in PATH; install claude or codex")
	}
}

func filterUsableSelectionCandidates(candidates []string, disabledEngines, disabledTargets, availability map[string]bool) []string {
	if len(candidates) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		target, err := parseSelectionTarget(candidate, "candidate")
		if err != nil {
			continue
		}
		if disabledEngines[target.Engine] || disabledTargets[candidate] {
			continue
		}
		if !availability[target.Engine] {
			continue
		}
		filtered = append(filtered, candidate)
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func compileConfigSelectionPolicy(cfg *Config) EffectiveSelectionPolicy {
	if cfg == nil {
		return EffectiveSelectionPolicy{}
	}
	policy := EffectiveSelectionPolicy{
		MasterEffort:   cfg.Master.Effort,
		ResearchEffort: cfg.Roles.Research.Effort,
		DevelopEffort:  cfg.Roles.Develop.Effort,
	}
	appendUniqueSelectionTarget(&policy.MasterCandidates, cfg.Master.Engine, cfg.Master.Model)
	appendUniqueSelectionTarget(&policy.ResearchCandidates, cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	appendUniqueSelectionTarget(&policy.DevelopCandidates, cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	return policy
}

func hasConfiguredSelectionTargets(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	return (strings.TrimSpace(cfg.Master.Engine) != "" && strings.TrimSpace(cfg.Master.Model) != "") ||
		(strings.TrimSpace(cfg.Roles.Research.Engine) != "" && strings.TrimSpace(cfg.Roles.Research.Model) != "") ||
		(strings.TrimSpace(cfg.Roles.Develop.Engine) != "" && strings.TrimSpace(cfg.Roles.Develop.Model) != "")
}

func appendUniqueSelectionTarget(targets *[]string, engine, model string) {
	if targets == nil {
		return
	}
	engine = strings.TrimSpace(engine)
	model = strings.TrimSpace(model)
	if engine == "" || model == "" {
		return
	}
	candidate := engine + "/" + model
	for _, existing := range *targets {
		if existing == candidate {
			return
		}
	}
	*targets = append(*targets, candidate)
}

func applyEffectiveSelectionPolicy(cfg *Config, policy EffectiveSelectionPolicy) {
	if cfg == nil {
		return
	}
	if target, ok := firstSelectionTarget(policy.MasterCandidates); ok {
		cfg.Master.Engine = target.Engine
		cfg.Master.Model = target.Model
	}
	if policy.MasterEffort != "" {
		cfg.Master.Effort = policy.MasterEffort
	}
	if target, ok := firstSelectionTarget(policy.ResearchCandidates); ok {
		cfg.Roles.Research.Engine = target.Engine
		cfg.Roles.Research.Model = target.Model
	}
	if policy.ResearchEffort != "" {
		cfg.Roles.Research.Effort = policy.ResearchEffort
	}
	if target, ok := firstSelectionTarget(policy.DevelopCandidates); ok {
		cfg.Roles.Develop.Engine = target.Engine
		cfg.Roles.Develop.Model = target.Model
	}
	if policy.DevelopEffort != "" {
		cfg.Roles.Develop.Effort = policy.DevelopEffort
	}
}

func firstSelectionTarget(candidates []string) (SelectionTarget, bool) {
	if len(candidates) == 0 {
		return SelectionTarget{}, false
	}
	target, err := parseSelectionTarget(candidates[0], "candidate")
	if err != nil {
		return SelectionTarget{}, false
	}
	return target, true
}
