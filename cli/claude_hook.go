package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const claudeHookUsage = "usage: goalx claude-hook permission-request|elicitation|notification"

func ClaudeHook(_ string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf(claudeHookUsage)
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read claude hook input: %w", err)
	}
	var output []byte
	switch strings.TrimSpace(args[0]) {
	case "permission-request":
		output, err = buildClaudePermissionRequestHookOutput(input)
	case "elicitation":
		output, err = buildClaudeElicitationHookOutput(input)
		if err == nil {
			err = recordClaudeElicitationFact(input)
		}
	case "notification":
		err = recordClaudeNotification(input)
	default:
		return fmt.Errorf(claudeHookUsage)
	}
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
	payload, err := parseClaudeHookPayload(input, "PermissionRequest")
	if err != nil || payload == nil {
		return nil, err
	}
	decision := map[string]any{
		"behavior": "allow",
	}
	if updates := selectClaudePermissionUpdates(coerceArray(payload["permission_suggestions"])); len(updates) > 0 {
		decision["updatedPermissions"] = updates
	}
	return marshalClaudeHookOutput(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "PermissionRequest",
			"decision":      decision,
		},
	})
}

func buildClaudeElicitationHookOutput(input []byte) ([]byte, error) {
	payload, err := parseClaudeHookPayload(input, "Elicitation")
	if err != nil || payload == nil {
		return nil, err
	}
	return marshalClaudeHookOutput(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName": "Elicitation",
			"action":        "cancel",
		},
	})
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

func recordClaudeElicitationFact(input []byte) error {
	payload, err := parseClaudeHookPayload(input, "Elicitation")
	if err != nil || payload == nil {
		return err
	}
	mode := strings.TrimSpace(toString(payload["mode"]))
	server := strings.TrimSpace(toString(payload["mcp_server_name"]))
	message := compactHookText(toString(payload["message"]))
	url := strings.TrimSpace(toString(payload["url"]))
	body := fmt.Sprintf("Claude MCP elicitation auto-cancelled for unattended GoalX run; target=%s server=%s mode=%s message=%s", resolveClaudeHookTargetLabel(payload), blankAsUnknown(server), blankAsUnknown(mode), blankAsUnknown(message))
	if url != "" {
		body += " url=" + url
	}
	return appendClaudeHookMasterUrgent(payload, "provider-elicitation-cancelled", body)
}

func recordClaudeNotification(input []byte) error {
	payload, err := parseClaudeHookPayload(input, "Notification")
	if err != nil || payload == nil {
		return err
	}
	notificationType := strings.TrimSpace(toString(payload["notification_type"]))
	if notificationType != claudePermissionNotificationMatcher && notificationType != claudeElicitationNotificationMatcher {
		return nil
	}
	title := compactHookText(toString(payload["title"]))
	message := compactHookText(toString(payload["message"]))
	body := fmt.Sprintf("Claude dialog still surfaced in unattended GoalX run; target=%s type=%s title=%s message=%s", resolveClaudeHookTargetLabel(payload), notificationType, blankAsUnknown(title), blankAsUnknown(message))
	return appendClaudeHookMasterUrgent(payload, "provider-dialog-visible", body)
}

func appendClaudeHookMasterUrgent(payload map[string]any, typ, body string) error {
	cwd := strings.TrimSpace(toString(payload["cwd"]))
	runDir, _, ok := resolveClaudeHookRunContext(cwd)
	if !ok {
		return nil
	}
	_, err := appendControlInboxMessage(runDir, "master", typ, "goalx claude hook", body, true)
	return err
}

func resolveClaudeHookTargetLabel(payload map[string]any) string {
	cwd := strings.TrimSpace(toString(payload["cwd"]))
	runDir, target, ok := resolveClaudeHookRunContext(cwd)
	if !ok || target == "" {
		return blankAsUnknown(target)
	}
	if target != "master" {
		return target
	}
	if pathHasPrefix(filepath.Clean(cwd), filepath.Clean(RunWorktreePath(runDir))) {
		return "master"
	}
	return "shared-run-worktree"
}

func resolveClaudeHookRunContext(cwd string) (runDir, target string, ok bool) {
	absCWD, err := filepath.Abs(strings.TrimSpace(cwd))
	if err != nil || strings.TrimSpace(absCWD) == "" {
		return "", "", false
	}
	if runDir, ok = enclosingRunDirFromWorktree(absCWD); ok {
		return resolveClaudeHookRunContextForRun(absCWD, runDir)
	}

	projectRoot := CanonicalProjectRoot(absCWD)
	if strings.TrimSpace(projectRoot) != "" {
		if reg, err := LoadProjectRegistry(projectRoot); err == nil && reg != nil {
			runNames := make([]string, 0, len(reg.ActiveRuns))
			if focused := strings.TrimSpace(reg.FocusedRun); focused != "" {
				if _, ok := reg.ActiveRuns[focused]; ok {
					runNames = append(runNames, focused)
				}
			}
			for name := range reg.ActiveRuns {
				if name == reg.FocusedRun {
					continue
				}
				runNames = append(runNames, name)
			}
			for _, runName := range runNames {
				candidateRunDir := goalx.RunDir(projectRoot, runName)
				if runDir, target, ok := resolveClaudeHookRunContextForRun(absCWD, candidateRunDir); ok {
					return runDir, target, true
				}
			}
		}
	}

	if global, err := LoadGlobalRunRegistry(); err == nil && global != nil {
		for _, ref := range global.Runs {
			if strings.TrimSpace(ref.RunDir) == "" {
				continue
			}
			if runDir, target, ok := resolveClaudeHookRunContextForRun(absCWD, ref.RunDir); ok {
				return runDir, target, true
			}
		}
	}
	return "", "", false
}

func resolveClaudeHookRunContextForRun(absCWD, runDir string) (runDirOut, target string, ok bool) {
	if !pathHasPrefix(absCWD, RunWorktreePath(runDir)) {
		sessionsState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
		if err == nil && sessionsState != nil {
			for name, session := range sessionsState.Sessions {
				if strings.TrimSpace(session.WorktreePath) == "" {
					continue
				}
				if pathHasPrefix(absCWD, session.WorktreePath) {
					return runDir, name, true
				}
			}
		}
		return "", "", false
	}
	return runDir, "master", true
}

func parseClaudeHookPayload(input []byte, eventName string) (map[string]any, error) {
	if len(strings.TrimSpace(string(input))) == 0 {
		return nil, nil
	}
	var payload map[string]any
	if err := json.Unmarshal(input, &payload); err != nil {
		return nil, fmt.Errorf("parse claude %s payload: %w", eventName, err)
	}
	if strings.TrimSpace(toString(payload["hook_event_name"])) != eventName {
		return nil, nil
	}
	return payload, nil
}

func marshalClaudeHookOutput(doc map[string]any) ([]byte, error) {
	out, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal claude hook output: %w", err)
	}
	return append(out, '\n'), nil
}

func compactHookText(s string) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if len(s) <= 200 {
		return s
	}
	return strings.TrimSpace(s[:197]) + "..."
}

func blankAsUnknown(s string) string {
	if strings.TrimSpace(s) == "" {
		return "unknown"
	}
	return s
}
