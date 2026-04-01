package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestResolveLocalRunUsesConfiguredRunRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	// Create project config with custom run_root
	projectCfg := []byte(`
run_root: ./.goalx/runs
`)
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Load config to get the resolved run root
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	// Create a run in the configured location
	runName := "test-run"
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	// Create a minimal run-spec.yaml
	runSpec := []byte(`
name: test-run
mode: worker
objective: test
`)
	if err := os.WriteFile(RunSpecPath(runDir), runSpec, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}

	// Resolve the run
	rc, err := resolveLocalRun(projectRoot, runName)
	if err != nil {
		t.Fatalf("resolveLocalRun: %v", err)
	}

	if rc.RunDir != runDir {
		t.Errorf("RunDir = %q, want %q", rc.RunDir, runDir)
	}
	if rc.Name != runName {
		t.Errorf("Name = %q, want %q", rc.Name, runName)
	}
}

func TestResolveLocalRunFallbackToLegacyLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()

	// Create a run in the legacy location (no run_root configured)
	runName := "legacy-run"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	// Create a minimal run-spec.yaml
	runSpec := []byte(`
name: legacy-run
mode: worker
objective: test
`)
	if err := os.WriteFile(RunSpecPath(runDir), runSpec, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}

	// Resolve the run (should find it in legacy location)
	rc, err := resolveLocalRun(projectRoot, runName)
	if err != nil {
		t.Fatalf("resolveLocalRun: %v", err)
	}

	legacyRunDir := goalx.RunDir(projectRoot, runName)
	if rc.RunDir != legacyRunDir {
		t.Errorf("RunDir = %q, want legacy %q", rc.RunDir, legacyRunDir)
	}
}

func TestResolveLocalRunPrefersConfiguredOverLegacy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	// Create project config with custom run_root
	projectCfg := []byte(`
run_root: ./.goalx/runs
`)
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Load config to get the resolved run root
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "test-run"

	// Create run in BOTH configured and legacy locations
	// Configured location
	configuredDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(configuredDir, 0o755); err != nil {
		t.Fatalf("mkdir configured run dir: %v", err)
	}
	configuredSpec := []byte(`
name: test-run
mode: worker
objective: configured run
`)
	if err := os.WriteFile(RunSpecPath(configuredDir), configuredSpec, 0o644); err != nil {
		t.Fatalf("write configured run spec: %v", err)
	}

	// Legacy location
	legacyDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy run dir: %v", err)
	}
	legacySpec := []byte(`
name: test-run
mode: worker
objective: legacy run
`)
	if err := os.WriteFile(RunSpecPath(legacyDir), legacySpec, 0o644); err != nil {
		t.Fatalf("write legacy run spec: %v", err)
	}

	// Resolve should prefer configured location
	rc, err := resolveLocalRun(projectRoot, runName)
	if err != nil {
		t.Fatalf("resolveLocalRun: %v", err)
	}

	if rc.RunDir != configuredDir {
		t.Errorf("RunDir = %q, want configured %q (not legacy %q)", rc.RunDir, configuredDir, legacyDir)
	}
}

func TestNewStartRunStateUsesConfiguredRunRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	// Create project config with custom run_root
	projectCfg := []byte(`
name: test-run
run_root: ./.goalx/runs
`)
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Load config
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	// Create run state with config
	state := newStartRunState(projectRoot, &layers.Config)

	expectedRunDir := goalx.ResolveRunDir(projectRoot, "test-run", &layers.Config)
	if state.runDir != expectedRunDir {
		t.Errorf("state.runDir = %q, want %q", state.runDir, expectedRunDir)
	}

	// Verify it's under the project root, not under ~/.goalx
	if !filepath.IsAbs(state.runDir) {
		t.Errorf("state.runDir should be absolute, got %q", state.runDir)
	}

	rel, err := filepath.Rel(projectRoot, state.runDir)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	if rel == "" || rel[0] == '.' && len(rel) > 1 && rel[1] == '.' {
		t.Errorf("state.runDir should be under project root, got %q", state.runDir)
	}
}

func TestNewStartRunStateUsesLegacyPathWithoutConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()

	// No config - should use legacy path
	cfg := &goalx.Config{Name: "test-run"}
	state := newStartRunState(projectRoot, cfg)

	legacyRunDir := goalx.RunDir(projectRoot, "test-run")
	if state.runDir != legacyRunDir {
		t.Errorf("state.runDir = %q, want legacy %q", state.runDir, legacyRunDir)
	}
}

func TestResolveIntegrationStateUsesConfiguredRunRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./.goalx/runs\n"), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "integration-run"
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	state := &IntegrationState{
		Version:             1,
		CurrentExperimentID: "exp_1",
		CurrentBranch:       "goalx/integration-run/root",
		CurrentCommit:       "abc123",
		UpdatedAt:           "2026-04-01T00:00:00Z",
	}
	if err := SaveIntegrationState(IntegrationStatePath(runDir), state); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}

	got, err := ResolveIntegrationState(projectRoot, runName)
	if err != nil {
		t.Fatalf("ResolveIntegrationState: %v", err)
	}
	if got == nil {
		t.Fatal("ResolveIntegrationState returned nil")
	}
	if got.CurrentExperimentID != state.CurrentExperimentID {
		t.Errorf("CurrentExperimentID = %q, want %q", got.CurrentExperimentID, state.CurrentExperimentID)
	}
}
