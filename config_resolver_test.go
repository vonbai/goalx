package goalx

import (
	"errors"
	"strings"
	"testing"
)

type resolverTestLayers struct {
	Base           Config
	ManualDraft    *Config
	DetectedPreset string
}

type resolverTestRequest struct {
	Preset       string
	Mode         Mode
	MasterEngine string
	MasterModel  string
}

type resolverTestResult struct {
	Preset string
	Config Config
}

func resolveConfigFixture(layers resolverTestLayers, req resolverTestRequest) (resolverTestResult, error) {
	base := layers.Base
	catalogPresets := copyPresetCatalog(Presets)
	catalogDimensions := copyStringCatalog(BuiltinDimensions)
	attachCatalogs(&base, catalogPresets, catalogDimensions)
	resolved, err := resolveConfigWithDetector(&ConfigLayers{
		Config:     base,
		Engines:    copyEngines(BuiltinEngines),
		Presets:    catalogPresets,
		Dimensions: catalogDimensions,
	}, ResolveRequest{
		ManualDraft: layers.ManualDraft,
		Mode:        req.Mode,
		Preset:      req.Preset,
		MasterOverride: &MasterConfig{
			Engine: req.MasterEngine,
			Model:  req.MasterModel,
		},
	}, func() (string, error) {
		return layers.DetectedPreset, nil
	})
	if err != nil {
		return resolverTestResult{}, err
	}
	return resolverTestResult{
		Preset: resolved.Config.Preset,
		Config: resolved.Config,
	}, nil
}

func TestResolveConfigSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		layers resolverTestLayers
		req    resolverTestRequest
		want   resolverTestResult
	}{
		{
			name: "explicit codex preset stays codex even with both engines present",
			layers: resolverTestLayers{
				Base: Config{
					Name:            "demo",
					Mode:            ModeDevelop,
					Objective:       "lock config state",
					Target:          TargetConfig{Files: []string{"README.md"}},
					LocalValidation: LocalValidationConfig{Command: "go test ./..."},
				},
				DetectedPreset: "hybrid",
			},
			req: resolverTestRequest{
				Preset: "codex",
				Mode:   ModeDevelop,
			},
			want: resolverTestResult{
				Preset: "codex",
				Config: Config{
					Preset: "codex",
					Mode:   ModeDevelop,
					Master: MasterConfig{Engine: "codex", Model: "gpt-5.4"},
				},
			},
		},
		{
			name: "shared config baseline applies before detection",
			layers: resolverTestLayers{
				Base: Config{
					Name:            "demo",
					Mode:            ModeDevelop,
					Objective:       "lock config state",
					Preset:          "claude-h",
					Target:          TargetConfig{Files: []string{"README.md"}},
					LocalValidation: LocalValidationConfig{Command: "go test ./..."},
				},
				DetectedPreset: "hybrid",
			},
			req: resolverTestRequest{
				Mode: ModeDevelop,
			},
			want: resolverTestResult{
				Preset: "claude-h",
				Config: Config{
					Preset: "claude-h",
					Mode:   ModeDevelop,
					Master: MasterConfig{Engine: "claude-code", Model: "opus"},
				},
			},
		},
		{
			name: "unset preset uses the discovered preset",
			layers: resolverTestLayers{
				Base: Config{
					Name:            "demo",
					Mode:            ModeDevelop,
					Objective:       "lock config state",
					Target:          TargetConfig{Files: []string{"README.md"}},
					LocalValidation: LocalValidationConfig{Command: "go test ./..."},
				},
				DetectedPreset: "claude-h",
			},
			req: resolverTestRequest{
				Mode: ModeDevelop,
			},
			want: resolverTestResult{
				Preset: "claude-h",
				Config: Config{
					Preset: "claude-h",
					Mode:   ModeDevelop,
					Master: MasterConfig{Engine: "claude-code", Model: "opus"},
				},
			},
		},
		{
			name: "manual draft overlay wins over shared config baseline",
			layers: resolverTestLayers{
				Base: Config{
					Name:            "demo",
					Mode:            ModeDevelop,
					Objective:       "lock config state",
					Preset:          "codex",
					Target:          TargetConfig{Files: []string{"README.md"}},
					LocalValidation: LocalValidationConfig{Command: "go test ./..."},
				},
				ManualDraft: &Config{
					Preset: "claude-h",
				},
				DetectedPreset: "hybrid",
			},
			req: resolverTestRequest{
				Mode: ModeDevelop,
			},
			want: resolverTestResult{
				Preset: "claude-h",
				Config: Config{
					Preset: "claude-h",
					Mode:   ModeDevelop,
					Master: MasterConfig{Engine: "claude-code", Model: "opus"},
				},
			},
		},
		{
			name: "cli override wins over manual draft role defaults",
			layers: resolverTestLayers{
				Base: Config{
					Name:            "demo",
					Mode:            ModeDevelop,
					Objective:       "lock config state",
					Target:          TargetConfig{Files: []string{"README.md"}},
					LocalValidation: LocalValidationConfig{Command: "go test ./..."},
				},
				ManualDraft: &Config{
					Preset: "claude",
				},
				DetectedPreset: "hybrid",
			},
			req: resolverTestRequest{
				Mode:         ModeDevelop,
				MasterEngine: "codex",
				MasterModel:  "gpt-5.4",
			},
			want: resolverTestResult{
				Preset: "claude",
				Config: Config{
					Preset: "claude",
					Mode:   ModeDevelop,
					Master: MasterConfig{Engine: "codex", Model: "gpt-5.4"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveConfigFixture(tt.layers, tt.req)
			if err != nil {
				t.Fatalf("resolveConfigFixture: %v", err)
			}
			if got.Preset != tt.want.Preset {
				t.Fatalf("preset = %q, want %q", got.Preset, tt.want.Preset)
			}
			if got.Config.Preset != tt.want.Config.Preset {
				t.Fatalf("config.preset = %q, want %q", got.Config.Preset, tt.want.Config.Preset)
			}
				if got.Config.Master.Engine != tt.want.Config.Master.Engine || got.Config.Master.Model != tt.want.Config.Master.Model {
					t.Fatalf("master = %#v, want %#v", got.Config.Master, tt.want.Config.Master)
				}
			})
		}
	}

func TestResolveConfigReturnsErrorWhenNoEngineCanBeSelected(t *testing.T) {
	t.Parallel()

	base := Config{
		Name:            "demo",
		Mode:            ModeDevelop,
		Objective:       "ship it",
		Target:          TargetConfig{Files: []string{"README.md"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
	}
	attachCatalogs(&base, copyPresetCatalog(Presets), copyStringCatalog(BuiltinDimensions))

	_, err := resolveConfigWithDetector(&ConfigLayers{
		Config:     base,
		Engines:    copyEngines(BuiltinEngines),
		Presets:    copyPresetCatalog(Presets),
		Dimensions: copyStringCatalog(BuiltinDimensions),
	}, ResolveRequest{}, func() (string, error) {
		return "", errors.New("no supported engines found in PATH")
	})
	if err == nil || !strings.Contains(err.Error(), "no supported engines found in PATH") {
		t.Fatalf("resolveConfigWithDetector error = %v, want no supported engines", err)
	}
}

func TestResolveConfigPreservesBuiltinRoutingDefaults(t *testing.T) {
	t.Parallel()

	base := BuiltinDefaults
	base.Name = "demo"
	base.Mode = ModeDevelop
	base.Objective = "lock config state"
	base.Target = TargetConfig{Files: []string{"README.md"}}
	base.LocalValidation = LocalValidationConfig{Command: "go test ./..."}
	attachCatalogs(&base, copyPresetCatalog(Presets), copyStringCatalog(BuiltinDimensions))

	resolved, err := resolveConfigWithDetector(&ConfigLayers{
		Config:     base,
		Engines:    copyEngines(BuiltinEngines),
		Presets:    copyPresetCatalog(Presets),
		Dimensions: copyStringCatalog(BuiltinDimensions),
	}, ResolveRequest{}, func() (string, error) {
		return "hybrid", nil
	})
	if err != nil {
		t.Fatalf("resolveConfigWithDetector: %v", err)
	}

	if len(resolved.Config.Routing.Rules) == 0 {
		t.Fatal("routing.rules is empty")
	}
	if got := resolved.Config.Routing.Rules[0].Role; got == "" {
		t.Fatalf("routing.rules[0].role = %q, want non-empty", got)
	}
	if got := resolved.Config.Preferences.Develop.Guidance; got != "主力 gpt-5.4 medium。简单修复用 fast。" {
		t.Fatalf("develop guidance = %q", got)
	}
}

func TestResolveConfigRejectsUnknownPreset(t *testing.T) {
	t.Parallel()

	base := Config{
		Name:            "demo",
		Mode:            ModeDevelop,
		Objective:       "ship it",
		Preset:          "missing-preset",
		Target:          TargetConfig{Files: []string{"README.md"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
	}
	attachCatalogs(&base, copyPresetCatalog(Presets), copyStringCatalog(BuiltinDimensions))

	_, err := resolveConfigWithDetector(&ConfigLayers{
		Config:     base,
		Engines:    copyEngines(BuiltinEngines),
		Presets:    copyPresetCatalog(Presets),
		Dimensions: copyStringCatalog(BuiltinDimensions),
	}, ResolveRequest{}, func() (string, error) {
		return "hybrid", nil
	})
	if err == nil || err.Error() != `unknown preset "missing-preset"` {
		t.Fatalf("resolveConfigWithDetector error = %v, want unknown preset", err)
	}
}
