package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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

	var text string
	if data, err := os.ReadFile(cfgPath); err == nil {
		text = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read codex config: %w", err)
	}

	section := `[projects."` + escapeTOMLString(path) + `"]`
	text = upsertTOMLKey(text, section, `trust_level = "trusted"`)
	text = upsertTOMLKey(text, section, `sandbox_mode = "danger-full-access"`)
	if err := writeFilePreserveMode(cfgPath, []byte(text), 0o600); err != nil {
		return fmt.Errorf("write codex config: %w", err)
	}
	return nil
}

func ensureClaudeTrusted(path string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home dir: %w", err)
	}
	cfgPath := filepath.Join(home, ".claude.json")

	doc := map[string]any{}
	if data, err := os.ReadFile(cfgPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", cfgPath, err)
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", cfgPath, err)
	}

	projects := coerceObject(doc["projects"])
	entry := coerceObject(projects[path])
	entry["allowedTools"] = coerceArray(entry["allowedTools"])
	entry["mcpContextUris"] = coerceArray(entry["mcpContextUris"])
	entry["mcpServers"] = coerceObject(entry["mcpServers"])
	entry["enabledMcpjsonServers"] = coerceArray(entry["enabledMcpjsonServers"])
	entry["disabledMcpjsonServers"] = coerceArray(entry["disabledMcpjsonServers"])
	entry["hasTrustDialogAccepted"] = true
	entry["projectOnboardingSeenCount"] = 1
	entry["hasClaudeMdExternalIncludesApproved"] = false
	entry["hasClaudeMdExternalIncludesWarningShown"] = false
	entry["hasCompletedProjectOnboarding"] = true
	projects[path] = entry
	doc["projects"] = projects

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", cfgPath, err)
	}
	out = append(out, '\n')
	if err := writeFilePreserveMode(cfgPath, out, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", cfgPath, err)
	}
	return nil
}

func upsertTOMLKey(text, section, keyLine string) string {
	key := strings.TrimSpace(keyLine)
	if idx := strings.Index(key, "="); idx >= 0 {
		key = strings.TrimSpace(key[:idx])
	}

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
		if strings.HasPrefix(strings.TrimSpace(lines[i]), key) {
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
