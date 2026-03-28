package cli

import (
	"fmt"
	"os"
	"path/filepath"

	goalx "github.com/vonbai/goalx"
)

func SharedProjectConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".goalx", "config.yaml")
}

func ManualDraftConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".goalx", "goalx.yaml")
}

func LoadManualDraftConfig(projectRoot, draftPath string) (*goalx.Config, map[string]goalx.EngineConfig, error) {
	if draftPath == "" {
		draftPath = ManualDraftConfigPath(projectRoot)
	}
	if _, err := os.Stat(draftPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("manual draft config not found: %s", draftPath)
		}
		return nil, nil, fmt.Errorf("manual draft config: %w", err)
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	draft, err := goalx.LoadYAML[goalx.Config](draftPath)
	if err != nil {
		return nil, nil, fmt.Errorf("manual draft config: %w", err)
	}
	resolved, err := goalx.ResolveConfig(layers, goalx.ResolveRequest{ManualDraft: &draft, RequireEngineAvailability: true})
	if err != nil {
		return nil, nil, err
	}
	resolved.Config.Context.Files = goalx.FilterExternalContextFiles(projectRoot, resolved.Config.Context.Files)
	return &resolved.Config, resolved.Engines, nil
}

func loadEngineCatalog(projectRoot string) (map[string]goalx.EngineConfig, error) {
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, err
	}
	return layers.Engines, nil
}

func loadDimensionCatalog(projectRoot string) (map[string]string, error) {
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, err
	}
	return layers.Dimensions, nil
}
