package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestCreateWorktreeDeletesStaleExistingBranch(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runGit(t, repo, "checkout", "-b", "goalx/demo/1")
	runGit(t, repo, "checkout", "-")

	worktree := filepath.Join(t.TempDir(), "wt")
	err := CreateWorktree(repo, worktree, "goalx/demo/1")
	if err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if err := exec.Command("git", "-C", repo, "rev-parse", "--verify", "goalx/demo/1").Run(); err != nil {
		t.Fatalf("branch ar/demo/1 should still exist: %v", err)
	}
	if _, err := os.Stat(worktree); err != nil {
		t.Fatalf("worktree should exist: %v", err)
	}
}

func TestCreateWorktreeRejectsBranchActiveInAnotherWorktree(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runGit(t, repo, "checkout", "-b", "goalx/demo/1")

	worktree := filepath.Join(t.TempDir(), "wt")
	err := CreateWorktree(repo, worktree, "goalx/demo/1")
	if err == nil || !strings.Contains(err.Error(), "already checked out") {
		t.Fatalf("CreateWorktree error = %v, want already checked out", err)
	}
}

func TestMergeWorktreeRejectsDirtyTree(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runGit(t, repo, "checkout", "-b", "feature")
	writeAndCommit(t, repo, "feature.txt", "feature", "feature commit")
	runGit(t, repo, "checkout", "-")

	if err := os.WriteFile(filepath.Join(repo, "dirty.txt"), []byte("dirty"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	if err := MergeWorktree(repo, "feature"); err == nil {
		t.Fatal("expected MergeWorktree to reject dirty worktree")
	}
}

func TestMergeWorktreeFallsBackToNoFF(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runGit(t, repo, "checkout", "-b", "feature")
	writeAndCommit(t, repo, "feature.txt", "feature", "feature commit")
	runGit(t, repo, "checkout", "-")
	writeAndCommit(t, repo, "main.txt", "main", "main commit")

	if err := MergeWorktree(repo, "feature"); err != nil {
		t.Fatalf("expected MergeWorktree to succeed with non-ff fallback, got: %v", err)
	}

	// Verify the feature file is present after merge.
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); err != nil {
		t.Fatal("feature.txt should exist after merge")
	}
}

func TestMergeWorktreeRejectsConflictsBeforeMerge(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "shared.txt", "base\n", "base commit")

	runGit(t, repo, "checkout", "-b", "feature")
	writeAndCommit(t, repo, "shared.txt", "feature\n", "feature commit")
	runGit(t, repo, "checkout", "-")
	writeAndCommit(t, repo, "shared.txt", "main\n", "main commit")

	err := MergeWorktree(repo, "feature")
	if err == nil || !strings.Contains(err.Error(), "merge conflict detected") {
		t.Fatalf("MergeWorktree error = %v, want merge conflict detected", err)
	}

	status, statErr := exec.Command("git", "-C", repo, "status", "--porcelain").CombinedOutput()
	if statErr != nil {
		t.Fatalf("git status: %v\n%s", statErr, string(status))
	}
	if strings.TrimSpace(string(status)) != "" {
		t.Fatalf("merge precheck should leave repo clean, got:\n%s", string(status))
	}
}

func TestMergeWorktreeAllowsLocalAutoresearchFiles(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runGit(t, repo, "checkout", "-b", "feature")
	writeAndCommit(t, repo, "feature.txt", "feature", "feature commit")
	runGit(t, repo, "checkout", "-")

	os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755)
	if err := os.WriteFile(filepath.Join(repo, ".goalx", "goalx.yaml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".goalx", "config.yaml"), []byte("parallel: 1\n"), 0o644); err != nil {
		t.Fatalf("write .goalx/config.yaml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write .claude/settings.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".codex"), 0o755); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".codex", "config.toml"), []byte("model = \"gpt-5.4\"\n"), 0o644); err != nil {
		t.Fatalf("write .codex/config.toml: %v", err)
	}

	if err := MergeWorktree(repo, "feature"); err != nil {
		t.Fatalf("expected MergeWorktree to allow local goalx files, got: %v", err)
	}
}

func TestHasDirtyWorktreeIgnoresClaudeDir(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	if err := os.MkdirAll(filepath.Join(repo, ".claude"), 0o755); err != nil {
		t.Fatalf("mkdir .claude: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".claude", "settings.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write .claude/settings.json: %v", err)
	}

	dirty, err := hasDirtyWorktree(repo)
	if err != nil {
		t.Fatalf("hasDirtyWorktree: %v", err)
	}
	if dirty {
		t.Fatal("expected .claude dir to be ignored by dirty worktree check")
	}
}

func TestHasDirtyWorktreeIgnoresCodexDir(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	if err := os.MkdirAll(filepath.Join(repo, ".codex"), 0o755); err != nil {
		t.Fatalf("mkdir .codex: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".codex", "config.toml"), []byte("model = \"gpt-5.4\"\n"), 0o644); err != nil {
		t.Fatalf("write .codex/config.toml: %v", err)
	}

	dirty, err := hasDirtyWorktree(repo)
	if err != nil {
		t.Fatalf("hasDirtyWorktree: %v", err)
	}
	if dirty {
		t.Fatal("expected .codex dir to be ignored by dirty worktree check")
	}
}

func TestDropRemovesRunDirectoryAndBranch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName := "drop-run"
	runDir := goalx.RunDir(repo, runName)
	for _, dir := range []string{
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "worktrees"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	snapshot := []byte("name: drop-run\nmode: research\nobjective: demo\ntarget:\n  files: [\"report.md\"]\nharness:\n  command: \"test -f base.txt\"\n")
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session journal: %v", err)
	}

	branch := "goalx/drop-run/1"
	worktreePath := filepath.Join(runDir, "worktrees", "drop-run-1")
	if err := CreateWorktree(repo, worktreePath, branch); err != nil {
		t.Fatalf("CreateWorktree: %v", err)
	}

	if err := Drop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Drop: %v", err)
	}

	if _, err := os.Stat(runDir); !os.IsNotExist(err) {
		t.Fatalf("run dir still exists: %v", err)
	}
	if err := exec.Command("git", "-C", repo, "rev-parse", "--verify", branch).Run(); err == nil {
		t.Fatalf("branch %s should be deleted", branch)
	}
}

func TestDropRefusesUnsavedRunWithArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName := "drop-run"
	runDir := goalx.RunDir(repo, runName)
	for _, dir := range []string{
		runDir,
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "worktrees"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	snapshot := []byte("name: drop-run\nmode: research\nobjective: demo\ntarget:\n  files: [\"report.md\"]\nharness:\n  command: \"test -f base.txt\"\n")
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if _, err := EnsureArtifactsManifest(runDir); err != nil {
		t.Fatalf("EnsureArtifactsManifest: %v", err)
	}
	reportPath := filepath.Join(runDir, "worktrees", "drop-run-1", "report.md")
	if err := os.MkdirAll(filepath.Dir(reportPath), 0o755); err != nil {
		t.Fatalf("mkdir report dir: %v", err)
	}
	if err := os.WriteFile(reportPath, []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := RegisterSessionArtifact(runDir, "session-1", ArtifactMeta{
		Kind:        "report",
		Path:        reportPath,
		RelPath:     "report.md",
		DurableName: "session-1-report.md",
	}); err != nil {
		t.Fatalf("RegisterSessionArtifact: %v", err)
	}

	err := Drop(repo, []string{"--run", runName})
	if err == nil || !strings.Contains(err.Error(), "save") {
		t.Fatalf("Drop error = %v, want save guidance", err)
	}
	if _, err := os.Stat(runDir); err != nil {
		t.Fatalf("run dir should remain after refused drop: %v", err)
	}
}

func TestInitBootstrapsGoalxExcludeRule(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	if err := Init(repo, []string{"audit auth flow", "--research"}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read .git/info/exclude: %v", err)
	}
	if !strings.Contains(string(data), ".goalx/") {
		t.Fatalf("exclude missing .goalx/ rule:\n%s", string(data))
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.name", "Test User")
	runGit(t, repo, "config", "user.email", "test@example.com")
	return repo
}

func writeAndCommit(t *testing.T, repo, name, content, message string) {
	t.Helper()

	path := filepath.Join(repo, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	runGit(t, repo, "add", name)
	runGit(t, repo, "commit", "-m", message)
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

func gitOutput(t *testing.T, repo string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
	return string(out)
}
