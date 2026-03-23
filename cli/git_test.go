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

func TestEnsureProjectGoalxIgnoredOnlyIgnoresManualScratchConfig(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	if err := EnsureProjectGoalxIgnored(repo); err != nil {
		t.Fatalf("EnsureProjectGoalxIgnored: %v", err)
	}

	gitDirOut, err := exec.Command("git", "-C", repo, "rev-parse", "--git-dir").CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse --git-dir: %v\n%s", err, string(gitDirOut))
	}
	excludePath := filepath.Join(strings.TrimSpace(string(gitDirOut)), "info", "exclude")
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(repo, excludePath)
	}
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, ".goalx/goalx.yaml") {
		t.Fatalf("exclude missing scratch config rule:\n%s", text)
	}
	if strings.Contains(text, ".goalx/\n") || strings.Contains(text, ".goalx/\r\n") {
		t.Fatalf("exclude should not blanket-ignore .goalx:\n%s", text)
	}
}

func TestEnsureProjectGoalxIgnoredMigratesLegacyBlanketRule(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	gitDirOut, err := exec.Command("git", "-C", repo, "rev-parse", "--git-dir").CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse --git-dir: %v\n%s", err, string(gitDirOut))
	}
	excludePath := filepath.Join(strings.TrimSpace(string(gitDirOut)), "info", "exclude")
	if !filepath.IsAbs(excludePath) {
		excludePath = filepath.Join(repo, excludePath)
	}
	if err := os.WriteFile(excludePath, []byte("*.log\n.goalx/\n"), 0o644); err != nil {
		t.Fatalf("seed exclude: %v", err)
	}

	if err := EnsureProjectGoalxIgnored(repo); err != nil {
		t.Fatalf("EnsureProjectGoalxIgnored: %v", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, goalxExcludeBegin) || !strings.Contains(text, ".goalx/goalx.yaml") {
		t.Fatalf("exclude missing managed rule:\n%s", text)
	}
	if strings.Contains(text, "\n.goalx/\n") || strings.HasSuffix(strings.TrimSpace(text), ".goalx/") {
		t.Fatalf("legacy blanket ignore should be removed:\n%s", text)
	}
	if !strings.Contains(text, "*.log") {
		t.Fatalf("non-goalx exclude rules should be preserved:\n%s", text)
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
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
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
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
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

func TestDropAcceptsLegacySavedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	runName := "saved-run"
	runDir := goalx.RunDir(repo, runName)
	for _, dir := range []string{
		filepath.Join(runDir, "journals"),
		filepath.Join(runDir, "worktrees"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	snapshot := []byte("name: saved-run\nmode: research\nobjective: demo\ntarget:\n  files: [\"report.md\"]\nharness:\n  command: \"test -f base.txt\"\n")
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	legacySaveDir := LegacySavedRunDir(repo, runName)
	if err := os.MkdirAll(legacySaveDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy save dir: %v", err)
	}

	if err := Drop(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Drop: %v", err)
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
	if !strings.Contains(string(data), ".goalx/goalx.yaml") {
		t.Fatalf("exclude missing .goalx/goalx.yaml rule:\n%s", string(data))
	}
}

func TestDirectCommandHelpPrintUsage(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func() error
		want string
	}{
		{name: "list", run: func() error { return List(t.TempDir(), []string{"--help"}) }, want: "usage: goalx list"},
		{name: "attach", run: func() error { return Attach(t.TempDir(), []string{"--help"}) }, want: "usage: goalx attach [--run NAME] [window]"},
		{name: "stop", run: func() error { return Stop(t.TempDir(), []string{"--help"}) }, want: "usage: goalx stop [--run NAME]"},
		{name: "review", run: func() error { return Review(t.TempDir(), []string{"--help"}) }, want: "usage: goalx review [--run NAME]"},
		{name: "diff", run: func() error { return Diff(t.TempDir(), []string{"--help"}) }, want: "usage: goalx diff [--run NAME] <session-a> [session-b]"},
		{name: "keep", run: func() error { return Keep(t.TempDir(), []string{"--help"}) }, want: "usage: goalx keep [--run NAME] <session-name>"},
		{name: "park", run: func() error { return Park(t.TempDir(), []string{"--help"}) }, want: "usage: goalx park [--run NAME] <session-name>"},
		{name: "resume", run: func() error { return Resume(t.TempDir(), []string{"--help"}) }, want: "usage: goalx resume [--run NAME] <session-name>"},
		{name: "save", run: func() error { return Save(t.TempDir(), []string{"--help"}) }, want: "usage: goalx save [NAME]"},
		{name: "verify", run: func() error { return Verify(t.TempDir(), []string{"--help"}) }, want: "usage: goalx verify [--run NAME]"},
		{name: "drop", run: func() error { return Drop(t.TempDir(), []string{"--help"}) }, want: "usage: goalx drop [--run NAME]"},
		{name: "report", run: func() error { return Report(t.TempDir(), []string{"--help"}) }, want: "usage: goalx report [--run NAME]"},
		{name: "archive", run: func() error { return Archive(t.TempDir(), []string{"--help"}) }, want: "usage: goalx archive [--run NAME] <session-name>"},
		{name: "serve", run: func() error { return Serve(t.TempDir(), []string{"--help"}) }, want: "usage: goalx serve"},
	} {
		out := captureStdout(t, func() {
			if err := tc.run(); err != nil {
				t.Fatalf("%s --help: %v", tc.name, err)
			}
		})
		if !strings.Contains(out, tc.want) {
			t.Fatalf("%s --help output = %q, want %q", tc.name, out, tc.want)
		}
	}
}

func TestAttachDistinguishesDegradedTransportFromStoppedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runDir := writeRunSpecFixture(t, repo, &goalx.Config{
		Name:      "attach-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex"},
	})
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := SaveProjectRegistry(repo, &ProjectRegistry{
		Version:    1,
		FocusedRun: "attach-run",
		ActiveRuns: map[string]ProjectRunRef{"attach-run": {Name: "attach-run", State: "active"}},
	}); err != nil {
		t.Fatalf("SaveProjectRegistry active: %v", err)
	}

	err := Attach(repo, []string{"--run", "attach-run"})
	if err == nil || !strings.Contains(err.Error(), "transport unavailable") {
		t.Fatalf("Attach degraded err = %v, want transport unavailable", err)
	}

	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "stopped"}); err != nil {
		t.Fatalf("SaveControlRunState stopped: %v", err)
	}
	err = Attach(repo, []string{"--run", "attach-run"})
	if err == nil || !strings.Contains(err.Error(), "run may have stopped") {
		t.Fatalf("Attach stopped err = %v, want stopped hint", err)
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
