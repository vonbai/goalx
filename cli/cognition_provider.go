package cli

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

const gitNexusPinnedVersion = "1.5.0"

var lookPathFunc = exec.LookPath
var gitNexusProbeFunc = probeGitNexusInvocation

type CognitionProvider interface {
	Name() string
	Discover(scopePath string) (CognitionProviderState, error)
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
		RepoRoot:       repoRoot,
		HeadRevision:   headRevision,
		Capabilities:   []string{"file_inventory", "file_search", "file_read", "git_diff"},
	}, nil
}

func (gitNexusCognitionProvider) Discover(scopePath string) (CognitionProviderState, error) {
	repoRoot, _ := gitRepoRootIfAvailable(scopePath)
	headRevision, _ := gitRevisionIfAvailable(scopePath, "HEAD")
	storagePath := filepath.Join(scopePath, ".gitnexus")
	if _, err := lookPathFunc("gitnexus"); err == nil {
		available := gitNexusProbeFunc("binary", scopePath) == nil
		return CognitionProviderState{
			Name:           "gitnexus",
			InvocationKind: "binary",
			Available:      available,
			Command:        "gitnexus",
			RepoRoot:       repoRoot,
			StoragePath:    storagePath,
			HeadRevision:   headRevision,
			Capabilities:   []string{"query", "context", "impact", "detect_changes", "processes"},
		}, nil
	}
	if _, err := lookPathFunc("npx"); err == nil {
		available := gitNexusProbeFunc("npx", scopePath) == nil
		return CognitionProviderState{
			Name:           "gitnexus",
			InvocationKind: "npx",
			Available:      available,
			Command:        "npx -y gitnexus@" + gitNexusPinnedVersion,
			Version:        gitNexusPinnedVersion,
			RepoRoot:       repoRoot,
			StoragePath:    storagePath,
			HeadRevision:   headRevision,
			Capabilities:   []string{"query", "context", "impact", "detect_changes", "processes"},
		}, nil
	}
	return CognitionProviderState{
		Name:           "gitnexus",
		InvocationKind: "none",
		Available:      false,
		RepoRoot:       repoRoot,
		StoragePath:    storagePath,
		HeadRevision:   headRevision,
		Capabilities:   []string{"query", "context", "impact", "detect_changes", "processes"},
	}, nil
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

func probeGitNexusInvocation(invocationKind, scopePath string) error {
	var cmd *exec.Cmd
	switch strings.TrimSpace(invocationKind) {
	case "binary":
		cmd = exec.Command("gitnexus", "status")
	case "npx":
		cmd = exec.Command("npx", "-y", "gitnexus@"+gitNexusPinnedVersion, "status")
	default:
		return fmt.Errorf("unknown gitnexus probe invocation_kind %q", invocationKind)
	}
	cmd.Dir = scopePath
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("probe gitnexus via %s: %w: %s", invocationKind, err, strings.TrimSpace(string(out)))
	}
	return nil
}
