package goalx

import "testing"

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
	}, func() string {
		return layers.DetectedPreset
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
					Name:      "demo",
					Mode:      ModeDevelop,
					Objective: "lock config state",
					Target:    TargetConfig{Files: []string{"README.md"}},
					Harness:   HarnessConfig{Command: "go test ./..."},
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
					Name:      "demo",
					Mode:      ModeDevelop,
					Objective: "lock config state",
					Preset:    "claude-h",
					Target:    TargetConfig{Files: []string{"README.md"}},
					Harness:   HarnessConfig{Command: "go test ./..."},
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
					Name:      "demo",
					Mode:      ModeDevelop,
					Objective: "lock config state",
					Target:    TargetConfig{Files: []string{"README.md"}},
					Harness:   HarnessConfig{Command: "go test ./..."},
				},
				DetectedPreset: "claude",
			},
			req: resolverTestRequest{
				Mode: ModeDevelop,
			},
			want: resolverTestResult{
				Preset: "claude",
				Config: Config{
					Preset: "claude",
					Mode:   ModeDevelop,
					Master: MasterConfig{Engine: "claude-code", Model: "opus"},
				},
			},
		},
		{
			name: "manual draft overlay wins over shared config baseline",
			layers: resolverTestLayers{
				Base: Config{
					Name:      "demo",
					Mode:      ModeDevelop,
					Objective: "lock config state",
					Preset:    "codex",
					Target:    TargetConfig{Files: []string{"README.md"}},
					Harness:   HarnessConfig{Command: "go test ./..."},
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
					Name:      "demo",
					Mode:      ModeDevelop,
					Objective: "lock config state",
					Target:    TargetConfig{Files: []string{"README.md"}},
					Harness:   HarnessConfig{Command: "go test ./..."},
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
