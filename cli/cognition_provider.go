package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const gitNexusPinnedVersion = "1.5.0"

var lookPathFunc = exec.LookPath
var gitNexusStatusFunc = loadGitNexusStatus
var gitNexusAnalyzeFunc = runGitNexusAnalyze
var gitNexusAnalyzeExecFunc = executeGitNexusAnalyze

type CognitionProvider interface {
	Name() string
	Discover(scopePath string) (CognitionProviderState, error)
	Refresh(scopePath string) (CognitionProviderState, error)
}

type repoNativeCognitionProvider struct{}
type gitNexusCognitionProvider struct{}

func (repoNativeCognitionProvider) Name() string { return "repo-native" }
func (gitNexusCognitionProvider) Name() string   { return "gitnexus" }

func DiscoverCognitionScope(scopeName, scopePath string) (CognitionScopeState, error) {
	scopePath = strings.TrimSpace(scopePath)
	if scopePath == "" {
		return CognitionScopeState{}, fmt.Errorf("cognition scope path is required")
	}
	providers := []CognitionProvider{
		repoNativeCognitionProvider{},
		gitNexusCognitionProvider{},
	}
	scope := CognitionScopeState{
		Scope:        strings.TrimSpace(scopeName),
		WorktreePath: scopePath,
		Providers:    []CognitionProviderState{},
	}
	for _, provider := range providers {
		state, err := provider.Discover(scopePath)
		if err != nil {
			return CognitionScopeState{}, err
		}
		scope.Providers = append(scope.Providers, state)
	}
	return scope, nil
}

func (repoNativeCognitionProvider) Discover(scopePath string) (CognitionProviderState, error) {
	headRevision, _ := gitRevisionIfAvailable(scopePath, "HEAD")
	repoRoot, _ := gitRepoRootIfAvailable(scopePath)
	return CognitionProviderState{
		Name:           "repo-native",
		InvocationKind: "builtin",
		Available:      true,
		IndexState:     "fresh",
		RepoRoot:       repoRoot,
		HeadRevision:   headRevision,
		Capabilities:   []string{"file_inventory", "file_search", "file_read", "git_diff"},
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (provider repoNativeCognitionProvider) Refresh(scopePath string) (CognitionProviderState, error) {
	return provider.Discover(scopePath)
}

func (gitNexusCognitionProvider) Discover(scopePath string) (CognitionProviderState, error) {
	repoRoot, _ := gitRepoRootIfAvailable(scopePath)
	headRevision, _ := gitRevisionIfAvailable(scopePath, "HEAD")
	storagePath := filepath.Join(scopePath, ".gitnexus")
	if _, err := lookPathFunc("gitnexus"); err == nil {
		return discoverGitNexusProviderState("binary", scopePath, repoRoot, storagePath, headRevision), nil
	}
	if _, err := lookPathFunc("npx"); err == nil {
		return discoverGitNexusProviderState("npx", scopePath, repoRoot, storagePath, headRevision), nil
	}
	return CognitionProviderState{
		Name:           "gitnexus",
		InvocationKind: "none",
		Available:      false,
		IndexState:     "unknown",
		RepoRoot:       repoRoot,
		StoragePath:    storagePath,
		HeadRevision:   headRevision,
		Capabilities:   []string{"query", "context", "impact", "detect_changes", "processes"},
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (provider gitNexusCognitionProvider) Refresh(scopePath string) (CognitionProviderState, error) {
	state, err := provider.Discover(scopePath)
	if err != nil {
		return state, err
	}
	if !state.Available {
		return state, nil
	}
	switch state.IndexState {
	case "missing", "stale":
		if err := gitNexusAnalyzeFunc(state.InvocationKind, scopePath); err != nil {
			state.LastRefreshError = err.Error()
			return state, nil
		}
		state, err = provider.Discover(scopePath)
		if err != nil {
			return state, err
		}
		state.IndexProvenance = "local"
		state.AnalyzedInScopeAt = time.Now().UTC().Format(time.RFC3339)
		return state, nil
	default:
		return state, nil
	}
}

func discoverGitNexusProviderState(invocationKind, scopePath, repoRoot, storagePath, headRevision string) CognitionProviderState {
	state := CognitionProviderState{
		Name:                    "gitnexus",
		InvocationKind:          strings.TrimSpace(invocationKind),
		Available:               false,
		ReadTransportsSupported: []string{"cli", "mcp"},
		MCPServerCommand:        buildGitNexusMCPServerCommand(strings.TrimSpace(invocationKind)),
		MCPToolsSupported:       []string{"list_repos", "query", "context", "impact", "detect_changes", "rename"},
		MCPResourcesSupported:   []string{"gitnexus://repos", "gitnexus://repo/{name}/context", "gitnexus://repo/{name}/processes", "gitnexus://repo/{name}/process/{name}", "gitnexus://repo/{name}/clusters", "gitnexus://repo/{name}/schema"},
		RepoRoot:                strings.TrimSpace(repoRoot),
		StoragePath:             strings.TrimSpace(storagePath),
		HeadRevision:            strings.TrimSpace(headRevision),
		Capabilities:            []string{"query", "context", "impact", "detect_changes", "processes"},
		IndexState:              "unknown",
		CheckedAt:               time.Now().UTC().Format(time.RFC3339),
	}
	switch state.InvocationKind {
	case "binary":
		state.Command = "gitnexus"
	case "npx":
		state.Command = "npx -y gitnexus@" + gitNexusPinnedVersion
		state.Version = gitNexusPinnedVersion
	}
	if strings.TrimSpace(state.RepoRoot) != "" {
		state.RegistryName = filepath.Base(state.RepoRoot)
	}
	output, err := gitNexusStatusFunc(state.InvocationKind, scopePath)
	if err != nil {
		state.LastRefreshError = err.Error()
		return state
	}
	state.Available = true
	applyGitNexusStatus(&state, output)
	return state
}

func buildGitNexusMCPServerCommand(invocationKind string) string {
	switch strings.TrimSpace(invocationKind) {
	case "binary":
		return "gitnexus mcp"
	case "npx":
		return "npx -y gitnexus@" + gitNexusPinnedVersion + " mcp"
	default:
		return ""
	}
}

func applyGitNexusStatus(state *CognitionProviderState, output string) {
	if state == nil {
		return
	}
	text := strings.TrimSpace(output)
	if text == "" {
		state.IndexState = "unknown"
		return
	}
	if strings.Contains(text, "Repository not indexed.") || strings.Contains(text, "stale KuzuDB index") {
		state.IndexState = "missing"
		return
	}
	shortIndexed := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Repository:"):
			if repoRoot := strings.TrimSpace(strings.TrimPrefix(line, "Repository:")); repoRoot != "" {
				state.RepoRoot = repoRoot
				state.RegistryName = filepath.Base(repoRoot)
			}
		case strings.HasPrefix(line, "Indexed commit:"):
			shortIndexed = strings.TrimSpace(strings.TrimPrefix(line, "Indexed commit:"))
		case strings.HasPrefix(line, "Current commit:"):
			if current := strings.TrimSpace(strings.TrimPrefix(line, "Current commit:")); current != "" && state.HeadRevision == "" {
				state.HeadRevision = current
			}
		case strings.HasPrefix(line, "Status:"):
			statusText := strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
			switch {
			case strings.Contains(statusText, "up-to-date"):
				state.IndexState = "fresh"
			case strings.Contains(statusText, "stale"):
				state.IndexState = "stale"
			default:
				state.IndexState = "unknown"
			}
		}
	}
	if shortIndexed != "" {
		if full, err := gitRevisionIfAvailable(state.RepoRoot, shortIndexed); err == nil && strings.TrimSpace(full) != "" {
			state.IndexedRevision = strings.TrimSpace(full)
		} else {
			state.IndexedRevision = shortIndexed
		}
	}
	if state.IndexState == "fresh" && state.IndexedRevision == "" && state.HeadRevision != "" {
		state.IndexedRevision = state.HeadRevision
	}
	if state.IndexState == "stale" && state.IndexedRevision != "" && state.HeadRevision != "" {
		if staleCommits, err := gitAheadCountIfAvailable(state.RepoRoot, state.IndexedRevision, state.HeadRevision); err == nil && staleCommits > 0 {
			state.StaleCommits = staleCommits
		}
	}
}

func gitRevisionIfAvailable(worktreePath, rev string) (string, error) {
	if _, err := lookPathFunc("git"); err != nil {
		return "", err
	}
	return gitRevision(worktreePath, rev)
}

func gitRepoRootIfAvailable(worktreePath string) (string, error) {
	if _, err := lookPathFunc("git"); err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--show-toplevel").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel in %s: %w: %s", worktreePath, err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func gitAheadCountIfAvailable(worktreePath, baseRevision, headRevision string) (int, error) {
	if _, err := lookPathFunc("git"); err != nil {
		return 0, err
	}
	worktreePath = strings.TrimSpace(worktreePath)
	if worktreePath == "" {
		return 0, fmt.Errorf("git ahead count worktree path is required")
	}
	out, err := exec.Command("git", "-C", worktreePath, "rev-list", "--count", strings.TrimSpace(baseRevision)+".."+strings.TrimSpace(headRevision)).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("git rev-list --count %s..%s in %s: %w: %s", baseRevision, headRevision, worktreePath, err, out)
	}
	value := strings.TrimSpace(string(out))
	if value == "" {
		return 0, nil
	}
	var count int
	if _, err := fmt.Sscanf(value, "%d", &count); err != nil {
		return 0, fmt.Errorf("parse git ahead count %q: %w", value, err)
	}
	return count, nil
}

func loadGitNexusStatus(invocationKind, scopePath string) (string, error) {
	var cmd *exec.Cmd
	switch strings.TrimSpace(invocationKind) {
	case "binary":
		cmd = exec.Command("gitnexus", "status")
	case "npx":
		cmd = exec.Command("npx", "-y", "gitnexus@"+gitNexusPinnedVersion, "status")
	default:
		return "", fmt.Errorf("unknown gitnexus status invocation_kind %q", invocationKind)
	}
	cmd.Dir = scopePath
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gitnexus status via %s: %w: %s", invocationKind, err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func runGitNexusAnalyze(invocationKind, scopePath string) error {
	return withGitNexusSideEffectGuard(scopePath, func() error {
		return gitNexusAnalyzeExecFunc(invocationKind, scopePath)
	})
}

func executeGitNexusAnalyze(invocationKind, scopePath string) error {
	var cmd *exec.Cmd
	switch strings.TrimSpace(invocationKind) {
	case "binary":
		cmd = exec.Command("gitnexus", "analyze", "--skip-agents-md", scopePath)
	case "npx":
		cmd = exec.Command("npx", "-y", "gitnexus@"+gitNexusPinnedVersion, "analyze", "--skip-agents-md", scopePath)
	default:
		return fmt.Errorf("unknown gitnexus analyze invocation_kind %q", invocationKind)
	}
	cmd.Dir = scopePath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gitnexus analyze via %s: %w: %s", invocationKind, err, strings.TrimSpace(string(out)))
	}
	return nil
}
