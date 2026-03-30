package cli

import (
	"os"
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
	runDir := goalx.RunDir(repo, cfg.Name)
	seedGuidanceSessionFixture(t, runDir, cfg)
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	identity.BaseBranchSelector = "run-root"
	identity.BaseBranch = "goalx/" + cfg.Name + "/root"
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity for rewrite: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity rewrite: %v", err)
	}

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
		"Experiment ledger:",
		"Integration state:",
		"Closeout/evidence surface:",
		"Context index:",
		"Memory query:",
		"Memory context:",
		"## Provider Runtime",
		"GoalX canonical provider runtime is tmux + interactive TUI.",
		"GoalX provider runtime does not change durable ownership boundaries.",
		"session-1",
		"base selector",
		"run-root",
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
		"## provider-runtime",
		"## tell",
		"only merges committed session branch history",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("afford output missing %q:\n%s", want, out)
		}
	}
}

func TestAffordCommandPrintsSessionWorktreeBoundaryFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	identity.BaseBranchSelector = "run-root"
	identity.BaseBranch = "goalx/" + cfg.Name + "/root"
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity for rewrite: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Afford(repo, []string{"--run", cfg.Name, "session-1"}); err != nil {
			t.Fatalf("Afford: %v", err)
		}
	})

	for _, want := range []string{
		"## worktree-boundary",
		"Default edit boundary is this session's dedicated worktree.",
		"Do not edit the source root or run-root worktree from a dedicated session unless master explicitly redirects you to inspect or integrate there.",
		"Recorded parent/base selector: `run-root`.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("afford output missing %q:\n%s", want, out)
		}
	}
}

func TestAffordCommandPrintsDeclaredReadonlyBoundaryFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	identity.Target = goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}}
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Afford(repo, []string{"--run", cfg.Name, "session-1"}); err != nil {
			t.Fatalf("Afford: %v", err)
		}
	})

	for _, want := range []string{
		"Declared readonly paths: `.`.",
		"GoalX records this as an execution contract.",
		"stop and redirect instead of crossing the readonly boundary",
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

	for _, want := range []string{
		`"context_index_path"`,
		`"experiments_log_path"`,
		`"integration_state_path"`,
		`"memory_query_path"`,
		`"memory_context_path"`,
		`"run_name": "guidance-run"`,
		`"run_identity"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("context json output missing %q:\n%s", want, out)
		}
	}
}

func TestContextIndexIncludesMemoryQueryAndContextPaths(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}

	if index.MemoryQueryPath != MemoryQueryPath(runDir) {
		t.Fatalf("MemoryQueryPath = %q, want %q", index.MemoryQueryPath, MemoryQueryPath(runDir))
	}
	if index.MemoryContextPath != MemoryContextPath(runDir) {
		t.Fatalf("MemoryContextPath = %q, want %q", index.MemoryContextPath, MemoryContextPath(runDir))
	}
}

func TestContextDoesNotMutateCanonicalMemory(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	payloads := writeCanonicalMemorySentinels(t, home)

	out := captureStdout(t, func() {
		if err := Context(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Context: %v", err)
		}
	})
	if !strings.Contains(out, "# GoalX Context") {
		t.Fatalf("context output missing header:\n%s", out)
	}

	for path, want := range payloads {
		assertFileUnchanged(t, path, want)
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
		ExperimentID:    "exp_guidance_claude_session_1",
		RoleKind:        "research",
		Mode:            string(goalx.ModeWorker),
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
		"## provider-runtime",
		"claude-code",
		"Provider runtime and bootstrap facts for `session-1` (`claude-code`).",
		"GoalX canonical provider runtime is tmux + interactive TUI.",
		"GoalX provider runtime does not change durable ownership boundaries.",
		"Claude root sessions cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions.",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("afford output missing %q:\n%s", want, out)
		}
	}
}
