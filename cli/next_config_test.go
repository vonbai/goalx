package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateNextConfigRejectsInvalidFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Parallel:   99,
		Dimensions: []string{"unknown"},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Parallel != 10 {
		t.Fatalf("parallel = %d, want 10", got.Parallel)
	}
	if got.Dimensions != nil {
		t.Fatalf("dimensions = %#v, want nil", got.Dimensions)
	}
}

func TestValidateNextConfigNormalizesPhaseHandoffFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Objective:  " custom objective ",
		Context:    []string{" docs/plan.md ", " ", "README.md"},
		Dimensions: []string{" depth ", "adversarial"},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Objective != "custom objective" {
		t.Fatalf("objective = %q, want custom objective", got.Objective)
	}
	if len(got.Context) != 2 || got.Context[0] != "docs/plan.md" || got.Context[1] != "README.md" {
		t.Fatalf("context = %#v, want trimmed non-empty paths", got.Context)
	}
	if len(got.Dimensions) != 2 || got.Dimensions[0] != "depth" || got.Dimensions[1] != "adversarial" {
		t.Fatalf("dimensions = %#v, want [depth adversarial]", got.Dimensions)
	}
}

func TestValidateNextConfigRejectsInvalidPhaseHandoffFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Parallel:   -1,
		Dimensions: []string{"not-a-dimension"},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Parallel != 0 {
		t.Fatalf("parallel = %d, want 0", got.Parallel)
	}
	if got.Dimensions != nil {
		t.Fatalf("dimensions = %#v, want nil", got.Dimensions)
	}
}

func TestValidateNextConfigDropsLegacySelectionFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfgYAML := `
engines:
  localai:
    command: "localai --model {model_id}"
    prompt: "Read {protocol}"
    models:
      small: local-small
`
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	var decoded nextConfigJSON
	if err := json.Unmarshal([]byte(`{
		"parallel": 3,
		"objective": "continue",
		"context": ["README.md"],
		"dimensions": ["depth"],
		"preset": "claude",
		"engine": "localai",
		"model": "small",
		"mode": "develop",
		"master_engine": "localai",
		"master_model": "small",
		"route_role": "research",
		"route_profile": "research_deep",
		"effort": "high",
		"master_effort": "high"
	}`), &decoded); err != nil {
		t.Fatalf("unmarshal next_config: %v", err)
	}

	got := validateNextConfig(projectRoot, &decoded)
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	data, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal next_config: %v", err)
	}
	for _, needle := range []string{
		`"preset"`,
		`"engine"`,
		`"model"`,
		`"mode"`,
		`"master_engine"`,
		`"master_model"`,
		`"route_role"`,
		`"route_profile"`,
		`"effort"`,
		`"master_effort"`,
	} {
		if strings.Contains(string(data), needle) {
			t.Fatalf("validated next_config still exposes legacy selection field %s: %s", needle, string(data))
		}
	}
}
