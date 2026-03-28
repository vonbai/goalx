package goalx

import "fmt"

// ResolvedConfig is the fully resolved run config plus the local catalogs used to build it.
type ResolvedConfig struct {
	Config     Config
	Engines    map[string]EngineConfig
	Presets    map[string]PresetConfig
	Dimensions map[string]string
}

// ResolveConfig applies request overrides to loaded config layers and returns one resolved config.
func ResolveConfig(layers *ConfigLayers, req ResolveRequest) (*ResolvedConfig, error) {
	return resolveConfigWithOptions(layers, req, DetectPresetFromEnvironment, true)
}

// ResolveConfigPreview resolves config layers without enforcing launch-time validation.
// This is used for draft-generation flows that intentionally allow placeholders.
func ResolveConfigPreview(layers *ConfigLayers, req ResolveRequest) (*ResolvedConfig, error) {
	return resolveConfigWithOptions(layers, req, DetectPresetFromEnvironment, false)
}

func resolveConfigWithDetector(layers *ConfigLayers, req ResolveRequest, detect func() (string, error)) (*ResolvedConfig, error) {
	return resolveConfigWithOptions(layers, req, detect, true)
}

func resolveConfigWithOptions(layers *ConfigLayers, req ResolveRequest, detect func() (string, error), validate bool) (*ResolvedConfig, error) {
	if layers == nil {
		return nil, fmt.Errorf("config layers are required")
	}

	cfg := layers.Config
	attachCatalogs(&cfg, copyPresetCatalog(layers.Presets), copyStringCatalog(layers.Dimensions))

	if req.ManualDraft != nil {
		mergeConfig(&cfg, req.ManualDraft)
		attachCatalogs(&cfg, copyPresetCatalog(layers.Presets), copyStringCatalog(layers.Dimensions))
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
	if req.Preset != "" {
		cfg.Preset = req.Preset
	}
	if req.Parallel > 0 {
		cfg.Parallel = req.Parallel
	}
	if req.ClearSessions {
		cfg.Sessions = nil
	}
	applyResolveTargetOverride(&cfg, req.TargetOverride)
	applyResolveLocalValidationOverride(&cfg, req.LocalValidationOverride)

	if cfg.Preset == "" {
		preset, err := detect()
		if err != nil {
			if validate || req.RequireEngineAvailability {
				return nil, err
			}
		} else {
			cfg.Preset = preset
		}
	}
	if cfg.Preset != "" && !hasPresetSelection(&cfg, cfg.Preset) {
		return nil, fmt.Errorf("unknown preset %q", cfg.Preset)
	}

	applyPreset(&cfg)
	applyResolveOverrides(&cfg, req)

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
		Config:     cfg,
		Engines:    copyEngines(layers.Engines),
		Presets:    copyPresetCatalog(layers.Presets),
		Dimensions: copyStringCatalog(layers.Dimensions),
	}, nil
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
