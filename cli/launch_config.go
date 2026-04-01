package cli

import (
	"fmt"
	"os"
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
		context, err := ResolveContextInputsFrom(projectRoot, opts.ContextPaths)
		if err != nil {
			return nil, fmt.Errorf("resolve context: %w", err)
		}
		resolved.Config.Context = MergeContextConfigs(resolved.Config.Context, context)
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
		context, err := ResolveContextInputsFrom(projectRoot, opts.ContextPaths)
		if err != nil {
			return nil, fmt.Errorf("resolve context: %w", err)
		}
		resolved.Config.Context = MergeContextConfigs(resolved.Config.Context, context)
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
		Name:          launchConfigName(projectRoot, opts),
		Mode:          opts.Mode,
		Objective:     opts.Objective,
		Parallel:      opts.Parallel,
		ClearSessions: true,
	}
	_ = projectRoot
	_ = baseCfg

	masterOverride, workerOverride, err := launchRoleOverrides(opts)
	if err != nil {
		return goalx.ResolveRequest{}, err
	}

	req.MasterOverride = masterOverride
	req.WorkerOverride = workerOverride
	if opts.Readonly {
		req.TargetOverride = &goalx.TargetConfig{Readonly: []string{"."}}
	}
	return req, nil
}

func launchConfigName(projectRoot string, opts launchOptions) string {
	if opts.Name != "" {
		return opts.Name
	}
	return nextAvailableRunName(projectRoot, goalx.Slugify(opts.Objective))
}

func nextAvailableRunName(projectRoot, base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "run"
	}
	candidate := base
	for i := 2; ; i++ {
		if _, err := os.Stat(goalx.RunDir(projectRoot, candidate)); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
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

	if len(opts.Subs) == 0 && (len(opts.Dimensions) > 0 || opts.Effort != "") {
		size := cfg.Parallel
		if size < 1 {
			size = 1
		}
		cfg.Sessions = make([]goalx.SessionConfig, size)
		for i := range cfg.Sessions {
			cfg.Sessions[i] = goalx.SessionConfig{
				Effort:     opts.Effort,
				Dimensions: append([]string(nil), opts.Dimensions...),
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
					Engine:     engine,
					Model:      model,
					Effort:     opts.Effort,
					Dimensions: append([]string(nil), opts.Dimensions...),
				})
			}
		}
	}

	return nil
}

func launchRoleOverrides(opts launchOptions) (*goalx.MasterConfig, *goalx.SessionConfig, error) {
	var masterOverride *goalx.MasterConfig
	if opts.Master != "" || opts.MasterEffort != "" || opts.Effort != "" {
		override := &goalx.MasterConfig{}
		if opts.Master != "" {
			engine, model, err := parseEngineModelValue("--master", opts.Master)
			if err != nil {
				return nil, nil, err
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

	var workerOverride *goalx.SessionConfig
	if opts.Worker != "" || opts.WorkerEffort != "" || opts.Effort != "" {
		override := &goalx.SessionConfig{}
		if opts.Worker != "" {
			engine, model, err := parseEngineModelValue("--worker", opts.Worker)
			if err != nil {
				return nil, nil, err
			}
			override.Engine = engine
			override.Model = model
		}
		if opts.WorkerEffort != "" {
			override.Effort = opts.WorkerEffort
		} else if opts.Effort != "" {
			override.Effort = opts.Effort
		}
		workerOverride = override
	}

	return masterOverride, workerOverride, nil
}

func parseEngineModelValue(flagName, value string) (string, string, error) {
	parts := strings.SplitN(strings.TrimSpace(value), "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("%s expects engine/model, got %q", flagName, value)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
