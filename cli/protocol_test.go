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
		RunName:      "demo",
		Objective:    "ship it",
		Mode:         goalx.ModeDevelop,
		Engine:       "codex",
		ProjectRoot:  "/tmp/project",
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
		"## Run State",
		"Journal: `/tmp/journal.jsonl`",
		"Guidance: `/tmp/guidance.md`",
		"Objective: ship it",
		"Gate: `go test ./...`",
		"## Resume From Durable State",
		"Do not rebuild the full chat history",
		"Read the recent journal tail",
		"Read the latest master guidance",
		"Inspect the current worktree state",
		"Resume from the current files and latest durable state",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
	if strings.Contains(text, "reconstruct context") {
		t.Fatalf("rendered protocol should not emphasize reconstructing full context:\n%s", text)
	}
}

func TestRenderSubagentProtocolIncludesEngineSpecificGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:      "demo",
		Objective:    "investigate auth",
		Mode:         goalx.ModeResearch,
		Engine:       "claude-code",
		ProjectRoot:  "/tmp/project",
		SessionName:  "session-1",
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
		RunName:      "demo",
		Objective:    "ship it",
		Mode:         goalx.ModeDevelop,
		Engine:       "codex",
		ProjectRoot:  "/tmp/project",
		SessionName:  "session-1",
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
		"Rely on the current filesystem and durable run state.",
		"re-check `/tmp/guidance.md` before idling",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolKeepsResearchMethodologyConcise(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:      "demo",
		Objective:    "investigate auth",
		Mode:         goalx.ModeResearch,
		Engine:       "codex",
		ProjectRoot:  "/tmp/project",
		SessionName:  "session-1",
		Target:       goalx.TargetConfig{Files: []string{"report.md"}},
		JournalPath:  "/tmp/journal.jsonl",
		GuidancePath: "/tmp/guidance.md",
		Context:      goalx.ContextConfig{Files: []string{"/tmp/context.md"}},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	modeSection := sectionBetween(string(out), "## Mode: Research", "## Context")
	for _, want := range []string{
		"findings will be verified by the master",
		"Quantify",
		"Every major finding MUST have a Counter-evidence section",
		"## Key Findings",
		"## Recommendation",
		"## Priority Fix List (if applicable)",
		"dispatchable_slices",
		"directly adopt",
		"Each round should produce NEW insight",
		"Do not declare yourself done",
	} {
		if !strings.Contains(modeSection, want) {
			t.Fatalf("research mode section missing %q:\n%s", want, modeSection)
		}
	}
	if got := nonEmptyLineCount(modeSection); got > 25 {
		t.Fatalf("research mode section has %d non-empty lines, want <= 25:\n%s", got, modeSection)
	}
}

func TestRenderSubagentProtocolKeepsDevelopMethodologyConcise(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:      "demo",
		Objective:    "ship it",
		Mode:         goalx.ModeDevelop,
		Engine:       "codex",
		ProjectRoot:  "/tmp/project",
		SessionName:  "session-1",
		Target:       goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:      goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:  "/tmp/journal.jsonl",
		GuidancePath: "/tmp/guidance.md",
		Context:      goalx.ContextConfig{Files: []string{"/tmp/context.md"}},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	modeSection := sectionBetween(string(out), "## Mode: Develop", "## Context")
	for _, want := range []string{
		"one coherent capability slice at a time",
		"Your code will be reviewed. Every change must be justified and minimal.",
		"Run the full gate",
		"Atomic commit",
		"Keep changes minimal and correct. Do not add unrelated improvements, but do not cut corners on the change you are making.",
		"go test ./...",
	} {
		if !strings.Contains(modeSection, want) {
			t.Fatalf("develop mode section missing %q:\n%s", want, modeSection)
		}
	}
	if strings.Contains(modeSection, "avoid gold-plating") {
		t.Fatalf("develop mode section should replace legacy gold-plating guidance:\n%s", modeSection)
	}
	if got := nonEmptyLineCount(modeSection); got > 25 {
		t.Fatalf("develop mode section has %d non-empty lines, want <= 25:\n%s", got, modeSection)
	}
}

func TestRenderSubagentProtocolIncludesQualityJournalAndSelfCheck(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:      "demo",
		Objective:    "ship it",
		Mode:         goalx.ModeDevelop,
		Engine:       "codex",
		ProjectRoot:  "/tmp/project",
		SessionName:  "session-1",
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
		`"quality":"A|B|C"`,
		`"owner_scope":"..."`,
		`"blocked_by":"..."`,
		`"depends_on":["session-2"]`,
		`"can_split":true`,
		`"suggested_next":"..."`,
		"A=strong evidence+tested counter-evidence+actionable",
		"B=reasonable but gaps remain",
		"C=preliminary flag for deepening",
		"If you are blocked, append a `status:\"stuck\"` entry with the blocker, dependency, ownership scope, and the next smallest useful split the master could dispatch.",
		"## Self-Check",
		"Did I cover the full assigned scope and nothing extra?",
		"Did I verify counter-evidence or alternative explanations where applicable?",
		"If any answer is no, fix it before declaring yourself idle.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolIncludesTeamContext(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "investigate auth",
		Mode:                goalx.ModeResearch,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		Target:              goalx.TargetConfig{Files: []string{"report.md"}},
		SessionName:         "session-1",
		JournalPath:         "/tmp/journal.jsonl",
		GuidancePath:        "/tmp/guidance.md",
		AcceptancePath:      "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalContractPath:    "/tmp/goal-contract.json",
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
		"goal-contract.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolMakesGoalContractBoundaryExplicit(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeDevelop,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		SessionName:         "session-1",
		Target:              goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:             goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:         "/tmp/journal.jsonl",
		GuidancePath:        "/tmp/guidance.md",
		GoalContractPath:    "/tmp/goal-contract.json",
		AcceptancePath:      "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/worktree-1", Mode: goalx.ModeDevelop},
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
		"Goal contract: `/tmp/goal-contract.json`",
		"The goal contract defines what must be true before the overall goal can be considered complete.",
		"Your current assignment defines what to do next, not what counts as full completion.",
		"one coherent capability slice at a time",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "one fix at a time") {
		t.Fatalf("rendered protocol should replace one-fix framing with capability-slice framing:\n%s", text)
	}
}

func TestRenderMasterProtocolIncludesGoalContractChecklistInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeDevelop,
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptancePath:      "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalContractPath:    "/tmp/goal-contract.json",
		StatusPath:          "/tmp/status.json",
		EngineCommand:       "claude --model claude-opus-4-6 --permission-mode auto",
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
		"## Mode",
		"The user specified **develop** mode.",
		"## Your Job",
		"goal contract",
		"execution_state\":\"active|waiting_external|dispatchable\"",
		"acceptance.json",
		"goal-contract.json",
		"goalx verify --run demo",
		"completeness",
		"evidence density (code refs)",
		"counter-evidence",
		"quantification",
		"actionability",
		"structure",
		"/tmp/acceptance.md",
		"goalx add --run demo",
		"goalx tell --run demo session-N",
		"goalx park --run demo session-N",
		"goalx resume --run demo session-N",
		"dispatcher and referee",
		"check evidence density, counter-evidence presence, and actionability of findings",
		"If any required item is uncovered, that is a scheduling bug.",
		"If parallel capacity exists and independent required work remains, dispatch it now instead of waiting.",
		"waiting_external",
		"dispatchable",
		"If a required item stays stuck, reassign it, split it, or take it over yourself.",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh guidance before launching another session.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	if strings.Contains(text, "tmux send-keys -t ar-demo:<window> Enter") {
		t.Fatalf("rendered master protocol should not tell master to hand-send tmux enter nudges:\n%s", text)
	}
	if strings.Contains(text, "3-7 testable bullets") {
		t.Fatalf("rendered master protocol should not cap required goal coverage:\n%s", text)
	}
}

func TestRenderMasterProtocolIncludesResearchModeLaunchGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "audit auth",
		RunName:        "demo",
		Mode:           goalx.ModeResearch,
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
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
		"## Mode",
		"The user specified **research** mode.",
		"goalx add --run demo",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolOmitsLegacyPlannedSessionsAndPresetDisplays(t *testing.T) {
	runDir := t.TempDir()
	outPath := filepath.Join(runDir, "master.md")
	data := map[string]any{
		"Objective": "ship it",
		"RunName":   "demo",
		"Mode":      goalx.ModeDevelop,
		"Preset":    "claude",
		"Engines":   goalx.BuiltinEngines,
		"Master":    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		"PlannedSessions": []map[string]string{
			{
				"Name":   "session-1",
				"Engine": "codex",
				"Model":  "codex",
				"Hint":   "P0 fixes",
			},
		},
	}

	if err := renderTemplate("templates/master.md.tmpl", outPath, data); err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	out, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, unwanted := range []string{
		"## Session Plan",
		"- Preset: claude",
		"session-1: codex/codex (P0 fixes)",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit %q", unwanted)
		}
	}
}

func TestRenderMasterProtocolIncludesTransitionRecommendationInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "ship it",
		RunName:        "demo",
		Mode:           goalx.ModeDevelop,
		Engines:        goalx.BuiltinEngines,
		Master:         goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		AcceptancePath: "/tmp/acceptance.md",
		StatusPath:     "/tmp/status.json",
		EngineCommand:  "claude --model claude-opus-4-6 --permission-mode auto",
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
		"goalx save demo && goalx debate && goalx start",
		"goalx save demo && goalx implement && goalx start",
		"goalx stop --run demo",
		"## Status Contract",
		"You drive the transition.",
		"Set `keep_session` when a develop-mode session should be merged",
		"Do NOT just write a recommendation and wait.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesMixedModeCoordinationGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeDevelop,
		Engines:             goalx.BuiltinEngines,
		Master:              goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptancePath:      "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalContractPath:    "/tmp/goal-contract.json",
		MasterJournalPath:   "/tmp/master.jsonl",
		StatusPath:          "/tmp/status.json",
		CoordinationPath:    "/tmp/coordination.json",
		MasterInboxPath:     "/tmp/control/master-inbox.jsonl",
		MasterStatePath:     "/tmp/control/master-state.json",
		HeartbeatStatePath:  "/tmp/control/heartbeat.json",
		EngineCommand:       "claude --model claude-opus-4-6 --permission-mode auto",
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
		"coordination.json",
		"master-inbox.jsonl",
		"master-state.json",
		"heartbeat.json",
		"dispatchable_slices",
		"goalx add --run demo --mode research",
		"goalx tell --run demo session-N",
		"goalx park --run demo session-N",
		"goalx resume --run demo session-N",
		"temporary research session",
		"Research-mode sessions produce evidence and reports, not mergeable code changes.",
		"Check the coordination digest version each heartbeat.",
		"Default to current repo state, control files, runtime state, and latest session outputs.",
		"Only reread older journal history when the current state is ambiguous",
		"You may reorder, delegate, or temporarily postpone required work within the current goal",
		"you may not declare the goal complete while any required item remains unfinished, blocked, or merely scheduled for later",
		"If any required item is uncovered, that is a scheduling bug.",
		"If parallel capacity exists and independent required work remains, dispatch it now instead of waiting.",
		"waiting_external",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh guidance before launching another session.",
		"Improvement backlog",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func sectionBetween(text, start, end string) string {
	startIdx := strings.Index(text, start)
	if startIdx < 0 {
		return ""
	}
	text = text[startIdx:]
	endIdx := strings.Index(text, end)
	if endIdx < 0 {
		return text
	}
	return text[:endIdx]
}

func nonEmptyLineCount(text string) int {
	count := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
