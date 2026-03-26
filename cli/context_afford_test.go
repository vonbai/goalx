package cli

import (
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestContextCommandRejectsPositionalRunWhenRunFlagProvided(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)

	err := Context(repo, []string{"--run", cfg.Name, "other-run"})
	if err == nil || !strings.Contains(err.Error(), contextUsage) {
		t.Fatalf("Context error = %v, want usage error", err)
	}
}

func TestAffordCommandRejectsPositionalRunWhenRunFlagProvided(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)

	err := Afford(repo, []string{"--run", cfg.Name, "other-run", "master"})
	if err == nil || !strings.Contains(err.Error(), affordUsage) {
		t.Fatalf("Afford error = %v, want usage error", err)
	}
}

func TestContextCommandPrintsRunIndex(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, goalx.RunDir(repo, cfg.Name), cfg)

	out := captureStdout(t, func() {
		if err := Context(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Context: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Context",
		"## Run Identity",
		"Objective:",
		"Run dir:",
		"Context index:",
		"## Provider Facts",
		"GoalX canonical provider runtime is tmux + interactive TUI.",
		"Interactive Codex sessions can use installed skills and configured MCP servers from the native TUI.",
		"session-1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("context output missing %q:\n%s", want, out)
		}
	}
}

func TestAffordCommandPrintsMarkdownAffordances(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, goalx.RunDir(repo, cfg.Name), cfg)

	out := captureStdout(t, func() {
		if err := Afford(repo, []string{"--run", cfg.Name, "master"}); err != nil {
			t.Fatalf("Afford: %v", err)
		}
	})

	for _, want := range []string{
		"# GoalX Affordances",
		"goalx context --run " + cfg.Name,
		"goalx afford --run " + cfg.Name + " master",
		"## provider-facts",
		"## tell",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("afford output missing %q:\n%s", want, out)
		}
	}
}

func TestAffordCommandJsonAllowsFlagBeforeTarget(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)

	out := captureStdout(t, func() {
		if err := Afford(repo, []string{"--run", cfg.Name, "--json", "master"}); err != nil {
			t.Fatalf("Afford --json: %v", err)
		}
	})

	if !strings.Contains(out, `"run_name": "guidance-run"`) || !strings.Contains(out, `"target": "master"`) {
		t.Fatalf("afford json output missing expected keys:\n%s", out)
	}
}

func TestContextCommandJsonPrintsMachineReadableIndex(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)

	out := captureStdout(t, func() {
		if err := Context(repo, []string{"--run", cfg.Name, "--json"}); err != nil {
			t.Fatalf("Context --json: %v", err)
		}
	})

	if !strings.Contains(out, `"context_index_path"`) || !strings.Contains(out, `"run_name": "guidance-run"`) || !strings.Contains(out, `"run_identity"`) {
		t.Fatalf("context json output missing expected keys:\n%s", out)
	}
}

func TestAffordCommandPrintsProviderFactsForClaudeSession(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	sessionName := "session-1"
	if err := EnsureSessionControl(runDir, sessionName); err != nil {
		t.Fatalf("EnsureSessionControl: %v", err)
	}
	identity := &SessionIdentity{
		Version:         1,
		SessionName:     sessionName,
		RoleKind:        "research",
		Mode:            string(goalx.ModeResearch),
		Engine:          "claude-code",
		Model:           "opus",
		OriginCharterID: meta.CharterID,
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, sessionName), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Afford(repo, []string{"--run", cfg.Name, sessionName}); err != nil {
			t.Fatalf("Afford: %v", err)
		}
	})

	for _, want := range []string{
		"## provider-facts",
		"claude-code",
		"Provider-native capability facts for `session-1` (`claude-code`).",
		"GoalX canonical provider runtime is tmux + interactive TUI.",
		"Interactive Claude sessions can use installed skills, plugins, and MCP servers from the native TUI.",
		"Claude root sessions cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("afford output missing %q:\n%s", want, out)
		}
	}
}
