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
	contract := []byte(`{
  "version": 1,
  "objective": "ship feature",
  "items": [
    {
      "id": "req-1",
      "kind": "user_required",
      "requirement": "ship feature",
      "status": "done",
      "satisfaction_basis": "preexisting",
      "evidence": ["/tmp/e2e.txt"]
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "proof") && !strings.Contains(err.Error(), "evidence_class") {
		t.Fatalf("Verify error = %v, want structured proof failure", err)
	}
}

func TestVerifyWritesCanonicalCompletionProofManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))

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
	contract := []byte(`{
  "version": 1,
  "objective": "ship feature",
  "items": [
    {
      "id": "req-1",
      "kind": "user_required",
      "requirement": "ship feature",
      "status": "done",
      "satisfaction_basis": "run_change",
      "evidence": ["/tmp/e2e.txt"],
      "evidence_class": "artifact",
      "counter_evidence": ["checked for missing files"],
      "semantic_match": "exact"
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
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
		`"acceptance_status": "passed"`,
		`"goal_contract_version": 1`,
		`"proof_items": [`,
		`"requirement_id": "req-1"`,
		`"evidence_class": "artifact"`,
		`"semantic_match": "exact"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("proof manifest missing %q:\n%s", want, text)
		}
	}
}
