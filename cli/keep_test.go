package cli

import (
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

func TestKeepSkipsRootMergeWhenRunTreeAlreadyIntegrated(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	if err := EnsureProjectGoalxIgnored(repo); err != nil {
		t.Fatalf("EnsureProjectGoalxIgnored: %v", err)
	}

	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	runName := "keep-run"
	runDir := writeKeepRunFixture(t, repo, runName)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		ProjectRoot:  repo,
		RunID:        "run_keep",
		RootRunID:    "run_keep",
		Epoch:        1,
		BaseRevision: baseRevision,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	runWT := RunWorktreePath(runDir)
	runBranch := fmt.Sprintf("goalx/%s/root", runName)
	if err := CreateWorktree(repo, runWT, runBranch); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	writeAndCommit(t, runWT, "README.md", "base\nroot change\n", "root change")

	writeAndCommit(t, repo, "README.md", "base\nroot change\n", "manual integrate")
	headBefore := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))

	out := captureStdout(t, func() {
		if err := Keep(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Keep: %v", err)
		}
	})
	if !strings.Contains(out, "Run worktree already integrated into source root.") {
		t.Fatalf("keep output missing already-integrated message:\n%s", out)
	}

	headAfter := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	if headAfter != headBefore {
		t.Fatalf("keep should not create a new merge commit when trees already match: before=%s after=%s", headBefore, headAfter)
	}
}

func TestKeepRejectsRootMergeWhenTargetHeadLeftRunBaseLineage(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base commit")
	if err := EnsureProjectGoalxIgnored(repo); err != nil {
		t.Fatalf("EnsureProjectGoalxIgnored: %v", err)
	}

	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	runName := "keep-run"
	runDir := writeKeepRunFixture(t, repo, runName)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		ProjectRoot:  repo,
		RunID:        "run_keep",
		RootRunID:    "run_keep",
		Epoch:        1,
		BaseRevision: baseRevision,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	runWT := RunWorktreePath(runDir)
	runBranch := fmt.Sprintf("goalx/%s/root", runName)
	if err := CreateWorktree(repo, runWT, runBranch); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	writeAndCommit(t, runWT, "README.md", "base\nroot change\n", "root change")

	runGit(t, repo, "checkout", "--orphan", "other-root")
	writeAndCommit(t, repo, "README.md", "other root\n", "other root")

	err := Keep(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Keep to reject merging into a target outside the run base lineage")
	}
	for _, want := range []string{"base revision", "does not descend"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Keep error = %v, want substring %q", err, want)
		}
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
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), makeKeepSessionIdentity(t, runDir, "session-1", runName, "run-root", runBranch)); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
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

	integration, err := LoadIntegrationState(IntegrationStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadIntegrationState: %v", err)
	}
	if integration == nil {
		t.Fatal("integration state missing")
	}
	if integration.CurrentExperimentID != identityExperimentID(t, runDir, "session-1") {
		t.Fatalf("CurrentExperimentID = %q", integration.CurrentExperimentID)
	}
	if integration.CurrentBranch != sessionBranch {
		t.Fatalf("CurrentBranch = %q, want %q", integration.CurrentBranch, sessionBranch)
	}
	if integration.LastMethod != "keep" {
		t.Fatalf("LastMethod = %q, want keep", integration.LastMethod)
	}
	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) == 0 || events[len(events)-1].Kind != "experiment.integrated" {
		t.Fatalf("unexpected experiment events: %#v", events)
	}
}

func TestKeepReportsDirtyRunWorktreePath(t *testing.T) {
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
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), makeKeepSessionIdentity(t, runDir, "session-1", runName, "run-root", runBranch)); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	writeAndCommit(t, sessionWT, "feature.txt", "session change\n", "session change")
	if err := os.WriteFile(filepath.Join(runWT, "dirty.txt"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}

	err := Keep(repo, []string{"--run", runName, "session-1"})
	if err == nil {
		t.Fatal("expected Keep to reject dirty run worktree")
	}
	for _, want := range []string{
		"merge target",
		runWT,
		"dirty.txt",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Keep error = %v, want substring %q", err, want)
		}
	}
}

func TestKeepRejectsDirtySessionBoundaryBeforeMerging(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(sessionWT, "feature.txt"), []byte("dirty session change\n"), 0o644); err != nil {
		t.Fatalf("write dirty feature: %v", err)
	}
	identity := makeKeepSessionIdentity(t, runDir, "session-1", runName, "run-root", runBranch)
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "idle",
		Mode:         string(goalx.ModeDevelop),
		Branch:       sessionBranch,
		WorktreePath: sessionWT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	err := Keep(repo, []string{"--run", runName, "session-1"})
	if err == nil {
		t.Fatal("expected Keep to reject dirty session worktree")
	}
	for _, want := range []string{"session-1", "uncommitted changes", "commit", "feature.txt"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Keep error = %v, want substring %q", err, want)
		}
	}
}

func TestKeepRejectsSessionWithoutCommittedBoundaryRelativeToParent(t *testing.T) {
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
	identity := makeKeepSessionIdentity(t, runDir, "session-1", runName, "run-root", runBranch)
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         "session-1",
		State:        "idle",
		Mode:         string(goalx.ModeDevelop),
		Branch:       sessionBranch,
		WorktreePath: sessionWT,
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	err := Keep(repo, []string{"--run", runName, "session-1"})
	if err == nil {
		t.Fatal("expected Keep to reject session branch with no committed boundary")
	}
	for _, want := range []string{"session-1", "no committed branch changes", runBranch} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Keep error = %v, want substring %q", err, want)
		}
	}
}

func TestKeepHelpExplainsRunAndSessionMergeSemantics(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Keep(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Keep --help: %v", err)
		}
	})
	for _, want := range []string{
		"usage: goalx keep [--run NAME] [session-name]",
		"merge the run worktree branch into the source root",
		"require source-root HEAD to still descend from the run base revision",
		"merge that develop session branch into the run worktree",
		"only committed session branch history is merged",
		"dirty session worktrees must be committed first",
		"inspect the session worktree and merge manually",
		"this does not merge into the source root yet",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("keep help missing %q:\n%s", want, out)
		}
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
		Name:            runName,
		Mode:            goalx.ModeDevelop,
		Objective:       "keep demo",
		Target:          goalx.TargetConfig{Files: []string{"README.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	charter, err := NewRunCharter(runDir, cfg.Name, cfg.Objective, meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	return runDir
}

func makeKeepSessionIdentity(t *testing.T, runDir, sessionName, runName, baseSelector, baseBranch string) *SessionIdentity {
	t.Helper()

	identity, err := NewSessionIdentity(runDir, sessionName, sessionRoleKind(goalx.ModeDevelop), goalx.ModeDevelop, "codex", "gpt-5.4", "", "", "", goalx.TargetConfig{Files: []string{"."}})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	identity.BaseBranchSelector = baseSelector
	identity.BaseBranch = baseBranch
	return identity
}

func identityExperimentID(t *testing.T, runDir, sessionName string) string {
	t.Helper()
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, sessionName))
	if err != nil {
		t.Fatalf("LoadSessionIdentity(%s): %v", sessionName, err)
	}
	if identity == nil {
		t.Fatalf("%s identity missing", sessionName)
	}
	return identity.ExperimentID
}
