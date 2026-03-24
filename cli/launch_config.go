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
	cfg.DiversityHints = nil
	if opts.Preset != "" {
		cfg.Preset = opts.Preset
	}
	goalx.ApplyPreset(&cfg)
	if err := applyLaunchRoleOverrides(&cfg, opts); err != nil {
		return nil, err
	}
	syncLegacySessionFallback(&cfg)
	if cfg.Parallel < 1 {
		cfg.Parallel = 1
	}

	if cfg.Mode == goalx.ModeResearch {
		if len(opts.Strategies) > 0 {
			hints, err := goalx.ResolveStrategies(opts.Strategies)
			if err != nil {
				return nil, err
			}
			cfg.DiversityHints = hints
		} else if cfg.Parallel >= 2 {
			defaults := goalx.DefaultStrategies(cfg.Parallel)
			hints, err := goalx.ResolveStrategies(defaults)
			if err != nil {
				return nil, err
			}
			cfg.DiversityHints = hints
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
		cfg.Parallel = 0
		cfg.DiversityHints = nil
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

func applyLaunchRoleOverrides(cfg *goalx.Config, opts launchOptions) error {
	if cfg == nil {
		return fmt.Errorf("launch config is nil")
	}
	if opts.Master != "" {
		engine, model, err := parseEngineModelValue("--master", opts.Master)
		if err != nil {
			return err
		}
		cfg.Master.Engine = engine
		cfg.Master.Model = model
	}
	if opts.ResearchRole != "" {
		engine, model, err := parseEngineModelValue("--research-role", opts.ResearchRole)
		if err != nil {
			return err
		}
		cfg.Roles.Research.Engine = engine
		cfg.Roles.Research.Model = model
	}
	if opts.DevelopRole != "" {
		engine, model, err := parseEngineModelValue("--develop-role", opts.DevelopRole)
		if err != nil {
			return err
		}
		cfg.Roles.Develop.Engine = engine
		cfg.Roles.Develop.Model = model
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

func syncLegacySessionFallback(cfg *goalx.Config) {
	if cfg == nil {
		return
	}
	roleDefault := goalx.EffectiveSessionConfig(cfg, -1)
	cfg.Engine = roleDefault.Engine
	cfg.Model = roleDefault.Model
}
