package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestUpsertGlobalRunStoresConfiguredRunDir(t *testing.T) {
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
	layers.Config.Name = runName
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	// Create run-spec.yaml
	runSpec := []byte(`
name: test-run
mode: worker
objective: test
`)
	if err := os.WriteFile(RunSpecPath(runDir), runSpec, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}

	// Upsert global run with config that has RunRoot set
	cfg := &layers.Config
	if err := UpsertGlobalRun(projectRoot, cfg, "active"); err != nil {
		t.Fatalf("UpsertGlobalRun: %v", err)
	}

	// Verify the registry stores the actual configured run dir
	matches, err := LookupGlobalRuns(runName)
	if err != nil {
		t.Fatalf("LookupGlobalRuns: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].RunDir != runDir {
		t.Errorf("RunDir = %q, want %q", matches[0].RunDir, runDir)
	}
}

func TestLookupGlobalRunFindsRunInConfiguredRoot(t *testing.T) {
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

	// Load config
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	// Create a run in the configured location
	runName := "configured-run"
	layers.Config.Name = runName
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	// Create run metadata
	meta := &RunMetadata{
		RunID:       "run_test123",
		ProjectRoot: projectRoot,
		Objective:   "test objective",
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	// Upsert global run with config
	cfg := &layers.Config
	if err := UpsertGlobalRun(projectRoot, cfg, "active"); err != nil {
		t.Fatalf("UpsertGlobalRun: %v", err)
	}

	// Lookup by run ID should work
	matches, err := LookupGlobalRuns("run_test123")
	if err != nil {
		t.Fatalf("LookupGlobalRuns: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	if matches[0].RunDir != runDir {
		t.Errorf("RunDir = %q, want %q", matches[0].RunDir, runDir)
	}
}

func TestListDerivedRunStatesIncludesConfiguredRoot(t *testing.T) {
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

	// Load config
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	// Create a run in the configured location
	runName := "list-test-run"
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	// Create run-spec.yaml
	runSpec := []byte(`
name: list-test-run
mode: worker
objective: test listing
`)
	if err := os.WriteFile(RunSpecPath(runDir), runSpec, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}

	// Create control state to mark it as active
	controlState := &ControlRunState{
		GoalState:       "open",
		ContinuityState: "running",
		Phase:           runStatusPhaseWorking,
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), controlState); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	// List should find the run
	states, err := listDerivedRunStates(projectRoot)
	if err != nil {
		t.Fatalf("listDerivedRunStates: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].Name != runName {
		t.Errorf("Name = %q, want %q", states[0].Name, runName)
	}
	if states[0].RunDir != runDir {
		t.Errorf("RunDir = %q, want %q", states[0].RunDir, runDir)
	}
}

func TestCanonicalProjectRootResolvesFromConfiguredRunWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./.goalx/runs"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Load config
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	// Create a run with worktree in configured location
	runName := "worktree-test"
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	worktreeDir := filepath.Join(runDir, "worktrees", runName+"-1")
	if err := os.MkdirAll(worktreeDir, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	// Create run metadata
	meta := &RunMetadata{
		RunID:       "run_worktree_test",
		ProjectRoot: projectRoot,
		Objective:   "test worktree resolution",
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	// CanonicalProjectRoot should resolve from worktree to project root
	resolved := CanonicalProjectRoot(worktreeDir)
	if resolved != projectRoot {
		t.Errorf("CanonicalProjectRoot(%q) = %q, want %q", worktreeDir, resolved, projectRoot)
	}
}

func TestResolveLocalRunFallsBackToRegistryAfterRunRootConfigChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./.goalx/runs-a\n"), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "drift-run"
	layers.Config.Name = runName
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runSpec := []byte(`
name: drift-run
mode: worker
objective: drift test
run_root: ./.goalx/runs-a
`)
	if err := os.WriteFile(RunSpecPath(runDir), runSpec, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := UpsertGlobalRun(projectRoot, &layers.Config, "active"); err != nil {
		t.Fatalf("UpsertGlobalRun: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./.goalx/runs-b\n"), 0o644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	rc, err := resolveLocalRun(projectRoot, runName)
	if err != nil {
		t.Fatalf("resolveLocalRun: %v", err)
	}
	if rc.RunDir != runDir {
		t.Errorf("RunDir = %q, want %q", rc.RunDir, runDir)
	}
}

func TestListDerivedRunStatesIncludesRegistryRunAfterRunRootConfigChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./.goalx/runs-a\n"), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "registry-list-run"
	layers.Config.Name = runName
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	runSpec := []byte(`
name: registry-list-run
mode: worker
objective: list drift test
run_root: ./.goalx/runs-a
`)
	if err := os.WriteFile(RunSpecPath(runDir), runSpec, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	controlState := &ControlRunState{
		GoalState:       "open",
		ContinuityState: "running",
		Phase:           runStatusPhaseWorking,
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), controlState); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := UpsertGlobalRun(projectRoot, &layers.Config, "active"); err != nil {
		t.Fatalf("UpsertGlobalRun: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./.goalx/runs-b\n"), 0o644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	states, err := listDerivedRunStates(projectRoot)
	if err != nil {
		t.Fatalf("listDerivedRunStates: %v", err)
	}
	if len(states) != 1 {
		t.Fatalf("expected 1 state, got %d", len(states))
	}
	if states[0].RunDir != runDir {
		t.Errorf("RunDir = %q, want %q", states[0].RunDir, runDir)
	}
}
