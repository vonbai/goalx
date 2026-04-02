package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestRefreshCognitionStateForRunRefreshesRunRootAndSessionScopes(t *testing.T) {
	prevLookPath := lookPathFunc
	prevStatus := gitNexusStatusFunc
	prevAnalyze := gitNexusAnalyzeFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusStatusFunc = prevStatus
		gitNexusAnalyzeFunc = prevAnalyze
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "gitnexus":
			return "/usr/bin/" + name, nil
		default:
			return "", fmt.Errorf("missing")
		}
	}

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	statusCalls := map[string]int{}
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		statusCalls[scopePath]++
		if statusCalls[scopePath] == 1 {
			return "Repository not indexed.\nRun: gitnexus analyze\n", nil
		}
		return fmt.Sprintf("Repository: %s\nIndexed: 4/1/2026, 12:00:00 AM\nIndexed commit: abc1234\nCurrent commit: abc1234\nStatus: ✅ up-to-date\n", scopePath), nil
	}

	analyzeCalls := map[string]int{}
	gitNexusAnalyzeFunc = func(invocationKind, scopePath string) error {
		analyzeCalls[scopePath]++
		return nil
	}

	if err := RefreshCognitionStateForRun(runDir, runName); err != nil {
		t.Fatalf("RefreshCognitionStateForRun: %v", err)
	}

	state, err := LoadCognitionState(CognitionStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadCognitionState: %v", err)
	}
	if state == nil || len(state.Scopes) < 2 {
		t.Fatalf("cognition state = %#v, want run-root and session scope", state)
	}

	runRootFound := false
	sessionFound := false
	for _, scope := range state.Scopes {
		for _, provider := range scope.Providers {
			if provider.Name != "gitnexus" {
				continue
			}
			if provider.IndexState != "fresh" {
				t.Fatalf("scope %s gitnexus provider = %+v, want fresh", scope.Scope, provider)
			}
			switch scope.Scope {
			case "run-root":
				runRootFound = true
			case "session-1":
				sessionFound = true
			}
		}
	}
	if !runRootFound || !sessionFound {
		t.Fatalf("cognition scopes = %#v, want gitnexus for run-root and session-1", state.Scopes)
	}
	if analyzeCalls[RunWorktreePath(runDir)] != 1 {
		t.Fatalf("run-root analyze calls = %d, want 1", analyzeCalls[RunWorktreePath(runDir)])
	}
	if analyzeCalls[WorktreePath(runDir, runName, 1)] != 1 {
		t.Fatalf("session-1 analyze calls = %d, want 1", analyzeCalls[WorktreePath(runDir, runName, 1)])
	}
}

func TestRefreshCognitionScopeForcesLocalAnalyzeForFreshSeededGitNexusCache(t *testing.T) {
	prevLookPath := lookPathFunc
	prevStatus := gitNexusStatusFunc
	prevAnalyze := gitNexusAnalyzeFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusStatusFunc = prevStatus
		gitNexusAnalyzeFunc = prevAnalyze
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "gitnexus":
			return "/usr/bin/" + name, nil
		default:
			return "", fmt.Errorf("missing")
		}
	}

	scopePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scopePath, ".gitnexus"), 0o755); err != nil {
		t.Fatalf("mkdir .gitnexus: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scopePath, ".gitnexus", "meta.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write meta.json: %v", err)
	}

	statusCalls := 0
	gitNexusStatusFunc = func(invocationKind, worktree string) (string, error) {
		statusCalls++
		return fmt.Sprintf("Repository: %s\nIndexed: 4/1/2026, 12:00:00 AM\nIndexed commit: abc1234\nCurrent commit: abc1234\nStatus: ✅ up-to-date\n", worktree), nil
	}
	analyzeCalls := 0
	gitNexusAnalyzeFunc = func(invocationKind, worktree string) error {
		analyzeCalls++
		return nil
	}

	scope, err := RefreshCognitionScope("run-root", scopePath, nil)
	if err != nil {
		t.Fatalf("RefreshCognitionScope: %v", err)
	}
	var provider CognitionProviderState
	for _, item := range scope.Providers {
		if item.Name == "gitnexus" {
			provider = item
			break
		}
	}
	if analyzeCalls != 1 {
		t.Fatalf("analyze calls = %d, want 1 for fresh seeded cache", analyzeCalls)
	}
	if provider.IndexProvenance != "local" || provider.AnalyzedInScopeAt == "" {
		t.Fatalf("provider = %+v, want locally analyzed provenance", provider)
	}
	if statusCalls < 2 {
		t.Fatalf("status calls = %d, want discovery before and after analyze", statusCalls)
	}
}

func TestRefreshCognitionScopeMarksSeededCacheUnknownWhenForcedAnalyzeFails(t *testing.T) {
	prevLookPath := lookPathFunc
	prevStatus := gitNexusStatusFunc
	prevAnalyze := gitNexusAnalyzeFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusStatusFunc = prevStatus
		gitNexusAnalyzeFunc = prevAnalyze
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "gitnexus":
			return "/usr/bin/" + name, nil
		default:
			return "", fmt.Errorf("missing")
		}
	}

	scopePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(scopePath, ".gitnexus"), 0o755); err != nil {
		t.Fatalf("mkdir .gitnexus: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scopePath, ".gitnexus", "meta.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write meta.json: %v", err)
	}

	gitNexusStatusFunc = func(invocationKind, worktree string) (string, error) {
		return fmt.Sprintf("Repository: %s\nIndexed: 4/1/2026, 12:00:00 AM\nIndexed commit: abc1234\nCurrent commit: abc1234\nStatus: ✅ up-to-date\n", worktree), nil
	}
	gitNexusAnalyzeFunc = func(invocationKind, worktree string) error {
		return fmt.Errorf("forced local analyze failed")
	}

	scope, err := RefreshCognitionScope("run-root", scopePath, nil)
	if err != nil {
		t.Fatalf("RefreshCognitionScope: %v", err)
	}
	var provider CognitionProviderState
	for _, item := range scope.Providers {
		if item.Name == "gitnexus" {
			provider = item
			break
		}
	}
	if provider.IndexProvenance != "seeded" || provider.IndexState != "unknown" {
		t.Fatalf("provider = %+v, want seeded unknown after failed forced analyze", provider)
	}
	if provider.LastRefreshError == "" {
		t.Fatalf("provider = %+v, want refresh error recorded", provider)
	}
}
