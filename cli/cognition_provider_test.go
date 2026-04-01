package cli

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

func TestDiscoverCognitionScopeIncludesRepoNativeAndPinnedNPXGitNexus(t *testing.T) {
	prev := lookPathFunc
	prevProbe := gitNexusProbeFunc
	t.Cleanup(func() {
		lookPathFunc = prev
		gitNexusProbeFunc = prevProbe
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git":
			return "/usr/bin/git", nil
		case "gitnexus":
			return "", fmt.Errorf("missing")
		case "npx":
			return "/usr/bin/npx", nil
		default:
			return "", fmt.Errorf("missing")
		}
	}
	gitNexusProbeFunc = func(invocationKind, scopePath string) error { return nil }

	repo := makeTrackedRepo(t)
	scope, err := DiscoverCognitionScope("run-root", repo)
	if err != nil {
		t.Fatalf("DiscoverCognitionScope: %v", err)
	}
	if len(scope.Providers) != 2 {
		t.Fatalf("providers = %#v, want repo-native + gitnexus", scope.Providers)
	}
	if scope.Providers[0].Name != "repo-native" || scope.Providers[0].InvocationKind != "builtin" {
		t.Fatalf("repo-native provider = %+v, want builtin", scope.Providers[0])
	}
	if scope.Providers[1].Name != "gitnexus" || scope.Providers[1].InvocationKind != "npx" || scope.Providers[1].Version != gitNexusPinnedVersion {
		t.Fatalf("gitnexus provider = %+v, want pinned npx", scope.Providers[1])
	}
}

func TestBuildContextIndexIncludesCognitionProviderFacts(t *testing.T) {
	prev := lookPathFunc
	prevProbe := gitNexusProbeFunc
	t.Cleanup(func() {
		lookPathFunc = prev
		gitNexusProbeFunc = prevProbe
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "npx":
			return "/usr/bin/" + name, nil
		case "gitnexus":
			return "", fmt.Errorf("missing")
		case "claude", "codex", "tmux":
			return "", fmt.Errorf("missing")
		default:
			return "", fmt.Errorf("missing")
		}
	}
	gitNexusProbeFunc = func(invocationKind, scopePath string) error { return nil }

	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if len(index.CognitionScopes) != 1 {
		t.Fatalf("cognition_scopes = %#v, want one scope", index.CognitionScopes)
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{
		"## Cognition",
		"Provider: `repo-native invocation=builtin available=true",
		"Provider: `gitnexus invocation=npx available=true version=" + gitNexusPinnedVersion,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildAffordancesIncludesCognitionFacts(t *testing.T) {
	prev := lookPathFunc
	prevProbe := gitNexusProbeFunc
	t.Cleanup(func() {
		lookPathFunc = prev
		gitNexusProbeFunc = prevProbe
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "npx":
			return "/usr/bin/" + name, nil
		case "gitnexus":
			return "", fmt.Errorf("missing")
		case "claude", "codex", "tmux":
			return "", fmt.Errorf("missing")
		default:
			return "", fmt.Errorf("missing")
		}
	}
	gitNexusProbeFunc = func(invocationKind, scopePath string) error { return nil }

	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	doc, err := BuildAffordances(repo, cfg.Name, runDir, "master")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}
	found := false
	for _, item := range doc.Items {
		if item.ID != "cognition" {
			continue
		}
		found = true
		if !strings.Contains(strings.Join(item.Facts, "\n"), "provider=gitnexus invocation=npx available=true version="+gitNexusPinnedVersion) {
			t.Fatalf("cognition affordance facts = %#v, want pinned gitnexus npx fact", item.Facts)
		}
	}
	if !found {
		t.Fatalf("affordances missing cognition item: %#v", doc.Items)
	}
}

func TestDiscoverCognitionScopeMarksNPXGitNexusUnavailableWhenProbeFails(t *testing.T) {
	prevLookPath := lookPathFunc
	prevProbe := gitNexusProbeFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusProbeFunc = prevProbe
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "npx":
			return "/usr/bin/" + name, nil
		case "gitnexus":
			return "", fmt.Errorf("missing")
		default:
			return "", fmt.Errorf("missing")
		}
	}
	gitNexusProbeFunc = func(invocationKind, scopePath string) error {
		if invocationKind != "npx" {
			t.Fatalf("probe invocation_kind = %q, want npx", invocationKind)
		}
		if strings.TrimSpace(scopePath) == "" {
			t.Fatal("probe scopePath unexpectedly empty")
		}
		return fmt.Errorf("npx probe failed")
	}

	repo := makeTrackedRepo(t)
	scope, err := DiscoverCognitionScope("run-root", repo)
	if err != nil {
		t.Fatalf("DiscoverCognitionScope: %v", err)
	}
	if len(scope.Providers) != 2 {
		t.Fatalf("providers = %#v, want repo-native + gitnexus", scope.Providers)
	}
	got := scope.Providers[1]
	if got.Name != "gitnexus" {
		t.Fatalf("provider = %+v, want gitnexus", got)
	}
	if got.InvocationKind != "npx" {
		t.Fatalf("invocation_kind = %q, want npx", got.InvocationKind)
	}
	if got.Available {
		t.Fatalf("available = true, want false when npx probe fails: %+v", got)
	}
}

func makeTrackedRepo(t *testing.T) string {
	t.Helper()
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "hi\n", "init")
	if err := exec.Command("git", "-C", repo, "rev-parse", "--verify", "HEAD").Run(); err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	return repo
}
