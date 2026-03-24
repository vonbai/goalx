package goalx

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Mode is the run mode: research, develop, or auto.
type Mode string

const (
	ModeResearch Mode = "research"
	ModeDevelop  Mode = "develop"
	ModeAuto     Mode = "auto"
)

// Config is the merged configuration for a single run.
type Config struct {
	Name        string            `yaml:"name"`
	Mode        Mode              `yaml:"mode"`
	Objective   string            `yaml:"objective"`
	Description string            `yaml:"description,omitempty"`
	Preset      string            `yaml:"preset,omitempty"`
	Preferences PreferencesConfig `yaml:"preferences,omitempty"`
	// Deprecated: use Roles.Research/Develop instead.
	Engine string `yaml:"engine,omitempty"` // legacy fallback for session defaults
	// Deprecated: use Roles.Research/Develop instead.
	Model          string             `yaml:"model,omitempty"` // legacy fallback for session defaults
	Roles          RoleDefaultsConfig `yaml:"roles,omitempty"`
	Parallel       int                `yaml:"parallel,omitempty"`
	DiversityHints []string           `yaml:"diversity_hints,omitempty"`
	Sessions       []SessionConfig    `yaml:"sessions,omitempty"`
	Target         TargetConfig       `yaml:"target"`
	Harness        HarnessConfig      `yaml:"harness,omitempty"`
	Acceptance     AcceptanceConfig   `yaml:"acceptance,omitempty"`
	Context        ContextConfig      `yaml:"context,omitempty"`
	Budget         BudgetConfig       `yaml:"budget,omitempty"`
	Master         MasterConfig       `yaml:"master,omitempty"`
	Serve          ServeConfig        `yaml:"serve,omitempty"`
}

type TargetConfig struct {
	Files    []string `yaml:"files"`
	Readonly []string `yaml:"readonly,omitempty"`
}

type HarnessConfig struct {
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

type AcceptanceConfig struct {
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

type ContextConfig struct {
	Files []string `yaml:"files,omitempty"` // external file refs only
	Refs  []string `yaml:"refs,omitempty"`  // any: paths, URLs, notes
}

type BudgetConfig struct {
	MaxDuration time.Duration `yaml:"max_duration,omitempty"`
	MaxRounds   int           `yaml:"max_rounds,omitempty"`
}

type MasterConfig struct {
	Engine        string        `yaml:"engine,omitempty"`
	Model         string        `yaml:"model,omitempty"`
	CheckInterval time.Duration `yaml:"check_interval,omitempty"`
}

type ServeConfig struct {
	Bind            string            `yaml:"bind,omitempty"`
	Token           string            `yaml:"token,omitempty"`
	Workspaces      map[string]string `yaml:"workspaces,omitempty"`
	NotificationURL string            `yaml:"notification_url,omitempty"`
}

type SessionConfig struct {
	Hint    string         `yaml:"hint,omitempty"`
	Engine  string         `yaml:"engine,omitempty"`
	Model   string         `yaml:"model,omitempty"`
	Mode    Mode           `yaml:"mode,omitempty"`
	Target  *TargetConfig  `yaml:"target,omitempty"`
	Harness *HarnessConfig `yaml:"harness,omitempty"`
}

type PreferencesConfig struct {
	Research PreferencePolicy `yaml:"research,omitempty"`
	Develop  PreferencePolicy `yaml:"develop,omitempty"`
	Simple   PreferencePolicy `yaml:"simple,omitempty"`
}

type PreferencePolicy struct {
	Engines  []string `yaml:"engines,omitempty"`
	Strategy string   `yaml:"strategy,omitempty"`
}

type RoleDefaultsConfig struct {
	Research SessionConfig `yaml:"research,omitempty"`
	Develop  SessionConfig `yaml:"develop,omitempty"`
}

// EngineConfig defines how to launch an AI engine.
type EngineConfig struct {
	Description string            `yaml:"description,omitempty"`
	Command     string            `yaml:"command"`
	Prompt      string            `yaml:"prompt"`
	Models      map[string]string `yaml:"models"`
}

// PresetConfig defines engine/model for master, research, and develop roles.
type PresetConfig struct {
	Master   MasterConfig  `yaml:"master,omitempty"`
	Research SessionConfig `yaml:"research,omitempty"`
	Develop  SessionConfig `yaml:"develop,omitempty"`
}

// UserConfig is the top-level user config file (~/.goalx/config.yaml).
type UserConfig struct {
	Defaults    Config                  `yaml:"defaults,omitempty"`
	Engines     map[string]EngineConfig `yaml:"engines,omitempty"`
	Preferences PreferencesConfig       `yaml:"preferences,omitempty"`
	Serve       ServeConfig             `yaml:"serve,omitempty"`
	Presets     map[string]PresetConfig `yaml:"presets,omitempty"`
	Strategies  map[string]string       `yaml:"strategies,omitempty"`
}

// Presets — named engine/model combinations for different workflows.
var Presets = map[string]PresetConfig{
	"claude": {
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
		Research: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		Develop:  SessionConfig{Engine: "codex", Model: "codex"},
	},
	"claude-h": {
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
		Research: SessionConfig{Engine: "claude-code", Model: "opus"},
		Develop:  SessionConfig{Engine: "claude-code", Model: "opus"},
	},
	"codex": {
		Master:   MasterConfig{Engine: "codex", Model: "codex"},
		Research: SessionConfig{Engine: "codex", Model: "codex"},
		Develop:  SessionConfig{Engine: "codex", Model: "codex"},
	},
	"mixed": {
		Master:   MasterConfig{Engine: "codex", Model: "codex"},
		Research: SessionConfig{Engine: "claude-code", Model: "opus"},
		Develop:  SessionConfig{Engine: "codex", Model: "codex"},
	},
	"hybrid": {
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
		Research: SessionConfig{Engine: "claude-code", Model: "opus"},
		Develop:  SessionConfig{Engine: "codex", Model: "codex"},
	},
}

// BuiltinEngines are the default engine definitions.
var BuiltinEngines = map[string]EngineConfig{
	"claude-code": {
		Description: "Deep reasoning and long-form research synthesis.",
		Command:     "claude --model {model_id} --permission-mode auto",
		Prompt:      "Read {protocol} and follow it exactly.",
		Models: map[string]string{
			"opus":   "claude-opus-4-6",
			"sonnet": "claude-sonnet-4-6",
			"haiku":  "claude-haiku-4-5",
		},
	},
	"codex": {
		Description: "Fast code editing, testing, and implementation work.",
		Command:     "codex -m {model_id} -a never -s danger-full-access",
		Prompt:      "Read {protocol} and follow it exactly.",
		Models: map[string]string{
			"codex":    "gpt-5.4",
			"best":     "gpt-5.4",
			"balanced": "gpt-5.4",
			"fast":     "gpt-5.4-mini",
		},
	},
	"aider": {
		Description: "Interactive multi-file editing with explicit diffs.",
		Command:     "aider --model {model_id} --no-auto-commits --yes",
		Prompt:      "/read {protocol}",
		Models: map[string]string{
			"opus":   "claude-opus-4-6",
			"sonnet": "claude-sonnet-4-6",
		},
	},
}

// BuiltinDefaults are the hardcoded default values.
var BuiltinDefaults = Config{
	Preset:   "codex",
	Mode:     ModeDevelop,
	Parallel: 1,
	Master: MasterConfig{
		CheckInterval: 2 * time.Minute,
	},
	Budget: BudgetConfig{
		MaxDuration: 8 * time.Hour,
	},
}

// LoadYAML loads a config file, returning zero value if not found.
func LoadYAML[T any](path string) (T, error) {
	var cfg T
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func loadBaseConfigRaw(projectRoot string) (Config, map[string]EngineConfig, error) {
	cfg := BuiltinDefaults
	engines := copyEngines(BuiltinEngines)

	// Layer 2: user config
	home, _ := os.UserHomeDir()
	userCfg, err := LoadYAML[UserConfig](filepath.Join(home, ".goalx", "config.yaml"))
	if err != nil {
		return Config{}, nil, fmt.Errorf("user config: %w", err)
	}
	applyConfigEnvelope(&cfg, &engines, &userCfg)

	// Layer 3: project config
	projectConfigPath := filepath.Join(projectRoot, ".goalx", "config.yaml")
	projEnvelope, err := LoadYAML[UserConfig](projectConfigPath)
	if err != nil {
		return Config{}, nil, fmt.Errorf("project config: %w", err)
	}
	applyConfigEnvelope(&cfg, &engines, &projEnvelope)
	projCfg, err := LoadYAML[Config](projectConfigPath)
	if err != nil {
		return Config{}, nil, fmt.Errorf("project config: %w", err)
	}
	mergeConfig(&cfg, &projCfg)
	cfg.Context.Files = filterExternalContextFiles(projectRoot, cfg.Context.Files)

	return cfg, engines, nil
}

func applyConfigEnvelope(cfg *Config, engines *map[string]EngineConfig, overlay *UserConfig) {
	if cfg == nil || overlay == nil {
		return
	}
	mergeConfig(cfg, &overlay.Defaults)
	mergePreferencesConfig(&cfg.Preferences, &overlay.Preferences)
	mergeServeConfig(&cfg.Serve, &overlay.Serve)
	if engines != nil {
		for k, v := range overlay.Engines {
			(*engines)[k] = v
		}
	}
	for k, v := range overlay.Presets {
		Presets[k] = v
	}
	for k, v := range overlay.Strategies {
		BuiltinStrategies[k] = v
	}
}

func finalizeLoadedConfig(projectRoot string, cfg Config) *Config {
	cfg.Context.Files = filterExternalContextFiles(projectRoot, cfg.Context.Files)

	// Apply preset to fill in engine/model gaps
	applyPreset(&cfg)

	// Apply defaults for parallel
	if cfg.Parallel < 1 {
		cfg.Parallel = 1
	}

	return &cfg
}

// LoadRawBaseConfig loads built-in, user, and project config layers without
// applying preset-derived engine/model defaults.
func LoadRawBaseConfig(projectRoot string) (*Config, map[string]EngineConfig, error) {
	cfg, engines, err := loadBaseConfigRaw(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	return &cfg, engines, nil
}

// LoadConfig loads and merges shared config layers for the current project.
func LoadConfig(projectRoot string) (*Config, map[string]EngineConfig, error) {
	cfg, engines, err := loadBaseConfigRaw(projectRoot)
	if err != nil {
		return nil, nil, err
	}

	return finalizeLoadedConfig(projectRoot, cfg), engines, nil
}

// LoadConfigWithManualDraft loads shared config layers, then overlays an
// explicit manual draft config file such as .goalx/goalx.yaml.
func LoadConfigWithManualDraft(projectRoot, draftPath string) (*Config, map[string]EngineConfig, error) {
	cfg, engines, err := loadBaseConfigRaw(projectRoot)
	if err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(draftPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("manual draft config not found: %s", draftPath)
		}
		return nil, nil, fmt.Errorf("manual draft config: %w", err)
	}

	runCfg, err := LoadYAML[Config](draftPath)
	if err != nil {
		return nil, nil, fmt.Errorf("manual draft config: %w", err)
	}
	mergeConfig(&cfg, &runCfg)

	return finalizeLoadedConfig(projectRoot, cfg), engines, nil
}

// applyPreset fills in engine/model from the preset if not already set.
func applyPreset(cfg *Config) {
	preset, ok := Presets[cfg.Preset]
	if !ok {
		preset = Presets["codex"]
	}

	// Master
	if cfg.Master.Engine == "" {
		cfg.Master.Engine = preset.Master.Engine
	}
	if cfg.Master.Model == "" {
		cfg.Master.Model = preset.Master.Model
	}

	// Role defaults
	if cfg.Roles.Research.Engine == "" {
		cfg.Roles.Research.Engine = preset.Research.Engine
	}
	if cfg.Roles.Research.Model == "" {
		cfg.Roles.Research.Model = preset.Research.Model
	}
	if cfg.Roles.Develop.Engine == "" {
		cfg.Roles.Develop.Engine = preset.Develop.Engine
	}
	if cfg.Roles.Develop.Model == "" {
		cfg.Roles.Develop.Model = preset.Develop.Model
	}

	// Legacy run-level fallback remains readable for existing configs/specs.
	legacy := defaultRoleSession(cfg, cfg.Mode)
	if cfg.Engine == "" {
		cfg.Engine = legacy.Engine
	}
	if cfg.Model == "" {
		cfg.Model = legacy.Model
	}
}

// ApplyPreset fills missing engine/model fields from the selected preset.
func ApplyPreset(cfg *Config) {
	applyPreset(cfg)
}

// ValidateConfig checks the config before creating any side effects.
func ValidateConfig(cfg *Config, engines map[string]EngineConfig) error {
	if cfg.Objective == "" {
		return fmt.Errorf("objective is required")
	}
	if cfg.Mode != ModeResearch && cfg.Mode != ModeDevelop && cfg.Mode != ModeAuto {
		return fmt.Errorf("mode must be 'research', 'develop', or 'auto', got %q", cfg.Mode)
	}
	if cfg.Name == "" {
		return fmt.Errorf("name is required (use --name or let goalx init generate one)")
	}

	// Check target
	if len(cfg.Target.Files) == 0 {
		return fmt.Errorf("target.files is required")
	}
	for _, f := range cfg.Target.Files {
		if strings.HasPrefix(f, "TODO") {
			return fmt.Errorf("target.files contains placeholder %q — edit the manual draft config first", f)
		}
	}

	// Check harness
	if cfg.Harness.Command == "" || strings.HasPrefix(cfg.Harness.Command, "TODO") {
		return fmt.Errorf("harness.command is required (set in the explicit manual draft config or .goalx/config.yaml)")
	}
	if cmd := strings.TrimSpace(cfg.Acceptance.Command); cmd != "" && strings.HasPrefix(cmd, "TODO") {
		return fmt.Errorf("acceptance.command contains placeholder %q — edit the manual draft config first", cfg.Acceptance.Command)
	}

	// Check engine/model can resolve and won't block on known interactive prompts.
	if err := validateInteractiveEngine(engines, cfg.Master.Engine, cfg.Master.Model, "master"); err != nil {
		return err
	}
	sessions := ExpandSessions(cfg)
	if len(sessions) == 0 {
		sessions = []SessionConfig{{}}
	}
	for i, sess := range sessions {
		if sess.Mode != "" && sess.Mode != ModeResearch && sess.Mode != ModeDevelop {
			return fmt.Errorf("session-%d mode must be 'research' or 'develop', got %q", i+1, sess.Mode)
		}
		engine := sess.Engine
		if engine == "" {
			engine = cfg.Engine
		}
		model := sess.Model
		if model == "" {
			model = cfg.Model
		}
		if err := validateInteractiveEngine(engines, engine, model, fmt.Sprintf("session-%d", i+1)); err != nil {
			return err
		}
	}

	// Check sessions vs parallel
	if len(cfg.Sessions) > 0 && cfg.Parallel > 1 {
		return fmt.Errorf("cannot use both 'sessions' and 'parallel > 1' — pick one")
	}
	if len(cfg.DiversityHints) > 0 && len(cfg.Sessions) > 0 {
		return fmt.Errorf("cannot use both 'diversity_hints' and 'sessions' — pick one")
	}

	return nil
}

// ResolveEngineCommand builds the final shell command for an engine+model.
func ResolveEngineCommand(engines map[string]EngineConfig, engine, model string) (string, error) {
	ec, ok := engines[engine]
	if !ok {
		return "", fmt.Errorf("unknown engine %q", engine)
	}
	modelID, err := resolveModelID(engines, engine, model)
	if err != nil {
		return "", err
	}
	cmd := strings.ReplaceAll(ec.Command, "{model_id}", modelID)
	if engine == "claude-code" && os.Getenv("IS_SANDBOX") == "1" {
		cmd = strings.ReplaceAll(cmd, "--permission-mode auto", "--permission-mode bypassPermissions")
	}
	return cmd, nil
}

func resolveModelID(engines map[string]EngineConfig, engine, model string) (string, error) {
	ec, ok := engines[engine]
	if !ok {
		return "", fmt.Errorf("unknown engine %q", engine)
	}
	modelID, ok := ec.Models[model]
	if !ok {
		modelID = model
	}
	return modelID, nil
}

func validateInteractiveEngine(engines map[string]EngineConfig, engine, model, role string) error {
	if err := validateModelAliasForEngine(engines, engine, model, role); err != nil {
		return err
	}
	if _, err := ResolveEngineCommand(engines, engine, model); err != nil {
		return fmt.Errorf("%s engine: %w", role, err)
	}
	modelID, err := resolveModelID(engines, engine, model)
	if err != nil {
		return fmt.Errorf("%s engine: %w", role, err)
	}
	if engine == "codex" && (modelID == "gpt-5.3-codex" || modelID == "gpt-5.2") {
		return fmt.Errorf("%s engine: codex model %q resolves to %s, which triggers an interactive migration prompt in Codex CLI; use gpt-5.4 or gpt-5.4-mini instead", role, model, modelID)
	}
	return nil
}

func validateModelAliasForEngine(engines map[string]EngineConfig, engine, model, role string) error {
	if model == "" {
		return nil
	}
	ec, ok := engines[engine]
	if !ok {
		return nil
	}
	if _, ok := ec.Models[model]; ok {
		return nil
	}
	if ModelAliasBelongsToOtherEngine(engines, engine, model) {
		return fmt.Errorf("%s engine: model alias %q is not valid for engine %q", role, model, engine)
	}
	return nil
}

// ModelAliasBelongsToOtherEngine reports whether model is a known alias for an engine other than engine.
func ModelAliasBelongsToOtherEngine(engines map[string]EngineConfig, engine, model string) bool {
	ec, ok := engines[engine]
	if !ok {
		return false
	}
	if _, ok := ec.Models[model]; ok {
		return false
	}
	for otherEngine, other := range engines {
		if otherEngine == engine {
			continue
		}
		if _, ok := other.Models[model]; ok {
			return true
		}
	}
	return false
}

// ResolvePrompt builds the prompt for an engine, substituting {protocol}.
func ResolvePrompt(engines map[string]EngineConfig, engine, protocolPath string) string {
	ec, ok := engines[engine]
	if !ok {
		return fmt.Sprintf("Read %s and follow it exactly.", protocolPath)
	}
	return strings.ReplaceAll(ec.Prompt, "{protocol}", protocolPath)
}

// ExpandSessions converts parallel+diversity_hints into explicit session configs.
func ExpandSessions(cfg *Config) []SessionConfig {
	if len(cfg.Sessions) > 0 {
		return cfg.Sessions
	}
	sessions := make([]SessionConfig, cfg.Parallel)
	for i := range sessions {
		if i < len(cfg.DiversityHints) {
			sessions[i].Hint = cfg.DiversityHints[i]
		}
	}
	return sessions
}

// EffectiveSessionConfig resolves a session against run-level defaults.
func EffectiveSessionConfig(cfg *Config, idx int) SessionConfig {
	var out SessionConfig
	if cfg == nil {
		return out
	}

	sessions := ExpandSessions(cfg)
	if idx >= 0 && idx < len(sessions) {
		out = sessions[idx]
	}
	out.Mode = ResolveSessionMode(cfg.Mode, out.Mode)
	roleDefault := defaultRoleSession(cfg, out.Mode)
	if out.Engine == "" {
		out.Engine = roleDefault.Engine
		if out.Engine == "" {
			out.Engine = cfg.Engine
		}
	}
	if out.Model == "" {
		out.Model = roleDefault.Model
		if out.Model == "" {
			out.Model = cfg.Model
		}
	}

	target := cfg.Target
	if out.Target != nil {
		if len(out.Target.Files) > 0 {
			target.Files = append([]string(nil), out.Target.Files...)
		}
		if len(out.Target.Readonly) > 0 {
			target.Readonly = append([]string(nil), out.Target.Readonly...)
		}
	}
	out.Target = &target

	harness := cfg.Harness
	if out.Harness != nil {
		if out.Harness.Command != "" {
			harness.Command = out.Harness.Command
		}
		if out.Harness.Timeout > 0 {
			harness.Timeout = out.Harness.Timeout
		}
	}
	out.Harness = &harness

	return out
}

// ResolveSessionMode normalizes a session mode against the enclosing run mode.
// Sessions only run in research or develop mode. Auto runs default sessions to
// develop until the master chooses a specific session mode.
func ResolveSessionMode(runMode, sessionMode Mode) Mode {
	switch sessionMode {
	case ModeResearch, ModeDevelop:
		return sessionMode
	}
	if runMode == ModeResearch {
		return ModeResearch
	}
	return ModeDevelop
}

func defaultRoleSession(cfg *Config, mode Mode) SessionConfig {
	if cfg == nil {
		return SessionConfig{}
	}
	switch mode {
	case ModeResearch:
		return cfg.Roles.Research
	case ModeDevelop:
		return cfg.Roles.Develop
	default:
		if cfg.Mode == ModeResearch {
			return cfg.Roles.Research
		}
		return cfg.Roles.Develop
	}
}

// ResolveAcceptanceCommand returns the acceptance command, falling back to the
// regular harness when no dedicated acceptance command is configured.
func ResolveAcceptanceCommand(cfg *Config) string {
	cmd, _ := ResolveAcceptanceCommandSource(cfg)
	return cmd
}

// ResolveAcceptanceCommandSource returns the effective acceptance command plus
// the config field it came from ("acceptance" or "harness").
func ResolveAcceptanceCommandSource(cfg *Config) (string, string) {
	if cfg == nil {
		return "", ""
	}
	if cmd := strings.TrimSpace(cfg.Acceptance.Command); cmd != "" {
		return cmd, "acceptance"
	}
	if cmd := strings.TrimSpace(cfg.Harness.Command); cmd != "" {
		return cmd, "harness"
	}
	return "", ""
}

// ProjectID returns a slug from the project root path.
func ProjectID(projectRoot string) string {
	abs, _ := filepath.Abs(projectRoot)
	s := strings.TrimPrefix(abs, "/")
	return slugify(s)
}

// RunDir returns the run directory: ~/.goalx/runs/{projectID}/{name}
func RunDir(projectRoot, name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".goalx", "runs", ProjectID(projectRoot), name)
}

// TmuxSessionName returns the tmux session name for a run.
func TmuxSessionName(projectRoot, name string) string {
	return "gx-" + ProjectID(projectRoot) + "-" + slugify(name)
}

// Slugify generates a URL-safe slug from a string.
func Slugify(s string) string {
	return slugify(s)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.TrimRight(s, "-")
	}
	return s
}

func mergeConfig(base, overlay *Config) {
	if overlay.Name != "" {
		base.Name = overlay.Name
	}
	if overlay.Mode != "" {
		base.Mode = overlay.Mode
	}
	if overlay.Objective != "" {
		base.Objective = overlay.Objective
	}
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if overlay.Preset != "" {
		base.Preset = overlay.Preset
	}
	mergePreferencesConfig(&base.Preferences, &overlay.Preferences)
	if overlay.Engine != "" {
		base.Engine = overlay.Engine
	}
	if overlay.Model != "" {
		base.Model = overlay.Model
	}
	if overlay.Roles.Research.Engine != "" {
		base.Roles.Research.Engine = overlay.Roles.Research.Engine
	}
	if overlay.Roles.Research.Model != "" {
		base.Roles.Research.Model = overlay.Roles.Research.Model
	}
	if overlay.Roles.Develop.Engine != "" {
		base.Roles.Develop.Engine = overlay.Roles.Develop.Engine
	}
	if overlay.Roles.Develop.Model != "" {
		base.Roles.Develop.Model = overlay.Roles.Develop.Model
	}
	if overlay.Parallel > 0 {
		base.Parallel = overlay.Parallel
	}
	if len(overlay.DiversityHints) > 0 {
		base.DiversityHints = overlay.DiversityHints
	}
	if len(overlay.Sessions) > 0 {
		base.Sessions = overlay.Sessions
	}
	if len(overlay.Target.Files) > 0 {
		base.Target.Files = overlay.Target.Files
	}
	if len(overlay.Target.Readonly) > 0 {
		base.Target.Readonly = overlay.Target.Readonly
	}
	if overlay.Harness.Command != "" {
		base.Harness = overlay.Harness
	}
	if overlay.Acceptance.Command != "" {
		base.Acceptance.Command = overlay.Acceptance.Command
	}
	if overlay.Acceptance.Timeout > 0 {
		base.Acceptance.Timeout = overlay.Acceptance.Timeout
	}
	if len(overlay.Context.Files) > 0 || len(overlay.Context.Refs) > 0 {
		base.Context = overlay.Context
	}
	if overlay.Budget.MaxDuration > 0 {
		base.Budget.MaxDuration = overlay.Budget.MaxDuration
	}
	if overlay.Budget.MaxRounds > 0 {
		base.Budget.MaxRounds = overlay.Budget.MaxRounds
	}
	if overlay.Master.Engine != "" {
		base.Master.Engine = overlay.Master.Engine
	}
	if overlay.Master.Model != "" {
		base.Master.Model = overlay.Master.Model
	}
	if overlay.Master.CheckInterval > 0 {
		base.Master.CheckInterval = overlay.Master.CheckInterval
	}
	if overlay.Serve.Bind != "" {
		base.Serve.Bind = overlay.Serve.Bind
	}
	if overlay.Serve.Token != "" {
		base.Serve.Token = overlay.Serve.Token
	}
	if len(overlay.Serve.Workspaces) > 0 {
		base.Serve.Workspaces = overlay.Serve.Workspaces
	}
	if overlay.Serve.NotificationURL != "" {
		base.Serve.NotificationURL = overlay.Serve.NotificationURL
	}
}

func mergePreferencesConfig(base, overlay *PreferencesConfig) {
	mergePreferencePolicy(&base.Research, overlay.Research)
	mergePreferencePolicy(&base.Develop, overlay.Develop)
	mergePreferencePolicy(&base.Simple, overlay.Simple)
}

func mergeServeConfig(base, overlay *ServeConfig) {
	if base == nil || overlay == nil {
		return
	}
	if overlay.Bind != "" {
		base.Bind = overlay.Bind
	}
	if overlay.Token != "" {
		base.Token = overlay.Token
	}
	if len(overlay.Workspaces) > 0 {
		base.Workspaces = overlay.Workspaces
	}
	if overlay.NotificationURL != "" {
		base.NotificationURL = overlay.NotificationURL
	}
}

func mergePreferencePolicy(base *PreferencePolicy, overlay PreferencePolicy) {
	if len(overlay.Engines) > 0 {
		base.Engines = overlay.Engines
	}
	if overlay.Strategy != "" {
		base.Strategy = overlay.Strategy
	}
}

func copyEngines(src map[string]EngineConfig) map[string]EngineConfig {
	dst := make(map[string]EngineConfig, len(src))
	for k, v := range src {
		models := make(map[string]string, len(v.Models))
		for mk, mv := range v.Models {
			models[mk] = mv
		}
		v.Models = models
		dst[k] = v
	}
	return dst
}

func filterExternalContextFiles(projectRoot string, files []string) []string {
	if len(files) == 0 {
		return nil
	}

	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		absRoot = filepath.Clean(projectRoot)
	}
	runRoot := filepath.Join(absRoot, ".goalx", "runs")

	var filtered []string
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		if strings.TrimSpace(file) == "" {
			continue
		}

		normalized := file
		if filepath.IsAbs(file) {
			normalized = filepath.Clean(file)
		} else {
			normalized = filepath.Join(absRoot, file)
		}

		if pathWithin(normalized, absRoot) && !pathWithin(normalized, runRoot) {
			continue
		}
		if seen[normalized] {
			continue
		}
		filtered = append(filtered, normalized)
		seen[normalized] = true
	}
	return filtered
}

func pathWithin(path, root string) bool {
	path = filepath.Clean(path)
	root = filepath.Clean(root)
	if path == root {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}
