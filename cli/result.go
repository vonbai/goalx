package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	goalx "github.com/vonbai/goalx"
)

type resultRun struct {
	Dir string
}

// Result prints the canonical run-level result surface.
func Result(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx result [NAME] [--full]") {
		return nil
	}
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

	target, err := resolveResultRun(projectRoot, runName)
	if err != nil {
		return err
	}

	data, err := loadResultSurface(target.Dir)
	if err != nil {
		return err
	}
	if full {
		fmt.Print(string(data))
		return nil
	}
	printRunResult(data)
	return nil
}

func resolveResultRun(projectRoot, runName string) (*resultRun, error) {
	// Load config to get saved_run_root setting
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg := &layers.Config

	location, err := ResolveSavedRunLocationWithConfig(projectRoot, runName, cfg)
	if err == nil {
		_, loadErr := LoadSavedRunSpec(location.Dir)
		if loadErr != nil {
			return nil, fmt.Errorf("load saved config: %w", loadErr)
		}
		return &resultRun{
			Dir: location.Dir,
		}, nil
	}

	var multipleErr MultipleSavedRunsError
	switch {
	case errors.As(err, &multipleErr):
		return nil, fmt.Errorf("%s (specify NAME)", multipleErr.Error())
	case !errors.Is(err, os.ErrNotExist):
		return nil, err
	}

	rc, activeErr := ResolveRun(projectRoot, runName)
	if activeErr == nil {
		return &resultRun{
			Dir: rc.RunDir,
		}, nil
	}

	if strings.TrimSpace(runName) != "" {
		return nil, fmt.Errorf("saved run %q not found", runName)
	}
	return nil, fmt.Errorf("no saved runs found")
}

func loadResultSurface(runDir string) ([]byte, error) {
	summaryPath := SummaryPath(runDir)
	data, err := os.ReadFile(summaryPath)
	if err == nil && len(data) > 0 {
		return data, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read summary: %w", err)
	}
	return nil, fmt.Errorf("summary.md missing at %s; final result is not available yet (use goalx review or inspect reports/ for in-progress outputs)", summaryPath)
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

func printRunResult(data []byte) {
	fmt.Println("=== Result ===")
	fmt.Print(renderResultSummary(data))
	fmt.Println()
	fmt.Println()
	fmt.Println("Full report: goalx result --full")
}

func renderResultSummary(data []byte) string {
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
