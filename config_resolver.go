package goalx

import (
	"fmt"
)

// ResolvedConfig is the fully resolved run config plus the local catalogs used to build it.
type ResolvedConfig struct {
	Config            Config
	Engines           map[string]EngineConfig
	Dimensions        map[string]string
	SelectionPolicy   EffectiveSelectionPolicy
	ExplicitSelection bool
}

// ResolveConfig applies request overrides to loaded config layers and returns one resolved config.
func ResolveConfig(layers *ConfigLayers, req ResolveRequest) (*ResolvedConfig, error) {
	return resolveConfigWithOptions(layers, req, true)
}

// ResolveConfigPreview resolves config layers without enforcing launch-time validation.
// This is used for draft-generation flows that intentionally allow placeholders.
func ResolveConfigPreview(layers *ConfigLayers, req ResolveRequest) (*ResolvedConfig, error) {
	return resolveConfigWithOptions(layers, req, false)
}

func resolveConfigWithOptions(layers *ConfigLayers, req ResolveRequest, validate bool) (*ResolvedConfig, error) {
	if layers == nil {
		return nil, fmt.Errorf("config layers are required")
	}

	cfg := layers.Config
	attachDimensionCatalog(&cfg, copyStringCatalog(layers.Dimensions))

	if req.ManualDraft != nil {
		if hasSelectionConfig(req.ManualDraft.Selection) {
			return nil, fmt.Errorf("manual draft config: selection is only supported in ~/.goalx/config.yaml")
		}
		mergeConfig(&cfg, req.ManualDraft)
		attachDimensionCatalog(&cfg, copyStringCatalog(layers.Dimensions))
	}

	if req.Name != "" {
		cfg.Name = req.Name
	}
	if req.Mode != "" {
		cfg.Mode = req.Mode
	}
	if req.Objective != "" {
		cfg.Objective = req.Objective
	}
	if req.Parallel > 0 {
		cfg.Parallel = req.Parallel
	}
	if req.ClearSessions {
		cfg.Sessions = nil
	}
	applyResolveTargetOverride(&cfg, req.TargetOverride)
	applyResolveLocalValidationOverride(&cfg, req.LocalValidationOverride)

	explicitSelection := hasSelectionConfig(cfg.Selection)
	if explicitSelection {
		normalized, err := normalizeSelectionConfig(cfg.Selection, layers.Engines)
		if err != nil {
			return nil, err
		}
		cfg.Selection = normalized
	}

	policy := EffectiveSelectionPolicy{}
	implicitSelection := false
	if explicitSelection {
		var err error
		policy, _, err = resolveEffectiveSelectionPolicy(&cfg, layers.Engines)
		if err != nil {
			return nil, err
		}
		applyEffectiveSelectionPolicy(&cfg, policy)
	} else {
		var err error
		policy, implicitSelection, err = resolveImplicitSelectionPolicy(layers.Engines, validate || req.RequireEngineAvailability)
		if err != nil {
			if !hasConfiguredSelectionTargets(&cfg) {
				return nil, err
			}
			policy = compileConfigSelectionPolicy(&cfg)
			implicitSelection = false
		}
		if implicitSelection {
			fillMissingSelectionDefaults(&cfg, policy)
		}
	}

	applyResolveOverrides(&cfg, req)

	if !explicitSelection {
		policy = overlaySelectionPolicyDefaults(policy, &cfg)
	}

	if cfg.Parallel < 1 {
		cfg.Parallel = 1
	}

	if validate {
		if err := ValidateConfig(&cfg, layers.Engines); err != nil {
			return nil, err
		}
	}
	if req.RequireEngineAvailability {
		if err := validateLaunchAvailability(&cfg, layers.Engines); err != nil {
			return nil, err
		}
	}

	return &ResolvedConfig{
		Config:            cfg,
		Engines:           copyEngines(layers.Engines),
		Dimensions:        copyStringCatalog(layers.Dimensions),
		SelectionPolicy:   policy,
		ExplicitSelection: explicitSelection,
	}, nil
}

func resolveImplicitSelectionPolicy(engines map[string]EngineConfig, strict bool) (EffectiveSelectionPolicy, bool, error) {
	policy, err := compileExplicitSelectionPolicy(SelectionConfig{}, engines, DetectAvailableEngines(engines))
	if err != nil {
		if strict {
			return EffectiveSelectionPolicy{}, false, err
		}
		return EffectiveSelectionPolicy{}, false, nil
	}
	return policy, true, nil
}

func fillMissingSelectionDefaults(cfg *Config, policy EffectiveSelectionPolicy) {
	if cfg == nil {
		return
	}
	if cfg.Master.Engine == "" || cfg.Master.Model == "" {
		if target, ok := firstSelectionTarget(policy.MasterCandidates); ok {
			if cfg.Master.Engine == "" {
				cfg.Master.Engine = target.Engine
			}
			if cfg.Master.Model == "" {
				cfg.Master.Model = target.Model
			}
		}
	}
	if cfg.Master.Effort == "" && policy.MasterEffort != "" {
		cfg.Master.Effort = policy.MasterEffort
	}
	if cfg.Roles.Research.Engine == "" || cfg.Roles.Research.Model == "" {
		if target, ok := firstSelectionTarget(policy.ResearchCandidates); ok {
			if cfg.Roles.Research.Engine == "" {
				cfg.Roles.Research.Engine = target.Engine
			}
			if cfg.Roles.Research.Model == "" {
				cfg.Roles.Research.Model = target.Model
			}
		}
	}
	if cfg.Roles.Research.Effort == "" && policy.ResearchEffort != "" {
		cfg.Roles.Research.Effort = policy.ResearchEffort
	}
	if cfg.Roles.Develop.Engine == "" || cfg.Roles.Develop.Model == "" {
		if target, ok := firstSelectionTarget(policy.DevelopCandidates); ok {
			if cfg.Roles.Develop.Engine == "" {
				cfg.Roles.Develop.Engine = target.Engine
			}
			if cfg.Roles.Develop.Model == "" {
				cfg.Roles.Develop.Model = target.Model
			}
		}
	}
	if cfg.Roles.Develop.Effort == "" && policy.DevelopEffort != "" {
		cfg.Roles.Develop.Effort = policy.DevelopEffort
	}
}

func overlaySelectionPolicyDefaults(policy EffectiveSelectionPolicy, cfg *Config) EffectiveSelectionPolicy {
	if cfg == nil {
		return policy
	}
	policy.MasterCandidates = promoteSelectionTarget(policy.MasterCandidates, cfg.Master.Engine, cfg.Master.Model)
	policy.ResearchCandidates = promoteSelectionTarget(policy.ResearchCandidates, cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	policy.DevelopCandidates = promoteSelectionTarget(policy.DevelopCandidates, cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	if cfg.Master.Effort != "" {
		policy.MasterEffort = cfg.Master.Effort
	}
	if cfg.Roles.Research.Effort != "" {
		policy.ResearchEffort = cfg.Roles.Research.Effort
	}
	if cfg.Roles.Develop.Effort != "" {
		policy.DevelopEffort = cfg.Roles.Develop.Effort
	}
	return policy
}

func promoteSelectionTarget(candidates []string, engine, model string) []string {
	if engine == "" || model == "" {
		return append([]string(nil), candidates...)
	}
	target := engine + "/" + model
	out := []string{target}
	for _, candidate := range candidates {
		if candidate == target {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func applyResolveOverrides(cfg *Config, req ResolveRequest) {
	if cfg == nil {
		return
	}
	if req.MasterOverride != nil {
		if req.MasterOverride.Engine != "" {
			cfg.Master.Engine = req.MasterOverride.Engine
		}
		if req.MasterOverride.Model != "" {
			cfg.Master.Model = req.MasterOverride.Model
		}
		if req.MasterOverride.Effort != "" {
			cfg.Master.Effort = req.MasterOverride.Effort
		}
	}
	if req.ResearchOverride != nil {
		if req.ResearchOverride.Engine != "" {
			cfg.Roles.Research.Engine = req.ResearchOverride.Engine
		}
		if req.ResearchOverride.Model != "" {
			cfg.Roles.Research.Model = req.ResearchOverride.Model
		}
		if req.ResearchOverride.Effort != "" {
			cfg.Roles.Research.Effort = req.ResearchOverride.Effort
		}
	}
	if req.DevelopOverride != nil {
		if req.DevelopOverride.Engine != "" {
			cfg.Roles.Develop.Engine = req.DevelopOverride.Engine
		}
		if req.DevelopOverride.Model != "" {
			cfg.Roles.Develop.Model = req.DevelopOverride.Model
		}
		if req.DevelopOverride.Effort != "" {
			cfg.Roles.Develop.Effort = req.DevelopOverride.Effort
		}
	}
}

func applyResolveTargetOverride(cfg *Config, override *TargetConfig) {
	if cfg == nil || override == nil {
		return
	}
	if len(override.Files) > 0 {
		cfg.Target.Files = append([]string(nil), override.Files...)
	}
	if len(override.Readonly) > 0 {
		cfg.Target.Readonly = append([]string(nil), override.Readonly...)
	}
}

func applyResolveLocalValidationOverride(cfg *Config, override *LocalValidationConfig) {
	if cfg == nil || override == nil {
		return
	}
	if override.Command != "" {
		cfg.LocalValidation.Command = override.Command
	}
	if override.Timeout > 0 {
		cfg.LocalValidation.Timeout = override.Timeout
	}
}
