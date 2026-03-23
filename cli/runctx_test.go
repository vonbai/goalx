package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestResolveRunPrefersFocusedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	runName := "beta"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: beta\nmode: develop\nobjective: keep moving\n"), 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}

	if err := SaveProjectRegistry(projectRoot, &ProjectRegistry{
		Version:    1,
		FocusedRun: runName,
		ActiveRuns: map[string]ProjectRunRef{
			"alpha": {Name: "alpha", State: "active"},
			"beta":  {Name: "beta", State: "active"},
		},
	}); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	rc, err := ResolveRun(projectRoot, "")
	if err != nil {
		t.Fatalf("ResolveRun: %v", err)
	}
	if rc.Name != runName {
		t.Fatalf("ResolveRun name = %q, want %q", rc.Name, runName)
	}
}

func TestResolveRunUsesLocalRunForBareExplicitName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectA := initNamedGitRepo(t, "project-a")
	projectB := initNamedGitRepo(t, "project-b")
	writeAndCommit(t, projectA, "README.md", "base", "base commit")
	writeAndCommit(t, projectB, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "global-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDirA := writeRunSpecFixture(t, projectA, cfg)
	if _, err := EnsureRunMetadata(runDirA, projectA, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := RegisterActiveRun(projectA, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}
	runDirB := writeRunSpecFixture(t, projectB, cfg)
	if _, err := EnsureRunMetadata(runDirB, projectB, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := RegisterActiveRun(projectB, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}

	rc, err := ResolveRun(projectB, cfg.Name)
	if err != nil {
		t.Fatalf("ResolveRun: %v", err)
	}
	if rc.ProjectRoot != projectB {
		t.Fatalf("ResolveRun project root = %q, want %q", rc.ProjectRoot, projectB)
	}
	if rc.RunDir != runDirB {
		t.Fatalf("ResolveRun run dir = %q, want %q", rc.RunDir, runDirB)
	}
	if rc.TmuxSession != goalx.TmuxSessionName(projectB, cfg.Name) {
		t.Fatalf("ResolveRun tmux session = %q, want %q", rc.TmuxSession, goalx.TmuxSessionName(projectB, cfg.Name))
	}
}

func TestResolveRunDoesNotUseBareGlobalLookup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectA := initNamedGitRepo(t, "project-a")
	projectB := initNamedGitRepo(t, "project-b")
	projectC := initNamedGitRepo(t, "project-c")
	writeAndCommit(t, projectA, "README.md", "base", "base commit")
	writeAndCommit(t, projectB, "README.md", "base", "base commit")
	writeAndCommit(t, projectC, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "shared-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	for _, root := range []string{projectA, projectB} {
		runDir := writeRunSpecFixture(t, root, cfg)
		if _, err := EnsureRunMetadata(runDir, root, cfg.Objective); err != nil {
			t.Fatalf("EnsureRunMetadata(%s): %v", root, err)
		}
		if err := RegisterActiveRun(root, cfg); err != nil {
			t.Fatalf("RegisterActiveRun(%s): %v", root, err)
		}
	}

	_, err := ResolveRun(projectC, cfg.Name)
	if err == nil {
		t.Fatal("ResolveRun succeeded, want not found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("ResolveRun error = %v, want not found message", err)
	}
}

func TestResolveRunUsesExplicitProjectSelectorForCrossProjectRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectA := initNamedGitRepo(t, "project-a")
	projectB := initNamedGitRepo(t, "project-b")
	writeAndCommit(t, projectA, "README.md", "base", "base commit")
	writeAndCommit(t, projectB, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "global-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, projectA, cfg)
	if _, err := EnsureRunMetadata(runDir, projectA, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := RegisterActiveRun(projectA, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}

	selector := goalx.ProjectID(projectA) + "/" + cfg.Name
	rc, err := ResolveRun(projectB, selector)
	if err != nil {
		t.Fatalf("ResolveRun: %v", err)
	}
	if rc.ProjectRoot != projectA {
		t.Fatalf("ResolveRun project root = %q, want %q", rc.ProjectRoot, projectA)
	}
}

func TestResolveRunUsesRunIDSelectorAcrossProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectA := initNamedGitRepo(t, "project-a")
	projectB := initNamedGitRepo(t, "project-b")
	writeAndCommit(t, projectA, "README.md", "base", "base commit")
	writeAndCommit(t, projectB, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "global-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, projectA, cfg)
	meta, err := EnsureRunMetadata(runDir, projectA, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := RegisterActiveRun(projectA, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}
	reg, err := LoadGlobalRunRegistry()
	if err != nil {
		t.Fatalf("LoadGlobalRunRegistry: %v", err)
	}
	ref := reg.Runs[globalRunKey(projectA, cfg.Name)]
	ref.RunID = ""
	reg.Runs[globalRunKey(projectA, cfg.Name)] = ref
	if err := SaveGlobalRunRegistry(reg); err != nil {
		t.Fatalf("SaveGlobalRunRegistry: %v", err)
	}

	rc, err := ResolveRun(projectB, meta.RunID)
	if err != nil {
		t.Fatalf("ResolveRun: %v", err)
	}
	if rc.ProjectRoot != projectA {
		t.Fatalf("ResolveRun project root = %q, want %q", rc.ProjectRoot, projectA)
	}
}

func TestResolveRunPrefersLocalRunNameThatLooksLikeRunID(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initNamedGitRepo(t, "project-a")
	writeAndCommit(t, projectRoot, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "run_demo",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, projectRoot, cfg)
	if _, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := RegisterActiveRun(projectRoot, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}

	rc, err := ResolveRun(projectRoot, cfg.Name)
	if err != nil {
		t.Fatalf("ResolveRun: %v", err)
	}
	if rc.Name != cfg.Name {
		t.Fatalf("ResolveRun name = %q, want %q", rc.Name, cfg.Name)
	}
}

func writeRunSpecFixture(t *testing.T, projectRoot string, cfg *goalx.Config) string {
	t.Helper()

	runDir := goalx.RunDir(projectRoot, cfg.Name)
	for _, dir := range []string{
		runDir,
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "worktrees"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal run spec: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	return runDir
}

func initNamedGitRepo(t *testing.T, name string) string {
	t.Helper()

	repo, err := os.MkdirTemp("", name+"-")
	if err != nil {
		t.Fatalf("mkdir temp repo %s: %v", name, err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(repo) })
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	return repo
}
