package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	goalx "github.com/vonbai/goalx"
)

// relaunchMaster recreates the master window from durable run state.
func relaunchMaster(projectRoot, runDir, tmuxSession string, cfg *goalx.Config) error {
	if cfg == nil {
		return fmt.Errorf("run config is nil")
	}
	effectiveCfg, err := configWithSelectionSnapshot(runDir, cfg)
	if err != nil {
		return err
	}

	engines, err := loadEngineCatalog(projectRoot)
	if err != nil {
		return fmt.Errorf("load config for engine resolution: %w", err)
	}
	spec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: effectiveCfg.Master.Engine,
		Model:  effectiveCfg.Master.Model,
		Effort: effectiveCfg.Master.Effort,
	})
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	engineCmd := spec.Command
	protocolPath := filepath.Join(runDir, "master.md")
	prompt := goalx.ResolvePrompt(engines, effectiveCfg.Master.Engine, protocolPath)

	meta, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective)
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	if err := EnsureSuccessCompilation(projectRoot, runDir, effectiveCfg, meta); err != nil {
		return fmt.Errorf("compile success plane: %w", err)
	}
	if err := ensureExperimentsSurface(runDir); err != nil {
		return fmt.Errorf("init experiments surface: %w", err)
	}
	masterData, err := buildMasterProtocolData(projectRoot, runDir, tmuxSession, effectiveCfg, engines, engineCmd, meta)
	if err != nil {
		return fmt.Errorf("build master protocol data: %w", err)
	}
	if err := RenderMasterProtocol(masterData, runDir); err != nil {
		return fmt.Errorf("render master protocol: %w", err)
	}
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	checkSec, _ := normalizeSidecarInterval(cfg.Master.CheckInterval)
	masterLeaseTTL := time.Duration(checkSec) * time.Second * 2
	workdir := RunWorktreePath(runDir)
	launchCmd := buildMasterLaunchCommand(goalxBin, cfg.Name, runDir, meta.RunID, meta.Epoch, masterLeaseTTL, engineCmd, prompt)

	if !SessionExists(tmuxSession) {
		if err := NewSessionWithCommand(tmuxSession, "master", workdir, launchCmd); err != nil {
			return fmt.Errorf("create master session: %w", err)
		}
		return nil
	}
	_ = KillWindow(tmuxSession, "master")
	if err := NewWindowWithCommand(tmuxSession, "master", workdir, launchCmd); err != nil {
		return fmt.Errorf("create master window: %w", err)
	}
	return nil
}

func relaunchMissingMasterWindow(projectRoot, runDir, tmuxSession string, cfg *goalx.Config) error {
	if cfg == nil {
		return fmt.Errorf("run config is nil")
	}
	effectiveCfg, err := configWithSelectionSnapshot(runDir, cfg)
	if err != nil {
		return err
	}
	masterPresence, err := LoadTargetPresenceFact(runDir, tmuxSession, "master")
	if err != nil {
		return fmt.Errorf("load master target presence: %w", err)
	}
	appendAuditLog(runDir, "target_relaunch_attempt target=master cause=%s session_exists=%t window_exists=%t", blankAsUnknown(masterPresence.State), masterPresence.SessionExists, masterPresence.WindowExists)
	if !masterPresence.SessionExists {
		err := fmt.Errorf("tmux session missing for master relaunch")
		appendAuditLog(runDir, "target_relaunch_result target=master result=failure cause=%s err=%v", blankAsUnknown(masterPresence.State), err)
		return err
	}

	engines, err := loadEngineCatalog(projectRoot)
	if err != nil {
		return fmt.Errorf("load config for engine resolution: %w", err)
	}
	spec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: effectiveCfg.Master.Engine,
		Model:  effectiveCfg.Master.Model,
		Effort: effectiveCfg.Master.Effort,
	})
	if err != nil {
		return fmt.Errorf("resolve engine: %w", err)
	}
	engineCmd := spec.Command
	protocolPath := filepath.Join(runDir, "master.md")
	prompt := goalx.ResolvePrompt(engines, effectiveCfg.Master.Engine, protocolPath)

	meta, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective)
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	if err := EnsureSuccessCompilation(projectRoot, runDir, effectiveCfg, meta); err != nil {
		appendAuditLog(runDir, "target_relaunch_result target=master result=failure cause=%s err=%v", blankAsUnknown(masterPresence.State), err)
		return fmt.Errorf("compile success plane: %w", err)
	}
	if err := ensureExperimentsSurface(runDir); err != nil {
		return fmt.Errorf("init experiments surface: %w", err)
	}
	masterData, err := buildMasterProtocolData(projectRoot, runDir, tmuxSession, effectiveCfg, engines, engineCmd, meta)
	if err != nil {
		return fmt.Errorf("build master protocol data: %w", err)
	}
	if err := RenderMasterProtocol(masterData, runDir); err != nil {
		return fmt.Errorf("render master protocol: %w", err)
	}
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable: %w", err)
	}
	checkSec, _ := normalizeSidecarInterval(cfg.Master.CheckInterval)
	masterLeaseTTL := time.Duration(checkSec) * time.Second * 2
	workdir := RunWorktreePath(runDir)
	launchCmd := buildMasterLaunchCommand(goalxBin, cfg.Name, runDir, meta.RunID, meta.Epoch, masterLeaseTTL, engineCmd, prompt)

	if err := NewWindowWithCommand(tmuxSession, "master", workdir, launchCmd); err != nil {
		appendAuditLog(runDir, "target_relaunch_result target=master result=failure cause=%s err=%v", blankAsUnknown(masterPresence.State), err)
		return fmt.Errorf("create master window: %w", err)
	}
	appendAuditLog(runDir, "target_relaunch_result target=master result=success cause=%s", blankAsUnknown(masterPresence.State))
	return nil
}

func configWithSelectionSnapshot(runDir string, cfg *goalx.Config) (*goalx.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("run config is nil")
	}
	copyCfg := *cfg
	snapshot, err := LoadSelectionSnapshot(SelectionSnapshotPath(runDir))
	if err != nil {
		return nil, fmt.Errorf("load selection snapshot: %w", err)
	}
	if snapshot != nil {
		applySelectionSnapshotConfig(&copyCfg, snapshot)
	}
	return &copyCfg, nil
}
