package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var claudeAllowedTools = []string{
	"Bash", "Read", "Write", "Edit", "Glob", "Grep",
	"WebFetch", "WebSearch", "Agent",
	"TaskCreate", "TaskUpdate", "LSP", "NotebookEdit", "EnterPlanMode",
}

var claudeContext7Tools = []string{
	"mcp__context7__resolve-library-id",
	"mcp__context7__query-docs",
}

const claudeMCPPermissionHookMatcher = "mcp__.*"
const claudeMCPElicitationHookMatcher = ".*"
const claudePermissionNotificationMatcher = "permission_prompt"
const claudeElicitationNotificationMatcher = "elicitation_dialog"

// EnsureEngineTrusted pre-accepts workspace trust for interactive engines so
// freshly created worktrees can start unattended.
func EnsureEngineTrusted(engine, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve trust path %s: %w", path, err)
	}

	switch engine {
	case "codex":
		return ensureCodexTrusted(absPath)
	case "claude-code":
		return ensureClaudeTrusted(absPath)
	default:
		return nil
	}
}

func ensureCodexTrusted(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	cfgPath := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return fmt.Errorf("mkdir codex config dir: %w", err)
	}

	section := `[projects."` + escapeTOMLString(path) + `"]`
	if err := mutateStructuredFile(
		cfgPath,
		0o600,
		func(data []byte) (*string, error) {
			text := string(data)
			return &text, nil
		},
		func() *string {
			text := ""
			return &text
		},
		func(text *string) error {
			*text = upsertTOMLKey(*text, section, `trust_level = "trusted"`)
			return nil
		},
		func(text *string) ([]byte, error) {
			return []byte(*text), nil
		},
	); err != nil {
		return fmt.Errorf("write codex config: %w", err)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return fmt.Errorf("verify codex config: %w", err)
	}
	text := string(data)
	if !strings.Contains(text, section) || !strings.Contains(text, `trust_level = "trusted"`) {
		return fmt.Errorf("verify codex config: missing trusted entry for %s", path)
	}
	return nil
}

func ensureClaudeTrusted(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	cfgPath := filepath.Join(home, ".claude.json")

	if err := mutateStructuredFile(
		cfgPath,
		0o600,
		func(data []byte) (*map[string]any, error) {
			doc := map[string]any{}
			if len(data) == 0 {
				return &doc, nil
			}
			if err := json.Unmarshal(data, &doc); err != nil {
				return nil, fmt.Errorf("parse %s: %w", cfgPath, err)
			}
			return &doc, nil
		},
		func() *map[string]any {
			doc := map[string]any{}
			return &doc
		},
		func(doc *map[string]any) error {
			projects := coerceObject((*doc)["projects"])
			entry := coerceObject(projects[path])
			sourceEntry := claudeSourceProjectEntry(path, projects)
			globalServers := coerceObject((*doc)["mcpServers"])
			globalEnabled := coerceArray((*doc)["enabledMcpjsonServers"])
			globalDisabled := coerceArray((*doc)["disabledMcpjsonServers"])
			globalContextURIs := coerceArray((*doc)["mcpContextUris"])

			entry["mcpContextUris"] = mergeUniqueStrings(
				globalContextURIs,
				coerceArray(sourceEntry["mcpContextUris"]),
				coerceArray(entry["mcpContextUris"]),
			)
			entry["mcpServers"] = mergeObjects(
				mergeObjects(globalServers, coerceObject(sourceEntry["mcpServers"])),
				coerceObject(entry["mcpServers"]),
			)
			entry["enabledMcpjsonServers"] = mergeUniqueStrings(
				globalEnabled,
				coerceArray(sourceEntry["enabledMcpjsonServers"]),
				coerceArray(entry["enabledMcpjsonServers"]),
			)
			entry["disabledMcpjsonServers"] = mergeUniqueStrings(
				globalDisabled,
				coerceArray(sourceEntry["disabledMcpjsonServers"]),
				coerceArray(entry["disabledMcpjsonServers"]),
			)
			requiredTools := append([]string(nil), claudeAllowedTools...)
			if _, ok := coerceObject(entry["mcpServers"])["context7"]; ok {
				requiredTools = append(requiredTools, claudeContext7Tools...)
			}
			entry["allowedTools"] = mergeAllowedTools(
				mergeUniqueStrings(coerceArray(sourceEntry["allowedTools"]), coerceArray(entry["allowedTools"])),
				requiredTools,
			)
			entry["hasTrustDialogAccepted"] = true
			entry["projectOnboardingSeenCount"] = 1
			entry["hasClaudeMdExternalIncludesApproved"] = true
			entry["hasClaudeMdExternalIncludesWarningShown"] = false
			entry["hasCompletedProjectOnboarding"] = true
			projects[path] = entry
			(*doc)["projects"] = projects
			return nil
		},
		func(doc *map[string]any) ([]byte, error) {
			out, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal %s: %w", cfgPath, err)
			}
			return append(out, '\n'), nil
		},
	); err != nil {
		return fmt.Errorf("write %s: %w", cfgPath, err)
	}
	if err := ensureClaudeProjectLocalHooks(path); err != nil {
		return err
	}
	return nil
}

func ensureClaudeProjectLocalHooks(path string) error {
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable for claude hook bootstrap: %w", err)
	}
	settingsPath := filepath.Join(path, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("mkdir claude local settings dir: %w", err)
	}

	permissionCommand := shellQuote(goalxBin) + " claude-hook permission-request"
	elicitationCommand := shellQuote(goalxBin) + " claude-hook elicitation"
	notificationCommand := shellQuote(goalxBin) + " claude-hook notification"
	if err := mutateStructuredFile(
		settingsPath,
		0o644,
		func(data []byte) (*map[string]any, error) {
			doc := map[string]any{}
			if len(data) == 0 {
				return &doc, nil
			}
			if err := json.Unmarshal(data, &doc); err != nil {
				return nil, fmt.Errorf("parse %s: %w", settingsPath, err)
			}
			return &doc, nil
		},
		func() *map[string]any {
			doc := map[string]any{}
			return &doc
		},
		func(doc *map[string]any) error {
			hooks := coerceObject((*doc)["hooks"])
			hooks["PermissionRequest"] = appendClaudeHookEntry(
				coerceArray(hooks["PermissionRequest"]),
				claudeMCPPermissionHookMatcher,
				permissionCommand,
			)
			hooks["Elicitation"] = appendClaudeHookEntry(
				coerceArray(hooks["Elicitation"]),
				claudeMCPElicitationHookMatcher,
				elicitationCommand,
			)
			hooks["Notification"] = appendClaudeHookEntry(
				coerceArray(hooks["Notification"]),
				claudePermissionNotificationMatcher,
				notificationCommand,
			)
			hooks["Notification"] = appendClaudeHookEntry(
				coerceArray(hooks["Notification"]),
				claudeElicitationNotificationMatcher,
				notificationCommand,
			)
			(*doc)["hooks"] = hooks
			return nil
		},
		func(doc *map[string]any) ([]byte, error) {
			out, err := json.MarshalIndent(doc, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("marshal %s: %w", settingsPath, err)
			}
			return append(out, '\n'), nil
		},
	); err != nil {
		return fmt.Errorf("write %s: %w", settingsPath, err)
	}
	if err := verifyClaudeProjectLocalHooks(path); err != nil {
		return err
	}
	return nil
}

func verifyClaudeProjectLocalHooks(path string) error {
	goalxBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve goalx executable for claude hook verification: %w", err)
	}
	settingsPath := filepath.Join(path, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return fmt.Errorf("read %s for hook verification: %w", settingsPath, err)
	}
	doc := map[string]any{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse %s for hook verification: %w", settingsPath, err)
	}
	hooks := coerceObject(doc["hooks"])
	permissionCommand := shellQuote(goalxBin) + " claude-hook permission-request"
	elicitationCommand := shellQuote(goalxBin) + " claude-hook elicitation"
	notificationCommand := shellQuote(goalxBin) + " claude-hook notification"
	required := []struct {
		name    string
		entries []any
		matcher string
		command string
	}{
		{name: "PermissionRequest", entries: coerceArray(hooks["PermissionRequest"]), matcher: claudeMCPPermissionHookMatcher, command: permissionCommand},
		{name: "Elicitation", entries: coerceArray(hooks["Elicitation"]), matcher: claudeMCPElicitationHookMatcher, command: elicitationCommand},
		{name: "Notification(permission_prompt)", entries: coerceArray(hooks["Notification"]), matcher: claudePermissionNotificationMatcher, command: notificationCommand},
		{name: "Notification(elicitation_dialog)", entries: coerceArray(hooks["Notification"]), matcher: claudeElicitationNotificationMatcher, command: notificationCommand},
	}
	for _, item := range required {
		if claudeHookMatcherHasExactCommand(item.entries, item.matcher, item.command) {
			continue
		}
		return fmt.Errorf("verify %s: missing %s hook matcher %q command %q", settingsPath, item.name, item.matcher, item.command)
	}
	return nil
}

func claudeHookMatcherHasExactCommand(entries []any, matcher, command string) bool {
	for _, raw := range entries {
		entry := coerceObject(raw)
		if strings.TrimSpace(toString(entry["matcher"])) != matcher {
			continue
		}
		for _, hookRaw := range coerceArray(entry["hooks"]) {
			hook := coerceObject(hookRaw)
			if strings.TrimSpace(toString(hook["type"])) != "command" {
				continue
			}
			if strings.TrimSpace(toString(hook["command"])) == command {
				return true
			}
		}
	}
	return false
}

func claudeSourceProjectEntry(path string, projects map[string]any) map[string]any {
	sourcePath := claudeSourceProjectPath(path, projects)
	if sourcePath == "" {
		return map[string]any{}
	}
	return coerceObject(projects[sourcePath])
}

func claudeSourceProjectPath(path string, projects map[string]any) string {
	if sourcePath := gitCommonDirProjectPath(path); sourcePath != "" {
		if sourcePath != path {
			if _, ok := projects[sourcePath]; ok {
				return sourcePath
			}
		}
	}

	best := ""
	for candidate := range projects {
		if candidate == path || looksLikeWorktreeProjectPath(candidate) {
			continue
		}
		if !pathHasPrefix(path, candidate) {
			continue
		}
		if len(candidate) > len(best) {
			best = candidate
		}
	}
	return best
}

func gitCommonDirProjectPath(path string) string {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	commonDir := strings.TrimSpace(string(out))
	if commonDir == "" {
		return ""
	}
	return filepath.Clean(filepath.Dir(commonDir))
}

func looksLikeWorktreeProjectPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.Contains(path, "/.goalx/") || strings.Contains(path, "/worktrees/")
}

func pathHasPrefix(path, prefix string) bool {
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+string(filepath.Separator))
}

func upsertTOMLKey(text, section, keyLine string) string {
	lines := strings.Split(text, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == section {
			start = i
			break
		}
	}

	if start == -1 {
		text = strings.TrimRight(text, "\n")
		if text != "" {
			text += "\n\n"
		}
		return text + section + "\n" + keyLine + "\n"
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			end = i
			break
		}
	}

	for i := start + 1; i < end; i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "trust_level") {
			lines[i] = keyLine
			return strings.Join(lines, "\n")
		}
	}

	lines = append(lines[:start+1], append([]string{keyLine}, lines[start+1:]...)...)
	return strings.Join(lines, "\n")
}

func escapeTOMLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

func writeFilePreserveMode(path string, data []byte, defaultMode os.FileMode) error {
	mode := defaultMode
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
	}
	return os.WriteFile(path, data, mode)
}

func coerceObject(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func coerceArray(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	return []any{}
}

func mergeAllowedTools(existing []any, required []string) []any {
	merged := append([]any(nil), existing...)
	seen := map[string]bool{}
	for _, item := range existing {
		if tool, ok := item.(string); ok {
			seen[tool] = true
		}
	}
	for _, tool := range required {
		if seen[tool] {
			continue
		}
		merged = append(merged, tool)
		seen[tool] = true
	}
	return merged
}

func mergeUniqueStrings(arrays ...[]any) []any {
	var merged []any
	seen := map[string]bool{}
	for _, array := range arrays {
		for _, item := range array {
			s, ok := item.(string)
			if !ok || seen[s] {
				continue
			}
			merged = append(merged, s)
			seen[s] = true
		}
	}
	return merged
}

func mergeObjects(base, overlay map[string]any) map[string]any {
	merged := map[string]any{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overlay {
		merged[key] = value
	}
	return merged
}

func appendClaudeHookEntry(entries []any, matcher, command string) []any {
	for _, raw := range entries {
		entry := coerceObject(raw)
		if strings.TrimSpace(toString(entry["matcher"])) != matcher {
			continue
		}
		for _, hookRaw := range coerceArray(entry["hooks"]) {
			hook := coerceObject(hookRaw)
			if strings.TrimSpace(toString(hook["type"])) == "command" && strings.TrimSpace(toString(hook["command"])) == command {
				return entries
			}
		}
	}
	out := append([]any(nil), entries...)
	out = append(out, map[string]any{
		"matcher": matcher,
		"hooks": []any{
			map[string]any{
				"type":    "command",
				"command": command,
			},
		},
	})
	return out
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
