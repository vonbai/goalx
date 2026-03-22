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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
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
      "satisfaction_basis": "preexisting"
    }
  ]
}`)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
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
		`"command": "printf 'e2e ok\n'"`,
		`"command_source": "acceptance"`,
		`"baseline_command": "printf 'e2e ok\n'"`,
		`"scope_type": "baseline"`,
		`"last_exit_code": 0`,
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}

	statusData, err := os.ReadFile(filepath.Join(repo, ".goalx", "status.json"))
	if err != nil {
		t.Fatalf("read status.json: %v", err)
	}
	statusText := string(statusData)
	for _, want := range []string{
		`"acceptance_status":"passed"`,
		`"acceptance_exit_code":0`,
		`"completion_mode":"verification_only"`,
		`"code_changed":false`,
	} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("status.json missing %q:\n%s", want, statusText)
		}
	}
}

func TestVerifyFallsBackToHarnessAndRecordsFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
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
      "satisfaction_basis": "preexisting"
    }
  ]
}`)
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
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
		`"command_source": "harness"`,
		`"baseline_command": "test -f DOES-NOT-EXIST"`,
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}
}

func TestVerifyFailsWhenAcceptanceCommandDiffersFromBaselineWithoutScopeMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName := "verify-acceptance-scope"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir project .goalx: %v", err)
	}

	snapshot := []byte(`name: verify-acceptance-scope
mode: develop
objective: ship feature
harness:
  command: "printf 'baseline gate\n'"
`)
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
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
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	state := &AcceptanceState{
		Version:        1,
		BaselineCommand: "printf 'baseline gate\\n'",
		BaselineSource:  "harness",
		Command:         "printf 'narrow gate\\n'",
		CommandSource:   "master",
		Status:          acceptanceStatusPending,
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), state); err != nil {
		t.Fatalf("write acceptance state: %v", err)
	}

	err := Verify(repo, []string{"--run", runName})
	if err == nil {
		t.Fatal("expected Verify to fail")
	}
	if !strings.Contains(err.Error(), "scope") {
		t.Fatalf("Verify error = %v, want scope failure", err)
	}
}

func TestVerifyRecordsImplementationAndVerificationWhenRunChangedCode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	baseRevision := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))

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
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), snapshot, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
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
      "evidence": ["/tmp/e2e.txt"]
    }
  ]
}`)
	if err := os.WriteFile(GoalContractPath(runDir), contract, 0o644); err != nil {
		t.Fatalf("write goal contract: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    "ship feature",
		BaseRevision: baseRevision,
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}

	writeAndCommit(t, repo, "feature.txt", "feature", "run change")

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	statusData, err := os.ReadFile(filepath.Join(repo, ".goalx", "status.json"))
	if err != nil {
		t.Fatalf("read status.json: %v", err)
	}
	statusText := string(statusData)
	for _, want := range []string{
		`"completion_mode":"implementation_and_verification"`,
		`"code_changed":true`,
		`"base_revision":"` + baseRevision + `"`,
	} {
		if !strings.Contains(statusText, want) {
			t.Fatalf("status.json missing %q:\n%s", want, statusText)
		}
	}
}
