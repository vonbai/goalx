package goalx

import (
	"errors"
	"testing"
)

type resolverTestLayers struct {
	Base           Config
	DetectedPreset string
}

type resolverTestRequest struct {
	Preset string
	Mode   Mode
}

type resolverTestResult struct {
	Preset string
	Config Config
}

var errResolverNotImplemented = errors.New("resolver not implemented")

func resolveConfigFixture(layers resolverTestLayers, req resolverTestRequest) (resolverTestResult, error) {
	_ = layers
	_ = req
	return resolverTestResult{}, errResolverNotImplemented
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
