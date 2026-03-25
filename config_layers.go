package goalx

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigLayers contains the merged config plus local engine/preset/dimension catalogs.
// The catalogs are load-scoped and must not mutate package-level built-ins.
type ConfigLayers struct {
	Config     Config
	Engines    map[string]EngineConfig
	Presets    map[string]PresetConfig
	Dimensions map[string]string
}

// LoadConfigLayers loads built-in, user, and project config layers without
// applying preset-derived engine/model defaults.
func LoadConfigLayers(projectRoot string) (*ConfigLayers, error) {
	cfg := BuiltinDefaults
	engines := copyEngines(BuiltinEngines)
	presets := copyPresetCatalog(Presets)
	dimensions := copyStringCatalog(BuiltinDimensions)

	home, _ := os.UserHomeDir()
	userCfg, err := LoadYAML[UserConfig](filepath.Join(home, ".goalx", "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("user config: %w", err)
	}
	applyConfigEnvelope(&cfg, &engines, &presets, &dimensions, &userCfg)

	projectConfigPath := filepath.Join(projectRoot, ".goalx", "config.yaml")
	projectCfg, err := LoadYAML[UserConfig](projectConfigPath)
	if err != nil {
		return nil, fmt.Errorf("project config: %w", err)
	}
	applyConfigEnvelope(&cfg, &engines, &presets, &dimensions, &projectCfg)
	cfg.Context.Files = FilterExternalContextFiles(projectRoot, cfg.Context.Files)
	attachCatalogs(&cfg, presets, dimensions)

	return &ConfigLayers{
		Config:     cfg,
		Engines:    engines,
		Presets:    presets,
		Dimensions: dimensions,
	}, nil
}

func attachCatalogs(cfg *Config, presets map[string]PresetConfig, dimensions map[string]string) {
	if cfg == nil {
		return
	}
	cfg.presetCatalog = presets
	cfg.dimensionCatalog = dimensions
}

func copyPresetCatalog(src map[string]PresetConfig) map[string]PresetConfig {
	dst := make(map[string]PresetConfig, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyStringCatalog(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
