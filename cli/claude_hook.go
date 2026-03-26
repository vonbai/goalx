package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const claudeHookUsage = "usage: goalx claude-hook permission-request"

func ClaudeHook(_ string, args []string) error {
	if len(args) != 1 || strings.TrimSpace(args[0]) != "permission-request" {
		return fmt.Errorf(claudeHookUsage)
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read claude hook input: %w", err)
	}
	output, err := buildClaudePermissionRequestHookOutput(input)
	if err != nil {
		return err
	}
	if len(output) == 0 {
		return nil
	}
	if _, err := os.Stdout.Write(output); err != nil {
		return fmt.Errorf("write claude hook output: %w", err)
	}
	return nil
}

func buildClaudePermissionRequestHookOutput(input []byte) ([]byte, error) {
	if len(strings.TrimSpace(string(input))) == 0 {
		return nil, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, fmt.Errorf("parse claude PermissionRequest payload: %w", err)
	}
	if strings.TrimSpace(toString(payload["hook_event_name"])) != "PermissionRequest" {
		return nil, nil
	}
	decision := map[string]any{
		"behavior": "allow",
	}
	if updates := selectClaudePermissionUpdates(coerceArray(payload["permission_suggestions"])); len(updates) > 0 {
		decision["updatedPermissions"] = updates
	}
	out, err := json.Marshal(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PermissionRequest",
			"decision":      decision,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("marshal claude PermissionRequest hook output: %w", err)
	}
	return append(out, '\n'), nil
}

func selectClaudePermissionUpdates(suggestions []any) []any {
	var firstAllow any
	for _, raw := range suggestions {
		suggestion := coerceObject(raw)
		if strings.TrimSpace(toString(suggestion["behavior"])) != "allow" {
			continue
		}
		if strings.TrimSpace(toString(suggestion["destination"])) == "localSettings" {
			return []any{suggestion}
		}
		if firstAllow == nil {
			firstAllow = suggestion
		}
	}
	if firstAllow == nil {
		return nil
	}
	return []any{firstAllow}
}
