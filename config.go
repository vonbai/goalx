package goalx

import (
	"fmt"
	"os"
	"os/exec"
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
	Name        string             `yaml:"name"`
	Mode        Mode               `yaml:"mode"`
	Objective   string             `yaml:"objective"`
	Description string             `yaml:"description,omitempty"`
	Preset      string             `yaml:"preset,omitempty"`
	Preferences PreferencesConfig  `yaml:"preferences,omitempty"`
	Roles       RoleDefaultsConfig `yaml:"roles,omitempty"`
	Routing     RoutingTableConfig `yaml:"routing,omitempty"`
	Parallel    int                `yaml:"parallel,omitempty"`
	Sessions    []SessionConfig    `yaml:"sessions,omitempty"`
	Target      TargetConfig       `yaml:"target"`
	Harness     HarnessConfig      `yaml:"harness,omitempty"`
	Acceptance  AcceptanceConfig   `yaml:"acceptance,omitempty"`
	Context     ContextConfig      `yaml:"context,omitempty"`
	Budget      BudgetConfig       `yaml:"budget,omitempty"`
	Master      MasterConfig       `yaml:"master,omitempty"`
	Serve       ServeConfig        `yaml:"serve,omitempty"`

	presetCatalog    map[string]PresetConfig
	dimensionCatalog map[string]string
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
	Effort        EffortLevel   `yaml:"effort,omitempty"`
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
	Effort  EffortLevel    `yaml:"effort,omitempty"`
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
	Guidance string `yaml:"guidance,omitempty"`
}

type RoleDefaultsConfig struct {
	Research SessionConfig `yaml:"research,omitempty"`
	Develop  SessionConfig `yaml:"develop,omitempty"`
}

type EffortLevel string

const (
	EffortAuto    EffortLevel = "auto"
	EffortMinimal EffortLevel = "minimal"
	EffortLow     EffortLevel = "low"
	EffortMedium  EffortLevel = "medium"
	EffortHigh    EffortLevel = "high"
	EffortMax     EffortLevel = "max"
)

type ExecutionProfile struct {
	Engine string      `yaml:"engine,omitempty"`
	Model  string      `yaml:"model,omitempty"`
	Effort EffortLevel `yaml:"effort,omitempty"`
}

type RoutingTableConfig struct {
	Profiles map[string]ExecutionProfile  `yaml:"profiles,omitempty"`
	Table    map[string]map[string]string `yaml:"table,omitempty"`
}

// EngineConfig defines how to launch an AI engine.
type EngineConfig struct {
	Description string            `yaml:"description,omitempty"`
	Command     string            `yaml:"command"`
	Prompt      string            `yaml:"prompt"`
	Models      map[string]string `yaml:"models"`
	EffortMode  string            `yaml:"effort_mode,omitempty"`
	EffortFlag  string            `yaml:"effort_flag,omitempty"`
	EffortKey   string            `yaml:"effort_key,omitempty"`
	EffortMap   map[string]string `yaml:"effort_map,omitempty"`
}

// PresetConfig defines engine/model for master, research, and develop roles.
type PresetConfig struct {
	Master   MasterConfig  `yaml:"master,omitempty"`
	Research SessionConfig `yaml:"research,omitempty"`
	Develop  SessionConfig `yaml:"develop,omitempty"`
}

// UserConfig is the top-level user config file (~/.goalx/config.yaml).
type UserConfig struct {
	Config     `yaml:",inline"`
	Engines    map[string]EngineConfig `yaml:"engines,omitempty"`
	Presets    map[string]PresetConfig `yaml:"presets,omitempty"`
	Dimensions map[string]string       `yaml:"dimensions,omitempty"`
}

// Presets — named engine/model combinations for different workflows.
var Presets = map[string]PresetConfig{
	"claude": {
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
		Research: SessionConfig{Engine: "claude-code", Model: "sonnet"},
		Develop:  SessionConfig{Engine: "codex", Model: "gpt-5.4"},
	},
	"claude-h": {
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
		Research: SessionConfig{Engine: "claude-code", Model: "opus"},
		Develop:  SessionConfig{Engine: "claude-code", Model: "opus"},
	},
	"codex": {
		Master:   MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Research: SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		Develop:  SessionConfig{Engine: "codex", Model: "gpt-5.4"},
	},
	"mixed": {
		Master:   MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Research: SessionConfig{Engine: "claude-code", Model: "opus"},
		Develop:  SessionConfig{Engine: "codex", Model: "gpt-5.4"},
	},
	"hybrid": {
		Master:   MasterConfig{Engine: "claude-code", Model: "opus"},
		Research: SessionConfig{Engine: "claude-code", Model: "opus"},
		Develop:  SessionConfig{Engine: "codex", Model: "gpt-5.4"},
	},
}

// BuiltinEngines are the default engine definitions.
var BuiltinEngines = map[string]EngineConfig{
	"claude-code": {
		Description: "Deep reasoning and long-form research synthesis.",
		Command:     "claude --model {model_id} --permission-mode auto",
		Prompt:      "Read {protocol} and follow it exactly.",
		EffortMode:  "flag",
		EffortFlag:  "--effort",
		EffortMap: map[string]string{
			"minimal": "low",
			"low":     "low",
			"medium":  "medium",
			"high":    "high",
			"max":     "max",
		},
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
		EffortMode:  "config",
		EffortKey:   "model_reasoning_effort",
		EffortMap: map[string]string{
			"minimal": "low",
			"low":     "low",
			"medium":  "medium",
			"high":    "high",
			"max":     "xhigh",
		},
		Models: map[string]string{
			"codex":    "gpt-5.4",
			"best":     "gpt-5.4",
			"balanced": "gpt-5.4",
			"fast":     "gpt-5.4-mini",
			"gpt-5.4":  "gpt-5.4",
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
	Preset:   "", // empty: auto-detected from installed engines at runtime
	Mode:     ModeDevelop,
	Parallel: 1,
	Master: MasterConfig{
		CheckInterval: 2 * time.Minute,
	},
	Budget: BudgetConfig{
		MaxDuration: 8 * time.Hour,
	},
}

func ParseEffortLevel(raw string) (EffortLevel, error) {
	level := EffortLevel(strings.TrimSpace(raw))
	switch level {
	case "", EffortAuto, EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortMax:
		return level, nil
	default:
		return "", fmt.Errorf("invalid effort %q", raw)
	}
}

func validateEffortLevel(level EffortLevel, field string) error {
	normalized, err := ParseEffortLevel(string(level))
	if err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	if normalized == "" {
		return nil
	}
	return nil
}

type LaunchRequest struct {
	Engine string
	Model  string
	Effort EffortLevel
}

type LaunchSpec struct {
	Command         string
	RequestedEffort EffortLevel
	EffectiveEffort string
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

func applyConfigEnvelope(cfg *Config, engines *map[string]EngineConfig, presets *map[string]PresetConfig, dimensions *map[string]string, overlay *UserConfig) {
	if cfg == nil || overlay == nil {
		return
	}
	mergeConfig(cfg, &overlay.Config)
	if engines != nil {
		for k, v := range overlay.Engines {
			(*engines)[k] = v
		}
	}
	if presets != nil {
		for k, v := range overlay.Presets {
			(*presets)[k] = v
		}
	}
	if dimensions != nil {
		for k, v := range overlay.Dimensions {
			(*dimensions)[k] = v
		}
	}
}

// DetectPresetFromEnvironment checks which engines are available and picks
// the best preset. If both claude and codex are installed, use "hybrid"
// (master=opus, sessions=codex). If only one is available, use its preset.
func DetectPresetFromEnvironment() string {
	hasClaude := commandExists("claude")
	hasCodex := commandExists("codex")
	switch {
	case hasClaude && hasCodex:
		return "hybrid"
	case hasClaude:
		return "claude"
	case hasCodex:
		return "codex"
	default:
		return "codex" // fallback
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// applyPreset fills in missing master/role defaults from the selected preset.
func applyPreset(cfg *Config) {
	catalog := presetCatalogFor(cfg)
	preset, ok := catalog[cfg.Preset]
	if !ok {
		preset, ok = catalog["codex"]
		if !ok {
			preset = Presets["codex"]
		}
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
}

func presetCatalogFor(cfg *Config) map[string]PresetConfig {
	if cfg != nil && len(cfg.presetCatalog) > 0 {
		return cfg.presetCatalog
	}
	return Presets
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

	if err := validateEffortLevel(cfg.Master.Effort, "master.effort"); err != nil {
		return err
	}
	if err := validateLaunchRequest(engines, LaunchRequest{
		Engine: cfg.Master.Engine,
		Model:  cfg.Master.Model,
		Effort: cfg.Master.Effort,
	}, "master"); err != nil {
		return err
	}

	for name, profile := range cfg.Routing.Profiles {
		if err := validateEffortLevel(profile.Effort, fmt.Sprintf("routing.profiles.%s.effort", name)); err != nil {
			return err
		}
		if profile.Engine == "" && profile.Model == "" && profile.Effort == "" {
			return fmt.Errorf("routing.profiles.%s must set at least one of engine, model, or effort", name)
		}
		if profile.Model != "" && profile.Engine == "" {
			return fmt.Errorf("routing.profiles.%s.model requires engine", name)
		}
		if profile.Engine != "" {
			if err := validateLaunchRequest(engines, LaunchRequest{
				Engine: profile.Engine,
				Model:  profile.Model,
				Effort: profile.Effort,
			}, fmt.Sprintf("routing profile %q", name)); err != nil {
				return err
			}
		}
	}
	for role, dimensions := range cfg.Routing.Table {
		for dimension, profileName := range dimensions {
			if _, ok := cfg.Routing.Profiles[profileName]; !ok {
				return fmt.Errorf("routing.table.%s.%s references unknown profile %q", role, dimension, profileName)
			}
		}
	}

	sessions := ExpandSessions(cfg)
	if len(sessions) == 0 {
		sessions = []SessionConfig{EffectiveSessionConfig(cfg, -1)}
	}
	for i := range sessions {
		sess := EffectiveSessionConfig(cfg, i)
		if sess.Mode != "" && sess.Mode != ModeResearch && sess.Mode != ModeDevelop {
			return fmt.Errorf("session-%d mode must be 'research' or 'develop', got %q", i+1, sess.Mode)
		}
		if err := validateEffortLevel(sess.Effort, fmt.Sprintf("session-%d.effort", i+1)); err != nil {
			return err
		}
		if err := validateLaunchRequest(engines, LaunchRequest{
			Engine: sess.Engine,
			Model:  sess.Model,
			Effort: sess.Effort,
		}, fmt.Sprintf("session-%d", i+1)); err != nil {
			return err
		}
	}

	return nil
}

func ResolveLaunchSpec(engines map[string]EngineConfig, req LaunchRequest) (LaunchSpec, error) {
	ec, ok := engines[req.Engine]
	if !ok {
		return LaunchSpec{}, fmt.Errorf("unknown engine %q", req.Engine)
	}
	modelID, err := resolveModelID(engines, req.Engine, req.Model)
	if err != nil {
		return LaunchSpec{}, err
	}
	cmd := strings.ReplaceAll(ec.Command, "{model_id}", modelID)
	if req.Engine == "claude-code" && os.Getenv("IS_SANDBOX") == "1" {
		cmd = strings.ReplaceAll(cmd, "--permission-mode auto", "--permission-mode bypassPermissions")
	}
	spec := LaunchSpec{
		Command:         cmd,
		RequestedEffort: req.Effort,
	}
	if req.Effort == "" || req.Effort == EffortAuto {
		return spec, nil
	}

	effective := string(req.Effort)
	if mapped, ok := ec.EffortMap[string(req.Effort)]; ok && strings.TrimSpace(mapped) != "" {
		effective = mapped
	}
	spec.EffectiveEffort = effective

	switch ec.EffortMode {
	case "flag":
		if ec.EffortFlag != "" {
			spec.Command = spec.Command + " " + ec.EffortFlag + " " + effective
		}
	case "config":
		if ec.EffortKey != "" {
			spec.Command = spec.Command + fmt.Sprintf(" -c %s=%q", ec.EffortKey, effective)
		}
	}
	return spec, nil
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
	return validateLaunchRequest(engines, LaunchRequest{Engine: engine, Model: model}, role)
}

func validateLaunchRequest(engines map[string]EngineConfig, req LaunchRequest, role string) error {
	if err := validateModelAliasForEngine(engines, req.Engine, req.Model, role); err != nil {
		return err
	}
	spec, err := ResolveLaunchSpec(engines, req)
	if err != nil {
		return fmt.Errorf("%s engine: %w", role, err)
	}
	modelID, err := resolveModelID(engines, req.Engine, req.Model)
	if err != nil {
		return fmt.Errorf("%s engine: %w", role, err)
	}
	if req.Engine == "codex" && (modelID == "gpt-5.3-codex" || modelID == "gpt-5.2") {
		return fmt.Errorf("%s engine: codex model %q resolves to %s, which triggers an interactive migration prompt in Codex CLI; use gpt-5.4 or gpt-5.4-mini instead", role, req.Model, modelID)
	}
	_ = spec
	return nil
}

func validateInteractiveEngineLegacy(engines map[string]EngineConfig, engine, model, role string) error {
	if err := validateModelAliasForEngine(engines, engine, model, role); err != nil {
		return err
	}
	return validateLaunchRequest(engines, LaunchRequest{Engine: engine, Model: model}, role)
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

// ExpandSessions converts the configured session count into explicit session configs.
func ExpandSessions(cfg *Config) []SessionConfig {
	if cfg == nil {
		return nil
	}
	size := len(cfg.Sessions)
	if cfg.Parallel > size {
		size = cfg.Parallel
	}
	if size == 0 {
		return nil
	}
	sessions := make([]SessionConfig, size)
	copy(sessions, cfg.Sessions)
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
	}
	if out.Model == "" {
		out.Model = roleDefault.Model
	}
	if out.Effort == "" {
		out.Effort = roleDefault.Effort
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
	if overlay.Roles.Research.Engine != "" {
		base.Roles.Research.Engine = overlay.Roles.Research.Engine
	}
	if overlay.Roles.Research.Model != "" {
		base.Roles.Research.Model = overlay.Roles.Research.Model
	}
	if overlay.Roles.Research.Effort != "" {
		base.Roles.Research.Effort = overlay.Roles.Research.Effort
	}
	if overlay.Roles.Develop.Engine != "" {
		base.Roles.Develop.Engine = overlay.Roles.Develop.Engine
	}
	if overlay.Roles.Develop.Model != "" {
		base.Roles.Develop.Model = overlay.Roles.Develop.Model
	}
	if overlay.Roles.Develop.Effort != "" {
		base.Roles.Develop.Effort = overlay.Roles.Develop.Effort
	}
	if len(overlay.Routing.Profiles) > 0 {
		base.Routing.Profiles = copyProfiles(overlay.Routing.Profiles)
	}
	if len(overlay.Routing.Table) > 0 {
		base.Routing.Table = copyRoutingTable(overlay.Routing.Table)
	}
	if overlay.Parallel > 0 {
		base.Parallel = overlay.Parallel
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
		base.Harness.Command = overlay.Harness.Command
	}
	if overlay.Harness.Timeout > 0 {
		base.Harness.Timeout = overlay.Harness.Timeout
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
	if overlay.Master.Effort != "" {
		base.Master.Effort = overlay.Master.Effort
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
	if overlay.Guidance != "" {
		base.Guidance = overlay.Guidance
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
		if len(v.EffortMap) > 0 {
			effortMap := make(map[string]string, len(v.EffortMap))
			for ek, ev := range v.EffortMap {
				effortMap[ek] = ev
			}
			v.EffortMap = effortMap
		}
		dst[k] = v
	}
	return dst
}

func copyProfiles(src map[string]ExecutionProfile) map[string]ExecutionProfile {
	dst := make(map[string]ExecutionProfile, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyRoutingTable(src map[string]map[string]string) map[string]map[string]string {
	dst := make(map[string]map[string]string, len(src))
	for role, dims := range src {
		cloned := make(map[string]string, len(dims))
		for dim, profile := range dims {
			cloned[dim] = profile
		}
		dst[role] = cloned
	}
	return dst
}

func FilterExternalContextFiles(projectRoot string, files []string) []string {
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
