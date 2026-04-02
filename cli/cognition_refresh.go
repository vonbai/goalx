package cli

import (
	"path/filepath"
	"strings"
	"time"
)

func RefreshCognitionStateForRun(runDir, runName string) error {
	if strings.TrimSpace(runDir) == "" {
		return nil
	}
	previous := map[string]map[string]CognitionProviderState{}
	if current, err := LoadCognitionState(CognitionStatePath(runDir)); err != nil {
		return err
	} else if current != nil {
		for _, scope := range current.Scopes {
			if previous[scope.Scope] == nil {
				previous[scope.Scope] = map[string]CognitionProviderState{}
			}
			for _, provider := range scope.Providers {
				previous[scope.Scope][provider.Name] = provider
			}
		}
	}
	scopes := []CognitionScopeState{}

	runRootScope, err := RefreshCognitionScope("run-root", RunWorktreePath(runDir), previous["run-root"])
	if err != nil {
		return err
	}
	scopes = append(scopes, runRootScope)

	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return err
	}
	sessionsState, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return err
	}
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return err
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		worktreePath := resolvedSessionWorktreePath(runDir, cfg.Name, sessionName, sessionsState)
		if strings.TrimSpace(worktreePath) == "" || worktreePath == RunWorktreePath(runDir) {
			continue
		}
		scope, err := RefreshCognitionScope(sessionName, worktreePath, previous[sessionName])
		if err != nil {
			return err
		}
		scopes = append(scopes, scope)
	}

	return SaveCognitionState(CognitionStatePath(runDir), &CognitionState{
		Version: 1,
		Scopes:  scopes,
	})
}

func RefreshCognitionScope(scopeName, scopePath string, previous map[string]CognitionProviderState) (CognitionScopeState, error) {
	scopePath = strings.TrimSpace(scopePath)
	if scopePath == "" {
		return CognitionScopeState{}, nil
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
		state, err := provider.Refresh(scopePath)
		if err != nil {
			return CognitionScopeState{}, err
		}
		if provider.Name() == "gitnexus" {
			prev := CognitionProviderState{}
			if previous != nil {
				prev = previous[provider.Name()]
			}
			state = applyGitNexusScopeTrustModel(scopePath, prev, state)
		}
		scope.Providers = append(scope.Providers, state)
	}
	return scope, nil
}

func applyGitNexusScopeTrustModel(scopePath string, previous, current CognitionProviderState) CognitionProviderState {
	seededMetaPresent := fileExists(filepath.Join(scopePath, ".gitnexus", "meta.json"))
	switch {
	case current.IndexProvenance == "local":
		return current
	case previous.IndexProvenance == "local" && previous.AnalyzedInScopeAt != "":
		current.IndexProvenance = previous.IndexProvenance
		current.AnalyzedInScopeAt = previous.AnalyzedInScopeAt
		return current
	case !seededMetaPresent:
		return current
	case !current.Available:
		current.IndexProvenance = "seeded"
		current.IndexState = "unknown"
		return current
	case current.IndexState == "missing" || current.IndexState == "stale":
		current.IndexProvenance = "seeded"
		return current
	default:
		if previous.IndexProvenance == "" && previous.AnalyzedInScopeAt == "" {
			if err := gitNexusAnalyzeFunc(current.InvocationKind, scopePath); err == nil {
				refreshed := current
				if rediscovered, discoverErr := (gitNexusCognitionProvider{}).Discover(scopePath); discoverErr == nil {
					refreshed = rediscovered
				}
				refreshed.IndexProvenance = "local"
				refreshed.AnalyzedInScopeAt = time.Now().UTC().Format(time.RFC3339)
				return refreshed
			} else {
				current.IndexProvenance = "seeded"
				current.IndexState = "unknown"
				current.LastRefreshError = err.Error()
				return current
			}
		}
		current.IndexProvenance = "seeded"
		return current
	}
}
