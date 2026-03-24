package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestVerifyFailsWhenRequiredItemLacksStructuredProof(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)

	runName := "verify-proof-missing"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-proof-missing
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
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
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "claimed"
    }
  ],
  "optional": []
}`)
	if err := os.WriteFile(GoalPath(runDir), goal, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "evidence") {
		t.Fatalf("Verify error = %v, want evidence failure", err)
	}
}

func TestVerifyWritesCanonicalCompletionProofManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
	ensureSharedProofEvidence(t)

	runName := "verify-proof-manifest"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-proof-manifest
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
`)
	if err := os.WriteFile(RunSpecPath(runDir), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: baseRevision,
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
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

	writeAndCommit(t, repo, "feature.txt", "feature", "run change")

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	proofPath := CompletionStatePath(runDir)
	wantPath := filepath.Join(runDir, "proof", "completion.json")
	if proofPath != wantPath {
		t.Fatalf("CompletionStatePath = %q, want %q", proofPath, wantPath)
	}

	data, err := os.ReadFile(proofPath)
	if err != nil {
		t.Fatalf("read proof manifest: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"result": "done"`,
		`"acceptance_status": "passed"`,
		`"goal_version": 1`,
		`"goal_satisfied": true`,
		`"items": [`,
		`"goal_item_id": "req-1"`,
		`"verdict": "satisfied"`,
		`"basis": "run_change"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("proof manifest missing %q:\n%s", want, text)
		}
	}
}

func TestVerifyFailsWhenStructuredProofEvidencePathDoesNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName := "verify-proof-missing-evidence"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	snapshot := []byte(`name: verify-proof-missing-evidence
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
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
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "claimed",
      "evidence_paths": ["/tmp/goalx-missing-proof-evidence.txt"]
    }
  ],
  "optional": []
}`)
	if err := os.WriteFile(GoalPath(runDir), goal, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Verify error = %v, want missing evidence path failure", err)
	}
}

func TestVerifyFailsWhenRequiredGoalItemRemainsOpen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName := "verify-open-goal-item"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	evidencePath := filepath.Join(runDir, "artifact.txt")
	if err := os.WriteFile(evidencePath, []byte("artifact"), 0o644); err != nil {
		t.Fatalf("write evidence artifact: %v", err)
	}

	snapshot := []byte(`name: verify-open-goal-item
mode: develop
objective: ship feature
acceptance:
  command: "printf 'e2e ok\n'"
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
	goal := []byte(`{
  "version": 1,
  "required": [
    {
      "id": "req-1",
      "text": "ship feature",
      "source": "user",
      "state": "open",
      "evidence_paths": ["` + evidencePath + `"]
    }
  ],
  "optional": []
}`)
	if err := os.WriteFile(GoalPath(runDir), goal, 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "open") && !strings.Contains(err.Error(), "unsatisfied") {
		t.Fatalf("Verify error = %v, want open required item failure", err)
	}

	proofData, readErr := os.ReadFile(CompletionStatePath(runDir))
	if readErr != nil {
		t.Fatalf("read completion proof: %v", readErr)
	}
	if !strings.Contains(string(proofData), `"result": "phase_complete_but_goal_incomplete"`) {
		t.Fatalf("completion proof missing phase_complete_but_goal_incomplete:\n%s", proofData)
	}
}
