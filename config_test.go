package goalx

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func loadResolvedConfigForTest(projectRoot, draftPath string) (*Config, map[string]EngineConfig, error) {
	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	req := ResolveRequest{}
	if draftPath != "" {
		if _, err := os.Stat(draftPath); err != nil {
			if os.IsNotExist(err) {
				return nil, nil, fmt.Errorf("manual draft config not found: %s", draftPath)
			}
			return nil, nil, fmt.Errorf("manual draft config: %w", err)
		}
		draft, err := LoadYAML[Config](draftPath)
		if err != nil {
			return nil, nil, fmt.Errorf("manual draft config: %w", err)
		}
		req.ManualDraft = &draft
	}
	resolved, err := ResolveConfigPreview(layers, req)
	if err != nil {
		return nil, nil, err
	}
	resolved.Config.Context.Files = FilterExternalContextFiles(projectRoot, resolved.Config.Context.Files)
	return &resolved.Config, resolved.Engines, nil
}

func loadRawSharedConfigForTest(projectRoot string) (*Config, map[string]EngineConfig, error) {
	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	cfg := layers.Config
	return &cfg, layers.Engines, nil
}

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
	if name != "gx-home-user-projects-myapp-event-sourcing" {
		t.Errorf("TmuxSessionName = %q", name)
	}
}

func TestLoadConfigLayersRejectsRemovedPresetAndRoutingFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte(`
preset: codex
routing:
  profiles:
    build_fast:
      engine: codex
      model: gpt-5.4-mini
`), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	if _, err := LoadConfigLayers(projectRoot); err == nil {
		t.Fatal("LoadConfigLayers unexpectedly accepted removed preset/routing fields")
	}
}

func TestBuiltinDefaultsIncludePreferencesAndDimensions(t *testing.T) {
	if got := BuiltinDimensions["audit"]; got == "" {
		t.Fatal("builtin audit dimension missing")
	}
	if got := BuiltinDefaults.Preferences.Worker.Guidance; got != "默认 gpt-5.4 medium。复杂分析、架构分歧或高风险收口可升到 high 或改用 opus；轻量切片用 fast。" {
		t.Fatalf("worker guidance = %q", got)
	}
	if got := BuiltinDefaults.Preferences.Simple.Guidance; got != "轻量任务用 fast。" {
		t.Fatalf("simple guidance = %q", got)
	}
}

func TestResolveSelectionUsesUserScopedPolicy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pathDir := t.TempDir()
	writeSelectionTestShim(t, pathDir, "claude")
	writeSelectionTestShim(t, pathDir, "codex")
	t.Setenv("PATH", pathDir)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
selection:
  disabled_targets:
    - claude-code/sonnet
  master_candidates:
    - claude-code/sonnet
    - codex/gpt-5.4
  worker_candidates:
    - codex/fast
  master_effort: high
  worker_effort: low
`) + "\n")
	if err := os.WriteFile(filepath.Join(userGoalxDir, "config.yaml"), userCfg, 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projectRoot := t.TempDir()
	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	resolved, err := ResolveConfig(layers, ResolveRequest{
		Name:                      "demo",
		Mode:                      ModeAuto,
		Objective:                 "ship it",
		RequireEngineAvailability: true,
		TargetOverride:            &TargetConfig{Files: []string{"README.md"}},
		LocalValidationOverride:   &LocalValidationConfig{Command: "go test ./..."},
	})
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}

	if resolved.Config.Master.Engine != "codex" || resolved.Config.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want codex/gpt-5.4", resolved.Config.Master.Engine, resolved.Config.Master.Model)
	}
	if resolved.Config.Master.Effort != EffortHigh {
		t.Fatalf("master effort = %q, want high", resolved.Config.Master.Effort)
	}
	if resolved.Config.Roles.Worker.Engine != "codex" || resolved.Config.Roles.Worker.Model != "fast" {
		t.Fatalf("worker = %s/%s, want codex/fast", resolved.Config.Roles.Worker.Engine, resolved.Config.Roles.Worker.Model)
	}
	if resolved.Config.Roles.Worker.Effort != EffortLow {
		t.Fatalf("worker effort = %q, want low", resolved.Config.Roles.Worker.Effort)
	}
}

func TestResolveConfigUsesImplicitSelectionDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pathDir := t.TempDir()
	writeSelectionTestShim(t, pathDir, "claude")
	writeSelectionTestShim(t, pathDir, "codex")
	t.Setenv("PATH", pathDir)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
target:
  files: ["README.md"]
local_validation:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	resolved, err := ResolveConfig(layers, ResolveRequest{
		Name:                      "demo",
		Mode:                      ModeAuto,
		Objective:                 "ship it",
		RequireEngineAvailability: true,
	})
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}

	if resolved.Config.Master.Engine != "codex" || resolved.Config.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want codex/gpt-5.4", resolved.Config.Master.Engine, resolved.Config.Master.Model)
	}
	if resolved.Config.Roles.Worker.Engine != "codex" || resolved.Config.Roles.Worker.Model != "gpt-5.4" {
		t.Fatalf("worker = %s/%s, want codex/gpt-5.4", resolved.Config.Roles.Worker.Engine, resolved.Config.Roles.Worker.Model)
	}
	if resolved.SelectionPolicy.MasterCandidates[0] != "codex/gpt-5.4" {
		t.Fatalf("master_candidates = %#v, want codex first", resolved.SelectionPolicy.MasterCandidates)
	}
}

func TestImplicitSelectionDefaultsFillMissingTargetsWithoutOverwritingLegacyValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pathDir := t.TempDir()
	writeSelectionTestShim(t, pathDir, "claude")
	writeSelectionTestShim(t, pathDir, "codex")
	t.Setenv("PATH", pathDir)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
master:
  effort: max
roles:
  worker:
    engine: claude-code
    model: sonnet
    effort: low
target:
  files: ["README.md"]
local_validation:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	resolved, err := ResolveConfig(layers, ResolveRequest{
		Name:                      "demo",
		Mode:                      ModeWorker,
		Objective:                 "ship it",
		RequireEngineAvailability: true,
	})
	if err != nil {
		t.Fatalf("ResolveConfig: %v", err)
	}

	if resolved.Config.Master.Engine != "codex" || resolved.Config.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want codex/gpt-5.4", resolved.Config.Master.Engine, resolved.Config.Master.Model)
	}
	if resolved.Config.Master.Effort != EffortMax {
		t.Fatalf("master effort = %q, want explicit max preserved", resolved.Config.Master.Effort)
	}
	if resolved.Config.Roles.Worker.Engine != "claude-code" || resolved.Config.Roles.Worker.Model != "sonnet" {
		t.Fatalf("worker = %s/%s, want explicit claude-code/sonnet preserved", resolved.Config.Roles.Worker.Engine, resolved.Config.Roles.Worker.Model)
	}
	if resolved.Config.Roles.Worker.Effort != EffortLow {
		t.Fatalf("worker effort = %q, want explicit low preserved", resolved.Config.Roles.Worker.Effort)
	}
}

func TestLoadConfigLayersRejectsProjectScopedSelection(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
selection:
  master_candidates:
    - codex/gpt-5.4
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	_, err := LoadConfigLayers(projectRoot)
	if err == nil || !strings.Contains(err.Error(), "project config: selection is only supported in ~/.goalx/config.yaml") {
		t.Fatalf("LoadConfigLayers error = %v, want project selection rejection", err)
	}
}

func TestResolveConfigPreviewRejectsSelectionInManualDraft(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}

	_, err = ResolveConfigPreview(layers, ResolveRequest{
		ManualDraft: &Config{
			Name:      "draft",
			Mode:      ModeWorker,
			Objective: "ship it",
			Selection: SelectionConfig{
				MasterCandidates: []string{"codex/gpt-5.4"},
			},
			Target:          TargetConfig{Files: []string{"README.md"}},
			LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "manual draft config: selection is only supported in ~/.goalx/config.yaml") {
		t.Fatalf("ResolveConfigPreview error = %v, want manual draft selection rejection", err)
	}
}

func TestResolveSelectionRejectsMalformedAndDuplicateTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pathDir := t.TempDir()
	writeSelectionTestShim(t, pathDir, "codex")
	t.Setenv("PATH", pathDir)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
selection:
  master_candidates:
    - codex
  worker_candidates:
    - codex/gpt-5.4
    - codex/gpt-5.4
`) + "\n")
	if err := os.WriteFile(filepath.Join(userGoalxDir, "config.yaml"), userCfg, 0o644); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projectRoot := t.TempDir()
	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	_, err = ResolveConfigPreview(layers, ResolveRequest{
		Name:                    "demo",
		Mode:                    ModeWorker,
		Objective:               "ship it",
		TargetOverride:          &TargetConfig{Files: []string{"README.md"}},
		LocalValidationOverride: &LocalValidationConfig{Command: "go test ./..."},
	})
	if err == nil || (!strings.Contains(err.Error(), "selection.master_candidates[0]") && !strings.Contains(err.Error(), "selection.worker_candidates contains duplicate")) {
		t.Fatalf("ResolveConfigPreview error = %v, want malformed or duplicate selection target", err)
	}
}

func writeSelectionTestShim(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write %s shim: %v", name, err)
	}
}

func TestLoadConfigUsesBuiltinPreferencesAndDimensionsWhenUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	cfg, _, err := loadResolvedConfigForTest(projectRoot, "")
	if err != nil {
		t.Fatalf("loadResolvedConfigForTest: %v", err)
	}
	if got := cfg.Preferences.Worker.Guidance; got != "默认 gpt-5.4 medium。复杂分析、架构分歧或高风险收口可升到 high 或改用 opus；轻量切片用 fast。" {
		t.Fatalf("worker guidance = %q", got)
	}
	if got := cfg.Preferences.Simple.Guidance; got != "轻量任务用 fast。" {
		t.Fatalf("simple guidance = %q", got)
	}
	if got := cfg.dimensionCatalog["audit"]; got == "" {
		t.Fatal("resolved config missing builtin audit dimension")
	}
}

func TestResolveLaunchSpecClaude(t *testing.T) {
	spec, err := ResolveLaunchSpec(BuiltinEngines, LaunchRequest{
		Engine: "claude-code",
		Model:  "opus",
		Effort: EffortHigh,
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Command != "claude --model claude-opus-4-6 --permission-mode auto --effort high" {
		t.Errorf("command = %q", spec.Command)
	}
	if spec.RequestedEffort != EffortHigh {
		t.Errorf("requested_effort = %q", spec.RequestedEffort)
	}
	if spec.EffectiveEffort != "high" {
		t.Errorf("effective_effort = %q", spec.EffectiveEffort)
	}
}

func TestResolveLaunchSpecUsesBypassPermissionsInSandbox(t *testing.T) {
	t.Setenv("IS_SANDBOX", "1")

	spec, err := ResolveLaunchSpec(BuiltinEngines, LaunchRequest{
		Engine: "claude-code",
		Model:  "opus",
		Effort: EffortMedium,
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Command != "claude --model claude-opus-4-6 --permission-mode bypassPermissions --effort medium" {
		t.Errorf("command = %q", spec.Command)
	}
}

func TestResolveLaunchSpecCodexMapsEffort(t *testing.T) {
	spec, err := ResolveLaunchSpec(BuiltinEngines, LaunchRequest{
		Engine: "codex",
		Model:  "gpt-5.4",
		Effort: EffortMax,
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Command != "codex -m gpt-5.4 -a never -s danger-full-access -c model_reasoning_effort=\"xhigh\"" {
		t.Errorf("command = %q", spec.Command)
	}
	if spec.EffectiveEffort != "xhigh" {
		t.Errorf("effective_effort = %q", spec.EffectiveEffort)
	}
}

func TestResolveLaunchSpecLiteralModel(t *testing.T) {
	spec, err := ResolveLaunchSpec(BuiltinEngines, LaunchRequest{
		Engine: "codex",
		Model:  "gpt-5.2",
		Effort: EffortLow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if spec.Command != "codex -m gpt-5.2 -a never -s danger-full-access -c model_reasoning_effort=\"low\"" {
		t.Errorf("literal model: command = %q", spec.Command)
	}
}

func TestResolveLaunchSpecUnknownEngine(t *testing.T) {
	_, err := ResolveLaunchSpec(BuiltinEngines, LaunchRequest{Engine: "unknown-engine", Model: "x"})
	if err == nil {
		t.Error("expected error for unknown engine")
	}
}

func TestResolvePrompt(t *testing.T) {
	claudePrompt := ResolvePrompt(BuiltinEngines, "claude-code", "/tmp/master.md")
	if claudePrompt != "Read /tmp/master.md and follow it exactly." {
		t.Errorf("claude prompt = %q", claudePrompt)
	}

	codexPrompt := ResolvePrompt(BuiltinEngines, "codex", "/tmp/master.md")
	if codexPrompt != "Read /tmp/master.md and follow it exactly. Execute protocol instructions by taking real tool actions in this turn; do not stop after stating intent." {
		t.Errorf("codex prompt = %q", codexPrompt)
	}
}

func TestExpandSessionsConfiguredSessions(t *testing.T) {
	cfg := Config{
		Sessions: []SessionConfig{
			{},
			{},
			{},
		},
	}
	sessions := ExpandSessions(&cfg)
	if len(sessions) != 3 {
		t.Fatalf("len = %d, want 3", len(sessions))
	}
	if sessions[0].Hint != "" || sessions[2].Hint != "" {
		t.Errorf("parallel expansion should not inject hints: %#v", sessions)
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
		Mode:      ModeWorker,
		Objective: "test objective",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		Master:          MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateConfigAcceptsAutoMode(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeAuto,
		Objective: "test objective",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		Master:          MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err != nil {
		t.Fatalf("expected auto mode to validate, got: %v", err)
	}
}

func TestResolveAcceptanceCommandDoesNotFallBackToLocalValidation(t *testing.T) {
	cfg := &Config{
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
	}

	if got := ResolveAcceptanceCommand(cfg); got != "" {
		t.Fatalf("ResolveAcceptanceCommand() = %q, want empty command", got)
	}
}

func TestResolveAcceptanceCommandUsesAcceptanceOverride(t *testing.T) {
	cfg := &Config{
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		Acceptance:      AcceptanceConfig{Command: "go test -run E2E ./..."},
	}

	if got := ResolveAcceptanceCommand(cfg); got != "go test -run E2E ./..." {
		t.Fatalf("ResolveAcceptanceCommand() = %q, want acceptance command", got)
	}
}

func TestValidateConfigMissingObjective(t *testing.T) {
	cfg := &Config{Name: "test", Mode: ModeWorker}
	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Error("expected error for missing objective")
	}
}

func TestValidateConfigPlaceholder(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeWorker,
		Objective: "test",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target:          TargetConfig{Files: []string{"TODO: specify"}},
		LocalValidation: LocalValidationConfig{Command: "go test"},
		Master:          MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Error("expected error for placeholder in target.files")
	}
}

func TestValidateConfigAllowsExplicitSessions(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeWorker,
		Objective: "test",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Sessions:        []SessionConfig{{Hint: "a"}},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test"},
		Master:          MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestValidateConfigRejectsAcceptancePlaceholder(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeWorker,
		Objective: "test objective",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		Acceptance: AcceptanceConfig{
			Command: "TODO: add e2e command",
		},
		Master: MasterConfig{Engine: "claude-code", Model: "opus"},
	}

	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Fatal("expected error for placeholder in acceptance.command")
	}
}

func TestLoadYAMLRejectsRemovedAndUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "serve block",
			content: strings.TrimSpace(`
name: demo
objective: ship it
target:
  files: [README.md]
local_validation:
  command: go test ./...
serve:
  bind: 127.0.0.1:9800
`) + "\n",
			want: "field serve",
		},
		{
			name: "notification_url field",
			content: strings.TrimSpace(`
name: demo
objective: ship it
target:
  files: [README.md]
local_validation:
  command: go test ./...
notification_url: https://example.invalid/hook
`) + "\n",
			want: "field notification_url",
		},
		{
			name: "unknown top level field",
			content: strings.TrimSpace(`
name: demo
objective: ship it
target:
  files: [README.md]
local_validation:
  command: go test ./...
unexpected: value
`) + "\n",
			want: "field unexpected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := LoadYAML[Config](path)
			if err == nil {
				t.Fatalf("LoadYAML unexpectedly succeeded for %s", tt.name)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("LoadYAML error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestLoadConfigMergesTopLevelUserPreferences(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
preferences:
  worker:
    guidance: multi-perspective
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
preferences:
  worker:
    guidance: speed
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
mode: worker
objective: ship it
target:
  files: [README.md]
local_validation:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	cfg, _, err := loadResolvedConfigForTest(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Preferences.Worker.Guidance != "speed" {
		t.Fatalf("worker guidance = %q, want speed", cfg.Preferences.Worker.Guidance)
	}
}

func TestLoadConfigReadsTopLevelSelectionAndEffort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
selection:
  master_candidates:
    - claude-code/opus
  worker_candidates:
    - codex/gpt-5.4-mini
  worker_effort: medium
master:
  engine: claude-code
  model: opus
  effort: high
roles:
  worker:
    engine: codex
    model: gpt-5.4
    effort: medium
preferences:
  worker:
    guidance: default to gpt-5.4 medium
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
local_validation:
  command: go build ./... && go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, _, err := loadResolvedConfigForTest(projectRoot, "")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" || cfg.Master.Effort != EffortHigh {
		t.Fatalf("master = %#v, want claude-code/opus/high", cfg.Master)
	}
	if cfg.Roles.Worker.Effort != EffortMedium {
		t.Fatalf("worker effort = %q, want medium", cfg.Roles.Worker.Effort)
	}
	if cfg.Preferences.Worker.Guidance != "default to gpt-5.4 medium" {
		t.Fatalf("worker guidance = %q, want top-level guidance", cfg.Preferences.Worker.Guidance)
	}
	if cfg.Roles.Worker.Engine != "codex" || cfg.Roles.Worker.Model != "gpt-5.4-mini" {
		t.Fatalf("worker = %#v, want codex/gpt-5.4-mini", cfg.Roles.Worker)
	}
	if cfg.LocalValidation.Command != "go build ./... && go test ./..." {
		t.Fatalf("local_validation.command = %q, want project local validation", cfg.LocalValidation.Command)
	}
}

func TestLoadConfigMergesLocalValidationTimeoutWithoutCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
local_validation:
  timeout: 30s
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, _, err := loadRawSharedConfigForTest(projectRoot)
	if err != nil {
		t.Fatalf("LoadRawBaseConfig: %v", err)
	}
	if cfg.LocalValidation.Timeout != 30*time.Second {
		t.Fatalf("local_validation.timeout = %v, want %v", cfg.LocalValidation.Timeout, 30*time.Second)
	}
}

func TestLoadConfigLayersRejectsLegacyParallelField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
parallel: 4
master:
  engine: codex
  model: fast
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	if _, err := LoadConfigLayers(projectRoot); err == nil || !strings.Contains(err.Error(), "parallel") {
		t.Fatalf("LoadConfigLayers error = %v, want legacy parallel rejection", err)
	}
}

func TestLoadConfigProjectDefaultsOverrideUserDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: claude-code
    model: sonnet
sessions:
  - hint: user-session
    engine: claude-code
    model: sonnet
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
master:
  engine: codex
  model: fast
roles:
  worker:
    engine: codex
    model: fast
sessions:
  - hint: project-session-1
    engine: codex
    model: best
  - hint: project-session-2
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
mode: worker
objective: ship it
target:
  files: [README.md]
local_validation:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	cfg, _, err := loadResolvedConfigForTest(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Roles.Worker.Engine != "codex" || cfg.Roles.Worker.Model != "fast" {
		t.Fatalf("worker role = %s/%s, want codex/fast", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "fast" {
		t.Fatalf("master engine/model = %s/%s, want codex/fast", cfg.Master.Engine, cfg.Master.Model)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 project sessions", cfg.Sessions)
	}
	if cfg.Sessions[0].Hint != "project-session-1" || cfg.Sessions[1].Hint != "project-session-2" {
		t.Fatalf("session hints = %#v, want project session hints", cfg.Sessions)
	}
}

func TestLoadConfigProjectEnvelopeOverridesUserEnvelope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
engines:
  codex:
    command: "codex --user {model_id}"
    prompt: "Read {protocol}"
    models:
      fast: user-fast
dimensions:
  architecture: "user architecture strategy"
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
engines:
  codex:
    command: "codex --project {model_id}"
    prompt: "Read {protocol}"
    models:
      fast: project-fast
dimensions:
  architecture: "project architecture strategy"
master:
  engine: codex
  model: fast
roles:
  worker:
    engine: codex
    model: fast
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
mode: worker
objective: ship it
target:
  files: [README.md]
local_validation:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	cfg, engines, err := loadResolvedConfigForTest(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "fast" {
		t.Fatalf("master engine/model = %s/%s, want codex/fast", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Worker.Engine != "codex" || cfg.Roles.Worker.Model != "fast" {
		t.Fatalf("develop role = %s/%s, want codex/fast", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
	if engines["codex"].Command != "codex --project {model_id}" {
		t.Fatalf("engines[codex].command = %q, want project override", engines["codex"].Command)
	}
	if engines["codex"].Models["fast"] != "project-fast" {
		t.Fatalf("engines[codex].models[fast] = %q, want project-fast", engines["codex"].Models["fast"])
	}
	if layers.Dimensions["architecture"] != "project architecture strategy" {
		t.Fatalf("layers.dimensions[architecture] = %q, want project override", layers.Dimensions["architecture"])
	}
	if _, ok := BuiltinDimensions["architecture"]; ok {
		t.Fatalf("global BuiltinDimensions leaked architecture entry")
	}
}

func TestLoadConfigLayersSequentialProjectsKeepCatalogsLocal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}

	projectA := t.TempDir()
	projectAGoalxDir := filepath.Join(projectA, ".goalx")
	if err := os.MkdirAll(projectAGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project A config dir: %v", err)
	}
	projectACfg := []byte(strings.TrimSpace(`
engines:
  codex:
    command: "codex --project-a {model_id}"
    prompt: "Read {protocol}"
    models:
      fast: project-a-fast
dimensions:
  architecture: "project A architecture"
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectAGoalxDir, "config.yaml"), projectACfg, 0o644); err != nil {
		t.Fatalf("write project A config: %v", err)
	}

	projectB := t.TempDir()
	projectBGoalxDir := filepath.Join(projectB, ".goalx")
	if err := os.MkdirAll(projectBGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project B config dir: %v", err)
	}
	projectBCfg := []byte(strings.TrimSpace(`
engines:
  codex:
    command: "codex --project-b {model_id}"
    prompt: "Read {protocol}"
    models:
      fast: project-b-fast
dimensions:
  architecture: "project B architecture"
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectBGoalxDir, "config.yaml"), projectBCfg, 0o644); err != nil {
		t.Fatalf("write project B config: %v", err)
	}

	layersA, err := LoadConfigLayers(projectA)
	if err != nil {
		t.Fatalf("LoadConfigLayers(projectA): %v", err)
	}
	layersB, err := LoadConfigLayers(projectB)
	if err != nil {
		t.Fatalf("LoadConfigLayers(projectB): %v", err)
	}

	if got := layersA.Dimensions["architecture"]; got != "project A architecture" {
		t.Fatalf("layersA.dimensions[architecture] = %q, want project A architecture", got)
	}
	if got := layersA.Engines["codex"].Command; got != "codex --project-a {model_id}" {
		t.Fatalf("layersA.engines[codex].command = %q, want project A override", got)
	}

	if got := layersB.Dimensions["architecture"]; got != "project B architecture" {
		t.Fatalf("layersB.dimensions[architecture] = %q, want project B architecture", got)
	}
	if got := layersB.Engines["codex"].Command; got != "codex --project-b {model_id}" {
		t.Fatalf("layersB.engines[codex].command = %q, want project B override", got)
	}
}

func TestLoadConfigDoesNotMutateSharedCatalogs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	origEngines := copyEngines(BuiltinEngines)
	origDimensions := cloneStringMap(BuiltinDimensions)
	t.Cleanup(func() {
		BuiltinEngines = origEngines
		BuiltinDimensions = origDimensions
	})

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
engines:
  codex:
    command: "codex --local {model_id}"
    prompt: "Read {protocol}"
    models:
      fast: local-fast
dimensions:
  reliability: "reliability: focus on stable behavior"
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
name: demo
mode: worker
objective: lock config state
target:
  files: [README.md]
local_validation:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	if _, _, err := loadResolvedConfigForTest(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml")); err != nil {
		t.Fatalf("LoadConfigWithManualDraft: %v", err)
	}

	if !reflect.DeepEqual(BuiltinEngines, origEngines) {
		t.Fatalf("BuiltinEngines mutated across config load: got %#v, want %#v", BuiltinEngines, origEngines)
	}
	if !reflect.DeepEqual(BuiltinDimensions, origDimensions) {
		t.Fatalf("BuiltinDimensions mutated across config load: got %#v, want %#v", BuiltinDimensions, origDimensions)
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
		Mode:      ModeWorker,
		Objective: "investigate",
		Target: TargetConfig{
			Files: []string{"report.md"},
		},
		LocalValidation: LocalValidationConfig{Command: "test -s report.md"},
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

	loaded, _, err := loadResolvedConfigForTest(projectRoot, filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
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
		Mode:      ModeWorker,
		Objective: "test",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "codex", Model: "gpt-5.2"},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test"},
		Master:          MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	err := ValidateConfig(cfg, BuiltinEngines)
	if err == nil {
		t.Fatal("expected error for old codex model")
	}
	if !strings.Contains(err.Error(), "interactive migration prompt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigRejectsCrossEngineModelAlias(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeWorker,
		Objective: "test",
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "codex", Model: "codex"},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test"},
		Master:          MasterConfig{Engine: "codex", Model: "opus"},
	}

	err := ValidateConfig(cfg, BuiltinEngines)
	if err == nil {
		t.Fatal("expected error for cross-engine model alias")
	}
	if !strings.Contains(err.Error(), `model alias "opus"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeConfig(t *testing.T) {
	base := Config{
		Name:            "base",
		Mode:            ModeWorker,
		LocalValidation: LocalValidationConfig{Command: "make test"},
	}
	overlay := Config{
		Objective:       "new objective",
		LocalValidation: LocalValidationConfig{Command: "go test"},
	}
	mergeConfig(&base, &overlay)
	if base.Name != "base" {
		t.Error("name should not be overridden by empty")
	}
	if base.Objective != "new objective" {
		t.Error("objective should be overridden")
	}
	if base.LocalValidation.Command != "go test" {
		t.Error("local_validation should be overridden")
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

func TestMergeConfigPreferencesFieldLevel(t *testing.T) {
	base := Config{
		Preferences: PreferencesConfig{
			Worker: PreferencePolicy{Guidance: "multi-perspective"},
			Simple: PreferencePolicy{Guidance: "keep it simple"},
		},
	}
	overlay := Config{
		Preferences: PreferencesConfig{
			Worker: PreferencePolicy{Guidance: "speed"},
		},
	}

	mergeConfig(&base, &overlay)

	if base.Preferences.Worker.Guidance != "speed" {
		t.Fatalf("Worker.Guidance = %q, want overlay guidance", base.Preferences.Worker.Guidance)
	}
	if base.Preferences.Simple.Guidance != "keep it simple" {
		t.Fatalf("Simple.Guidance = %q, want preserved base guidance", base.Preferences.Simple.Guidance)
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
	os.WriteFile(path, []byte("name: test-run\nmode: worker\nobjective: do stuff\n"), 0644)

	cfg, err := LoadYAML[Config](path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test-run" || cfg.Mode != ModeWorker {
		t.Errorf("cfg = %+v", cfg)
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func TestEffectiveSessionConfigOverridesRunDefaults(t *testing.T) {
	cfg := &Config{
		Mode:            ModeWorker,
		Roles:           RoleDefaultsConfig{Worker: SessionConfig{Engine: "claude-code", Model: "opus"}},
		Target:          TargetConfig{Files: []string{"src/"}, Readonly: []string{"vendor/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		Sessions: []SessionConfig{
			{
				Hint:            "investigate root cause",
				Mode:            ModeWorker,
				Target:          &TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}},
				LocalValidation: &LocalValidationConfig{Command: "test -s report.md"},
			},
		},
	}

	got := EffectiveSessionConfig(cfg, 0)
	if got.Mode != ModeWorker {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeWorker)
	}
	if !reflect.DeepEqual(got.Target.Files, []string{"report.md"}) {
		t.Fatalf("Target.Files = %#v, want report target", got.Target.Files)
	}
	if got.LocalValidation.Command != "test -s report.md" {
		t.Fatalf("LocalValidation.Command = %q, want session local validation override", got.LocalValidation.Command)
	}
	if got.Engine != "claude-code" || got.Model != "opus" {
		t.Fatalf("Engine/Model = %s/%s, want worker defaults", got.Engine, got.Model)
	}
}

func TestEffectiveSessionConfigInheritsRunDefaults(t *testing.T) {
	cfg := &Config{
		Mode:            ModeWorker,
		Roles:           RoleDefaultsConfig{Worker: SessionConfig{Engine: "codex", Model: "fast"}},
		Target:          TargetConfig{Files: []string{"src/"}, Readonly: []string{"vendor/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
		Sessions: []SessionConfig{
			{Hint: "implement fix"},
		},
	}

	got := EffectiveSessionConfig(cfg, 0)
	if got.Mode != "" {
		t.Fatalf("Mode = %q, want unset mode", got.Mode)
	}
	if !reflect.DeepEqual(got.Target.Files, []string{"src/"}) {
		t.Fatalf("Target.Files = %#v, want inherited run target", got.Target.Files)
	}
	if got.LocalValidation.Command != "go test ./..." {
		t.Fatalf("LocalValidation.Command = %q, want inherited run local validation", got.LocalValidation.Command)
	}
	if got.Engine != "codex" || got.Model != "fast" {
		t.Fatalf("Engine/Model = %s/%s, want inherited worker defaults", got.Engine, got.Model)
	}
}

func TestEffectiveSessionConfigUsesWorkerRoleDefaultsForAllSessions(t *testing.T) {
	cfg := &Config{
		Mode: ModeWorker,
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "codex", Model: "fast"},
		},
		Sessions: []SessionConfig{
			{Mode: ModeWorker},
			{Mode: ModeWorker},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
	}

	gotFirst := EffectiveSessionConfig(cfg, 0)
	gotSecond := EffectiveSessionConfig(cfg, 1)

	if gotFirst.Engine != "codex" || gotFirst.Model != "fast" {
		t.Fatalf("first session = %s/%s", gotFirst.Engine, gotFirst.Model)
	}
	if gotSecond.Engine != "codex" || gotSecond.Model != "fast" {
		t.Fatalf("second session = %s/%s", gotSecond.Engine, gotSecond.Model)
	}
}

func TestEffectiveSessionConfigAppliesWorkerDefaultsInAutoRuns(t *testing.T) {
	cfg := &Config{
		Mode: ModeAuto,
		Roles: RoleDefaultsConfig{
			Worker: SessionConfig{Engine: "codex", Model: "fast"},
		},
		Target:          TargetConfig{Files: []string{"src/"}},
		LocalValidation: LocalValidationConfig{Command: "go test ./..."},
	}

	got := EffectiveSessionConfig(cfg, 0)

	if got.Mode != "" {
		t.Fatalf("Mode = %q, want unset mode", got.Mode)
	}
	if got.Engine != "codex" || got.Model != "fast" {
		t.Fatalf("Engine/Model = %s/%s, want worker defaults", got.Engine, got.Model)
	}
}

func TestEffectiveSessionConfigLeavesEngineUnsetWithoutWorkerDefaults(t *testing.T) {
	cfg := &Config{
		Mode:   ModeWorker,
		Target: TargetConfig{Files: []string{"report.md"}},
		LocalValidation: LocalValidationConfig{
			Command: "test -s report.md",
		},
	}

	got := EffectiveSessionConfig(cfg, 0)
	if got.Engine != "" || got.Model != "" {
		t.Fatalf("role default session = %s/%s, want unset engine/model", got.Engine, got.Model)
	}
}
