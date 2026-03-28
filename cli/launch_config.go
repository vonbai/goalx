package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

func buildLaunchConfig(projectRoot string, opts launchOptions) (*goalx.Config, error) {
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load config layers: %w", err)
	}
	req, err := buildLaunchResolveRequest(projectRoot, layers.Config, opts)
	if err != nil {
		return nil, err
	}
	resolved, err := goalx.ResolveConfigPreview(layers, req)
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
	applyLaunchBudgetOverride(&resolved.Config, opts)
	return &resolved.Config, nil
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
	req.RequireEngineAvailability = true
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
	applyLaunchBudgetOverride(&resolved.Config, opts)
	return resolved, nil
}

func applyLaunchBudgetOverride(cfg *goalx.Config, opts launchOptions) {
	if cfg == nil {
		return
	}
	if opts.BudgetSet {
		cfg.Budget.MaxDuration = opts.Budget
		return
	}
	if opts.Intent == runIntentEvolve && cfg.Budget.MaxDuration <= 0 {
		cfg.Budget.MaxDuration = 8 * time.Hour
	}
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
	_ = projectRoot
	_ = baseCfg

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
		if _, err := goalx.ResolveDimensionSpecs(opts.Dimensions, dimensions); err != nil {
			return err
		}
	}

	if len(opts.Subs) == 0 && (len(opts.Dimensions) > 0 || opts.RouteRole != "" || opts.RouteProfile != "" || opts.Effort != "") {
		size := cfg.Parallel
		if size < 1 {
			size = 1
		}
		cfg.Sessions = make([]goalx.SessionConfig, size)
		for i := range cfg.Sessions {
			cfg.Sessions[i] = goalx.SessionConfig{
				Effort:       opts.Effort,
				RouteRole:    opts.RouteRole,
				RouteProfile: opts.RouteProfile,
				Dimensions:   append([]string(nil), opts.Dimensions...),
			}
		}
	}

	if len(opts.Subs) > 0 {
		cfg.Sessions = nil
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
					Engine:       engine,
					Model:        model,
					Effort:       opts.Effort,
					RouteRole:    opts.RouteRole,
					RouteProfile: opts.RouteProfile,
					Dimensions:   append([]string(nil), opts.Dimensions...),
				})
			}
		}
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

func parseEngineModelValue(flagName, value string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("%s expects engine/model, got %q", flagName, value)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
