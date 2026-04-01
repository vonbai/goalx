package goalx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigParsesRunRootFields(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	content := `
name: test
objective: test objective
run_root: ./custom-runs
saved_run_root: ./saved-runs
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadYAML[Config](configPath)
	if err != nil {
		t.Fatalf("LoadYAML: %v", err)
	}

	if cfg.RunRoot != "./custom-runs" {
		t.Errorf("RunRoot = %q, want ./custom-runs", cfg.RunRoot)
	}
	if cfg.SavedRunRoot != "./saved-runs" {
		t.Errorf("SavedRunRoot = %q, want ./saved-runs", cfg.SavedRunRoot)
	}
}

func TestResolveRunRoot(t *testing.T) {
	t.Parallel()

	projectRoot := "/home/user/projects/myapp"
	home, _ := os.UserHomeDir()
	legacyRunRoot := filepath.Join(home, ".goalx", "runs", ProjectID(projectRoot))

	tests := []struct {
		name        string
		runRoot     string
		projectRoot string
		want        string
	}{
		{
			name:        "empty uses_legacy",
			runRoot:     "",
			projectRoot: projectRoot,
			want:        legacyRunRoot,
		},
		{
			name:        "relative_resolves_against_project_root",
			runRoot:     "./custom-runs",
			projectRoot: projectRoot,
			want:        filepath.Join(projectRoot, "custom-runs"),
		},
		{
			name:        "absolute_passes_through",
			runRoot:     "/var/goalx/runs",
			projectRoot: projectRoot,
			want:        "/var/goalx/runs",
		},
		{
			name:        "relative_without_dot",
			runRoot:     "custom-runs",
			projectRoot: projectRoot,
			want:        filepath.Join(projectRoot, "custom-runs"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{RunRoot: tt.runRoot}
			got := ResolveRunRoot(tt.projectRoot, cfg)
			if got != tt.want {
				t.Errorf("ResolveRunRoot(%q, cfg) = %q, want %q", tt.projectRoot, got, tt.want)
			}
		})
	}
}

func TestResolveSavedRunRoot(t *testing.T) {
	t.Parallel()

	projectRoot := "/home/user/projects/myapp"
	home, _ := os.UserHomeDir()
	legacySavedRoot := filepath.Join(home, ".goalx", "runs", ProjectID(projectRoot), "saved")

	tests := []struct {
		name          string
		savedRunRoot  string
		projectRoot   string
		want          string
	}{
		{
			name:         "empty_uses_legacy",
			savedRunRoot: "",
			projectRoot:  projectRoot,
			want:         legacySavedRoot,
		},
		{
			name:         "relative_resolves_against_project_root",
			savedRunRoot: "./saved-runs",
			projectRoot:  projectRoot,
			want:         filepath.Join(projectRoot, "saved-runs"),
		},
		{
			name:         "absolute_passes_through",
			savedRunRoot: "/var/goalx/saved",
			projectRoot:  projectRoot,
			want:         "/var/goalx/saved",
		},
		{
			name:         "relative_without_dot",
			savedRunRoot: "saved-runs",
			projectRoot:  projectRoot,
			want:         filepath.Join(projectRoot, "saved-runs"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{SavedRunRoot: tt.savedRunRoot}
			got := ResolveSavedRunRoot(tt.projectRoot, cfg)
			if got != tt.want {
				t.Errorf("ResolveSavedRunRoot(%q, cfg) = %q, want %q", tt.projectRoot, got, tt.want)
			}
		})
	}
}

func TestResolveRunDir(t *testing.T) {
	t.Parallel()

	projectRoot := "/home/user/projects/myapp"
	home, _ := os.UserHomeDir()
	legacyRunDir := filepath.Join(home, ".goalx", "runs", ProjectID(projectRoot), "my-run")

	tests := []struct {
		name        string
		runRoot     string
		projectRoot string
		runName     string
		want        string
	}{
		{
			name:        "empty_uses_legacy",
			runRoot:     "",
			projectRoot: projectRoot,
			runName:     "my-run",
			want:        legacyRunDir,
		},
		{
			name:        "relative_custom_root",
			runRoot:     "./custom-runs",
			projectRoot: projectRoot,
			runName:     "my-run",
			want:        filepath.Join(projectRoot, "custom-runs", "my-run"),
		},
		{
			name:        "absolute_custom_root",
			runRoot:     "/var/goalx/runs",
			projectRoot: projectRoot,
			runName:     "my-run",
			want:        "/var/goalx/runs/my-run",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{RunRoot: tt.runRoot}
			got := ResolveRunDir(tt.projectRoot, tt.runName, cfg)
			if got != tt.want {
				t.Errorf("ResolveRunDir(%q, %q, cfg) = %q, want %q", tt.projectRoot, tt.runName, got, tt.want)
			}
		})
	}
}

func TestResolveSavedRunDir(t *testing.T) {
	t.Parallel()

	projectRoot := "/home/user/projects/myapp"
	home, _ := os.UserHomeDir()
	legacySavedDir := filepath.Join(home, ".goalx", "runs", ProjectID(projectRoot), "saved", "my-saved")

	tests := []struct {
		name          string
		savedRunRoot  string
		projectRoot   string
		runName       string
		want          string
	}{
		{
			name:         "empty_uses_legacy",
			savedRunRoot: "",
			projectRoot:  projectRoot,
			runName:      "my-saved",
			want:         legacySavedDir,
		},
		{
			name:         "relative_custom_root",
			savedRunRoot: "./saved-runs",
			projectRoot:  projectRoot,
			runName:      "my-saved",
			want:         filepath.Join(projectRoot, "saved-runs", "my-saved"),
		},
		{
			name:         "absolute_custom_root",
			savedRunRoot: "/var/goalx/saved",
			projectRoot:  projectRoot,
			runName:      "my-saved",
			want:         "/var/goalx/saved/my-saved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{SavedRunRoot: tt.savedRunRoot}
			got := ResolveSavedRunDir(tt.projectRoot, tt.runName, cfg)
			if got != tt.want {
				t.Errorf("ResolveSavedRunDir(%q, %q, cfg) = %q, want %q", tt.projectRoot, tt.runName, got, tt.want)
			}
		})
	}
}

func TestLoadConfigLayersIncludesRunRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}

	projectCfg := []byte(`
name: test-project
objective: test objective
run_root: ./project-runs
saved_run_root: ./project-saved
`)
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	if layers.Config.RunRoot != "./project-runs" {
		t.Errorf("RunRoot = %q, want ./project-runs", layers.Config.RunRoot)
	}
	if layers.Config.SavedRunRoot != "./project-saved" {
		t.Errorf("SavedRunRoot = %q, want ./project-saved", layers.Config.SavedRunRoot)
	}
}
