package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureEngineTrustedCodexExactPathIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgDir := filepath.Join(home, ".codex")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir codex dir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "config.toml")
	initial := "model = \"gpt-5.4\"\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	worktree := filepath.Join(t.TempDir(), "worktree")
	if err := EnsureEngineTrusted("codex", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted first: %v", err)
	}
	if err := EnsureEngineTrusted("codex", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted second: %v", err)
	}

	out, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(out)
	header := `[projects."` + escapeTOMLString(worktree) + `"]`
	if !strings.Contains(text, initial) {
		t.Fatalf("original config lost:\n%s", text)
	}
	if strings.Count(text, header) != 1 {
		t.Fatalf("expected one trust section for %s, got:\n%s", worktree, text)
	}
	if !strings.Contains(text, `trust_level = "trusted"`) {
		t.Fatalf("trust level missing:\n%s", text)
	}
}

func TestEnsureEngineTrustedClaudeWritesProjectTrust(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	claudePath := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"projects": map[string]any{
			"/home/user": map[string]any{
				"hasTrustDialogAccepted": false,
			},
		},
	}
	raw, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial json: %v", err)
	}
	if err := os.WriteFile(claudePath, raw, 0o644); err != nil {
		t.Fatalf("write claude json: %v", err)
	}

	worktree := filepath.Join(t.TempDir(), "wt")
	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted: %v", err)
	}

	out, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read claude json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal claude json: %v", err)
	}
	projects := doc["projects"].(map[string]any)
	entry := projects[worktree].(map[string]any)
	if got := entry["hasTrustDialogAccepted"]; got != true {
		t.Fatalf("hasTrustDialogAccepted = %#v, want true", got)
	}
	if got := entry["hasCompletedProjectOnboarding"]; got != true {
		t.Fatalf("hasCompletedProjectOnboarding = %#v, want true", got)
	}
	if got := entry["hasClaudeMdExternalIncludesApproved"]; got != true {
		t.Fatalf("hasClaudeMdExternalIncludesApproved = %#v, want true", got)
	}
	if got := entry["projectOnboardingSeenCount"]; got != float64(1) {
		t.Fatalf("projectOnboardingSeenCount = %#v, want 1", got)
	}
}

func TestEnsureEngineTrustedClaudeMergesAllowedTools(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	worktree := filepath.Join(t.TempDir(), "wt")
	claudePath := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"projects": map[string]any{
			worktree: map[string]any{
				"allowedTools": []any{"UserTool", "Bash"},
			},
		},
	}
	raw, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial json: %v", err)
	}
	if err := os.WriteFile(claudePath, raw, 0o644); err != nil {
		t.Fatalf("write claude json: %v", err)
	}

	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted: %v", err)
	}

	out, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read claude json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal claude json: %v", err)
	}
	entry := doc["projects"].(map[string]any)[worktree].(map[string]any)
	tools := entry["allowedTools"].([]any)

	counts := map[string]int{}
	for _, tool := range tools {
		if s, ok := tool.(string); ok {
			counts[s]++
		}
	}
	for _, want := range []string{"UserTool", "Bash", "TaskCreate", "TaskUpdate", "LSP"} {
		if counts[want] != 1 {
			t.Fatalf("allowedTools %q count = %d, want 1; got %#v", want, counts[want], tools)
		}
	}
}

func TestEnsureEngineTrustedClaudeInheritsSourceMCPConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	worktree := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", worktree, "-b", "feature")

	claudePath := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"projects": map[string]any{
			repo: map[string]any{
				"allowedTools": []any{"SourceTool"},
				"mcpServers": map[string]any{
					"context7": map[string]any{"type": "stdio", "command": "context7"},
				},
				"enabledMcpjsonServers":  []any{"context7"},
				"disabledMcpjsonServers": []any{"legacy"},
			},
			worktree: map[string]any{
				"allowedTools": []any{"WorktreeTool"},
				"mcpServers": map[string]any{
					"local": map[string]any{"type": "http", "url": "http://127.0.0.1:3000"},
				},
			},
		},
	}
	raw, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial json: %v", err)
	}
	if err := os.WriteFile(claudePath, raw, 0o644); err != nil {
		t.Fatalf("write claude json: %v", err)
	}

	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted: %v", err)
	}

	out, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read claude json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal claude json: %v", err)
	}
	entry := doc["projects"].(map[string]any)[worktree].(map[string]any)

	tools := entry["allowedTools"].([]any)
	counts := map[string]int{}
	for _, tool := range tools {
		if s, ok := tool.(string); ok {
			counts[s]++
		}
	}
	for _, want := range []string{"SourceTool", "WorktreeTool", "Bash"} {
		if counts[want] != 1 {
			t.Fatalf("allowedTools %q count = %d, want 1; got %#v", want, counts[want], tools)
		}
	}

	servers := entry["mcpServers"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Fatalf("mcpServers missing inherited source server: %#v", servers)
	}
	if _, ok := servers["local"]; !ok {
		t.Fatalf("mcpServers missing worktree-local server: %#v", servers)
	}

	enabled := entry["enabledMcpjsonServers"].([]any)
	if len(enabled) != 1 || enabled[0] != "context7" {
		t.Fatalf("enabledMcpjsonServers = %#v, want [context7]", enabled)
	}
	disabled := entry["disabledMcpjsonServers"].([]any)
	if len(disabled) != 1 || disabled[0] != "legacy" {
		t.Fatalf("disabledMcpjsonServers = %#v, want [legacy]", disabled)
	}
}
