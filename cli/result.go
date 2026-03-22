package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	full := false
	var positional []string
	for _, arg := range rest {
		if arg == "--full" {
			full = true
			continue
		}
		positional = append(positional, arg)
	}
	if runName == "" && len(positional) == 1 {
		runName = positional[0]
		positional = nil
	}
	if len(positional) > 0 {
		return fmt.Errorf("usage: goalx result [NAME] [--full]")
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
			reportData, reportErr := loadSavedResearchFallback(runDir)
			if reportErr != nil {
				return fmt.Errorf("read summary: %w", err)
			}
			data = reportData
		}
		if full {
			fmt.Print(string(data))
			return nil
		}
		printResearchResult(data)
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

func loadSavedResearchFallback(runDir string) ([]byte, error) {
	contextFiles, _, err := CollectSavedResearchContext(runDir)
	if err != nil {
		return nil, err
	}
	for _, path := range contextFiles {
		if filepath.Base(path) == "summary.md" {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil && len(data) > 0 {
			return data, nil
		}
	}
	return nil, fmt.Errorf("no saved research report found in %s", runDir)
}

func parseSections(data []byte) map[string]string {
	sections := make(map[string]string)
	var current string
	var body []string

	flush := func() {
		if current == "" {
			return
		}
		sections[current] = strings.TrimSpace(strings.Join(body, "\n"))
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "## ") {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			body = body[:0]
			continue
		}
		if current != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}

func printResearchResult(data []byte) {
	fmt.Println("=== Research Result ===")
	fmt.Print(renderResearchSummary(data))
	fmt.Println()
	fmt.Println()
	fmt.Println("Full report: goalx result --full")
}

func renderResearchSummary(data []byte) string {
	sections := parseSections(data)
	var parts []string

	if recommendation := firstNonEmptyLine(sections["Recommendation"]); recommendation != "" {
		parts = append(parts, "Recommendation: "+recommendation)
	}

	if findings := summarizeSectionLines(sections["Key Findings"], 5); findings != "" {
		parts = append(parts, "Key Findings:\n"+findings)
	}

	if fixes := strings.TrimSpace(sections["Priority Fix List"]); fixes != "" {
		parts = append(parts, "Priority Fix List:\n"+fixes)
	}

	if len(parts) == 0 {
		return strings.TrimSpace(string(data))
	}
	return strings.Join(parts, "\n\n")
}

func firstNonEmptyLine(section string) string {
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

func summarizeSectionLines(section string, limit int) string {
	if limit < 1 {
		return ""
	}

	var lines []string
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return ""
	}
	if len(lines) <= limit {
		return strings.Join(lines, "\n")
	}
	return strings.Join(append(lines[:limit], fmt.Sprintf("... (%d more lines)", len(lines)-limit)), "\n")
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
