package goalx

import (
	"bytes"
	"fmt"
	"io"
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
	Name            string                `yaml:"name"`
	Mode            Mode                  `yaml:"mode"`
	Objective       string                `yaml:"objective"`
	Description     string                `yaml:"description,omitempty"`
	Selection       SelectionConfig       `yaml:"selection,omitempty"`
	Preferences     PreferencesConfig     `yaml:"preferences,omitempty"`
	Roles           RoleDefaultsConfig    `yaml:"roles,omitempty"`
	Parallel        int                   `yaml:"parallel,omitempty"`
	Sessions        []SessionConfig       `yaml:"sessions,omitempty"`
	Target          TargetConfig          `yaml:"target"`
	LocalValidation LocalValidationConfig `yaml:"local_validation,omitempty"`
	Acceptance      AcceptanceConfig      `yaml:"acceptance,omitempty"`
	Context         ContextConfig         `yaml:"context,omitempty"`
	Budget          BudgetConfig          `yaml:"budget,omitempty"`
	Master          MasterConfig          `yaml:"master,omitempty"`
	Memory          MemoryConfig          `yaml:"memory,omitempty"`

	dimensionCatalog map[string]string
}

type TargetConfig struct {
	Files    []string `yaml:"files"`
	Readonly []string `yaml:"readonly,omitempty"`
}

type LocalValidationConfig struct {
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

type MemoryConfig struct {
	LLMExtract string `yaml:"llm_extract,omitempty"`
}

type SessionConfig struct {
	Hint            string                 `yaml:"hint,omitempty"`
	Engine          string                 `yaml:"engine,omitempty"`
	Model           string                 `yaml:"model,omitempty"`
	Effort          EffortLevel            `yaml:"effort,omitempty"`
	Mode            Mode                   `yaml:"mode,omitempty"`
	Dimensions      []string               `yaml:"dimensions,omitempty"`
	Target          *TargetConfig          `yaml:"target,omitempty"`
	LocalValidation *LocalValidationConfig `yaml:"local_validation,omitempty"`
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

type SelectionConfig struct {
	DisabledEngines    []string    `yaml:"disabled_engines,omitempty"`
	DisabledTargets    []string    `yaml:"disabled_targets,omitempty"`
	MasterCandidates   []string    `yaml:"master_candidates,omitempty"`
	ResearchCandidates []string    `yaml:"research_candidates,omitempty"`
	DevelopCandidates  []string    `yaml:"develop_candidates,omitempty"`
	MasterEffort       EffortLevel `yaml:"master_effort,omitempty"`
	ResearchEffort     EffortLevel `yaml:"research_effort,omitempty"`
	DevelopEffort      EffortLevel `yaml:"develop_effort,omitempty"`
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

// UserConfig is the top-level user config file (~/.goalx/config.yaml).
type UserConfig struct {
	Config     `yaml:",inline"`
	Engines    map[string]EngineConfig `yaml:"engines,omitempty"`
	Dimensions map[string]string       `yaml:"dimensions,omitempty"`
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
		Prompt:      "Read {protocol} and follow it exactly. Execute protocol instructions by taking real tool actions in this turn; do not stop after stating intent.",
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
	Mode:     ModeDevelop,
	Parallel: 1,
	Preferences: PreferencesConfig{
		Research: PreferencePolicy{
			Guidance: "默认 gpt-5.4 high。深度分析/架构设计用 opus。简单信息收集用 fast。",
		},
		Develop: PreferencePolicy{
			Guidance: "主力 gpt-5.4 medium。简单修复用 fast。",
		},
		Simple: PreferencePolicy{
			Guidance: "轻量任务用 fast。",
		},
	},
	Master: MasterConfig{
		CheckInterval: 2 * time.Minute,
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

func validateUniqueNames(values []string, field string) error {
	seen := make(map[string]bool, len(values))
	for _, raw := range values {
		name := strings.TrimSpace(raw)
		if name == "" {
			return fmt.Errorf("%s must not contain empty values", field)
		}
		if seen[name] {
			return fmt.Errorf("%s contains duplicate value %q", field, name)
		}
		seen[name] = true
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
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		if err == io.EOF {
			return cfg, nil
		}
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	return cfg, nil
}

func applyConfigEnvelope(cfg *Config, engines *map[string]EngineConfig, dimensions *map[string]string, overlay *UserConfig) {
	if cfg == nil || overlay == nil {
		return
	}
	mergeConfig(cfg, &overlay.Config)
	if engines != nil {
		for k, v := range overlay.Engines {
			(*engines)[k] = v
		}
	}
	if dimensions != nil {
		for k, v := range overlay.Dimensions {
			(*dimensions)[k] = v
		}
	}
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
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
	if err := validateMemoryConfig(cfg.Memory); err != nil {
		return err
	}

	for _, f := range cfg.Target.Files {
		if strings.HasPrefix(f, "TODO") {
			return fmt.Errorf("target.files contains placeholder %q — edit the manual draft config first", f)
		}
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
	for _, role := range selectedLaunchRoles(cfg) {
		if err := validateEffortLevel(role.cfg.Effort, role.name+".effort"); err != nil {
			return err
		}
		if strings.TrimSpace(role.cfg.Engine) != "" || strings.TrimSpace(role.cfg.Model) != "" {
			if err := validateLaunchRequest(engines, LaunchRequest{
				Engine: role.cfg.Engine,
				Model:  role.cfg.Model,
				Effort: role.cfg.Effort,
			}, role.name); err != nil {
				return err
			}
		}
	}

	sessions := ExpandSessions(cfg)
	for i := range sessions {
		sess := EffectiveSessionConfig(cfg, i)
		if sess.Mode != "" && sess.Mode != ModeResearch && sess.Mode != ModeDevelop {
			return fmt.Errorf("session-%d mode must be 'research' or 'develop', got %q", i+1, sess.Mode)
		}
		if err := validateEffortLevel(sess.Effort, fmt.Sprintf("session-%d.effort", i+1)); err != nil {
			return err
		}
		if _, err := resolveDimensionSpecsWithCatalog(resolveDimensionCatalog(cfg), sess.Dimensions); err != nil {
			return fmt.Errorf("session-%d dimensions: %w", i+1, err)
		}
		if strings.TrimSpace(sess.Engine) != "" || strings.TrimSpace(sess.Model) != "" {
			if err := validateLaunchRequest(engines, LaunchRequest{
				Engine: sess.Engine,
				Model:  sess.Model,
				Effort: sess.Effort,
			}, fmt.Sprintf("session-%d", i+1)); err != nil {
				return err
			}
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
	if req.Engine == "codex" && isBlockedCodexModel(modelID, req.Model) {
		return fmt.Errorf("%s engine: codex model %q resolves to %s, which triggers an interactive migration prompt in Codex CLI; use gpt-5.4 or gpt-5.4-mini instead", role, req.Model, modelID)
	}
	_ = spec
	return nil
}

func validateLaunchAvailability(cfg *Config, engines map[string]EngineConfig) error {
	if cfg == nil {
		return nil
	}

	seen := make(map[string]bool)
	check := func(role string, req LaunchRequest) error {
		if strings.TrimSpace(req.Engine) == "" {
			return nil
		}
		if seen[req.Engine] {
			return nil
		}
		seen[req.Engine] = true

		spec, err := ResolveLaunchSpec(engines, req)
		if err != nil {
			return fmt.Errorf("%s engine: %w", role, err)
		}
		binary := launchBinaryName(spec.Command)
		if binary == "" {
			return fmt.Errorf("%s engine: launch command is empty", role)
		}
		if !commandExists(binary) {
			return fmt.Errorf("%s engine: required command %q is not available on PATH", role, binary)
		}
		return nil
	}

	if err := check("master", LaunchRequest{Engine: cfg.Master.Engine, Model: cfg.Master.Model, Effort: cfg.Master.Effort}); err != nil {
		return err
	}
	for _, role := range selectedLaunchRoles(cfg) {
		if err := check(role.name, LaunchRequest{Engine: role.cfg.Engine, Model: role.cfg.Model, Effort: role.cfg.Effort}); err != nil {
			return err
		}
	}

	for i := range ExpandSessions(cfg) {
		effective := EffectiveSessionConfig(cfg, i)
		if err := check(fmt.Sprintf("session-%d", i+1), LaunchRequest{Engine: effective.Engine, Model: effective.Model, Effort: effective.Effort}); err != nil {
			return err
		}
	}
	return nil
}

type selectedLaunchRole struct {
	name string
	cfg  SessionConfig
}

func selectedLaunchRoles(cfg *Config) []selectedLaunchRole {
	if cfg == nil {
		return nil
	}
	switch cfg.Mode {
	case ModeResearch:
		return []selectedLaunchRole{{name: "roles.research", cfg: cfg.Roles.Research}}
	case ModeDevelop:
		return []selectedLaunchRole{{name: "roles.develop", cfg: cfg.Roles.Develop}}
	case ModeAuto:
		return []selectedLaunchRole{
			{name: "roles.research", cfg: cfg.Roles.Research},
			{name: "roles.develop", cfg: cfg.Roles.Develop},
		}
	default:
		return nil
	}
}

func launchBinaryName(command string) string {
	for _, field := range strings.Fields(command) {
		if field == "env" {
			continue
		}
		if strings.Contains(field, "=") && !strings.HasPrefix(field, "-") {
			continue
		}
		return field
	}
	return ""
}

func isBlockedCodexModel(modelID, rawModel string) bool {
	switch modelID {
	case "gpt-5.3-codex", "gpt-5.2":
		return true
	}
	switch rawModel {
	case "gpt-5.3-codex", "gpt-5.2":
		return true
	}
	return false
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

// EffectiveSessionConfig resolves a session against explicit run-level defaults.
func EffectiveSessionConfig(cfg *Config, idx int) SessionConfig {
	var out SessionConfig
	if cfg == nil {
		return out
	}

	sessions := ExpandSessions(cfg)
	if idx >= 0 && idx < len(sessions) {
		out = sessions[idx]
	}
	if len(out.Dimensions) > 0 {
		out.Dimensions = append([]string(nil), out.Dimensions...)
	}
	out = fillSessionDefaults(out, explicitRoleSession(cfg, out.Mode))

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

	localValidation := cfg.LocalValidation
	if out.LocalValidation != nil {
		if out.LocalValidation.Command != "" {
			localValidation.Command = out.LocalValidation.Command
		}
		if out.LocalValidation.Timeout > 0 {
			localValidation.Timeout = out.LocalValidation.Timeout
		}
	}
	out.LocalValidation = &localValidation

	return out
}

func explicitRoleSession(cfg *Config, mode Mode) SessionConfig {
	if cfg == nil {
		return SessionConfig{}
	}
	switch mode {
	case ModeResearch:
		return cfg.Roles.Research
	case ModeDevelop:
		return cfg.Roles.Develop
	}
	return SessionConfig{}
}

func fillSessionDefaults(session SessionConfig, defaults SessionConfig) SessionConfig {
	if session.Engine == "" {
		session.Engine = defaults.Engine
	}
	if session.Model == "" {
		session.Model = defaults.Model
	}
	if session.Effort == "" {
		session.Effort = defaults.Effort
	}
	return session
}

func resolveDimensionCatalog(cfg *Config) map[string]string {
	if cfg == nil || len(cfg.dimensionCatalog) == 0 {
		return BuiltinDimensions
	}
	return cfg.dimensionCatalog
}

// ResolveAcceptanceCommand returns the explicit acceptance command.
func ResolveAcceptanceCommand(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.Acceptance.Command)
}

// ResolveLocalValidationCommand returns the explicit local validation command.
func ResolveLocalValidationCommand(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	return strings.TrimSpace(cfg.LocalValidation.Command)
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
	if hasSelectionConfig(overlay.Selection) {
		base.Selection = copySelectionConfig(overlay.Selection)
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
	if overlay.LocalValidation.Command != "" {
		base.LocalValidation.Command = overlay.LocalValidation.Command
	}
	if overlay.LocalValidation.Timeout > 0 {
		base.LocalValidation.Timeout = overlay.LocalValidation.Timeout
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
	if overlay.Memory.LLMExtract != "" {
		base.Memory.LLMExtract = overlay.Memory.LLMExtract
	}
}

func mergePreferencesConfig(base, overlay *PreferencesConfig) {
	mergePreferencePolicy(&base.Research, overlay.Research)
	mergePreferencePolicy(&base.Develop, overlay.Develop)
	mergePreferencePolicy(&base.Simple, overlay.Simple)
}

func mergePreferencePolicy(base *PreferencePolicy, overlay PreferencePolicy) {
	if overlay.Guidance != "" {
		base.Guidance = overlay.Guidance
	}
}

func validateMemoryConfig(cfg MemoryConfig) error {
	switch strings.TrimSpace(cfg.LLMExtract) {
	case "", "off":
		return nil
	default:
		return fmt.Errorf("memory.llm_extract must be \"off\" when set, got %q", cfg.LLMExtract)
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

func (c *Config) DimensionCatalog() map[string]string {
	return copyStringCatalog(resolveDimensionCatalog(c))
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
