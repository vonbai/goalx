package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

func TestEnsureEngineTrustedCodexPreservesConcurrentEntries(t *testing.T) {
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

	worktrees := make([]string, 0, 12)
	root := t.TempDir()
	for i := 0; i < 12; i++ {
		worktrees = append(worktrees, filepath.Join(root, "wt-"+strconv.Itoa(i)))
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, worktree := range worktrees {
		worktree := worktree
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := EnsureEngineTrusted("codex", worktree); err != nil {
				t.Errorf("EnsureEngineTrusted(%s): %v", worktree, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	out, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, initial) {
		t.Fatalf("original config lost:\n%s", text)
	}
	for _, worktree := range worktrees {
		header := `[projects."` + escapeTOMLString(worktree) + `"]`
		if strings.Count(text, header) != 1 {
			t.Fatalf("expected one trust section for %s, got:\n%s", worktree, text)
		}
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

func TestEnsureEngineTrustedClaudeWritesLocalMCPPermissionHook(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	worktree := filepath.Join(t.TempDir(), "wt")
	settingsDir := filepath.Join(worktree, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.local.json")
	initial := map[string]any{
		"hooks": map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "startup",
					"hooks": []any{
						map[string]any{"type": "command", "command": "echo existing"},
					},
				},
			},
		},
	}
	raw, err := json.Marshal(initial)
	if err != nil {
		t.Fatalf("marshal initial settings: %v", err)
	}
	if err := os.WriteFile(settingsPath, raw, 0o644); err != nil {
		t.Fatalf("write initial settings: %v", err)
	}

	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted first: %v", err)
	}
	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted second: %v", err)
	}

	out, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.local.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal settings.local.json: %v", err)
	}
	hooks := doc["hooks"].(map[string]any)
	if _, ok := hooks["SessionStart"]; !ok {
		t.Fatalf("existing SessionStart hook lost: %#v", hooks)
	}
	permissionHooks := hooks["PermissionRequest"].([]any)
	count := 0
	for _, entry := range permissionHooks {
		entryObj := entry.(map[string]any)
		if entryObj["matcher"] != "mcp__.*" {
			continue
		}
		for _, hook := range entryObj["hooks"].([]any) {
			hookObj := hook.(map[string]any)
			if hookObj["type"] == "command" && strings.Contains(hookObj["command"].(string), "claude-hook permission-request") {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("permission hook count = %d, want 1; settings = %#v", count, hooks["PermissionRequest"])
	}
	elicitationHooks := hooks["Elicitation"].([]any)
	count = 0
	for _, entry := range elicitationHooks {
		entryObj := entry.(map[string]any)
		if entryObj["matcher"] != ".*" {
			continue
		}
		for _, hook := range entryObj["hooks"].([]any) {
			hookObj := hook.(map[string]any)
			if hookObj["type"] == "command" && strings.Contains(hookObj["command"].(string), "claude-hook elicitation") {
				count++
			}
		}
	}
	if count != 1 {
		t.Fatalf("elicitation hook count = %d, want 1; settings = %#v", count, hooks["Elicitation"])
	}
	notificationHooks := hooks["Notification"].([]any)
	permissionVisible := 0
	elicitationVisible := 0
	for _, entry := range notificationHooks {
		entryObj := entry.(map[string]any)
		matcher, _ := entryObj["matcher"].(string)
		for _, hook := range entryObj["hooks"].([]any) {
			hookObj := hook.(map[string]any)
			if hookObj["type"] != "command" || !strings.Contains(hookObj["command"].(string), "claude-hook notification") {
				continue
			}
			switch matcher {
			case "permission_prompt":
				permissionVisible++
			case "elicitation_dialog":
				elicitationVisible++
			}
		}
	}
	if permissionVisible != 1 || elicitationVisible != 1 {
		t.Fatalf("notification hooks = %#v, want permission_prompt and elicitation_dialog backstops once each", notificationHooks)
	}
}

func TestVerifyClaudeProjectLocalHooksPassesAfterTrustBootstrap(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	worktree := filepath.Join(t.TempDir(), "wt")
	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted: %v", err)
	}
	if err := verifyClaudeProjectLocalHooks(worktree); err != nil {
		t.Fatalf("verifyClaudeProjectLocalHooks: %v", err)
	}
}

func TestVerifyClaudeProjectLocalHooksRejectsMissingNotificationBackstop(t *testing.T) {
	worktree := filepath.Join(t.TempDir(), "wt")
	settingsDir := filepath.Join(worktree, ".claude")
	if err := os.MkdirAll(settingsDir, 0o755); err != nil {
		t.Fatalf("mkdir settings dir: %v", err)
	}
	goalxBin, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	doc := map[string]any{
		"hooks": map[string]any{
			"PermissionRequest": []any{
				map[string]any{
					"matcher": claudeMCPPermissionHookMatcher,
					"hooks": []any{map[string]any{
						"type":    "command",
						"command": shellQuote(goalxBin) + " claude-hook permission-request",
					}},
				},
			},
			"Elicitation": []any{
				map[string]any{
					"matcher": claudeMCPElicitationHookMatcher,
					"hooks": []any{map[string]any{
						"type":    "command",
						"command": shellQuote(goalxBin) + " claude-hook elicitation",
					}},
				},
			},
			"Notification": []any{
				map[string]any{
					"matcher": claudePermissionNotificationMatcher,
					"hooks": []any{map[string]any{
						"type":    "command",
						"command": shellQuote(goalxBin) + " claude-hook notification",
					}},
				},
			},
		},
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	settingsPath := filepath.Join(settingsDir, "settings.local.json")
	if err := os.WriteFile(settingsPath, raw, 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	err = verifyClaudeProjectLocalHooks(worktree)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "notification") {
		t.Fatalf("verifyClaudeProjectLocalHooks error = %v, want notification bootstrap error", err)
	}
}

func TestClaudePermissionRequestHookOutputPrefersLocalSettingsSuggestion(t *testing.T) {
	input := []byte(`{
	  "hook_event_name":"PermissionRequest",
	  "tool_name":"mcp__playwright__browser_navigate",
	  "permission_suggestions":[
	    {"type":"addRules","behavior":"allow","destination":"session","rules":[{"toolName":"mcp__playwright__browser_navigate"}]},
	    {"type":"addRules","behavior":"allow","destination":"localSettings","rules":[{"toolName":"mcp__playwright__browser_navigate"}]}
	  ]
	}`)

	out, err := buildClaudePermissionRequestHookOutput(input)
	if err != nil {
		t.Fatalf("buildClaudePermissionRequestHookOutput: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal hook output: %v", err)
	}
	hookSpecific := doc["hookSpecificOutput"].(map[string]any)
	if hookSpecific["hookEventName"] != "PermissionRequest" {
		t.Fatalf("hookEventName = %#v, want PermissionRequest", hookSpecific["hookEventName"])
	}
	decision := hookSpecific["decision"].(map[string]any)
	if decision["behavior"] != "allow" {
		t.Fatalf("behavior = %#v, want allow", decision["behavior"])
	}
	updates := decision["updatedPermissions"].([]any)
	if len(updates) != 1 {
		t.Fatalf("updatedPermissions len = %d, want 1", len(updates))
	}
	update := updates[0].(map[string]any)
	if update["destination"] != "localSettings" {
		t.Fatalf("destination = %#v, want localSettings", update["destination"])
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

func TestEnsureEngineTrustedClaudeMergesGlobalMCPConfigIntoWorktree(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	worktree := filepath.Join(t.TempDir(), "wt")
	runGit(t, repo, "worktree", "add", worktree, "-b", "feature")

	claudePath := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"mcpServers": map[string]any{
			"context7": map[string]any{"type": "stdio", "command": "npx"},
		},
		"enabledMcpjsonServers": []any{"context7"},
		"projects": map[string]any{
			repo: map[string]any{
				"mcpServers": map[string]any{
					"local": map[string]any{"type": "http", "url": "http://127.0.0.1:3000"},
				},
				"enabledMcpjsonServers":  []any{"local"},
				"disabledMcpjsonServers": []any{"legacy"},
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
	servers := entry["mcpServers"].(map[string]any)
	if _, ok := servers["context7"]; !ok {
		t.Fatalf("mcpServers missing global context7: %#v", servers)
	}
	if _, ok := servers["local"]; !ok {
		t.Fatalf("mcpServers missing source local: %#v", servers)
	}

	enabled := entry["enabledMcpjsonServers"].([]any)
	if len(enabled) != 2 || enabled[0] != "context7" || enabled[1] != "local" {
		t.Fatalf("enabledMcpjsonServers = %#v, want [context7 local]", enabled)
	}
	disabled := entry["disabledMcpjsonServers"].([]any)
	if len(disabled) != 1 || disabled[0] != "legacy" {
		t.Fatalf("disabledMcpjsonServers = %#v, want [legacy]", disabled)
	}
}

func TestEnsureEngineTrustedClaudeOnlyAddsContext7ToolsWhenConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	worktree := filepath.Join(t.TempDir(), "wt")
	claudePath := filepath.Join(home, ".claude.json")
	initial := map[string]any{
		"projects": map[string]any{
			worktree: map[string]any{},
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
		t.Fatalf("EnsureEngineTrusted first: %v", err)
	}

	out, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read claude json after first: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal claude json after first: %v", err)
	}
	entry := doc["projects"].(map[string]any)[worktree].(map[string]any)
	tools := entry["allowedTools"].([]any)
	for _, tool := range tools {
		if s, ok := tool.(string); ok && strings.HasPrefix(s, "mcp__context7__") {
			t.Fatalf("unexpected context7 tool without configured server: %#v", tools)
		}
	}

	doc["mcpServers"] = map[string]any{
		"context7": map[string]any{"type": "stdio", "command": "npx"},
	}
	raw, err = json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal updated claude json: %v", err)
	}
	if err := os.WriteFile(claudePath, raw, 0o644); err != nil {
		t.Fatalf("rewrite claude json: %v", err)
	}

	if err := EnsureEngineTrusted("claude-code", worktree); err != nil {
		t.Fatalf("EnsureEngineTrusted second: %v", err)
	}
	out, err = os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("read claude json after second: %v", err)
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal claude json after second: %v", err)
	}
	entry = doc["projects"].(map[string]any)[worktree].(map[string]any)
	tools = entry["allowedTools"].([]any)
	counts := map[string]int{}
	for _, tool := range tools {
		if s, ok := tool.(string); ok {
			counts[s]++
		}
	}
	for _, want := range []string{"mcp__context7__resolve-library-id", "mcp__context7__query-docs"} {
		if counts[want] != 1 {
			t.Fatalf("allowedTools %q count = %d, want 1; got %#v", want, counts[want], tools)
		}
	}
}
