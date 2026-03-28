package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateNextConfigRejectsInvalidFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Parallel:   99,
		Engine:     "unknown-engine",
		Dimensions: []string{"a", "b"},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Parallel != 10 {
		t.Fatalf("parallel = %d, want 10", got.Parallel)
	}
	if got.Engine != "" {
		t.Fatalf("engine = %q, want empty", got.Engine)
	}
}

func TestValidateNextConfigNormalizesExtendedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Mode:          " research ",
		MaxIterations: 7,
		Context:       []string{" docs/plan.md ", " ", "README.md"},
		MasterEngine:  " codex ",
		MasterModel:   " fast ",
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Mode != "research" {
		t.Fatalf("mode = %q, want research", got.Mode)
	}
	if got.MaxIterations != 7 {
		t.Fatalf("max_iterations = %d, want 7", got.MaxIterations)
	}
	if len(got.Context) != 2 || got.Context[0] != "docs/plan.md" || got.Context[1] != "README.md" {
		t.Fatalf("context = %#v, want trimmed non-empty paths", got.Context)
	}
	if got.MasterEngine != "codex" || got.MasterModel != "fast" {
		t.Fatalf("master engine/model = %q/%q, want codex/fast", got.MasterEngine, got.MasterModel)
	}
}

func TestValidateNextConfigRejectsInvalidExtendedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Mode:          "invalid",
		MaxIterations: 42,
		MasterEngine:  "unknown",
		MasterModel:   "fast",
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Mode != "" {
		t.Fatalf("mode = %q, want empty", got.Mode)
	}
	if got.MaxIterations != 0 {
		t.Fatalf("max_iterations = %d, want 0", got.MaxIterations)
	}
	if got.MasterEngine != "" || got.MasterModel != "" {
		t.Fatalf("master engine/model = %q/%q, want empty", got.MasterEngine, got.MasterModel)
	}
}

func TestValidateNextConfigUsesProjectEngineCatalog(t *testing.T) {
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

	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Engine:       "localai",
		Model:        "small",
		MasterEngine: "localai",
		MasterModel:  "small",
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Engine != "localai" || got.Model != "small" {
		t.Fatalf("engine/model = %q/%q, want localai/small", got.Engine, got.Model)
	}
	if got.MasterEngine != "localai" || got.MasterModel != "small" {
		t.Fatalf("master engine/model = %q/%q, want localai/small", got.MasterEngine, got.MasterModel)
	}
}
