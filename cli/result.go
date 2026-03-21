package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	goalx "github.com/vonbai/goalx"
)

type selectionJSON struct {
	Kept   string `json:"kept"`
	Branch string `json:"branch"`
}

// Result prints the saved result for a run. Research runs print summary.md,
// develop runs print the kept session plus branch history and diff stat.
func Result(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx result [NAME]")
	}

	runDir, err := resolveSavedRunDir(projectRoot, runName)
	if err != nil {
		return err
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(runDir, "goalx.yaml"))
	if err != nil {
		return fmt.Errorf("load saved config: %w", err)
	}

	if cfg.Mode == goalx.ModeResearch {
		data, err := os.ReadFile(filepath.Join(runDir, "summary.md"))
		if err != nil {
			return fmt.Errorf("read summary: %w", err)
		}
		fmt.Print(string(data))
		return nil
	}

	selection, err := loadResultSelection(projectRoot, runDir, cfg.Name)
	if err != nil {
		return err
	}

	fmt.Printf("Kept: %s\n", selection.Kept)
	logOut, err := exec.Command("git", "-C", projectRoot, "log", "--oneline", "-5", selection.Branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git log %s: %w: %s", selection.Branch, err, logOut)
	}
	fmt.Print(string(logOut))

	diffOut, err := exec.Command("git", "-C", projectRoot, "show", "--stat", "--format=", selection.Branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git show %s: %w: %s", selection.Branch, err, diffOut)
	}
	fmt.Print(string(diffOut))
	return nil
}

func resolveSavedRunDir(projectRoot, runName string) (string, error) {
	savesDir := filepath.Join(projectRoot, ".goalx", "runs")
	if runName == "" {
		_, runDir, err := findLatestSavedRun(savesDir, "")
		if err != nil {
			return "", fmt.Errorf("find latest saved run: %w", err)
		}
		return runDir, nil
	}
	return filepath.Join(savesDir, runName), nil
}

func loadResultSelection(projectRoot, savedRunDir, runName string) (*selectionJSON, error) {
	for _, path := range []string{
		filepath.Join(savedRunDir, "selection.json"),
		filepath.Join(goalx.RunDir(projectRoot, runName), "selection.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var selection selectionJSON
		if err := json.Unmarshal(data, &selection); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if selection.Kept == "" || selection.Branch == "" {
			return nil, fmt.Errorf("selection in %s is incomplete", path)
		}
		return &selection, nil
	}

	return nil, fmt.Errorf("selection.json not found for develop run %q", runName)
}
