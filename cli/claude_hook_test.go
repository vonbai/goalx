package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildClaudeElicitationHookOutputCancelsUnattendedRequests(t *testing.T) {
	input := []byte(`{
	  "hook_event_name":"Elicitation",
	  "mcp_server_name":"playwright",
	  "message":"Please authenticate",
	  "mode":"url",
	  "url":"https://example.test/login"
	}`)

	out, err := buildClaudeElicitationHookOutput(input)
	if err != nil {
		t.Fatalf("buildClaudeElicitationHookOutput: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("unmarshal hook output: %v", err)
	}
	hookSpecific := doc["hookSpecificOutput"].(map[string]any)
	if hookSpecific["hookEventName"] != "Elicitation" {
		t.Fatalf("hookEventName = %#v, want Elicitation", hookSpecific["hookEventName"])
	}
	if hookSpecific["action"] != "cancel" {
		t.Fatalf("action = %#v, want cancel", hookSpecific["action"])
	}
}

func TestRecordClaudeNotificationWritesUrgentMasterInboxMessage(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if err := os.MkdirAll(RunWorktreePath(runDir), 0o755); err != nil {
		t.Fatalf("mkdir run worktree: %v", err)
	}

	input := []byte(`{
	  "hook_event_name":"Notification",
	  "cwd":"` + RunWorktreePath(runDir) + `",
	  "notification_type":"permission_prompt",
	  "title":"Permission needed",
	  "message":"Claude needs your permission to use Playwright"
	}`)

	if err := recordClaudeNotification(input); err != nil {
		t.Fatalf("recordClaudeNotification: %v", err)
	}
	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	lines := splitNonEmptyLines(string(data))
	if len(lines) != 1 {
		t.Fatalf("master inbox lines = %d, want 1; got %q", len(lines), string(data))
	}
	var msg MasterInboxMessage
	if err := json.Unmarshal([]byte(lines[0]), &msg); err != nil {
		t.Fatalf("unmarshal master inbox message: %v", err)
	}
	if msg.Type != "provider-dialog-visible" || !msg.Urgent {
		t.Fatalf("master inbox message = %+v, want urgent provider-dialog-visible", msg)
	}
	for _, want := range []string{"target=master", "type=permission_prompt", "Playwright"} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("master inbox body missing %q: %s", want, msg.Body)
		}
	}
}

func TestRecordClaudeElicitationFactWritesUrgentMasterInboxMessage(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	seedGuidanceSessionFixture(t, runDir, cfg)

	input := []byte(`{
	  "hook_event_name":"Elicitation",
	  "cwd":"` + WorktreePath(runDir, cfg.Name, 1) + `",
	  "mcp_server_name":"playwright",
	  "message":"Please authenticate",
	  "mode":"url",
	  "url":"https://example.test/login"
	}`)

	if err := recordClaudeElicitationFact(input); err != nil {
		t.Fatalf("recordClaudeElicitationFact: %v", err)
	}
	data, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	lines := splitNonEmptyLines(string(data))
	if len(lines) != 1 {
		t.Fatalf("master inbox lines = %d, want 1; got %q", len(lines), string(data))
	}
	var msg MasterInboxMessage
	if err := json.Unmarshal([]byte(lines[0]), &msg); err != nil {
		t.Fatalf("unmarshal master inbox message: %v", err)
	}
	if msg.Type != "provider-elicitation-cancelled" || !msg.Urgent {
		t.Fatalf("master inbox message = %+v, want urgent provider-elicitation-cancelled", msg)
	}
	for _, want := range []string{"target=session-1", "server=playwright", "mode=url", "url=https://example.test/login"} {
		if !strings.Contains(msg.Body, want) {
			t.Fatalf("master inbox body missing %q: %s", want, msg.Body)
		}
	}
}

func TestResolveClaudeHookRunContextMatchesConfiguredProjectWorktrees(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.WorktreeRoot = ".worktrees"
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	if err := SaveGlobalRunRegistry(&GlobalRunRegistry{
		Version: 1,
		Runs: map[string]GlobalRunRef{
			"guidance": {
				Key:         "guidance",
				Name:        cfg.Name,
				ProjectRoot: repo,
				RunDir:      runDir,
				State:       "active",
			},
		},
	}); err != nil {
		t.Fatalf("SaveGlobalRunRegistry: %v", err)
	}

	runWT := RunWorktreePath(runDir)
	if err := os.MkdirAll(runWT, 0o755); err != nil {
		t.Fatalf("mkdir run worktree: %v", err)
	}
	if gotRunDir, target, ok := resolveClaudeHookRunContext(runWT); !ok || gotRunDir != runDir || target != "master" {
		t.Fatalf("resolveClaudeHookRunContext(runWT) = (%q, %q, %v), want (%q, master, true)", gotRunDir, target, ok, runDir)
	}

	seedGuidanceSessionFixture(t, runDir, cfg)
	sessionWT := WorktreePath(runDir, cfg.Name, 1)
	if gotRunDir, target, ok := resolveClaudeHookRunContext(sessionWT); !ok || gotRunDir != runDir || target != "session-1" {
		t.Fatalf("resolveClaudeHookRunContext(sessionWT) = (%q, %q, %v), want (%q, session-1, true)", gotRunDir, target, ok, runDir)
	}

	if got := CanonicalProjectRoot(runWT); got != repo {
		t.Fatalf("CanonicalProjectRoot(runWT) = %q, want %q", got, repo)
	}
}

func TestRunWorktreePathFallsBackToLegacyConfiguredRootName(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.WorktreeRoot = ".worktrees"
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	if _, err := EnsureRunMetadata(runDir, repo, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}

	legacyRoot := filepath.Join(repo, ".worktrees", cfg.Name+"-root")
	if err := os.MkdirAll(legacyRoot, 0o755); err != nil {
		t.Fatalf("mkdir legacy run worktree: %v", err)
	}

	if got := RunWorktreePath(runDir); got != legacyRoot {
		t.Fatalf("RunWorktreePath = %q, want legacy fallback %q", got, legacyRoot)
	}
}
