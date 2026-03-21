package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

// Implement generates a goalx.yaml for a develop round based on prior research/debate consensus.
func Implement(projectRoot string, args []string) error {
	savesDir := filepath.Join(projectRoot, ".goalx", "runs")

	// Prefer debate run (saved as mode=research, name=debate), then any research run
	var run, runDir string
	debateDir := filepath.Join(savesDir, "debate")
	if debateCfg, err2 := goalx.LoadYAML[goalx.Config](filepath.Join(debateDir, "goalx.yaml")); err2 == nil && debateCfg.Mode == goalx.ModeResearch {
		run, runDir = "debate", debateDir
	} else {
		var err error
		run, runDir, err = findLatestSavedRun(savesDir, goalx.ModeResearch)
		if err != nil {
			return fmt.Errorf("no saved research or debate run found in .goalx/runs/: %w", err)
		}
	}

	// Collect context files (summary + reports, absolute paths)
	var contextFiles []string
	absRunDir, _ := filepath.Abs(runDir)
	entries, _ := os.ReadDir(runDir)
	for _, e := range entries {
		name := e.Name()
		if name == "summary.md" || strings.HasSuffix(name, "-report.md") {
			contextFiles = append(contextFiles, filepath.Join(absRunDir, name))
		}
	}
	if len(contextFiles) == 0 {
		return fmt.Errorf("no reports/summary found in %s", runDir)
	}

	// Load the base config to get harness from project config
	baseCfg, _, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load base config: %w", err)
	}

	harness := baseCfg.Harness.Command
	if harness == "" {
		harness = "go build ./... && go test ./... -count=1 && go vet ./..."
	}
	targetFiles := baseCfg.Target.Files
	if len(targetFiles) == 0 {
		targetFiles = []string{"."}
	}

	// Read saved config for objective context
	savedCfg, _ := goalx.LoadYAML[goalx.Config](filepath.Join(runDir, "goalx.yaml"))
	objContext := savedCfg.Objective
	if objContext == "" {
		objContext = run
	}
	preset := savedCfg.Preset
	if preset == "" {
		preset = "claude"
	}
	parallel := savedCfg.Parallel
	if parallel < 1 {
		parallel = 2
	}

	cfg := goalx.Config{
		Name:      "implement",
		Mode:      goalx.ModeDevelop,
		Objective: fmt.Sprintf("实施 %s 的共识修复清单。严格按照 context 中的文档执行，不做额外改动。", run),
		Preset:    preset,
		Parallel:  parallel,
		DiversityHints: []string{
			"你负责优先级最高的修复项（P0 + P1 中不依赖其他文件的项）。逐个修复，每个修完跑一次 gate 验证。",
			"你负责剩余修复项（P2 + 重构类 P1）。先做独立的删除/清理，再做涉及多文件的重构。每步跑 gate。",
		},
		Context: goalx.ContextConfig{Files: contextFiles},
		Target: goalx.TargetConfig{
			Files: targetFiles,
		},
		Harness: goalx.HarnessConfig{Command: harness},
		Budget:  goalx.BudgetConfig{MaxDuration: 2 * 3600_000_000_000},
	}
	goalx.ApplyPreset(&cfg)

	goalxDir := filepath.Join(projectRoot, ".goalx")
	os.MkdirAll(goalxDir, 0755)
	outPath := filepath.Join(goalxDir, "goalx.yaml")
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("# goalx.yaml — implement fixes from %s\n", run)
	if err := os.WriteFile(outPath, append([]byte(header), data...), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated %s (implement from %s)\n", outPath, run)
	fmt.Printf("  context: %d files from .goalx/runs/%s/\n", len(contextFiles), run)
	fmt.Printf("  harness: %s\n", harness)
	fmt.Println("\n  Next: review goalx.yaml (check target.files + objective), then goalx start")
	return nil
}
