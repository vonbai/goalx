package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoverCognitionScopeIncludesRepoNativeAndPinnedNPXGitNexus(t *testing.T) {
	prev := lookPathFunc
	prevStatus := gitNexusStatusFunc
	t.Cleanup(func() {
		lookPathFunc = prev
		gitNexusStatusFunc = prevStatus
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
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		return "Repository not indexed.\nRun: gitnexus analyze\n", nil
	}

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
	if len(scope.Providers[0].ReadTransportsSupported) != 0 || scope.Providers[0].MCPServerCommand != "" {
		t.Fatalf("repo-native provider should not expose mcp capability facts: %+v", scope.Providers[0])
	}
	if scope.Providers[1].Name != "gitnexus" || scope.Providers[1].InvocationKind != "npx" || scope.Providers[1].Version != gitNexusPinnedVersion {
		t.Fatalf("gitnexus provider = %+v, want pinned npx", scope.Providers[1])
	}
	if scope.Providers[1].IndexState != "missing" {
		t.Fatalf("gitnexus index_state = %q, want missing", scope.Providers[1].IndexState)
	}
	if got := scope.Providers[1].ReadTransportsSupported; len(got) != 2 || got[0] != "cli" || got[1] != "mcp" {
		t.Fatalf("gitnexus read_transports_supported = %#v, want cli+mcp", got)
	}
	if scope.Providers[1].MCPServerCommand == "" || len(scope.Providers[1].MCPToolsSupported) == 0 || len(scope.Providers[1].MCPResourcesSupported) == 0 {
		t.Fatalf("gitnexus mcp capability facts missing: %+v", scope.Providers[1])
	}
}

func TestBuildContextIndexIncludesCognitionProviderFacts(t *testing.T) {
	prev := lookPathFunc
	prevStatus := gitNexusStatusFunc
	t.Cleanup(func() {
		lookPathFunc = prev
		gitNexusStatusFunc = prevStatus
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
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		return "Repository not indexed.\nRun: gitnexus analyze\n", nil
	}

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
		"Provider: `repo-native invocation=builtin available=true index_state=fresh",
		"Provider: `gitnexus invocation=npx available=true version=" + gitNexusPinnedVersion + " index_state=missing",
		"read_transports=cli,mcp",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestBuildAffordancesIncludesCognitionFacts(t *testing.T) {
	prev := lookPathFunc
	prevStatus := gitNexusStatusFunc
	t.Cleanup(func() {
		lookPathFunc = prev
		gitNexusStatusFunc = prevStatus
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
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		return "Repository not indexed.\nRun: gitnexus analyze\n", nil
	}

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
	prevStatus := gitNexusStatusFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusStatusFunc = prevStatus
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
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		if invocationKind != "npx" {
			t.Fatalf("probe invocation_kind = %q, want npx", invocationKind)
		}
		if strings.TrimSpace(scopePath) == "" {
			t.Fatal("probe scopePath unexpectedly empty")
		}
		return "", fmt.Errorf("npx probe failed")
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
	if got.IndexState != "unknown" {
		t.Fatalf("index_state = %q, want unknown when status probe fails", got.IndexState)
	}
}

func TestDiscoverCognitionScopeParsesFreshGitNexusStatus(t *testing.T) {
	prevLookPath := lookPathFunc
	prevStatus := gitNexusStatusFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusStatusFunc = prevStatus
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "gitnexus":
			return "/usr/bin/" + name, nil
		default:
			return "", fmt.Errorf("missing")
		}
	}
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		return `Repository: ` + scopePath + `
Indexed: 4/1/2026, 12:00:00 AM
Indexed commit: abc1234
Current commit: abc1234
Status: ✅ up-to-date
`, nil
	}

	repo := makeTrackedRepo(t)
	scope, err := DiscoverCognitionScope("run-root", repo)
	if err != nil {
		t.Fatalf("DiscoverCognitionScope: %v", err)
	}
	got := scope.Providers[1]
	if !got.Available || got.IndexState != "fresh" {
		t.Fatalf("provider = %+v, want available fresh gitnexus", got)
	}
	if got.IndexedRevision == "" || got.HeadRevision == "" {
		t.Fatalf("provider = %+v, want indexed and head revisions", got)
	}
}

func TestDiscoverCognitionScopeParsesStaleGitNexusStatus(t *testing.T) {
	prevLookPath := lookPathFunc
	prevStatus := gitNexusStatusFunc
	t.Cleanup(func() {
		lookPathFunc = prevLookPath
		gitNexusStatusFunc = prevStatus
	})
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git", "gitnexus":
			return "/usr/bin/" + name, nil
		default:
			return "", fmt.Errorf("missing")
		}
	}
	repo := makeTrackedRepo(t)
	writeAndCommit(t, repo, "second.txt", "next\n", "next")
	headOut, err := exec.Command("git", "-C", repo, "rev-parse", "--short=7", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse --short=7 HEAD: %v", err)
	}
	baseOut, err := exec.Command("git", "-C", repo, "rev-parse", "--short=7", "HEAD~1").Output()
	if err != nil {
		t.Fatalf("git rev-parse --short=7 HEAD~1: %v", err)
	}
	head := strings.TrimSpace(string(headOut))
	base := strings.TrimSpace(string(baseOut))
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		return fmt.Sprintf("Repository: %s\nIndexed: 4/1/2026, 12:00:00 AM\nIndexed commit: %s\nCurrent commit: %s\nStatus: ⚠️ stale (re-run gitnexus analyze)\n", scopePath, base, head), nil
	}

	scope, err := DiscoverCognitionScope("run-root", repo)
	if err != nil {
		t.Fatalf("DiscoverCognitionScope: %v", err)
	}
	got := scope.Providers[1]
	if !got.Available || got.IndexState != "stale" {
		t.Fatalf("provider = %+v, want available stale gitnexus", got)
	}
	if got.StaleCommits < 1 {
		t.Fatalf("provider = %+v, want stale_commits > 0", got)
	}
}

func TestGitNexusRefreshAnalyzesMissingIndexAndReturnsFreshState(t *testing.T) {
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
	repo := makeTrackedRepo(t)
	callCount := 0
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		callCount++
		if callCount == 1 {
			return "Repository not indexed.\nRun: gitnexus analyze\n", nil
		}
		headOut, err := exec.Command("git", "-C", repo, "rev-parse", "--short=7", "HEAD").Output()
		if err != nil {
			t.Fatalf("git rev-parse --short=7 HEAD: %v", err)
		}
		head := strings.TrimSpace(string(headOut))
		return fmt.Sprintf("Repository: %s\nIndexed: 4/1/2026, 12:00:00 AM\nIndexed commit: %s\nCurrent commit: %s\nStatus: ✅ up-to-date\n", scopePath, head, head), nil
	}
	analyzeCalls := 0
	gitNexusAnalyzeFunc = func(invocationKind, scopePath string) error {
		analyzeCalls++
		return nil
	}

	got, err := (gitNexusCognitionProvider{}).Refresh(repo)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if analyzeCalls != 1 {
		t.Fatalf("analyze calls = %d, want 1", analyzeCalls)
	}
	if !got.Available || got.IndexState != "fresh" {
		t.Fatalf("provider = %+v, want refreshed fresh state", got)
	}
}

func TestGitNexusRefreshSkipsAnalyzeWhenIndexAlreadyFresh(t *testing.T) {
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
	repo := makeTrackedRepo(t)
	headOut, err := exec.Command("git", "-C", repo, "rev-parse", "--short=7", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse --short=7 HEAD: %v", err)
	}
	head := strings.TrimSpace(string(headOut))
	gitNexusStatusFunc = func(invocationKind, scopePath string) (string, error) {
		return fmt.Sprintf("Repository: %s\nIndexed: 4/1/2026, 12:00:00 AM\nIndexed commit: %s\nCurrent commit: %s\nStatus: ✅ up-to-date\n", scopePath, head, head), nil
	}
	analyzeCalls := 0
	gitNexusAnalyzeFunc = func(invocationKind, scopePath string) error {
		analyzeCalls++
		return nil
	}

	got, err := (gitNexusCognitionProvider{}).Refresh(repo)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if analyzeCalls != 0 {
		t.Fatalf("analyze calls = %d, want 0", analyzeCalls)
	}
	if !got.Available || got.IndexState != "fresh" {
		t.Fatalf("provider = %+v, want fresh state without analyze", got)
	}
}

func TestRunGitNexusAnalyzeRestoresContextAndSkillSideEffects(t *testing.T) {
	repo := t.TempDir()
	prevExec := gitNexusAnalyzeExecFunc
	defer func() { gitNexusAnalyzeExecFunc = prevExec }()

	originalClaude := "original claude\n"
	if err := os.WriteFile(filepath.Join(repo, "CLAUDE.md"), []byte(originalClaude), 0o644); err != nil {
		t.Fatalf("write CLAUDE.md: %v", err)
	}
	skillsDir := filepath.Join(repo, ".claude", "skills", "gitnexus")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("original skill\n"), 0o644); err != nil {
		t.Fatalf("write existing skill: %v", err)
	}

	gitNexusAnalyzeExecFunc = func(invocationKind, scopePath string) error {
		if err := os.WriteFile(filepath.Join(scopePath, "AGENTS.md"), []byte("generated agents\n"), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(scopePath, "CLAUDE.md"), []byte("generated claude\n"), 0o644); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Join(scopePath, ".claude", "skills", "gitnexus"), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(scopePath, ".claude", "skills", "gitnexus", "SKILL.md"), []byte("generated skill\n"), 0o644); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(scopePath, ".claude", "skills", "gitnexus", "extra.md"), []byte("extra\n"), 0o644); err != nil {
			return err
		}
		return nil
	}

	if err := runGitNexusAnalyze("binary", repo); err != nil {
		t.Fatalf("runGitNexusAnalyze: %v", err)
	}

	claudeData, err := os.ReadFile(filepath.Join(repo, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if string(claudeData) != originalClaude {
		t.Fatalf("CLAUDE.md = %q, want restored original", string(claudeData))
	}
	if _, err := os.Stat(filepath.Join(repo, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("AGENTS.md should be removed after guarded analyze, stat err = %v", err)
	}
	skillData, err := os.ReadFile(filepath.Join(skillsDir, "SKILL.md"))
	if err != nil {
		t.Fatalf("read restored skill: %v", err)
	}
	if string(skillData) != "original skill\n" {
		t.Fatalf("skill file = %q, want restored original", string(skillData))
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "extra.md")); !os.IsNotExist(err) {
		t.Fatalf("extra generated skill side effect should be removed, stat err = %v", err)
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
