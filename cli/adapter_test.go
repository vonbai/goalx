package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAdapterBlocksOnUnreadSessionInbox(t *testing.T) {
	worktree := filepath.Join(t.TempDir(), "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	controlDir := filepath.Join(t.TempDir(), "control with 'quote'")
	if err := os.MkdirAll(filepath.Join(controlDir, "inbox"), 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	inboxPath := filepath.Join(controlDir, "inbox", "session 1.jsonl")
	cursorPath := filepath.Join(controlDir, "session 1-cursor.json")
	if err := os.WriteFile(inboxPath, []byte(`{"id":1,"type":"tell","source":"user","body":"pending","created_at":"2026-03-24T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write inbox file: %v", err)
	}

	if err := GenerateAdapter("claude-code", worktree, inboxPath, cursorPath); err != nil {
		t.Fatalf("GenerateAdapter: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(worktree, ".claude", "hooks.json"))
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var doc struct {
		Hooks []struct {
			Event   string `json:"event"`
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal hooks.json: %v", err)
	}
	if len(doc.Hooks) != 1 {
		t.Fatalf("len(Hooks) = %d, want 1", len(doc.Hooks))
	}

	out, err := exec.Command("bash", "-lc", doc.Hooks[0].Command).CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("run hook command: %v\n%s", err, string(out))
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("exit code = %d, want 2", exitErr.ExitCode())
	}
	if !strings.Contains(string(out), "INBOX PENDING") {
		t.Fatalf("hook output = %q, want inbox warning", string(out))
	}
	if !strings.Contains(string(out), inboxPath) {
		t.Fatalf("hook output = %q, want path %q", string(out), inboxPath)
	}
}

func TestGenerateMasterAdapterRequiresVerifiedCompletionForDone(t *testing.T) {
	projectRoot := t.TempDir()
	runStatePath := filepath.Join(projectRoot, ".goalx", "runs", "demo", "state", "run.json")
	proofPath := filepath.Join(projectRoot, ".goalx", "runs", "demo", "proof", "completion.json")
	for _, path := range []string{runStatePath, proofPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir dir for %s: %v", path, err)
		}
	}

	if err := GenerateMasterAdapter("claude-code", projectRoot, runStatePath, proofPath); err != nil {
		t.Fatalf("GenerateMasterAdapter first: %v", err)
	}
	if err := GenerateMasterAdapter("claude-code", projectRoot, runStatePath, proofPath); err != nil {
		t.Fatalf("GenerateMasterAdapter second: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(projectRoot, ".claude", "hooks.json"))
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}

	var doc struct {
		Hooks []struct {
			Event   string `json:"event"`
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal hooks.json: %v", err)
	}
	if len(doc.Hooks) != 1 {
		t.Fatalf("len(Hooks) = %d, want 1", len(doc.Hooks))
	}

	runHook := func() (string, error) {
		out, err := exec.Command("bash", "-lc", doc.Hooks[0].Command).CombinedOutput()
		return string(out), err
	}

	out, err := runHook()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("missing status should block stop, err=%v out=%q", err, out)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("missing status exit code = %d, want 2", exitErr.ExitCode())
	}
	if !strings.Contains(out, runStatePath) {
		t.Fatalf("missing status output = %q, want path %q", out, runStatePath)
	}

	if err := os.WriteFile(runStatePath, []byte(`{"phase":"running"}`), 0o644); err != nil {
		t.Fatalf("write running status: %v", err)
	}
	out, err = runHook()
	if !errors.As(err, &exitErr) {
		t.Fatalf("running status should block stop, err=%v out=%q", err, out)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("running status exit code = %d, want 2", exitErr.ExitCode())
	}

	if err := os.WriteFile(runStatePath, []byte(`{"phase":"complete","recommendation":"done"}`), 0o644); err != nil {
		t.Fatalf("write done run state: %v", err)
	}
	if err := os.WriteFile(proofPath, []byte(`{"acceptance_status":"pending","goal_contract_status":"pending","goal_required_remaining":1}`), 0o644); err != nil {
		t.Fatalf("write incomplete proof: %v", err)
	}
	out, err = runHook()
	if !errors.As(err, &exitErr) {
		t.Fatalf("unverified done should block stop, err=%v out=%q", err, out)
	}

	if err := os.WriteFile(runStatePath, []byte(`{"phase":"complete","recommendation":"more-research"}`), 0o644); err != nil {
		t.Fatalf("write more-research status: %v", err)
	}
	if out, err = runHook(); err != nil {
		t.Fatalf("more-research completion should allow stop, err=%v out=%q", err, out)
	}

	if err := os.WriteFile(runStatePath, []byte(`{"phase":"complete","recommendation":"done"}`), 0o644); err != nil {
		t.Fatalf("write done run state: %v", err)
	}
	if err := os.WriteFile(proofPath, []byte(`{"acceptance_status":"passed","goal_contract_status":"satisfied","goal_required_remaining":0}`), 0o644); err != nil {
		t.Fatalf("write incomplete provenance proof: %v", err)
	}
	out, err = runHook()
	if !errors.As(err, &exitErr) {
		t.Fatalf("done without completion provenance should block stop, err=%v out=%q", err, out)
	}

	if err := os.WriteFile(proofPath, []byte(`{"acceptance_status":"passed","goal_contract_status":"satisfied","goal_required_remaining":0,"completion_mode":"verification_only","code_changed":false}`), 0o644); err != nil {
		t.Fatalf("write verified done proof: %v", err)
	}
	if out, err = runHook(); err != nil {
		t.Fatalf("verified done should allow stop, err=%v out=%q", err, out)
	}
}
