package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

func buildLaunchConfig(projectRoot string, opts launchOptions) (*goalx.Config, error) {
	baseCfg, _, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load base config: %w", err)
	}

	cfg := *baseCfg
	cfg.Name = opts.Name
	if cfg.Name == "" {
		cfg.Name = goalx.Slugify(opts.Objective)
	}
	cfg.Mode = opts.Mode
	cfg.Objective = opts.Objective
	if opts.Parallel > 0 {
		cfg.Parallel = opts.Parallel
	}
	cfg.Sessions = nil
	if opts.Preset != "" {
		cfg.Preset = opts.Preset
	}
	goalx.ApplyPreset(&cfg)
	if err := applyLaunchRoleOverrides(&cfg, opts); err != nil {
		return nil, err
	}
	if cfg.Parallel < 1 {
		cfg.Parallel = 1
	}

	if len(opts.Dimensions) > 0 {
		hints, err := goalx.ResolveDimensions(opts.Dimensions)
		if err != nil {
			return nil, err
		}
		if cfg.Parallel < len(hints) {
			cfg.Parallel = len(hints)
		}
		cfg.Sessions = make([]goalx.SessionConfig, cfg.Parallel)
		sessionMode := goalx.ResolveSessionMode(cfg.Mode, "")
		for i, hint := range hints {
			cfg.Sessions[i] = goalx.SessionConfig{
				Hint: hint,
				Mode: sessionMode,
			}
		}
	}
	if len(cfg.Target.Files) == 0 {
		cfg.Target.Files = InferTarget(projectRoot)
	}
	if len(cfg.Target.Files) == 0 {
		cfg.Target = goalx.TargetConfig{Files: []string{"TODO: specify directories to modify"}}
	}
	if cfg.Harness.Command == "" {
		cfg.Harness.Command = InferHarness(projectRoot)
	}
	if cfg.Harness.Command == "" {
		cfg.Harness = goalx.HarnessConfig{Command: "TODO: build + test command"}
	}

	if len(opts.Subs) > 0 {
		cfg.Sessions = nil
		sessionMode := goalx.ResolveSessionMode(cfg.Mode, "")
		for _, sub := range opts.Subs {
			spec, countStr := sub, "1"
			if idx := strings.LastIndex(sub, ":"); idx > 0 {
				spec = sub[:idx]
				countStr = sub[idx+1:]
			}
			engine, model, err := parseEngineModelValue("--sub", spec)
			if err != nil {
				return nil, fmt.Errorf("invalid --sub format %q (expected engine/model or engine/model:N): %w", sub, err)
			}
			n, err := strconv.Atoi(countStr)
			if err != nil || n < 1 {
				return nil, fmt.Errorf("invalid --sub count %q in %q", countStr, sub)
			}
			for j := 0; j < n; j++ {
				cfg.Sessions = append(cfg.Sessions, goalx.SessionConfig{
					Engine: engine,
					Model:  model,
					Mode:   sessionMode,
				})
			}
		}
	}

	if opts.Auditor != "" {
		engine, model, err := parseEngineModelValue("--auditor", opts.Auditor)
		if err != nil {
			return nil, err
		}
		cfg.Sessions = append(cfg.Sessions, goalx.SessionConfig{
			Engine: engine,
			Model:  model,
			Effort: opts.Effort,
			Mode:   goalx.ResolveSessionMode(cfg.Mode, ""),
			Hint:   "Auditor: Review and challenge other sessions' work. Find flaws, missed edge cases, and incorrect assumptions.",
		})
	}

	if len(opts.ContextPaths) > 0 {
		contextFiles, err := DiscoverContextFiles(opts.ContextPaths)
		if err != nil {
			return nil, fmt.Errorf("discover context: %w", err)
		}
		cfg.Context = goalx.ContextConfig{Files: contextFiles}
	}

	cfg.Budget = goalx.BudgetConfig{MaxDuration: 6 * time.Hour}
	return &cfg, nil
}

func resolveLaunchConfig(projectRoot string, opts launchOptions) (*goalx.ResolvedConfig, error) {
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load config layers: %w", err)
	}

	req, err := buildLaunchResolveRequest(projectRoot, layers.Config, opts)
	if err != nil {
		return nil, err
	}
	resolved, err := goalx.ResolveConfig(layers, req)
	if err != nil {
		return nil, err
	}
	if err := applyLaunchSessionOverrides(&resolved.Config, opts, resolved.Dimensions); err != nil {
		return nil, err
	}
	if len(opts.ContextPaths) > 0 {
		contextFiles, err := DiscoverContextFiles(opts.ContextPaths)
		if err != nil {
			return nil, fmt.Errorf("discover context: %w", err)
		}
		resolved.Config.Context = goalx.ContextConfig{Files: contextFiles}
	}
	resolved.Config.Budget = goalx.BudgetConfig{MaxDuration: 6 * time.Hour}
	return resolved, nil
}

func buildLaunchResolveRequest(projectRoot string, baseCfg goalx.Config, opts launchOptions) (goalx.ResolveRequest, error) {
	req := goalx.ResolveRequest{
		Name:          launchConfigName(opts),
		Mode:          opts.Mode,
		Objective:     opts.Objective,
		Preset:        opts.Preset,
		Parallel:      opts.Parallel,
		ClearSessions: true,
	}
	if len(baseCfg.Target.Files) == 0 {
		target := InferTarget(projectRoot)
		if len(target) == 0 {
			target = []string{"TODO: specify directories to modify"}
		}
		req.TargetOverride = &goalx.TargetConfig{Files: target}
	}
	if baseCfg.Harness.Command == "" {
		harness := InferHarness(projectRoot)
		if harness == "" {
			harness = "TODO: build + test command"
		}
		req.HarnessOverride = &goalx.HarnessConfig{Command: harness}
	}

	masterOverride, researchOverride, developOverride, err := launchRoleOverrides(opts)
	if err != nil {
		return goalx.ResolveRequest{}, err
	}

	req.MasterOverride = masterOverride
	req.ResearchOverride = researchOverride
	req.DevelopOverride = developOverride
	return req, nil
}

func launchConfigName(opts launchOptions) string {
	if opts.Name != "" {
		return opts.Name
	}
	return goalx.Slugify(opts.Objective)
}

func applyLaunchSessionOverrides(cfg *goalx.Config, opts launchOptions, dimensions map[string]string) error {
	if cfg == nil {
		return fmt.Errorf("launch config is nil")
	}

	cfg.Sessions = nil
	if len(opts.Dimensions) > 0 {
		hints, err := goalx.ResolveDimensions(opts.Dimensions, dimensions)
		if err != nil {
			return err
		}
		if cfg.Parallel < len(hints) {
			cfg.Parallel = len(hints)
		}
		cfg.Sessions = make([]goalx.SessionConfig, cfg.Parallel)
		sessionMode := goalx.ResolveSessionMode(cfg.Mode, "")
		for i, hint := range hints {
			cfg.Sessions[i] = goalx.SessionConfig{
				Hint: hint,
				Mode: sessionMode,
			}
		}
	}

	if len(opts.Subs) > 0 {
		cfg.Sessions = nil
		sessionMode := goalx.ResolveSessionMode(cfg.Mode, "")
		for _, sub := range opts.Subs {
			spec, countStr := sub, "1"
			if idx := strings.LastIndex(sub, ":"); idx > 0 {
				spec = sub[:idx]
				countStr = sub[idx+1:]
			}
			engine, model, err := parseEngineModelValue("--sub", spec)
			if err != nil {
				return fmt.Errorf("invalid --sub format %q (expected engine/model or engine/model:N): %w", sub, err)
			}
			n, err := strconv.Atoi(countStr)
			if err != nil || n < 1 {
				return fmt.Errorf("invalid --sub count %q in %q", countStr, sub)
			}
			for j := 0; j < n; j++ {
				cfg.Sessions = append(cfg.Sessions, goalx.SessionConfig{
					Engine: engine,
					Model:  model,
					Mode:   sessionMode,
				})
			}
		}
	}

	if opts.Auditor != "" {
		engine, model, err := parseEngineModelValue("--auditor", opts.Auditor)
		if err != nil {
			return err
		}
		cfg.Sessions = append(cfg.Sessions, goalx.SessionConfig{
			Engine: engine,
			Model:  model,
			Effort: opts.Effort,
			Mode:   goalx.ResolveSessionMode(cfg.Mode, ""),
			Hint:   "Auditor: Review and challenge other sessions' work. Find flaws, missed edge cases, and incorrect assumptions.",
		})
	}
	return nil
}

func launchRoleOverrides(opts launchOptions) (*goalx.MasterConfig, *goalx.SessionConfig, *goalx.SessionConfig, error) {
	var masterOverride *goalx.MasterConfig
	if opts.Master != "" || opts.MasterEffort != "" || opts.Effort != "" {
		override := &goalx.MasterConfig{}
		if opts.Master != "" {
			engine, model, err := parseEngineModelValue("--master", opts.Master)
			if err != nil {
				return nil, nil, nil, err
			}
			override.Engine = engine
			override.Model = model
		}
		if opts.MasterEffort != "" {
			override.Effort = opts.MasterEffort
		} else if opts.Effort != "" {
			override.Effort = opts.Effort
		}
		masterOverride = override
	}

	var researchOverride *goalx.SessionConfig
	if opts.ResearchRole != "" || opts.ResearchEffort != "" || opts.Effort != "" {
		override := &goalx.SessionConfig{}
		if opts.ResearchRole != "" {
			engine, model, err := parseEngineModelValue("--research-role", opts.ResearchRole)
			if err != nil {
				return nil, nil, nil, err
			}
			override.Engine = engine
			override.Model = model
		}
		if opts.ResearchEffort != "" {
			override.Effort = opts.ResearchEffort
		} else if opts.Effort != "" {
			override.Effort = opts.Effort
		}
		researchOverride = override
	}

	var developOverride *goalx.SessionConfig
	if opts.DevelopRole != "" || opts.DevelopEffort != "" || opts.Effort != "" {
		override := &goalx.SessionConfig{}
		if opts.DevelopRole != "" {
			engine, model, err := parseEngineModelValue("--develop-role", opts.DevelopRole)
			if err != nil {
				return nil, nil, nil, err
			}
			override.Engine = engine
			override.Model = model
		}
		if opts.DevelopEffort != "" {
			override.Effort = opts.DevelopEffort
		} else if opts.Effort != "" {
			override.Effort = opts.Effort
		}
		developOverride = override
	}

	return masterOverride, researchOverride, developOverride, nil
}

func applyLaunchRoleOverrides(cfg *goalx.Config, opts launchOptions) error {
	if cfg == nil {
		return fmt.Errorf("launch config is nil")
	}
	masterOverride, researchOverride, developOverride, err := launchRoleOverrides(opts)
	if err != nil {
		return err
	}
	if masterOverride != nil {
		if masterOverride.Engine != "" {
			cfg.Master.Engine = masterOverride.Engine
		}
		if masterOverride.Model != "" {
			cfg.Master.Model = masterOverride.Model
		}
		if masterOverride.Effort != "" {
			cfg.Master.Effort = masterOverride.Effort
		}
	}
	if researchOverride != nil {
		if researchOverride.Engine != "" {
			cfg.Roles.Research.Engine = researchOverride.Engine
		}
		if researchOverride.Model != "" {
			cfg.Roles.Research.Model = researchOverride.Model
		}
		if researchOverride.Effort != "" {
			cfg.Roles.Research.Effort = researchOverride.Effort
		}
	}
	if developOverride != nil {
		if developOverride.Engine != "" {
			cfg.Roles.Develop.Engine = developOverride.Engine
		}
		if developOverride.Model != "" {
			cfg.Roles.Develop.Model = developOverride.Model
		}
		if developOverride.Effort != "" {
			cfg.Roles.Develop.Effort = developOverride.Effort
		}
	}
	return nil
}

func parseEngineModelValue(flagName, value string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("%s expects engine/model, got %q", flagName, value)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
