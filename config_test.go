package autoresearch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"event sourcing", "event-sourcing"},
		{"/data/dev/quantos", "data-dev-quantos"},
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
	id := ProjectID("/data/dev/quantos")
	if id != "data-dev-quantos" {
		t.Errorf("ProjectID = %q, want 'data-dev-quantos'", id)
	}
}

func TestRunDir(t *testing.T) {
	dir := RunDir("/data/dev/quantos", "event-sourcing")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".autoresearch", "runs", "data-dev-quantos", "event-sourcing")
	if dir != want {
		t.Errorf("RunDir = %q, want %q", dir, want)
	}
}

func TestTmuxSessionName(t *testing.T) {
	name := TmuxSessionName("/data/dev/quantos", "event-sourcing")
	if name != "goalx-data-dev-quantos-event-sourcing" {
		t.Errorf("TmuxSessionName = %q", name)
	}
}

func TestPresetDefault(t *testing.T) {
	cfg := Config{Preset: "default", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Errorf("master = %s/%s, want claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Engine != "codex" || cfg.Model != "codex" {
		t.Errorf("develop = %s/%s, want codex/codex", cfg.Engine, cfg.Model)
	}
}

func TestPresetResearch(t *testing.T) {
	cfg := Config{Preset: "default", Mode: ModeResearch}
	applyPreset(&cfg)
	if cfg.Engine != "claude-code" || cfg.Model != "sonnet" {
		t.Errorf("research = %s/%s, want claude-code/sonnet", cfg.Engine, cfg.Model)
	}
}

func TestPresetTurbo(t *testing.T) {
	cfg := Config{Preset: "turbo", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Master.Model != "sonnet" {
		t.Errorf("turbo master model = %q, want sonnet", cfg.Master.Model)
	}
	if cfg.Model != "fast" {
		t.Errorf("turbo develop model = %q, want fast", cfg.Model)
	}
}

func TestPresetDeep(t *testing.T) {
	cfg := Config{Preset: "deep", Mode: ModeDevelop}
	applyPreset(&cfg)
	if cfg.Model != "best" {
		t.Errorf("deep develop model = %q, want best", cfg.Model)
	}
}

func TestPresetNoOverrideExplicit(t *testing.T) {
	cfg := Config{Preset: "default", Mode: ModeDevelop, Engine: "aider", Model: "opus"}
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
	if cmd != "claude --model claude-opus-4-6 --permission-mode auto --disable-slash-commands" {
		t.Errorf("cmd = %q", cmd)
	}
}

func TestResolveEngineCommandCodex(t *testing.T) {
	cmd, err := ResolveEngineCommand(BuiltinEngines, "codex", "codex")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "codex -m gpt-5.4 --full-auto" {
		t.Errorf("cmd = %q", cmd)
	}
}

func TestResolveEngineCommandLiteralModel(t *testing.T) {
	cmd, err := ResolveEngineCommand(BuiltinEngines, "codex", "gpt-5.2")
	if err != nil {
		t.Fatal(err)
	}
	if cmd != "codex -m gpt-5.2 --full-auto" {
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
