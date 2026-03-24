package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestKeepMergesRunWorktreeIntoSourceRootWhenNoSessionProvided(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	if err := EnsureProjectGoalxIgnored(repo); err != nil {
		t.Fatalf("EnsureProjectGoalxIgnored: %v", err)
	}

	runName := "keep-run"
	runDir := writeKeepRunFixture(t, repo, runName)
	runWT := RunWorktreePath(runDir)
	runBranch := fmt.Sprintf("goalx/%s/root", runName)
	if err := CreateWorktree(repo, runWT, runBranch); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	writeAndCommit(t, runWT, "README.md", "base\nroot change\n", "root change")

	out := captureStdout(t, func() {
		if err := Keep(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Keep: %v", err)
		}
	})
	if !strings.Contains(out, "Merged run worktree into source root.") {
		t.Fatalf("keep output missing run-root merge message:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(repo, "README.md"))
	if err != nil {
		t.Fatalf("read source root README: %v", err)
	}
	if string(data) != "base\nroot change\n" {
		t.Fatalf("source root README = %q", string(data))
	}
}

func TestKeepMergesSessionBranchIntoRunWorktree(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	if err := EnsureProjectGoalxIgnored(repo); err != nil {
		t.Fatalf("EnsureProjectGoalxIgnored: %v", err)
	}

	runName := "keep-run"
	runDir := writeKeepRunFixture(t, repo, runName)
	runWT := RunWorktreePath(runDir)
	runBranch := fmt.Sprintf("goalx/%s/root", runName)
	if err := CreateWorktree(repo, runWT, runBranch); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}

	sessionWT := WorktreePath(runDir, runName, 1)
	sessionBranch := fmt.Sprintf("goalx/%s/1", runName)
	if err := CreateWorktree(runWT, sessionWT, sessionBranch); err != nil {
		t.Fatalf("CreateWorktree session root: %v", err)
	}
	writeAndCommit(t, sessionWT, "feature.txt", "session change\n", "session change")

	out := captureStdout(t, func() {
		if err := Keep(repo, []string{"--run", runName, "session-1"}); err != nil {
			t.Fatalf("Keep: %v", err)
		}
	})
	if !strings.Contains(out, "Merged goalx/keep-run/1 into run worktree.") {
		t.Fatalf("keep output missing session merge message:\n%s", out)
	}

	data, err := os.ReadFile(filepath.Join(runWT, "feature.txt"))
	if err != nil {
		t.Fatalf("read run worktree feature: %v", err)
	}
	if string(data) != "session change\n" {
		t.Fatalf("run worktree feature = %q", string(data))
	}
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); !os.IsNotExist(err) {
		t.Fatalf("source root should remain unchanged before root keep, stat err = %v", err)
	}

	selectionData, err := os.ReadFile(filepath.Join(runDir, "selection.json"))
	if err != nil {
		t.Fatalf("read selection.json: %v", err)
	}
	var selection map[string]string
	if err := json.Unmarshal(selectionData, &selection); err != nil {
		t.Fatalf("unmarshal selection.json: %v", err)
	}
	if selection["kept"] != "session-1" || selection["branch"] != sessionBranch {
		t.Fatalf("selection = %#v", selection)
	}
}

func writeKeepRunFixture(t *testing.T, repo, runName string) string {
	t.Helper()

	runDir := goalx.RunDir(repo, runName)
	if err := os.RemoveAll(runDir); err != nil {
		t.Fatalf("cleanup run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(runDir, "worktrees"), 0o755); err != nil {
		t.Fatalf("mkdir worktrees dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeDevelop,
		Objective: "keep demo",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
		Harness:   goalx.HarnessConfig{Command: "test -f README.md"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	return runDir
}
