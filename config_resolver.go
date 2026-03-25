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
	return resolveConfigWithDetector(layers, req, DetectPresetFromEnvironment)
}

func resolveConfigWithDetector(layers *ConfigLayers, req ResolveRequest, detect func() string) (*ResolvedConfig, error) {
	if layers == nil {
		return nil, fmt.Errorf("config layers are required")
	}

	cfg := layers.Config
	attachCatalogs(&cfg, copyPresetCatalog(layers.Presets), copyStringCatalog(layers.Dimensions))

	if req.ManualDraft != nil {
		mergeConfig(&cfg, req.ManualDraft)
		attachCatalogs(&cfg, copyPresetCatalog(layers.Presets), copyStringCatalog(layers.Dimensions))
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

	if cfg.Preset == "" {
		cfg.Preset = detect()
	}

	applyPreset(&cfg)
	applyResolveOverrides(&cfg, req)

	if cfg.Parallel < 1 {
		cfg.Parallel = 1
	}

	if err := ValidateConfig(&cfg, layers.Engines); err != nil {
		return nil, err
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
