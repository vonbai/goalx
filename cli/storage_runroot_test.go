package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestResolveSavedRunLocationUsesConfiguredSavedRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	// Create configured saved run root
	configuredSavedDir := filepath.Join(projectRoot, "saved-runs", "my-run")
	if err := os.MkdirAll(configuredSavedDir, 0o755); err != nil {
		t.Fatalf("mkdir configured saved dir: %v", err)
	}

	cfg := &goalx.Config{SavedRunRoot: "./saved-runs"}
	got, err := ResolveSavedRunLocationWithConfig(projectRoot, "my-run", cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig: %v", err)
	}
	if got.Dir != configuredSavedDir {
		t.Errorf("Dir = %q, want %q", got.Dir, configuredSavedDir)
	}
	if got.Legacy {
		t.Errorf("Legacy = true, want false")
	}
}

func TestResolveSavedRunLocationUsesProjectConfigByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-runs\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	savedDir := filepath.Join(projectRoot, "saved-runs", "my-run")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir configured saved dir: %v", err)
	}

	got, err := ResolveSavedRunLocation(projectRoot, "my-run")
	if err != nil {
		t.Fatalf("ResolveSavedRunLocation: %v", err)
	}
	if got.Dir != savedDir {
		t.Errorf("Dir = %q, want %q", got.Dir, savedDir)
	}
}

func TestResolveSavedRunLocationFallsBackToUserScopedWhenConfiguredEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	userScopedDir := SavedRunDir(projectRoot, "my-run")
	if err := os.MkdirAll(userScopedDir, 0o755); err != nil {
		t.Fatalf("mkdir user-scoped saved dir: %v", err)
	}

	cfg := &goalx.Config{} // SavedRunRoot is empty
	got, err := ResolveSavedRunLocationWithConfig(projectRoot, "my-run", cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig: %v", err)
	}
	if got.Dir != userScopedDir {
		t.Errorf("Dir = %q, want %q", got.Dir, userScopedDir)
	}
	if got.Legacy {
		t.Errorf("Legacy = true, want false")
	}
}

func TestResolveSavedRunLocationFallsBackToLegacyProjectLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	legacyDir := LegacySavedRunDir(projectRoot, "my-run")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy saved dir: %v", err)
	}

	cfg := &goalx.Config{} // SavedRunRoot is empty
	got, err := ResolveSavedRunLocationWithConfig(projectRoot, "my-run", cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig: %v", err)
	}
	if got.Dir != legacyDir {
		t.Errorf("Dir = %q, want %q", got.Dir, legacyDir)
	}
	if !got.Legacy {
		t.Errorf("Legacy = false, want true")
	}
}

func TestResolveSavedRunLocationFallbackOrder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	// Create all three possible locations
	configuredDir := filepath.Join(projectRoot, "saved-runs", "my-run")
	userScopedDir := SavedRunDir(projectRoot, "my-run")
	legacyDir := LegacySavedRunDir(projectRoot, "my-run")

	cfg := &goalx.Config{SavedRunRoot: "./saved-runs"}

	// When all exist, configured root wins
	for _, dir := range []string{configuredDir, userScopedDir, legacyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	got, err := ResolveSavedRunLocationWithConfig(projectRoot, "my-run", cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig: %v", err)
	}
	if got.Dir != configuredDir {
		t.Errorf("when all exist, Dir = %q, want configured %q", got.Dir, configuredDir)
	}

	// When configured doesn't exist, user-scoped wins
	if err := os.RemoveAll(configuredDir); err != nil {
		t.Fatalf("remove configured dir: %v", err)
	}
	got, err = ResolveSavedRunLocationWithConfig(projectRoot, "my-run", cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig (no configured): %v", err)
	}
	if got.Dir != userScopedDir {
		t.Errorf("when configured missing, Dir = %q, want user-scoped %q", got.Dir, userScopedDir)
	}

	// When user-scoped also doesn't exist, legacy wins
	if err := os.RemoveAll(userScopedDir); err != nil {
		t.Fatalf("remove user-scoped dir: %v", err)
	}
	got, err = ResolveSavedRunLocationWithConfig(projectRoot, "my-run", cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig (no user-scoped): %v", err)
	}
	if got.Dir != legacyDir {
		t.Errorf("when only legacy exists, Dir = %q, want legacy %q", got.Dir, legacyDir)
	}
}

func TestListSavedRunLocationsIncludesConfiguredRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	// Create runs in all three locations
	configuredDir := filepath.Join(projectRoot, "saved-runs", "run-1")
	userScopedDir := SavedRunDir(projectRoot, "run-2")
	legacyDir := LegacySavedRunDir(projectRoot, "run-3")

	for _, dir := range []string{configuredDir, userScopedDir, legacyDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	cfg := &goalx.Config{SavedRunRoot: "./saved-runs"}
	locations, err := ListSavedRunLocationsWithConfig(projectRoot, cfg)
	if err != nil {
		t.Fatalf("ListSavedRunLocationsWithConfig: %v", err)
	}

	seen := make(map[string]bool)
	for _, loc := range locations {
		seen[loc.Name] = true
	}

	// All three runs should be found
	if !seen["run-1"] {
		t.Errorf("missing run-1 from configured root")
	}
	if !seen["run-2"] {
		t.Errorf("missing run-2 from user-scoped root")
	}
	if !seen["run-3"] {
		t.Errorf("missing run-3 from legacy root")
	}
}

func TestSaveWritesToConfiguredSavedRunRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:          runName,
		Mode:          goalx.ModeWorker,
		Objective:     "inspect",
		Parallel:      1,
		SavedRunRoot:  "./custom-saved",
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})

	want := "saved custom report"
	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify saved to configured root
	expectedSavedDir := filepath.Join(projectRoot, "custom-saved", runName)
	savedPath := filepath.Join(expectedSavedDir, "session-1-report.md")
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != want+"\n" {
		t.Fatalf("saved report = %q, want %q", string(got), want+"\n")
	}
}

func TestResultFindsSavedRunInConfiguredRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()

	// Create saved run in configured root
	cfg := goalx.Config{
		Name:          "demo",
		Mode:          goalx.ModeWorker,
		Objective:     "inspect",
		SavedRunRoot:  "./custom-saved",
		Target: goalx.TargetConfig{
			Files: []string{"report.md"},
		},
	}
	savedDir := filepath.Join(projectRoot, "custom-saved", "demo")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir saved dir: %v", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(savedDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "summary.md"), []byte("# configured root summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	// Result should find the saved run from configured root when using config
	// Note: Result command needs to load config to know about saved_run_root
	// This test validates the resolver works; CLI integration is separate
	location, err := ResolveSavedRunLocationWithConfig(projectRoot, "demo", &cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig: %v", err)
	}
	if location.Dir != savedDir {
		t.Errorf("location.Dir = %q, want %q", location.Dir, savedDir)
	}
}

func TestResultFallsBackFromConfiguredToUserScoped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()

	// Config specifies a saved_run_root, but run exists in user-scoped location
	cfg := goalx.Config{
		Name:          "demo",
		Mode:          goalx.ModeWorker,
		Objective:     "inspect",
		SavedRunRoot:  "./custom-saved",
		Target: goalx.TargetConfig{
			Files: []string{"report.md"},
		},
	}

	// Create saved run in user-scoped location (not configured location)
	userScopedDir := SavedRunDir(projectRoot, "demo")
	if err := os.MkdirAll(userScopedDir, 0o755); err != nil {
		t.Fatalf("mkdir user-scoped dir: %v", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(userScopedDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userScopedDir, "summary.md"), []byte("# user-scoped summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	// Resolver should find user-scoped run when configured location doesn't exist
	location, err := ResolveSavedRunLocationWithConfig(projectRoot, "demo", &cfg)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocationWithConfig: %v", err)
	}
	if location.Dir != userScopedDir {
		t.Errorf("location.Dir = %q, want %q", location.Dir, userScopedDir)
	}
}

func TestLoadSavedPhaseSourceUsesConfiguredSavedRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./custom-saved\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	savedDir := filepath.Join(projectRoot, "custom-saved", "demo")
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir saved dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Context: goalx.ContextConfig{
			Files: []string{"report.md"},
		},
		Target: goalx.TargetConfig{Files: []string{"report.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(savedDir), data, 0o644); err != nil {
		t.Fatalf("write saved run spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "summary.md"), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := SaveRunIntake(SavedRunIntakePath(savedDir), &RunIntake{
		Version:      1,
		Objective:    cfg.Objective,
		Intent:       runIntentDeliver,
		ContextFiles: []string{"report.md"},
	}); err != nil {
		t.Fatalf("write intake: %v", err)
	}

	source, err := loadSavedPhaseSource(projectRoot, "demo")
	if err != nil {
		t.Fatalf("loadSavedPhaseSource: %v", err)
	}
	if source.Dir != savedDir {
		t.Errorf("Dir = %q, want %q", source.Dir, savedDir)
	}
	if len(source.Context.Files) == 0 {
		t.Fatalf("expected saved phase context files, got none")
	}
}

func TestResolveSavedRunLocationFallsBackToRegistryAfterSavedRunRootConfigChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-a\n"), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "saved-drift"
	layers.Config.Name = runName
	layers.Config.Mode = goalx.ModeWorker
	layers.Config.Objective = "inspect drift"
	layers.Config.Target = goalx.TargetConfig{Files: []string{"report.md"}}

	savedDir := goalx.ResolveSavedRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir saved dir: %v", err)
	}
	data, err := yaml.Marshal(&layers.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(savedDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := RegisterSavedRun(projectRoot, &layers.Config); err != nil {
		t.Fatalf("RegisterSavedRun: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-b\n"), 0o644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	location, err := ResolveSavedRunLocation(projectRoot, runName)
	if err != nil {
		t.Fatalf("ResolveSavedRunLocation: %v", err)
	}
	if location.Dir != savedDir {
		t.Errorf("Dir = %q, want %q", location.Dir, savedDir)
	}
}

func TestResolveSavedRunLocationListsRegistrySavedRunAfterSavedRunRootConfigChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-a\n"), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "saved-list-drift"
	layers.Config.Name = runName
	layers.Config.Mode = goalx.ModeWorker
	layers.Config.Objective = "inspect drift list"
	layers.Config.Target = goalx.TargetConfig{Files: []string{"report.md"}}

	savedDir := goalx.ResolveSavedRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir saved dir: %v", err)
	}
	data, err := yaml.Marshal(&layers.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(savedDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := RegisterSavedRun(projectRoot, &layers.Config); err != nil {
		t.Fatalf("RegisterSavedRun: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-b\n"), 0o644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	location, err := ResolveSavedRunLocation(projectRoot, "")
	if err != nil {
		t.Fatalf("ResolveSavedRunLocation: %v", err)
	}
	if location.Name != runName {
		t.Errorf("Name = %q, want %q", location.Name, runName)
	}
	if location.Dir != savedDir {
		t.Errorf("Dir = %q, want %q", location.Dir, savedDir)
	}
}

func TestLoadSavedPhaseSourceFallsBackToRegistryAfterSavedRunRootConfigChange(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-a\n"), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	runName := "phase-drift"
	layers.Config.Name = runName
	layers.Config.Mode = goalx.ModeWorker
	layers.Config.Objective = "inspect phase drift"
	layers.Config.Context = goalx.ContextConfig{Files: []string{"report.md"}}
	layers.Config.Target = goalx.TargetConfig{Files: []string{"report.md"}}

	savedDir := goalx.ResolveSavedRunDir(projectRoot, runName, &layers.Config)
	if err := os.MkdirAll(savedDir, 0o755); err != nil {
		t.Fatalf("mkdir saved dir: %v", err)
	}
	data, err := yaml.Marshal(&layers.Config)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(savedDir), data, 0o644); err != nil {
		t.Fatalf("write saved run spec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(savedDir, "summary.md"), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := SaveRunIntake(SavedRunIntakePath(savedDir), &RunIntake{
		Version:      1,
		Objective:    layers.Config.Objective,
		Intent:       runIntentDeliver,
		ContextFiles: []string{"report.md"},
	}); err != nil {
		t.Fatalf("write intake: %v", err)
	}
	if err := RegisterSavedRun(projectRoot, &layers.Config); err != nil {
		t.Fatalf("RegisterSavedRun: %v", err)
	}

	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("saved_run_root: ./saved-b\n"), 0o644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	source, err := loadSavedPhaseSource(projectRoot, runName)
	if err != nil {
		t.Fatalf("loadSavedPhaseSource: %v", err)
	}
	if source.Dir != savedDir {
		t.Errorf("Dir = %q, want %q", source.Dir, savedDir)
	}
}
