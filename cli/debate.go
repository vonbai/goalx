package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

// Debate generates a goalx.yaml for a debate round based on prior research.
// It finds the latest research run in .goalx/runs/, reads its reports,
// and creates a config with opposing diversity hints.
func Debate(projectRoot string, args []string, nc *nextConfigJSON) error {
	// Find the latest saved research run
	savesDir := filepath.Join(projectRoot, ".goalx", "runs")
	run, runDir, err := findLatestSavedRun(savesDir, goalx.ModeResearch)
	if err != nil {
		return fmt.Errorf("no saved research run found in .goalx/runs/: %w", err)
	}
	_, engines, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load base config: %w", err)
	}
	savedCfg, _ := goalx.LoadYAML[goalx.Config](filepath.Join(runDir, "goalx.yaml"))

	// Collect report files (absolute paths for worktree access)
	contextFiles, sessionNames, err := CollectSavedResearchContext(runDir)
	if err != nil {
		return fmt.Errorf("collect saved research context: %w", err)
	}
	if len(contextFiles) == 0 {
		return fmt.Errorf("no reports found in %s", runDir)
	}
	sort.Strings(sessionNames)

	var defaultHints []string
	if len(sessionNames) <= 1 {
		sessionName := "session-1"
		if len(sessionNames) == 1 {
			sessionName = sessionNames[0]
		}
		defaultHints = []string{
			fmt.Sprintf("你是倡导者。用代码证据支持 %s 报告的结论和方案。", sessionName),
			fmt.Sprintf("你是批评者。用代码证据挑战 %s 报告的每一个结论，寻找遗漏和替代方案。", sessionName),
		}
	} else {
		for i, sessionName := range sessionNames {
			others := make([]string, 0, len(sessionNames)-1)
			for j, other := range sessionNames {
				if i != j {
					others = append(others, other)
				}
			}
			defaultHints = append(defaultHints, fmt.Sprintf(
				"你支持 %s 的观点。用代码证据辩护 %s 报告中的结论，挑战 %s 的结论。如果对方证据更强，愿意让步。最终输出共识清单。",
				sessionName, sessionName, strings.Join(others, "、"),
			))
		}
	}

	cfg := buildPhaseConfig(goalx.ModeResearch, savedCfg, engines, nc)
	cfg.Name = "debate"
	cfg.Objective = nextConfigObjective(fmt.Sprintf("基于 %s 的独立调研报告，辩论分歧点并达成共识，输出统一的优先级修复清单", run), nc)
	cfg.DiversityHints = nextConfigHints(defaultHints, cfg.Parallel, nc)
	cfg.Context = goalx.ContextConfig{Files: contextFiles}
	cfg.Target = goalx.TargetConfig{
		Files:    []string{"report.md"},
		Readonly: []string{"."},
	}
	cfg.Harness = goalx.HarnessConfig{Command: "test -s report.md && echo 'ok'"}

	goalx.ApplyPreset(&cfg)

	goalxDir := filepath.Join(projectRoot, ".goalx")
	os.MkdirAll(goalxDir, 0755)
	outPath := filepath.Join(goalxDir, "goalx.yaml")
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return err
	}

	header := fmt.Sprintf("# goalx.yaml — debate round based on %s research\n", run)
	if err := os.WriteFile(outPath, append([]byte(header), data...), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated %s (debate based on %s)\n", outPath, run)
	fmt.Printf("  context: %d files from .goalx/runs/%s/\n", len(contextFiles), run)
	fmt.Println("\n  Next: review goalx.yaml, then goalx start")
	return nil
}

// findLatestSavedRun finds the most recently modified saved run with the given mode.
func findLatestSavedRun(savesDir string, mode goalx.Mode) (string, string, error) {
	entries, err := os.ReadDir(savesDir)
	if err != nil {
		return "", "", err
	}

	type runInfo struct {
		name    string
		dir     string
		modTime int64
	}
	var runs []runInfo

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(savesDir, e.Name())
		cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(dir, "goalx.yaml"))
		if err != nil {
			continue
		}
		if mode != "" && cfg.Mode != mode {
			continue
		}
		info, _ := e.Info()
		t := int64(0)
		if info != nil {
			t = info.ModTime().Unix()
		}
		runs = append(runs, runInfo{e.Name(), dir, t})
	}

	if len(runs) == 0 {
		return "", "", fmt.Errorf("no runs with mode %q", mode)
	}

	sort.Slice(runs, func(i, j int) bool {
		return runs[i].modTime > runs[j].modTime
	})

	return runs[0].name, runs[0].dir, nil
}
