package goalx

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigLayers contains the merged config plus local engine/dimension catalogs.
// The catalogs are load-scoped and must not mutate package-level built-ins.
type ConfigLayers struct {
	Config     Config
	Engines    map[string]EngineConfig
	Dimensions map[string]string
}

// LoadConfigLayers loads built-in, user, and project config layers.
func LoadConfigLayers(projectRoot string) (*ConfigLayers, error) {
	cfg := BuiltinDefaults
	engines := copyEngines(BuiltinEngines)
	dimensions := copyStringCatalog(BuiltinDimensions)

	home, _ := os.UserHomeDir()
	userCfg, err := LoadYAML[UserConfig](filepath.Join(home, ".goalx", "config.yaml"))
	if err != nil {
		return nil, fmt.Errorf("user config: %w", err)
	}
	applyConfigEnvelope(&cfg, &engines, &dimensions, &userCfg)

	projectConfigPath := filepath.Join(projectRoot, ".goalx", "config.yaml")
	projectCfg, err := LoadYAML[UserConfig](projectConfigPath)
	if err != nil {
		return nil, fmt.Errorf("project config: %w", err)
	}
	if hasSelectionConfig(projectCfg.Selection) {
		return nil, fmt.Errorf("project config: selection is only supported in ~/.goalx/config.yaml")
	}
	applyConfigEnvelope(&cfg, &engines, &dimensions, &projectCfg)
	cfg.Context.Files = FilterExternalContextFiles(projectRoot, cfg.Context.Files)
	attachDimensionCatalog(&cfg, dimensions)

	return &ConfigLayers{
		Config:     cfg,
		Engines:    engines,
		Dimensions: dimensions,
	}, nil
}

func attachDimensionCatalog(cfg *Config, dimensions map[string]string) {
	if cfg == nil {
		return
	}
	cfg.dimensionCatalog = dimensions
}

func copyStringCatalog(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
