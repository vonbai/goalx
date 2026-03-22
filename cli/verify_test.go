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
	} {
		if !strings.Contains(stateText, want) {
			t.Fatalf("acceptance state missing %q:\n%s", want, stateText)
		}
	}
}
