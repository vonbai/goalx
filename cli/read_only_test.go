package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestObserveLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	logPath := installFakeTmux(t, "master")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})
	if !strings.Contains(out, "## Run: "+runName) {
		t.Fatalf("observe output missing run header:\n%s", out)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, filepath.Join(repo, ".goalx", "status.json"), statusBefore)
	assertTmuxTouched(t, logPath)
}

func TestStatusLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	_, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", filepath.Base(runDir)}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})
	if !strings.Contains(out, "SESSION") {
		t.Fatalf("status output missing header:\n%s", out)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, filepath.Join(repo, ".goalx", "status.json"), statusBefore)
}

func TestReportLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)

	out := captureStdout(t, func() {
		if err := Report(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Report: %v", err)
		}
	})
	if !strings.Contains(out, "=== Report: "+runName+" ===") {
		t.Fatalf("report output missing header:\n%s", out)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, filepath.Join(repo, ".goalx", "status.json"), statusBefore)
}

func TestSaveLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "report.md"), []byte("saved report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	if err := Save(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, filepath.Join(repo, ".goalx", "status.json"), statusBefore)
}

func TestVerifyDoesNotRewriteRunStateFromStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	cfg := goalx.Config{
		Name:       runName,
		Mode:       goalx.ModeDevelop,
		Objective:  "ship feature",
		Acceptance: goalx.AcceptanceConfig{Command: "printf 'gate ok\\n'"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"verify-run","mode":"develop","objective":"ship feature","active":true,"phase":"working","recommendation":"keep going","updated_at":"2026-03-23T00:00:00Z"}`)
	if err := os.WriteFile(RunRuntimeStatePath(runDir), runStateBefore, 0o644); err != nil {
		t.Fatalf("write run state: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".goalx", "status.json"), []byte(`{"run":"verify-run","phase":"working","recommendation":"keep going"}`), 0o644); err != nil {
		t.Fatalf("write status cache: %v", err)
	}
	if err := os.WriteFile(GoalContractPath(runDir), []byte(`{"version":1,"objective":"ship feature","items":[{"id":"req-1","kind":"user_required","requirement":"ship feature","status":"done","satisfaction_basis":"preexisting"}]}`), 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{Version: 1, Objective: "ship feature", BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
}

func writeReadOnlyRunFixture(t *testing.T, repo string) (string, string, []byte, []byte) {
	t.Helper()

	runName := "readonly-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeDevelop,
		Objective: "read only",
		Master:    goalx.MasterConfig{Engine: "codex"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"readonly-run","mode":"develop","objective":"read only","active":true,"phase":"working","recommendation":"keep going","updated_at":"2026-03-23T00:00:00Z"}`)
	if err := os.WriteFile(RunRuntimeStatePath(runDir), runStateBefore, 0o644); err != nil {
		t.Fatalf("write run state: %v", err)
	}
	statusBefore := []byte(`{"run":"readonly-run","phase":"working","recommendation":"keep going","heartbeat":7,"heartbeat_seq":7,"heartbeat_lag":2,"master_wake_pending":true,"master_stale":false,"active":true}`)
	if err := os.WriteFile(filepath.Join(repo, ".goalx", "status.json"), statusBefore, 0o644); err != nil {
		t.Fatalf("write status cache: %v", err)
	}
	return runName, runDir, runStateBefore, statusBefore
}

func assertFileUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s changed:\nwant %s\ngot  %s", path, string(want), string(got))
	}
}

func assertTmuxTouched(t *testing.T, logPath string) {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Fatal("expected tmux log to be touched")
	}
}
