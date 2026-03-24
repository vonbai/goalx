package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestVerifyUsesAcceptanceCommandAndWritesState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-run
mode: develop
objective: ship feature
target:
  files: ["README.md"]
harness:
  command: "test -f DOES-NOT-EXIST"
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "claimed",
      "evidence_paths": ["/tmp/e2e.txt"]
    }
  ],
  "optional": []
}`)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	if err := os.WriteFile(GoalPath(runDir), goal, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	stateData, err := os.ReadFile(filepath.Join(runDir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance state: %v", err)
	}
	stateText := string(stateData)
	for _, want := range []string{
		`"status": "passed"`,
		`"default_command": "printf 'e2e ok\n'"`,
		`"effective_command": "printf 'e2e ok\n'"`,
		`"change_kind": "same"`,
		`"goal_version": 1`,
		`"exit_code": 0`,
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}

}

func TestVerifyRunsAcceptanceInsideRunWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	snapshot := []byte(`name: verify-run
mode: develop
objective: ship feature
acceptance:
  command: "test -f run-worktree-only.txt"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	if err := os.WriteFile(GoalPath(runDir), []byte(`{"version":1,"required":[{"id":"req-1","text":"ship feature","source":"user","state":"claimed","evidence_paths":["/tmp/e2e.txt"]}],"optional":[]}`), 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runWT, "run-worktree-only.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write run-worktree-only.txt: %v", err)
	}

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestVerifyFallsBackToHarnessAndRecordsFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	snapshot := []byte(`name: verify-run
mode: develop
objective: ship feature
target:
  files: ["README.md"]
harness:
  command: "test -f DOES-NOT-EXIST"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "claimed",
      "evidence_paths": ["/tmp/e2e.txt"]
    }
  ],
  "optional": []
}`)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	if err := os.WriteFile(GoalPath(runDir), goal, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}

	stateData, readErr := os.ReadFile(filepath.Join(runDir, "acceptance.json"))
	if readErr != nil {
		t.Fatalf("read acceptance state: %v", readErr)
	}
	stateText := string(stateData)
	for _, want := range []string{
		`"status": "failed"`,
		`"default_command": "test -f DOES-NOT-EXIST"`,
		`"effective_command": "test -f DOES-NOT-EXIST"`,
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}
}

func TestVerifyRecordsAcceptanceWhenRunChangedCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	ensureSharedProofEvidence(t)

	runName := "verify-code-changed"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-code-changed
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "claimed",
      "evidence_paths": ["/tmp/e2e.txt"],
      "note": "ready for verification"
    }
  ],
  "optional": []
}`)
	if err := os.WriteFile(GoalPath(runDir), goal, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: baseRevision,
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)

	writeAndCommit(t, repo, "feature.txt", "feature", "run change")

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	stateData, err := os.ReadFile(filepath.Join(runDir, "acceptance.json"))
	if err != nil {
		t.Fatalf("read acceptance state: %v", err)
	}
	stateText := string(stateData)
	if !strings.Contains(stateText, `"status": "passed"`) {
		t.Fatalf("acceptance state missing passed status:\n%s", stateText)
	}
}

func seedRunCharterForTests(t *testing.T, runDir, runName, projectRoot string) {
	t.Helper()

	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if meta == nil {
		t.Fatal("run metadata missing")
	}
	if meta.ProtocolVersion == 0 {
		meta.ProtocolVersion = 2
	}
	if meta.ProjectRoot == "" {
		meta.ProjectRoot = projectRoot
	}
	if meta.RunID == "" {
		meta.RunID = newRunID()
	}
	if meta.RootRunID == "" {
		meta.RootRunID = meta.RunID
	}
	if meta.Epoch == 0 {
		meta.Epoch = 1
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata normalize: %v", err)
	}
	charter, err := NewRunCharter(runDir, runName, "", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		t.Fatalf("hashRunCharter: %v", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata charter linkage: %v", err)
	}
}
