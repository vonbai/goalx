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
		Mode:           goalx.ModeDevelop,
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
		Sessions: []SessionData{
			{
				Name:         "session-1",
				WindowName:   "demo-1",
				WorktreePath: "/tmp/worktree",
				JournalPath:  "/tmp/journal.jsonl",
				GuidancePath: "/tmp/guidance.md",
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
		"## Before First Heartbeat",
		"Write an acceptance checklist",
		"/tmp/acceptance.md",
		"Then wait for Heartbeat prompts. Do NOT loop on your own.",
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
		Mode:           goalx.ModeResearch,
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
		Sessions: []SessionData{
			{
				Name:         "session-1",
				WindowName:   "demo-1",
				WorktreePath: "/tmp/worktree-1",
				JournalPath:  "/tmp/journal-1.jsonl",
				GuidancePath: "/tmp/guidance-1.md",
			},
			{
				Name:         "session-2",
				WindowName:   "demo-2",
				WorktreePath: "/tmp/worktree-2",
				JournalPath:  "/tmp/journal-2.jsonl",
				GuidancePath: "/tmp/guidance-2.md",
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
		"Decide which research dimensions matter most. If sessions are insufficient, use `goalx add`",
		"Write a distinct dimension assignment to each session's guidance file",
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
		Mode:           goalx.ModeDevelop,
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		StatusPath:     "/tmp/status.json",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
		Sessions: []SessionData{
			{
				Name:         "session-1",
				WindowName:   "demo-1",
				WorktreePath: "/tmp/worktree",
				JournalPath:  "/tmp/journal.jsonl",
				GuidancePath: "/tmp/guidance.md",
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
		"Write summary to `/tmp/summary.md`",
		"Update `/tmp/status.json` with JSON: phase, recommendation, heartbeat count, acceptance_met, keep_session, next_objective",
		`{"phase":"complete","recommendation":"implement","heartbeat":3,"acceptance_met":true,"keep_session":"session-1","next_objective":""}`,
		"Set `keep_session` when a develop-mode session should be merged after the run.",
		"Default action for the first 3+ heartbeats is **push deeper**.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}
