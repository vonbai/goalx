package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

type savedPhaseSource struct {
	Run          string
	Dir          string
	Mode         goalx.Mode
	Parallel     int
	Metadata     *RunMetadata
	Context      []string
	SessionNames []string
}

type phaseActionSpec struct {
	Kind         string
	Mode         goalx.Mode
	NoContextErr string
	DraftHeader  string
	DefaultHints func(*savedPhaseSource) []string
}

func loadSavedPhaseSource(projectRoot, runName string) (*savedPhaseSource, error) {
	runName = strings.TrimSpace(runName)
	if runName == "" {
		return nil, fmt.Errorf("saved run name is required")
	}
	location, err := ResolveSavedRunLocation(projectRoot, runName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("saved run %q not found", runName)
		}
		return nil, err
	}
	runDir := location.Dir
	cfg, err := LoadSavedRunSpec(runDir)
	if err != nil {
		return nil, fmt.Errorf("load saved run %q: %w", runName, err)
	}
	parallel := cfg.Parallel
	if parallel < len(cfg.Sessions) {
		parallel = len(cfg.Sessions)
	}
	contextFiles, sessionNames, err := CollectSavedResearchContext(runDir)
	if err != nil {
		return nil, fmt.Errorf("collect saved run context for %q: %w", runName, err)
	}
	meta, _ := LoadRunMetadata(filepath.Join(runDir, "run-metadata.json"))
	return &savedPhaseSource{
		Run:          runName,
		Dir:          runDir,
		Mode:         cfg.Mode,
		Parallel:     parallel,
		Metadata:     meta,
		Context:      contextFiles,
		SessionNames: sessionNames,
	}, nil
}

func derivePhaseRunName(sourceRun, phaseKind string, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if sourceRun == "" {
		return phaseKind
	}
	return goalx.Slugify(sourceRun + "-" + phaseKind)
}

func phaseSourceKind(source *savedPhaseSource) string {
	if source == nil {
		return ""
	}
	if source.Metadata != nil && source.Metadata.PhaseKind != "" {
		return source.Metadata.PhaseKind
	}
	if source.Mode != "" {
		return string(source.Mode)
	}
	return ""
}

func phaseRunMetadataPatch(source *savedPhaseSource, phaseKind string) *RunMetadata {
	patch := &RunMetadata{Intent: phaseKind, PhaseKind: phaseKind}
	if source == nil {
		return patch
	}
	patch.SourceRun = source.Run
	patch.SourcePhase = phaseSourceKind(source)
	patch.ParentRun = source.Run
	if source.Metadata == nil {
		return patch
	}
	if source.Metadata.RootRunID != "" {
		patch.RootRunID = source.Metadata.RootRunID
	} else if source.Metadata.RunID != "" {
		patch.RootRunID = source.Metadata.RunID
	}
	return patch
}

func runPhaseAction(projectRoot string, spec phaseActionSpec, opts phaseOptions, nc *nextConfigJSON) error {
	opts = mergeNextConfigIntoPhaseOptions(opts, nc, spec.Mode)

	source, err := loadSavedPhaseSource(projectRoot, opts.From)
	if err != nil {
		return err
	}
	if len(source.Context) == 0 {
		return fmt.Errorf(spec.NoContextErr, source.Dir)
	}

	cfg, engines, err := resolvePhaseConfig(projectRoot, spec.Kind, spec.Mode, source, opts)
	if err != nil {
		return err
	}
	contextFiles, err := phaseContextFiles(cfg, source, opts.ContextPaths)
	if err != nil {
		return err
	}
	dimensions, err := applyPhaseDimensions(opts)
	if err != nil {
		return err
	}

	applySessionHints(cfg, spec.DefaultHints(source))
	applySessionDimensions(cfg, dimensions, opts)
	cfg.Context = goalx.ContextConfig{Files: contextFiles, Refs: cfg.Context.Refs}

	if opts.WriteConfig {
		if err := writePhaseConfig(projectRoot, cfg, fmt.Sprintf(spec.DraftHeader, source.Run)); err != nil {
			return err
		}
		fmt.Printf("Generated manual draft %s (%s from %s)\n", ManualDraftConfigPath(projectRoot), spec.Kind, source.Run)
		fmt.Println("\n  Next: review .goalx/goalx.yaml, then goalx start --config .goalx/goalx.yaml")
		return nil
	}

	return startWithConfig(projectRoot, cfg, engines, phaseRunMetadataPatch(source, spec.Kind), false)
}

func resolvePhaseConfig(projectRoot string, phaseKind string, mode goalx.Mode, source *savedPhaseSource, opts phaseOptions) (*goalx.Config, map[string]goalx.EngineConfig, error) {
	if source == nil {
		return nil, nil, fmt.Errorf("saved phase source is required")
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, nil, fmt.Errorf("load config layers: %w", err)
	}
	req, err := buildPhaseResolveRequest(projectRoot, phaseKind, mode, source, layers.Config, opts)
	if err != nil {
		return nil, nil, err
	}
	resolved, err := goalx.ResolveConfig(layers, req)
	if err != nil {
		return nil, nil, err
	}
	if opts.BudgetSet {
		resolved.Config.Budget.MaxDuration = opts.Budget
	}
	return &resolved.Config, resolved.Engines, nil
}

func buildPhaseResolveRequest(projectRoot string, phaseKind string, mode goalx.Mode, source *savedPhaseSource, baseCfg goalx.Config, opts phaseOptions) (goalx.ResolveRequest, error) {
	if source == nil {
		return goalx.ResolveRequest{}, fmt.Errorf("saved phase source is required")
	}
	_ = projectRoot
	_ = baseCfg
	masterOverride, researchOverride, developOverride, err := launchRoleOverrides(launchOptions{
		Master:         opts.Master,
		ResearchRole:   opts.ResearchRole,
		DevelopRole:    opts.DevelopRole,
		Effort:         opts.Effort,
		MasterEffort:   opts.MasterEffort,
		ResearchEffort: opts.ResearchEffort,
		DevelopEffort:  opts.DevelopEffort,
	})
	if err != nil {
		return goalx.ResolveRequest{}, err
	}
	parallel := opts.Parallel
	if parallel < 1 {
		parallel = source.Parallel
	}
	req := goalx.ResolveRequest{
		Name:             derivePhaseRunName(source.Run, phaseKind, opts.Name),
		Mode:             mode,
		Objective:        resolvePhaseObjective(phaseKind, source.Run, opts.Objective),
		Preset:           opts.Preset,
		Parallel:         parallel,
		ClearSessions:    true,
		MasterOverride:   masterOverride,
		ResearchOverride: researchOverride,
		DevelopOverride:  developOverride,
		RequireEngineAvailability: true,
	}
	return req, nil
}

func resolvePhaseObjective(phaseKind string, sourceRun string, explicit string) string {
	if explicit != "" {
		return explicit
	}
	switch phaseKind {
	case "debate":
		return fmt.Sprintf("基于 %s 的独立调研报告，辩论分歧点并达成共识，输出统一的优先级修复清单。", sourceRun)
	case "implement":
		return fmt.Sprintf("实施 %s 的共识修复清单。严格按照 context 中的文档执行，不做额外改动。", sourceRun)
	case "explore":
		return fmt.Sprintf("基于 %s 的已有研究结果，继续扩展探索、验证盲点、寻找更优路径，并产出新的可执行切片。", sourceRun)
	default:
		return sourceRun
	}
}

func phaseContextFiles(cfg *goalx.Config, source *savedPhaseSource, extra []string) ([]string, error) {
	base := make([]string, 0)
	if cfg != nil {
		base = append(base, cfg.Context.Files...)
	}
	if source != nil {
		base = append(base, source.Context...)
	}
	return mergePhaseContext(base, extra)
}

func mergePhaseContext(base []string, extra []string) ([]string, error) {
	if len(extra) == 0 {
		return append([]string(nil), base...), nil
	}
	resolved, err := DiscoverContextFiles(extra)
	if err != nil {
		return nil, fmt.Errorf("discover context: %w", err)
	}
	merged := append([]string(nil), base...)
	seen := map[string]bool{}
	for _, path := range merged {
		seen[path] = true
	}
	for _, path := range resolved {
		if !seen[path] {
			merged = append(merged, path)
			seen[path] = true
		}
	}
	return merged, nil
}

func applyPhaseDimensions(opts phaseOptions) ([]string, error) {
	if len(opts.Dimensions) == 0 {
		return nil, nil
	}
	if _, err := goalx.ResolveDimensionSpecs(opts.Dimensions); err != nil {
		return nil, err
	}
	return append([]string(nil), opts.Dimensions...), nil
}

func applySessionHints(cfg *goalx.Config, hints []string) {
	if cfg == nil {
		return
	}
	size := cfg.Parallel
	if size < len(hints) {
		size = len(hints)
	}
	if size == 0 {
		cfg.Sessions = nil
		return
	}
	cfg.Sessions = make([]goalx.SessionConfig, size)
	for i, hint := range hints {
		cfg.Sessions[i] = goalx.SessionConfig{Hint: hint}
	}
}

func applySessionDimensions(cfg *goalx.Config, dimensions []string, opts phaseOptions) {
	if cfg == nil {
		return
	}
	if len(dimensions) == 0 && opts.RouteRole == "" && opts.RouteProfile == "" && opts.Effort == "" {
		return
	}
	size := cfg.Parallel
	if size < len(cfg.Sessions) {
		size = len(cfg.Sessions)
	}
	if size == 0 {
		size = 1
	}
	sessions := make([]goalx.SessionConfig, size)
	copy(sessions, cfg.Sessions)
	for i := range sessions {
		if len(dimensions) > 0 {
			sessions[i].Dimensions = append([]string(nil), dimensions...)
		}
		if opts.RouteRole != "" {
			sessions[i].RouteRole = opts.RouteRole
		}
		if opts.RouteProfile != "" {
			sessions[i].RouteProfile = opts.RouteProfile
		}
		if opts.Effort != "" && sessions[i].Effort == "" {
			sessions[i].Effort = opts.Effort
		}
	}
	cfg.Sessions = sessions
}

func writePhaseConfig(projectRoot string, cfg *goalx.Config, header string) error {
	goalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		return err
	}
	outPath := ManualDraftConfigPath(projectRoot)
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, append([]byte(header), data...), 0o644)
}
