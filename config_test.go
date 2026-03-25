package goalx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	if name != "gx-home-user-projects-myapp-event-sourcing" {
		t.Errorf("TmuxSessionName = %q", name)
	}
}

func TestPresetClaude(t *testing.T) {
	cfg := Config{Preset: "claude", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Errorf("master = %s/%s, want claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "gpt-5.4" {
		t.Errorf("develop role = %s/%s, want codex/gpt-5.4", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
}

func TestPresetClaudeResearch(t *testing.T) {
	cfg := Config{Preset: "claude", Mode: ModeResearch}
	applyPreset(&cfg)
	if cfg.Roles.Research.Engine != "claude-code" || cfg.Roles.Research.Model != "sonnet" {
		t.Errorf("research role = %s/%s, want claude-code/sonnet", cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	}
}

func TestPresetClaudeH(t *testing.T) {
	cfg := Config{Preset: "claude-h", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Model != "opus" {
		t.Errorf("claude-h master model = %q, want opus", cfg.Master.Model)
	}
	if cfg.Roles.Develop.Engine != "claude-code" || cfg.Roles.Develop.Model != "opus" {
		t.Errorf("claude-h develop role = %s/%s, want claude-code/opus", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
}

func TestPresetCodex(t *testing.T) {
	cfg := Config{Preset: "codex", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "gpt-5.4" {
		t.Errorf("codex master = %s/%s, want codex/gpt-5.4", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "gpt-5.4" {
		t.Errorf("codex develop role = %s/%s, want codex/gpt-5.4", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
}

func TestPresetMixed(t *testing.T) {
	cfg := Config{Preset: "mixed", Mode: ModeResearch}
	applyPreset(&cfg)
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "gpt-5.4" {
		t.Errorf("mixed master = %s/%s, want codex/gpt-5.4", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Research.Engine != "claude-code" || cfg.Roles.Research.Model != "opus" {
		t.Errorf("mixed research role = %s/%s, want claude-code/opus", cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	}
}

func TestPresetNoOverrideExplicit(t *testing.T) {
	cfg := Config{
		Preset: "claude",
		Mode:   ModeDevelop,
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "aider", Model: "opus"},
		},
	}
	applyPreset(&cfg)
	if cfg.Roles.Develop.Engine != "aider" || cfg.Roles.Develop.Model != "opus" {
		t.Errorf("explicit should not be overridden: %s/%s", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
}

func TestApplyPresetFillsRoleDefaults(t *testing.T) {
	cfg := Config{Preset: "hybrid", Mode: ModeResearch}
	ApplyPreset(&cfg)
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Fatalf("master = %s/%s", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Research.Engine != "claude-code" || cfg.Roles.Research.Model != "opus" {
		t.Fatalf("research role = %s/%s", cfg.Roles.Research.Engine, cfg.Roles.Research.Model)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "gpt-5.4" {
		t.Fatalf("develop role = %s/%s", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
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
	p := ResolvePrompt(BuiltinEngines, "claude-code", "/tmp/master.md")
	if p != "Read /tmp/master.md and follow it exactly." {
		t.Errorf("prompt = %q", p)
	}
}

func TestExpandSessions(t *testing.T) {
	cfg := Config{Parallel: 3}
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
		Mode:      ModeDevelop,
		Objective: "test objective",
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
		Master:  MasterConfig{Engine: "claude-code", Model: "opus"},
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
			Develop: SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
		Master:  MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err != nil {
		t.Fatalf("expected auto mode to validate, got: %v", err)
	}
}

func TestResolveAcceptanceCommandFallsBackToHarness(t *testing.T) {
	cfg := &Config{
		Harness: HarnessConfig{Command: "go test ./..."},
	}

	if got := ResolveAcceptanceCommand(cfg); got != "go test ./..." {
		t.Fatalf("ResolveAcceptanceCommand() = %q, want harness command", got)
	}
}

func TestResolveAcceptanceCommandUsesAcceptanceOverride(t *testing.T) {
	cfg := &Config{
		Harness:    HarnessConfig{Command: "go test ./..."},
		Acceptance: AcceptanceConfig{Command: "go test -run E2E ./..."},
	}

	if got := ResolveAcceptanceCommand(cfg); got != "go test -run E2E ./..." {
		t.Fatalf("ResolveAcceptanceCommand() = %q, want acceptance command", got)
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
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target:  TargetConfig{Files: []string{"TODO: specify"}},
		Harness: HarnessConfig{Command: "go test"},
		Master:  MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Error("expected error for placeholder in target.files")
	}
}

func TestValidateConfigAllowsSessionOverridesWithParallel(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeDevelop,
		Objective: "test",
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Parallel: 2,
		Sessions: []SessionConfig{{Hint: "a"}},
		Target:   TargetConfig{Files: []string{"src/"}},
		Harness:  HarnessConfig{Command: "go test"},
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
	}
	if err := ValidateConfig(cfg, BuiltinEngines); err != nil {
		t.Fatalf("ValidateConfig: %v", err)
	}
}

func TestValidateConfigRejectsAcceptancePlaceholder(t *testing.T) {
	cfg := &Config{
		Name:      "test",
		Mode:      ModeDevelop,
		Objective: "test objective",
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
		Acceptance: AcceptanceConfig{
			Command: "TODO: add e2e command",
		},
		Master: MasterConfig{Engine: "claude-code", Model: "opus"},
	}

	if err := ValidateConfig(cfg, BuiltinEngines); err == nil {
		t.Fatal("expected error for placeholder in acceptance.command")
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

	cfg, _, err := LoadConfigWithManualDraft(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
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

func TestLoadConfigMergesTopLevelUserPreferences(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
preferences:
  research:
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
  develop:
    guidance: speed
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
mode: develop
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	cfg, _, err := LoadConfigWithManualDraft(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Preferences.Research.Guidance != "multi-perspective" {
		t.Fatalf("research guidance = %q, want multi-perspective", cfg.Preferences.Research.Guidance)
	}
	if cfg.Preferences.Develop.Guidance != "speed" {
		t.Fatalf("develop guidance = %q, want speed", cfg.Preferences.Develop.Guidance)
	}
}

func TestLoadConfigReadsTopLevelRoutingAndEffort(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
preset: codex
master:
  engine: claude-code
  model: opus
  effort: high
roles:
  research:
    engine: codex
    model: gpt-5.4
    effort: high
  develop:
    engine: codex
    model: gpt-5.4
    effort: medium
routing:
  profiles:
    research_deep: { engine: claude-code, model: opus, effort: high }
    build_fast: { engine: codex, model: gpt-5.4-mini, effort: minimal }
  table:
    research: { medium: research_deep }
    develop: { low: build_fast }
preferences:
  research:
    guidance: default to gpt-5.4 high
  develop:
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
harness:
  command: go build ./... && go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, _, err := LoadConfig(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Preset != "codex" {
		t.Fatalf("preset = %q, want codex", cfg.Preset)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" || cfg.Master.Effort != EffortHigh {
		t.Fatalf("master = %#v, want claude-code/opus/high", cfg.Master)
	}
	if cfg.Roles.Research.Effort != EffortHigh {
		t.Fatalf("research effort = %q, want high", cfg.Roles.Research.Effort)
	}
	if cfg.Roles.Develop.Effort != EffortMedium {
		t.Fatalf("develop effort = %q, want medium", cfg.Roles.Develop.Effort)
	}
	if cfg.Preferences.Research.Guidance != "default to gpt-5.4 high" {
		t.Fatalf("research guidance = %q, want top-level guidance", cfg.Preferences.Research.Guidance)
	}
	if cfg.Preferences.Develop.Guidance != "default to gpt-5.4 medium" {
		t.Fatalf("develop guidance = %q, want top-level guidance", cfg.Preferences.Develop.Guidance)
	}
	if got := cfg.Routing.Profiles["research_deep"]; got.Engine != "claude-code" || got.Model != "opus" || got.Effort != EffortHigh {
		t.Fatalf("research_deep = %#v, want claude-code/opus/high", got)
	}
	if got := cfg.Routing.Table["develop"]["low"]; got != "build_fast" {
		t.Fatalf("routing.table.develop.low = %q, want build_fast", got)
	}
	if cfg.Harness.Command != "go build ./... && go test ./..." {
		t.Fatalf("harness.command = %q, want project harness", cfg.Harness.Command)
	}
}

func TestLoadConfigMergesHarnessTimeoutWithoutCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	projectGoalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(projectGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir project config dir: %v", err)
	}
	projectCfg := []byte(strings.TrimSpace(`
harness:
  timeout: 30s
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	cfg, _, err := LoadRawBaseConfig(projectRoot)
	if err != nil {
		t.Fatalf("LoadRawBaseConfig: %v", err)
	}
	if cfg.Harness.Timeout != 30*time.Second {
		t.Fatalf("harness.timeout = %v, want %v", cfg.Harness.Timeout, 30*time.Second)
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
parallel: 2
master:
  engine: claude-code
  model: opus
roles:
  develop:
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
parallel: 4
master:
  engine: codex
  model: fast
roles:
  develop:
    engine: codex
    model: fast
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
mode: develop
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	cfg, _, err := LoadConfigWithManualDraft(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "fast" {
		t.Fatalf("develop role = %s/%s, want codex/fast", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "fast" {
		t.Fatalf("master engine/model = %s/%s, want codex/fast", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", cfg.Parallel)
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
presets:
  project-dev:
    master: {engine: claude-code, model: opus}
    develop: {engine: claude-code, model: sonnet}
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
presets:
  project-dev:
    master: {engine: codex, model: fast}
    develop: {engine: codex, model: fast}
dimensions:
  architecture: "project architecture strategy"
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "config.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	runCfg := []byte(strings.TrimSpace(`
name: demo
mode: develop
preset: project-dev
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), runCfg, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	layers, err := LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	cfg, engines, err := LoadConfigWithManualDraft(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml"))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "fast" {
		t.Fatalf("master engine/model = %s/%s, want codex/fast", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "fast" {
		t.Fatalf("develop role = %s/%s, want codex/fast", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
	if engines["codex"].Command != "codex --project {model_id}" {
		t.Fatalf("engines[codex].command = %q, want project override", engines["codex"].Command)
	}
	if engines["codex"].Models["fast"] != "project-fast" {
		t.Fatalf("engines[codex].models[fast] = %q, want project-fast", engines["codex"].Models["fast"])
	}
	if layers.Presets["project-dev"].Master.Engine != "codex" || layers.Presets["project-dev"].Master.Model != "fast" {
		t.Fatalf("layers.presets[project-dev].master = %#v, want codex/fast", layers.Presets["project-dev"].Master)
	}
	if layers.Dimensions["architecture"] != "project architecture strategy" {
		t.Fatalf("layers.dimensions[architecture] = %q, want project override", layers.Dimensions["architecture"])
	}
	if _, ok := Presets["project-dev"]; ok {
		t.Fatalf("global Presets leaked project-dev entry")
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
presets:
  project-a:
    master: {engine: codex, model: fast}
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
presets:
  project-b:
    master: {engine: claude-code, model: opus}
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

	if got := layersA.Presets["project-a"].Master.Engine; got != "codex" {
		t.Fatalf("layersA.presets[project-a].master.engine = %q, want codex", got)
	}
	if _, ok := layersA.Presets["project-b"]; ok {
		t.Fatalf("layersA.presets unexpectedly contains project-b")
	}
	if got := layersA.Dimensions["architecture"]; got != "project A architecture" {
		t.Fatalf("layersA.dimensions[architecture] = %q, want project A architecture", got)
	}
	if got := layersA.Engines["codex"].Command; got != "codex --project-a {model_id}" {
		t.Fatalf("layersA.engines[codex].command = %q, want project A override", got)
	}

	if got := layersB.Presets["project-b"].Master.Engine; got != "claude-code" {
		t.Fatalf("layersB.presets[project-b].master.engine = %q, want claude-code", got)
	}
	if _, ok := layersB.Presets["project-a"]; ok {
		t.Fatalf("layersB.presets unexpectedly contains project-a")
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

	origPresets := clonePresetMap(Presets)
	origDimensions := cloneStringMap(BuiltinDimensions)
	t.Cleanup(func() {
		Presets = origPresets
		BuiltinDimensions = origDimensions
	})

	userGoalxDir := filepath.Join(home, ".goalx")
	if err := os.MkdirAll(userGoalxDir, 0o755); err != nil {
		t.Fatalf("mkdir user config dir: %v", err)
	}
	userCfg := []byte(strings.TrimSpace(`
presets:
  local-research:
    master: {engine: claude-code, model: opus}
    research: {engine: claude-code, model: opus}
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
mode: develop
objective: lock config state
target:
  files: [README.md]
harness:
  command: go test ./...
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectGoalxDir, "goalx.yaml"), projectCfg, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	if _, _, err := LoadConfigWithManualDraft(projectRoot, filepath.Join(projectGoalxDir, "goalx.yaml")); err != nil {
		t.Fatalf("LoadConfigWithManualDraft: %v", err)
	}

	if !reflect.DeepEqual(Presets, origPresets) {
		t.Fatalf("Presets mutated across config load: got %#v, want %#v", Presets, origPresets)
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

	loaded, _, err := LoadConfigWithManualDraft(projectRoot, filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
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
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "codex", Model: "gpt-5.2"},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test"},
		Master:  MasterConfig{Engine: "claude-code", Model: "opus"},
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
		Mode:      ModeDevelop,
		Objective: "test",
		Roles: RoleDefaultsConfig{
			Develop: SessionConfig{Engine: "codex", Model: "codex"},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test"},
		Master:  MasterConfig{Engine: "codex", Model: "opus"},
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

func TestMergeConfigPreferencesFieldLevel(t *testing.T) {
	base := Config{
		Preferences: PreferencesConfig{
			Research: PreferencePolicy{Guidance: "multi-perspective"},
			Simple:   PreferencePolicy{Guidance: "keep it simple"},
		},
	}
	overlay := Config{
		Preferences: PreferencesConfig{
			Develop: PreferencePolicy{Guidance: "speed"},
		},
	}

	mergeConfig(&base, &overlay)

	if base.Preferences.Research.Guidance != "multi-perspective" {
		t.Fatalf("Research.Guidance = %q, want preserved base guidance", base.Preferences.Research.Guidance)
	}
	if base.Preferences.Develop.Guidance != "speed" {
		t.Fatalf("Develop.Guidance = %q, want overlay guidance", base.Preferences.Develop.Guidance)
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
	os.WriteFile(path, []byte("name: test-run\nmode: research\nobjective: do stuff\n"), 0644)

	cfg, err := LoadYAML[Config](path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "test-run" || cfg.Mode != ModeResearch {
		t.Errorf("cfg = %+v", cfg)
	}
}

func clonePresetMap(src map[string]PresetConfig) map[string]PresetConfig {
	dst := make(map[string]PresetConfig, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
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
		Mode:    ModeDevelop,
		Roles:   RoleDefaultsConfig{Research: SessionConfig{Engine: "claude-code", Model: "opus"}},
		Target:  TargetConfig{Files: []string{"src/"}, Readonly: []string{"vendor/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
		Sessions: []SessionConfig{
			{
				Hint:    "investigate root cause",
				Mode:    ModeResearch,
				Target:  &TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}},
				Harness: &HarnessConfig{Command: "test -s report.md"},
			},
		},
	}

	got := EffectiveSessionConfig(cfg, 0)
	if got.Mode != ModeResearch {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeResearch)
	}
	if !reflect.DeepEqual(got.Target.Files, []string{"report.md"}) {
		t.Fatalf("Target.Files = %#v, want report target", got.Target.Files)
	}
	if got.Harness.Command != "test -s report.md" {
		t.Fatalf("Harness.Command = %q, want research harness", got.Harness.Command)
	}
}

func TestEffectiveSessionConfigInheritsRunDefaults(t *testing.T) {
	cfg := &Config{
		Mode:    ModeDevelop,
		Roles:   RoleDefaultsConfig{Develop: SessionConfig{Engine: "codex", Model: "fast"}},
		Target:  TargetConfig{Files: []string{"src/"}, Readonly: []string{"vendor/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
		Sessions: []SessionConfig{
			{Hint: "implement fix"},
		},
	}

	got := EffectiveSessionConfig(cfg, 0)
	if got.Mode != ModeDevelop {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeDevelop)
	}
	if !reflect.DeepEqual(got.Target.Files, []string{"src/"}) {
		t.Fatalf("Target.Files = %#v, want inherited run target", got.Target.Files)
	}
	if got.Harness.Command != "go test ./..." {
		t.Fatalf("Harness.Command = %q, want inherited run harness", got.Harness.Command)
	}
	if got.Engine != "codex" || got.Model != "fast" {
		t.Fatalf("Engine/Model = %s/%s, want codex/fast", got.Engine, got.Model)
	}
}

func TestEffectiveSessionConfigUsesModeSpecificRoleDefaults(t *testing.T) {
	cfg := &Config{
		Mode: ModeDevelop,
		Roles: RoleDefaultsConfig{
			Research: SessionConfig{Engine: "claude-code", Model: "opus"},
			Develop:  SessionConfig{Engine: "codex", Model: "fast"},
		},
		Sessions: []SessionConfig{
			{Mode: ModeResearch},
			{Mode: ModeDevelop},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
	}

	gotResearch := EffectiveSessionConfig(cfg, 0)
	gotDevelop := EffectiveSessionConfig(cfg, 1)

	if gotResearch.Engine != "claude-code" || gotResearch.Model != "opus" {
		t.Fatalf("research session = %s/%s", gotResearch.Engine, gotResearch.Model)
	}
	if gotDevelop.Engine != "codex" || gotDevelop.Model != "fast" {
		t.Fatalf("develop session = %s/%s", gotDevelop.Engine, gotDevelop.Model)
	}
}

func TestEffectiveSessionConfigDefaultsAutoRunsToDevelopMode(t *testing.T) {
	cfg := &Config{
		Mode: ModeAuto,
		Roles: RoleDefaultsConfig{
			Research: SessionConfig{Engine: "claude-code", Model: "opus"},
			Develop:  SessionConfig{Engine: "codex", Model: "fast"},
		},
		Target:  TargetConfig{Files: []string{"src/"}},
		Harness: HarnessConfig{Command: "go test ./..."},
	}

	got := EffectiveSessionConfig(cfg, 0)

	if got.Mode != ModeDevelop {
		t.Fatalf("Mode = %q, want %q", got.Mode, ModeDevelop)
	}
	if got.Engine != "codex" || got.Model != "fast" {
		t.Fatalf("Engine/Model = %s/%s, want codex/fast", got.Engine, got.Model)
	}
}

func TestEffectiveSessionConfigUsesRoleDefaults(t *testing.T) {
	cfg := &Config{
		Mode: ModeResearch,
		Roles: RoleDefaultsConfig{
			Research: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		},
		Target: TargetConfig{Files: []string{"report.md"}},
		Harness: HarnessConfig{
			Command: "test -s report.md",
		},
	}

	got := EffectiveSessionConfig(cfg, 0)
	if got.Engine != "claude-code" || got.Model != "sonnet" {
		t.Fatalf("role default session = %s/%s", got.Engine, got.Model)
	}
}
