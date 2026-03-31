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
	Selection    *SelectionSnapshot
	Context      goalx.ContextConfig
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
	context, sessionNames, err := CollectSavedPhaseContext(runDir, cfg.Context)
	if err != nil {
		return nil, fmt.Errorf("collect saved run context for %q: %w", runName, err)
	}
	selection, err := LoadSelectionSnapshot(SelectionSnapshotPath(runDir))
	if err != nil {
		return nil, fmt.Errorf("load saved selection snapshot for %q: %w", runName, err)
	}
	meta, _ := LoadRunMetadata(filepath.Join(runDir, "run-metadata.json"))
	return &savedPhaseSource{
		Run:          runName,
		Dir:          runDir,
		Mode:         cfg.Mode,
		Parallel:     parallel,
		Metadata:     meta,
		Selection:    selection,
		Context:      context,
		SessionNames: sessionNames,
	}, nil
}

func derivePhaseRunName(sourceRun, phaseKind string, explicit string) string {
	if explicit != "" {
		return explicit
	}
	if sourceRun == "" {
		return goalx.Slugify(phaseKind)
	}
	base := goalx.Slugify(sourceRun)
	suffix := goalx.Slugify(phaseKind)
	if base == "" {
		return suffix
	}
	if suffix == "" {
		return base
	}
	const maxSlugLen = 60
	maxBaseLen := maxSlugLen - len(suffix) - 1
	if maxBaseLen <= 0 {
		return suffix
	}
	if len(base) > maxBaseLen {
		base = strings.TrimRight(base[:maxBaseLen], "-")
	}
	if base == "" {
		return suffix
	}
	return base + "-" + suffix
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

func runPhaseAction(projectRoot string, spec phaseActionSpec, opts phaseOptions) error {
	source, err := loadSavedPhaseSource(projectRoot, opts.From)
	if err != nil {
		return err
	}
	if len(source.Context.Files) == 0 && len(source.Context.Refs) == 0 {
		return fmt.Errorf(spec.NoContextErr, source.Dir)
	}

	resolved, err := resolvePhaseConfig(projectRoot, spec.Kind, spec.Mode, source, opts)
	if err != nil {
		return err
	}
	cfg := &resolved.Config
	context, err := phaseContext(projectRoot, cfg, source, opts.ContextPaths)
	if err != nil {
		return err
	}
	dimensions, err := applyPhaseDimensions(opts)
	if err != nil {
		return err
	}

	applySessionHints(cfg, spec.DefaultHints(source))
	applySessionDimensions(cfg, dimensions, opts)
	cfg.Context = context

	if opts.WriteConfig {
		if err := writePhaseConfig(projectRoot, cfg, fmt.Sprintf(spec.DraftHeader, source.Run)); err != nil {
			return err
		}
		fmt.Printf("Generated manual draft %s (%s from %s)\n", ManualDraftConfigPath(projectRoot), spec.Kind, source.Run)
		fmt.Println("\n  Next: review .goalx/goalx.yaml, then goalx start --config .goalx/goalx.yaml")
		return nil
	}

	selectionSnapshot := BuildSelectionSnapshot(cfg, resolved.SelectionPolicy, resolved.ExplicitSelection)
	return startWithConfig(projectRoot, cfg, resolved.Engines, selectionSnapshot, phaseRunMetadataPatch(source, spec.Kind), false)
}

func resolvePhaseConfig(projectRoot string, phaseKind string, mode goalx.Mode, source *savedPhaseSource, opts phaseOptions) (*goalx.ResolvedConfig, error) {
	if source == nil {
		return nil, fmt.Errorf("saved phase source is required")
	}
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load config layers: %w", err)
	}
	req, err := buildPhaseResolveRequest(projectRoot, phaseKind, mode, source, layers.Config, opts)
	if err != nil {
		return nil, err
	}
	resolved, err := goalx.ResolveConfig(layers, req)
	if err != nil {
		return nil, err
	}
	if source.Selection != nil && !phaseSelectionOverrideRequested(opts) {
		applySelectionSnapshotConfig(&resolved.Config, source.Selection)
		resolved.SelectionPolicy = copySelectionPolicy(source.Selection.Policy)
		resolved.ExplicitSelection = source.Selection.ExplicitSelection
	}
	if opts.BudgetSet {
		resolved.Config.Budget.MaxDuration = opts.Budget
	}
	return resolved, nil
}

func buildPhaseResolveRequest(projectRoot string, phaseKind string, mode goalx.Mode, source *savedPhaseSource, baseCfg goalx.Config, opts phaseOptions) (goalx.ResolveRequest, error) {
	if source == nil {
		return goalx.ResolveRequest{}, fmt.Errorf("saved phase source is required")
	}
	_ = projectRoot
	_ = baseCfg
	masterOverride, workerOverride, err := launchRoleOverrides(launchOptions{
		Master:       opts.Master,
		Worker:       opts.Worker,
		Effort:       opts.Effort,
		MasterEffort: opts.MasterEffort,
		WorkerEffort: opts.WorkerEffort,
	})
	if err != nil {
		return goalx.ResolveRequest{}, err
	}
	parallel := opts.Parallel
	if parallel < 1 {
		parallel = source.Parallel
	}
	req := goalx.ResolveRequest{
		Name:                      phaseRunName(projectRoot, source.Run, phaseKind, opts.Name),
		Mode:                      mode,
		Objective:                 resolvePhaseObjective(phaseKind, source.Run, opts.Objective),
		Parallel:                  parallel,
		ClearSessions:             true,
		MasterOverride:            masterOverride,
		WorkerOverride:            workerOverride,
		RequireEngineAvailability: true,
	}
	if opts.Readonly {
		req.TargetOverride = &goalx.TargetConfig{Readonly: []string{"."}}
	}
	return req, nil
}

func phaseRunName(projectRoot, sourceRun, phaseKind, explicit string) string {
	if explicit != "" {
		return explicit
	}
	return nextAvailableRunName(projectRoot, derivePhaseRunName(sourceRun, phaseKind, ""))
}

func phaseSelectionOverrideRequested(opts phaseOptions) bool {
	return strings.TrimSpace(opts.Master) != "" ||
		strings.TrimSpace(opts.Worker) != "" ||
		opts.Effort != "" ||
		opts.MasterEffort != "" ||
		opts.WorkerEffort != ""
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

func phaseContext(projectRoot string, cfg *goalx.Config, source *savedPhaseSource, extra []string) (goalx.ContextConfig, error) {
	merged := goalx.ContextConfig{}
	if cfg != nil {
		merged = MergeContextConfigs(merged, cfg.Context)
	}
	if source != nil {
		merged = MergeContextConfigs(merged, source.Context)
	}
	if len(extra) == 0 {
		return merged, nil
	}
	resolved, err := ResolveContextInputsFrom(projectRoot, extra)
	if err != nil {
		return goalx.ContextConfig{}, fmt.Errorf("resolve context: %w", err)
	}
	return MergeContextConfigs(merged, resolved), nil
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
	if len(dimensions) == 0 && opts.Effort == "" {
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
	data, err := yaml.Marshal(manualDraftRenderableConfig(cfg))
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, append([]byte(header), data...), 0o644)
}
