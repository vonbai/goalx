package cli

import (
	"encoding/json"
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
	if err != nil {
		t.Fatalf("run hook command: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "MASTER GUIDANCE PENDING") {
		t.Fatalf("hook output = %q, want guidance warning", string(out))
	}
	if !strings.Contains(string(out), guidancePath) {
		t.Fatalf("hook output = %q, want path %q", string(out), guidancePath)
	}
}
