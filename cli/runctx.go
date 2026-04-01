package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// RunContext holds resolved paths for a run.
type RunContext struct {
	Name        string
	RunDir      string
	TmuxSession string
	ProjectRoot string
	Config      *goalx.Config
}

var errRunNotFound = errors.New("run not found")

// ResolveRun resolves run context. If runName is empty, it resolves the
// focused/only active run from the project registry.
func ResolveRun(projectRoot, runName string) (*RunContext, error) {
	if runName == "" {
		var err error
		runName, err = ResolveDefaultRunName(projectRoot)
		if err != nil {
			return nil, err
		}
	}

	return resolveExplicitRun(projectRoot, runName)
}

func resolveExplicitRun(projectRoot, selector string) (*RunContext, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, fmt.Errorf("run %q not found", selector)
	}
	projectID, runName := parseRunSelector(selector)
	if projectID != "" {
		if projectID == goalx.ProjectID(projectRoot) {
			return resolveLocalRun(projectRoot, runName)
		}
		return resolveRunFromGlobalRegistry(selector)
	}
	if rc, err := resolveLocalRun(projectRoot, selector); err == nil {
		return rc, nil
	} else if !isNotFoundRunError(err) {
		return nil, err
	}
	if isRunIDSelector(selector) {
		if rc, err := resolveRunFromGlobalRegistry(selector); err == nil {
			return rc, nil
		} else if !isNotFoundRunError(err) {
			return nil, err
		}
	}
	rc, err := resolveLocalRun(projectRoot, selector)
	if err == nil || !isNotFoundRunError(err) {
		return rc, err
	}
	hint, hintErr := crossProjectRunHint(projectRoot, selector)
	if hintErr != nil {
		return nil, hintErr
	}
	if hint != "" {
		return nil, fmt.Errorf("%w; %s", errRunNotFound, hint)
	}
	return nil, err
}

func resolveRunFromGlobalRegistry(selector string) (*RunContext, error) {
	matches, err := LookupGlobalRuns(selector)
	if err != nil {
		return nil, err
	}
	switch len(matches) {
	case 0:
		return nil, errRunNotFound
	case 1:
		return buildRunContext(matches[0].ProjectRoot, matches[0].RunDir, matches[0].Name)
	default:
		candidates := make([]string, 0, len(matches))
		for _, match := range matches {
			candidates = append(candidates, match.ProjectID+"/"+match.Name)
		}
		return nil, fmt.Errorf("multiple runs named %q: %s (use --run <project-id>/<run>)", selector, joinRunCandidates(candidates))
	}
}

func resolveLocalRun(projectRoot, selector string) (*RunContext, error) {
	projectID, runName := parseRunSelector(selector)
	if projectID != "" && projectID != goalx.ProjectID(projectRoot) {
		return nil, fmt.Errorf("run %q not found", selector)
	}

	// Try configured run root first, then legacy location
	runDirs := resolveRunDirCandidates(projectRoot, runName)
	for _, runDir := range runDirs {
		if _, err := os.Stat(RunSpecPath(runDir)); err == nil {
			return buildRunContext(projectRoot, runDir, runName)
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat run spec: %w", err)
		}
	}
	return nil, errRunNotFound
}

// resolveRunDirCandidates returns candidate run directories in priority order:
// 1. Configured run_root (if set)
// 2. Legacy ~/.goalx/runs/{projectID}/{runName}
func resolveRunDirCandidates(projectRoot, runName string) []string {
	var candidates []string

	// Check if project has a configured run_root
	if layers, err := goalx.LoadConfigLayers(projectRoot); err == nil && layers.Config.RunRoot != "" {
		configuredDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
		candidates = append(candidates, configuredDir)
	}

	// Always include legacy location as fallback
	legacyDir := goalx.RunDir(projectRoot, runName)
	candidates = append(candidates, legacyDir)

	return candidates
}

func buildRunContext(projectRoot, runDir, runName string) (*RunContext, error) {
	snapshot, err := LoadRunSpec(runDir)
	if err != nil {
		return nil, fmt.Errorf("load run spec: %w", err)
	}
	rc := &RunContext{
		Name:        runName,
		RunDir:      runDir,
		TmuxSession: goalx.TmuxSessionName(projectRoot, runName),
		ProjectRoot: projectRoot,
		Config:      snapshot,
	}
	if err := repairCompletedRunFinalization(rc); err != nil {
		return nil, err
	}
	return rc, nil
}

func isNotFoundRunError(err error) bool {
	return errors.Is(err, errRunNotFound)
}

func joinRunCandidates(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	out := candidates[0]
	for i := 1; i < len(candidates); i++ {
		out += ", " + candidates[i]
	}
	return out
}

func crossProjectRunHint(projectRoot, selector string) (string, error) {
	matches, err := LookupGlobalRuns(selector)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", nil
	}
	candidates := make([]string, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, match.ProjectID+"/"+match.Name)
	}
	return fmt.Sprintf("run %q not found in current project %q; use --run <project-id>/<run> (%s)", selector, goalx.ProjectID(projectRoot), joinRunCandidates(candidates)), nil
}
