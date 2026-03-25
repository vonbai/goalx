package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const maxNextConfigParallel = 10
const maxNextConfigIterations = 20

type nextConfigJSON struct {
	Parallel      int                 `json:"parallel,omitempty"`
	Engine        string              `json:"engine,omitempty"`
	Model         string              `json:"model,omitempty"`
	Effort        goalx.EffortLevel   `json:"effort,omitempty"`
	Preset        string              `json:"preset,omitempty"`
	Dimensions    []string            `json:"dimensions,omitempty"`
	BudgetSeconds int                 `json:"budget_seconds,omitempty"`
	Objective     string              `json:"objective,omitempty"`
	Harness       string              `json:"harness,omitempty"`
	Mode          string              `json:"mode,omitempty"`
	MaxIterations int                 `json:"max_iterations,omitempty"`
	Context       []string            `json:"context,omitempty"`
	MasterEngine  string              `json:"master_engine,omitempty"`
	MasterModel   string              `json:"master_model,omitempty"`
	MasterEffort  goalx.EffortLevel   `json:"master_effort,omitempty"`
	RouteProfile  string              `json:"route_profile,omitempty"`
	QuotaState    string              `json:"quota_state,omitempty"`
	Sessions      []sessionConfigJSON `json:"sessions,omitempty"`
}

type sessionConfigJSON struct {
	Hint   string `json:"hint,omitempty"`
	Engine string `json:"engine,omitempty"`
	Model  string `json:"model,omitempty"`
}

func validateNextConfig(projectRoot string, nc *nextConfigJSON) *nextConfigJSON {
	if nc == nil {
		return nil
	}

	validated := *nc
	validated.Engine = strings.TrimSpace(validated.Engine)
	validated.Model = strings.TrimSpace(validated.Model)
	validated.Preset = strings.TrimSpace(validated.Preset)
	validated.Objective = strings.TrimSpace(validated.Objective)
	validated.Harness = strings.TrimSpace(validated.Harness)
	validated.Mode = strings.TrimSpace(validated.Mode)
	validated.Context = normalizeNextConfigContext(validated.Context)
	validated.MasterEngine = strings.TrimSpace(validated.MasterEngine)
	validated.MasterModel = strings.TrimSpace(validated.MasterModel)
	validated.RouteProfile = strings.TrimSpace(validated.RouteProfile)
	validated.QuotaState = strings.TrimSpace(validated.QuotaState)
	validated.Sessions = normalizeNextConfigSessions(validated.Sessions)
	validated.Dimensions = normalizeNextConfigHints(validated.Dimensions, 0)
	if level, err := goalx.ParseEffortLevel(string(validated.Effort)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.effort=%q (%v)\n", validated.Effort, err)
		validated.Effort = ""
	} else {
		validated.Effort = level
	}
	if level, err := goalx.ParseEffortLevel(string(validated.MasterEffort)); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.master_effort=%q (%v)\n", validated.MasterEffort, err)
		validated.MasterEffort = ""
	} else {
		validated.MasterEffort = level
	}

	if validated.Parallel < 0 {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.parallel=%d (must be >= 0)\n", validated.Parallel)
		validated.Parallel = 0
	} else if validated.Parallel > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: capping next_config.parallel=%d to %d\n", validated.Parallel, maxNextConfigParallel)
		validated.Parallel = maxNextConfigParallel
	}
	if validated.BudgetSeconds < 0 {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.budget_seconds=%d (must be >= 0)\n", validated.BudgetSeconds)
		validated.BudgetSeconds = 0
	}
	if validated.MaxIterations < 0 || validated.MaxIterations > maxNextConfigIterations {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.max_iterations=%d (must be 1-%d or 0)\n", validated.MaxIterations, maxNextConfigIterations)
		validated.MaxIterations = 0
	}
	if len(validated.Dimensions) > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: truncating next_config.dimensions to %d entries\n", maxNextConfigParallel)
		validated.Dimensions = validated.Dimensions[:maxNextConfigParallel]
	}

	engines := goalx.BuiltinEngines
	if loadedEngines, err := loadEngineCatalog(projectRoot); err == nil && len(loadedEngines) > 0 {
		engines = loadedEngines
	}
	switch validated.Mode {
	case "", string(goalx.ModeResearch), string(goalx.ModeDevelop):
	default:
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.mode=%q (must be research or develop)\n", validated.Mode)
		validated.Mode = ""
	}
	if validated.Engine != "" {
		if _, ok := engines[validated.Engine]; !ok {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.engine=%q (unknown engine)\n", validated.Engine)
			validated.Engine = ""
		}
	}
	if len(validated.Dimensions) > 0 {
		if _, err := goalx.ResolveDimensions(validated.Dimensions); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.dimensions: %v\n", err)
			validated.Dimensions = nil
		}
	}
	validated.MasterEngine, validated.MasterModel = validateNamedEngineModelPair(engines, validated.MasterEngine, validated.MasterModel, "next_config.master")
	for i := range validated.Sessions {
		label := fmt.Sprintf("next_config.sessions[%d]", i)
		validated.Sessions[i].Engine, validated.Sessions[i].Model = validateNamedEngineModelPair(engines, validated.Sessions[i].Engine, validated.Sessions[i].Model, label)
	}

	return &validated
}

func nextConfigParallel(fallback int, nc *nextConfigJSON) int {
	if nc != nil && nc.Parallel > 0 {
		return nc.Parallel
	}
	return fallback
}

func nextConfigObjective(fallback string, nc *nextConfigJSON) string {
	if nc != nil && nc.Objective != "" {
		return nc.Objective
	}
	return fallback
}

func nextConfigBudget(fallback time.Duration, nc *nextConfigJSON) time.Duration {
	if nc != nil && nc.BudgetSeconds > 0 {
		return time.Duration(nc.BudgetSeconds) * time.Second
	}
	return fallback
}

func nextConfigHints(fallback []string, parallel int, nc *nextConfigJSON) []string {
	if nc == nil {
		return normalizeNextConfigHints(fallback, parallel)
	}
	dimensionHints := nextConfigDimensionHints(nc)
	if len(dimensionHints) == 0 {
		return normalizeNextConfigHints(fallback, parallel)
	}
	return normalizeNextConfigHints(dimensionHints, parallel)
}

func normalizeNextConfigHints(hints []string, parallel int) []string {
	if len(hints) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(hints))
	for _, hint := range hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		normalized = append(normalized, hint)
	}
	if len(normalized) == 0 {
		return nil
	}
	if parallel > 0 && len(normalized) > parallel {
		return normalized[:parallel]
	}
	return normalized
}

func nextConfigDimensionHints(nc *nextConfigJSON) []string {
	if nc == nil || len(nc.Dimensions) == 0 {
		return nil
	}
	hints, err := goalx.ResolveDimensions(nc.Dimensions)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.dimensions: %v\n", err)
		return nil
	}
	return hints
}

func resolveNextEngineModel(engines map[string]goalx.EngineConfig, defaultEngine, defaultModel string, nc *nextConfigJSON) (string, string) {
	if len(engines) == 0 {
		engines = goalx.BuiltinEngines
	}

	engine := defaultEngine
	model := defaultModel
	if nc == nil {
		return engine, model
	}

	if nc.Engine != "" {
		if _, ok := engines[nc.Engine]; ok {
			engine = nc.Engine
		} else {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.engine=%q (unknown engine)\n", nc.Engine)
		}
	}
	if nc.Model != "" {
		if err := validateNextConfigModel(engines, engine, nc.Model); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.model=%q for engine %q: %v\n", nc.Model, engine, err)
		} else {
			model = nc.Model
		}
	}

	return engine, model
}

func validateNextConfigModel(engines map[string]goalx.EngineConfig, engine, model string) error {
	if strings.TrimSpace(model) == "" {
		return nil
	}
	if strings.TrimSpace(engine) == "" {
		return fmt.Errorf("engine is required")
	}
	if goalx.ModelAliasBelongsToOtherEngine(engines, engine, model) {
		return fmt.Errorf("model alias belongs to a different engine")
	}
	if _, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{Engine: engine, Model: model}); err != nil {
		return err
	}

	modelID := model
	if ec, ok := engines[engine]; ok {
		if resolved, ok := ec.Models[model]; ok {
			modelID = resolved
		}
	}
	if engine == "codex" && (modelID == "gpt-5.3-codex" || modelID == "gpt-5.2") {
		return fmt.Errorf("model resolves to an interactive Codex migration target")
	}
	return nil
}

func validateNamedEngineModelPair(engines map[string]goalx.EngineConfig, engine, model, label string) (string, string) {
	if strings.TrimSpace(engine) == "" {
		if strings.TrimSpace(model) != "" {
			fmt.Fprintf(os.Stderr, "warning: ignoring %s.model=%q (engine is required)\n", label, model)
		}
		return "", ""
	}
	if _, ok := engines[engine]; !ok {
		fmt.Fprintf(os.Stderr, "warning: ignoring %s.engine=%q (unknown engine)\n", label, engine)
		return "", ""
	}
	if err := validateNextConfigModel(engines, engine, model); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring %s.model=%q for engine %q: %v\n", label, model, engine, err)
		return engine, ""
	}
	return engine, model
}

func normalizeNextConfigContext(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		normalized = append(normalized, path)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeNextConfigSessions(sessions []sessionConfigJSON) []sessionConfigJSON {
	if len(sessions) == 0 {
		return nil
	}

	normalized := make([]sessionConfigJSON, 0, len(sessions))
	for _, session := range sessions {
		session.Hint = strings.TrimSpace(session.Hint)
		session.Engine = strings.TrimSpace(session.Engine)
		session.Model = strings.TrimSpace(session.Model)
		if session.Hint == "" && session.Engine == "" && session.Model == "" {
			continue
		}
		normalized = append(normalized, session)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}
