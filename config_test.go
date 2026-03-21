package goalx

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"event sourcing", "event-sourcing"},
		{"/home/user/projects/myapp", "home-user-projects-myapp"},
		{"Hello World!!! Test", "hello-world-test"},
		{"已有event-sourcing方案", "event-sourcing"},
		{"", ""},
	}
	for _, tt := range tests {
		got := Slugify(tt.in)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestProjectID(t *testing.T) {
	id := ProjectID("/home/user/projects/myapp")
	if id != "home-user-projects-myapp" {
		t.Errorf("ProjectID = %q, want 'home-user-projects-myapp'", id)
	}
}

func TestRunDir(t *testing.T) {
	dir := RunDir("/home/user/projects/myapp", "event-sourcing")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".goalx", "runs", "home-user-projects-myapp", "event-sourcing")
	if dir != want {
		t.Errorf("RunDir = %q, want %q", dir, want)
	}
}

func TestTmuxSessionName(t *testing.T) {
	name := TmuxSessionName("/home/user/projects/myapp", "event-sourcing")
	if name != "goalx-home-user-projects-myapp-event-sourcing" {
		t.Errorf("TmuxSessionName = %q", name)
	}
}

func TestPresetClaude(t *testing.T) {
	cfg := Config{Preset: "claude", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Errorf("master = %s/%s, want claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Engine != "codex" || cfg.Model != "codex" {
		t.Errorf("develop = %s/%s, want codex/codex", cfg.Engine, cfg.Model)
	}
}

func TestPresetClaudeResearch(t *testing.T) {
	cfg := Config{Preset: "claude", Mode: ModeResearch}
	applyPreset(&cfg)
	if cfg.Engine != "claude-code" || cfg.Model != "sonnet" {
		t.Errorf("research = %s/%s, want claude-code/sonnet", cfg.Engine, cfg.Model)
	}
}

func TestPresetClaudeH(t *testing.T) {
	cfg := Config{Preset: "claude-h", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Model != "opus" {
		t.Errorf("claude-h master model = %q, want opus", cfg.Master.Model)
	}
	if cfg.Engine != "claude-code" || cfg.Model != "opus" {
		t.Errorf("claude-h develop = %s/%s, want claude-code/opus", cfg.Engine, cfg.Model)
	}
}

func TestPresetCodex(t *testing.T) {
	cfg := Config{Preset: "codex", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "codex" {
		t.Errorf("codex master = %s/%s, want codex/codex", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Engine != "codex" || cfg.Model != "codex" {
		t.Errorf("codex develop = %s/%s, want codex/codex", cfg.Engine, cfg.Model)
	}
}

func TestPresetMixed(t *testing.T) {
	cfg := Config{Preset: "mixed", Mode: ModeResearch}
	applyPreset(&cfg)
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "codex" {
		t.Errorf("mixed master = %s/%s, want codex/codex", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Engine != "claude-code" || cfg.Model != "opus" {
		t.Errorf("mixed research = %s/%s, want claude-code/opus", cfg.Engine, cfg.Model)
	}
}

func TestPresetNoOverrideExplicit(t *testing.T) {
	cfg := Config{Preset: "claude", Mode: ModeDevelop, Engine: "aider", Model: "opus"}
	applyPreset(&cfg)
	if cfg.Engine != "aider" || cfg.Model != "opus" {
		t.Errorf("explicit should not be overridden: %s/%s", cfg.Engine, cfg.Model)
	}
}

func TestResolveEngineCommand(t *testing.T) {
	cmd, err := ResolveEngineCommand(BuiltinEngines, "claude-code", "opus")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "claude --model claude-opus-4-6 --permission-mode auto" {
		t.Errorf("cmd = %q", cmd)
	}
}

func TestResolveEngineCommandCodex(t *testing.T) {
	cmd, err := ResolveEngineCommand(BuiltinEngines, "codex", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "codex -m gpt-5.4 -a never -s danger-full-access" {
		t.Errorf("cmd = %q", cmd)
	}
}

func TestResolveEngineCommandLiteralModel(t *testing.T) {
	cmd, err := ResolveEngineCommand(BuiltinEngines, "codex", "gpt-5.2")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "codex -m gpt-5.2 -a never -s danger-full-access" {
		t.Errorf("literal model: cmd = %q", cmd)
	}
}

func TestResolveEngineUnknown(t *testing.T) {
	_, err := ResolveEngineCommand(BuiltinEngines, "unknown-engine", "x")
	if err == nil {
		t.Error("expected error for unknown engine")
	}
}

func TestResolvePrompt(t *testing.T) {
	p := ResolvePrompt(BuiltinEngines, "claude-code", "/tmp/master.md")
	if p != "Read /tmp/master.md and follow it exactly." {
		t.Errorf("prompt = %q", p)
	}
}

func TestExpandSessions(t *testing.T) {
	cfg := Config{Parallel: 3, DiversityHints: []string{"a", "b", "c"}}
	sessions := ExpandSessions(&cfg)
	if len(sessions) != 3 {
		t.Fatalf("len = %d, want 3", len(sessions))
	}
	if sessions[0].Hint != "a" || sessions[2].Hint != "c" {
		t.Errorf("hints not set correctly")
	}
}

func TestExpandSessionsExplicit(t *testing.T) {
	cfg := Config{
		Sessions: []SessionConfig{
			{Hint: "x", Engine: "codex"},
			{Hint: "y"},
		},
	}
	sessions := ExpandSessions(&cfg)
	if len(sessions) != 2 || sessions[0].Engine != "codex" {
		t.Errorf("explicit sessions not used")
	}
}

func TestValidateConfigOK(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeDevelop,
		Objective: "test objective",
		Engine:    "claude-code",
		Model:     "sonnet",
		Target:    TargetConfig{Files: []string{"src/"}},
		Harness:   HarnessConfig{Command: "go test ./..."},
		Master:    MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateConfigMissingObjective(t *testing.T) {
	cfg := &Config{Name: "test", Mode: ModeDevelop}
	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Error("expected error for missing objective")
	}
}

func TestValidateConfigPlaceholder(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeDevelop,
		Objective: "test",
		Engine:    "claude-code",
		Model:     "sonnet",
		Target:    TargetConfig{Files: []string{"TODO: specify"}},
		Harness:   HarnessConfig{Command: "go test"},
		Master:    MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Error("expected error for placeholder in target.files")
	}
}

func TestValidateConfigSessionsConflict(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeDevelop,
		Objective: "test",
		Engine:    "claude-code",
		Model:     "sonnet",
		Parallel:  2,
		Sessions:  []SessionConfig{{Hint: "a"}},
		Target:    TargetConfig{Files: []string{"src/"}},
		Harness:   HarnessConfig{Command: "go test"},
		Master:    MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Error("expected error for sessions + parallel conflict")
	}
}

func TestLoadConfigMergesServeConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
defaults:
  serve:
    bind: 100.110.196.103:18790
    token: user-token
    workspaces:
      user: /srv/user
    notification_url: http://user.example/hook
`) + "\n")
	if err := os.WriteFile(filepath.Join(userGoalxDir, "config.yaml"), userCfg, 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
serve:
  token: project-token
  workspaces:
    project: /srv/project
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
serve:
  notification_url: http://run.example/hook
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	cfg, _, err := LoadConfig(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Serve.Bind != "100.110.196.103:18790" {
		t.Fatalf("serve.bind = %q, want 100.110.196.103:18790", cfg.Serve.Bind)
	}
	if cfg.Serve.Token != "project-token" {
		t.Fatalf("serve.token = %q, want project-token", cfg.Serve.Token)
	}
	if len(cfg.Serve.Workspaces) != 1 || cfg.Serve.Workspaces["project"] != "/srv/project" {
		t.Fatalf("serve.workspaces = %#v, want {project:/srv/project}", cfg.Serve.Workspaces)
	}
	if cfg.Serve.NotificationURL != "http://run.example/hook" {
		t.Fatalf("serve.notification_url = %q, want http://run.example/hook", cfg.Serve.NotificationURL)
	}
}

func TestLoadConfigFiltersContextFilesToExternalRefs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	repoFile := filepath.Join(projectRoot, "README.md")
	if err := os.WriteFile(repoFile, []byte("repo\n"), 0o644); err != nil {
		t.Fatalf("write repo file: %v", err)
	}

	runRef := filepath.Join(projectRoot, ".goalx", "runs", "demo", "summary.md")
	if err := os.MkdirAll(filepath.Dir(runRef), 0o755); err != nil {
		t.Fatalf("mkdir run ref dir: %v", err)
	}
	if err := os.WriteFile(runRef, []byte("summary\n"), 0o644); err != nil {
		t.Fatalf("write run ref: %v", err)
	}

	externalRoot := t.TempDir()
	externalRef := filepath.Join(externalRoot, "spec.md")
	if err := os.WriteFile(externalRef, []byte("spec\n"), 0o644); err != nil {
		t.Fatalf("write external ref: %v", err)
	}

	cfg := Config{
		Name:      "demo",
		Mode:      ModeResearch,
		Objective: "investigate",
		Target: TargetConfig{
			Files: []string{"report.md"},
		},
		Harness: HarnessConfig{Command: "test -s report.md"},
		Context: ContextConfig{
			Files: []string{repoFile, runRef, externalRef},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	loaded, _, err := LoadConfig(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(loaded.Context.Files) != 2 {
		t.Fatalf("context.files = %#v, want 2 external refs", loaded.Context.Files)
	}
	if loaded.Context.Files[0] != runRef || loaded.Context.Files[1] != externalRef {
		t.Fatalf("context.files = %#v, want [%q %q]", loaded.Context.Files, runRef, externalRef)
	}
}

func TestValidateConfigRejectsOldCodexModels(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeDevelop,
		Objective: "test",
		Engine:    "codex",
		Model:     "gpt-5.2",
		Target:    TargetConfig{Files: []string{"src/"}},
		Harness:   HarnessConfig{Command: "go test"},
		Master:    MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	err := ValidateConfig(cfg, BuiltinEngines)
	if err == nil {
		t.Fatal("expected error for old codex model")
	}
	if !strings.Contains(err.Error(), "interactive migration prompt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeConfig(t *testing.T) {
	base := Config{
		Name:    "base",
		Mode:    ModeDevelop,
		Harness: HarnessConfig{Command: "make test"},
	}
	overlay := Config{
		Objective: "new objective",
		Harness:   HarnessConfig{Command: "go test"},
	}
	mergeConfig(&base, &overlay)
	if base.Name != "base" {
		t.Error("name should not be overridden by empty")
	}
	if base.Objective != "new objective" {
		t.Error("objective should be overridden")
	}
	if base.Harness.Command != "go test" {
		t.Error("harness should be overridden")
	}
}

func TestMergeConfigDescription(t *testing.T) {
	base := Config{}
	overlay := Config{Description: "agent context"}

	mergeConfig(&base, &overlay)

	if base.Description != "agent context" {
		t.Fatalf("Description = %q, want %q", base.Description, "agent context")
	}
}

func TestMergeConfigTargetFieldLevel(t *testing.T) {
	// base has Readonly, overlay has Files → both preserved
	base := Config{Target: TargetConfig{Readonly: []string{"pkg/"}}}
	overlay := Config{Target: TargetConfig{Files: []string{"."}}}
	mergeConfig(&base, &overlay)
	if len(base.Target.Files) != 1 || base.Target.Files[0] != "." {
		t.Errorf("Target.Files should be set from overlay, got %v", base.Target.Files)
	}
	if len(base.Target.Readonly) != 1 || base.Target.Readonly[0] != "pkg/" {
		t.Errorf("Target.Readonly should be preserved from base, got %v", base.Target.Readonly)
	}

	// overlay has Readonly, base has Files → both preserved
	base2 := Config{Target: TargetConfig{Files: []string{"src/"}}}
	overlay2 := Config{Target: TargetConfig{Readonly: []string{"vendor/"}}}
	mergeConfig(&base2, &overlay2)
	if len(base2.Target.Files) != 1 || base2.Target.Files[0] != "src/" {
		t.Errorf("Target.Files should be preserved from base, got %v", base2.Target.Files)
	}
	if len(base2.Target.Readonly) != 1 || base2.Target.Readonly[0] != "vendor/" {
		t.Errorf("Target.Readonly should be set from overlay, got %v", base2.Target.Readonly)
	}
}

func TestMergeConfigServeFieldLevel(t *testing.T) {
	base := Config{
		Serve: ServeConfig{
			Bind:            "100.110.196.103:18800",
			Token:           "base-token",
			Workspaces:      map[string]string{"goalx": "/srv/goalx"},
			NotificationURL: "https://hub.example/hooks/wake",
		},
	}
	overlay := Config{
		Serve: ServeConfig{
			Bind:       "100.110.196.103:18801",
			Workspaces: map[string]string{"quantos": "/srv/quantos"},
		},
	}

	mergeConfig(&base, &overlay)

	if base.Serve.Bind != "100.110.196.103:18801" {
		t.Fatalf("Serve.Bind = %q, want %q", base.Serve.Bind, "100.110.196.103:18801")
	}
	if base.Serve.Token != "base-token" {
		t.Fatalf("Serve.Token = %q, want preserved base token", base.Serve.Token)
	}
	if base.Serve.NotificationURL != "https://hub.example/hooks/wake" {
		t.Fatalf("Serve.NotificationURL = %q, want preserved base notification URL", base.Serve.NotificationURL)
	}
	if len(base.Serve.Workspaces) != 1 || base.Serve.Workspaces["quantos"] != "/srv/quantos" {
		t.Fatalf("Serve.Workspaces = %#v, want overlay workspaces", base.Serve.Workspaces)
	}
}

func TestLoadYAMLNotFound(t *testing.T) {
	cfg, err := LoadYAML[Config]("/nonexistent/path.yaml")
	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}
	if cfg.Name != "" {
		t.Error("expected zero value config")
	}
}

func TestLoadYAMLValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte("name: test-run\nmode: research\nobjective: do stuff\n"), 0644)

	cfg, err := LoadYAML[Config](path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test-run" || cfg.Mode != ModeResearch {
		t.Errorf("cfg = %+v", cfg)
	}
}
