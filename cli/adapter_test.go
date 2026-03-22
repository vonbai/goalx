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

func TestGenerateAdapterQuotesGuidancePath(t *testing.T) {
	worktree := filepath.Join(t.TempDir(), "worktree")
	if err := os.MkdirAll(worktree, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	guidanceDir := filepath.Join(t.TempDir(), "guidance with 'quote'")
	if err := os.MkdirAll(guidanceDir, 0o755); err != nil {
		t.Fatalf("mkdir guidance dir: %v", err)
	}
	guidancePath := filepath.Join(guidanceDir, "session 1.md")
	if err := os.WriteFile(guidancePath, []byte("pending\n"), 0o644); err != nil {
		t.Fatalf("write guidance file: %v", err)
	}

	if err := GenerateAdapter("claude-code", worktree, guidancePath); err != nil {
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
	if !strings.Contains(string(out), "GUIDANCE PENDING") {
		t.Fatalf("hook output = %q, want guidance warning", string(out))
	}
	if !strings.Contains(string(out), guidancePath) {
		t.Fatalf("hook output = %q, want path %q", string(out), guidancePath)
	}
}

func TestGenerateMasterAdapterRequiresVerifiedCompletionForDone(t *testing.T) {
	projectRoot := t.TempDir()
	statusPath := filepath.Join(projectRoot, ".goalx", "status.json")
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatalf("mkdir status dir: %v", err)
	}

	if err := GenerateMasterAdapter("claude-code", projectRoot, statusPath); err != nil {
		t.Fatalf("GenerateMasterAdapter first: %v", err)
	}
	if err := GenerateMasterAdapter("claude-code", projectRoot, statusPath); err != nil {
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
	if !strings.Contains(out, statusPath) {
		t.Fatalf("missing status output = %q, want path %q", out, statusPath)
	}

	if err := os.WriteFile(statusPath, []byte(`{"phase":"running"}`), 0o644); err != nil {
		t.Fatalf("write running status: %v", err)
	}
	out, err = runHook()
	if !errors.As(err, &exitErr) {
		t.Fatalf("running status should block stop, err=%v out=%q", err, out)
	}
	if exitErr.ExitCode() != 2 {
		t.Fatalf("running status exit code = %d, want 2", exitErr.ExitCode())
	}

	if err := os.WriteFile(statusPath, []byte(`{"phase":"complete","recommendation":"done","acceptance_status":"pending"}`), 0o644); err != nil {
		t.Fatalf("write unverified done status: %v", err)
	}
	out, err = runHook()
	if !errors.As(err, &exitErr) {
		t.Fatalf("unverified done should block stop, err=%v out=%q", err, out)
	}

	if err := os.WriteFile(statusPath, []byte(`{"phase":"complete","recommendation":"more-research","acceptance_status":"failed"}`), 0o644); err != nil {
		t.Fatalf("write more-research status: %v", err)
	}
	if out, err = runHook(); err != nil {
		t.Fatalf("more-research completion should allow stop, err=%v out=%q", err, out)
	}

	if err := os.WriteFile(statusPath, []byte(`{"phase":"complete","recommendation":"done","acceptance_status":"passed"}`), 0o644); err != nil {
		t.Fatalf("write verified done status: %v", err)
	}
	if out, err = runHook(); err != nil {
		t.Fatalf("verified done should allow stop, err=%v out=%q", err, out)
	}
}
