package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestRenderSubagentProtocolIncludesResumeInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:    "ship it",
		Mode:         goalx.ModeDevelop,
		Engine:       "codex",
		Target:       goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:      goalx.HarnessConfig{Command: "go test ./..."},
		SessionName:  "session-1",
		JournalPath:  "/tmp/journal.jsonl",
		GuidancePath: "/tmp/guidance.md",
		WorktreePath: "/tmp/worktree",
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Critical References (do not forget)",
		"Journal: `/tmp/journal.jsonl`",
		"Guidance: `/tmp/guidance.md`",
		"Objective: ship it",
		"Gate: `go test ./...`",
		"Before doing new work, first reconstruct context",
		"Read your existing journal",
		"Read the latest master guidance",
		"Inspect the current worktree state",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolIncludesEngineSpecificGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:    "investigate auth",
		Mode:         goalx.ModeResearch,
		Engine:       "claude-code",
		Target:       goalx.TargetConfig{Files: []string{"report.md"}},
		JournalPath:  "/tmp/journal.jsonl",
		GuidancePath: "/tmp/guidance.md",
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Your Tools",
		"Agent tool",
		"WebSearch/WebFetch",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolIncludesCodexGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:    "ship it",
		Mode:         goalx.ModeDevelop,
		Engine:       "codex",
		Target:       goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:      goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:  "/tmp/journal.jsonl",
		GuidancePath: "/tmp/guidance.md",
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"You are running in Codex CLI with file system access and shell execution.",
		"Check your guidance file (`/tmp/guidance.md`) for new instructions from the master agent.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolIncludesTeamContext(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "investigate auth",
		Mode:           goalx.ModeResearch,
		Engine:         "codex",
		Target:         goalx.TargetConfig{Files: []string{"report.md"}},
		SessionName:    "session-1",
		JournalPath:    "/tmp/journal.jsonl",
		GuidancePath:   "/tmp/guidance.md",
		AcceptancePath: "/tmp/acceptance.md",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/worktree-1"},
			{Name: "session-2", WorktreePath: "/tmp/worktree-2"},
		},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Team Context",
		"session-1",
		"session-2",
		"of 2 sessions",
		"acceptance.md",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesGoalContractChecklistInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "ship it",
		RunName:        "demo",
		Mode:           goalx.ModeDevelop,
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		StatusPath:     "/tmp/status.json",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
		PlannedSessions: []PlannedSessionData{
			{
				Name:   "session-1",
				Engine: "codex",
				Model:  "codex",
				Hint:   "P0 fixes",
			},
		},
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Startup",
		"Write an acceptance checklist",
		"/tmp/acceptance.md",
		"goalx add --run demo",
		"Do NOT wait for external heartbeat prompts.",
		"## Guidance Writing Principles",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesResearchPreflightDimensionSelection(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "audit auth",
		RunName:        "demo",
		Mode:           goalx.ModeResearch,
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
		PlannedSessions: []PlannedSessionData{
			{
				Name:   "session-1",
				Engine: "codex",
				Model:  "codex",
				Hint:   "depth",
			},
			{
				Name:   "session-2",
				Engine: "codex",
				Model:  "codex",
				Hint:   "adversarial",
			},
		},
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"Assess the objective's scope (quick fix vs deep research)",
		"Review the configured session plan and decide which sessions to launch immediately versus later.",
		"Launch each chosen session yourself with `goalx add --run demo ...`.",
		"Write a distinct dimension assignment to each launched session's guidance file.",
		"Write an acceptance checklist (3-7 testable bullets)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesTransitionRecommendationInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "ship it",
		RunName:        "demo",
		Mode:           goalx.ModeDevelop,
		Preset:         "claude",
		Engines:        goalx.BuiltinEngines,
		Master:         goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		StatusPath:     "/tmp/status.json",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
		PlannedSessions: []PlannedSessionData{
			{
				Name:   "session-1",
				Engine: "codex",
				Model:  "codex",
				Hint:   "P0 fixes",
			},
		},
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"## Available Engines",
		"## Current Configuration",
		"- Run: demo",
		"- Preset: claude",
		"- session-1: codex/codex (P0 fixes)",
		"Write summary to `/tmp/summary.md`",
		"Update `/tmp/status.json`. Include `next_config` for every non-`done` recommendation:",
		"Supported `next_config` keys: `parallel`, `engine`, `model`, `preset`, `diversity_hints`, `strategies`, `budget_seconds`, `objective`, `harness`, `mode`, `max_iterations`, `context`, `master_engine`, `master_model`, `sessions`.",
		`{"phase":"complete","recommendation":"implement","heartbeat":5,"acceptance_met":true,"next_config":{"objective":"implement the agreed P0/P1 fixes","mode":"develop","parallel":3,"diversity_hints":["P0 fixes","P1 fixes","verification"],"engine":"codex","model":"codex","budget_seconds":7200}}`,
		`{"phase":"complete","recommendation":"debate","heartbeat":5,"acceptance_met":false,"next_config":{"objective":"debate the remaining disagreements","parallel":2,"diversity_hints":["advocate position","critic position"]}}`,
		`{"phase":"complete","recommendation":"more-research","heartbeat":5,"acceptance_met":false,"next_objective":"investigate the unresolved auth edge cases","next_config":{"parallel":2,"strategies":["depth","adversarial"],"budget_seconds":3600,"max_iterations":3,"context":["/path/to/prior-findings.md"]}}`,
		`{"phase":"complete","recommendation":"done","heartbeat":3,"acceptance_met":true,"keep_session":"session-1","next_objective":""}`,
		"| `implement` | Acceptance criteria are met for this phase and the next step is code changes. | true |",
		"Functional verification: for each acceptance checklist item, record concrete PASS/FAIL evidence beyond the gate.",
		"Set `keep_session` when a develop-mode session should be merged after the run.",
		"Default action for the first 3+ check cycles is **push deeper**.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}
