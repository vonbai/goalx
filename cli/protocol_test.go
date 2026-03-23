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
		"When you have 2+ independent research or analysis angles",
		"collapse the results back into this session's journal, report, or dispatchable_slices",
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
	for _, unwanted := range []string{
		"2+ independent research or analysis angles",
		"collapse the results back into this session's journal, report, or dispatchable_slices",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered codex protocol should not inherit claude-only agent guidance %q", unwanted)
		}
	}
}

func TestRenderSubagentProtocolIncludesOptimizerDoctrineInDevelopMode(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:      "demo",
		Objective:    "optimize discovery pipeline",
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
		"Treat the current implementation as evidence, not a boundary.",
		"For any non-trivial task, identify the root cause, bottleneck, or design flaw before patching symptoms.",
		"Do not assume the current module boundaries are correct.",
		"Compare the local patch path, the compatibility-preserving path, and the architecture-level path.",
		"If a deeper path materially improves the goal, report it clearly instead of silently following the old boundary.",
		"When a better architecture path is justified, emit dispatchable slices or report it clearly",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q:\n%s", want, text)
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
		`"depends_on":["master-rebuild"]`,
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
		"Never list your own session name in `depends_on`.",
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

func TestRenderMasterProtocolIncludesOptimizerDoctrine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:        "optimize pipeline discovery",
		RunName:          "demo",
		Mode:             goalx.ModeDevelop,
		TmuxSession:      "ar-demo",
		SummaryPath:      "/tmp/summary.md",
		AcceptancePath:   "/tmp/acceptance.md",
		CoordinationPath: "/tmp/coordination.json",
		EngineCommand:    "claude --model claude-opus-4-6 --permission-mode auto",
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
		"Treat every goal as a system optimization problem",
		"Existing implementation boundaries are evidence, not boundaries.",
		"identify the root cause or bottleneck first",
		"local patch path",
		"compatibility-preserving improvement path",
		"architecture-level redesign path",
		"Prefer the highest expected-value path",
		"Do not over-engineer for elegance alone",
		"Treat narrowed causes as hypotheses until a failing regression test or decisive counter-evidence confirms them.",
		"keep it short: current problem, chosen path, and one-line reason",
		"Do not ask the user to choose between implementation paths unless the choice materially changes scope, risk, acceptance, or irreversible cost.",
		"Otherwise decide yourself, record why, and execute.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolDefinesGenericLastMileAutonomy(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeDevelop,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		AcceptancePath:        "/tmp/acceptance.md",
		AcceptanceStatePath:   "/tmp/acceptance.json",
		GoalContractPath:      "/tmp/goal-contract.json",
		RunStatePath:          "/tmp/state/run.json",
		SessionsStatePath:     "/tmp/state/sessions.json",
		MasterInboxPath:       "/tmp/control/master-inbox.jsonl",
		MasterCursorPath:      "/tmp/control/master-cursor.json",
		ControlRunStatePath:   "/tmp/control/run-state.json",
		ControlEventsPath:     "/tmp/control/events.jsonl",
		ControlRemindersPath:  "/tmp/control/reminders.json",
		ControlDeliveriesPath: "/tmp/control/deliveries.json",
		StatusPath:            "/tmp/status.json",
		EngineCommand:         "claude --model claude-opus-4-6 --permission-mode auto",
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
		"Do not label work as external just because it happens late in the run or touches runtime state.",
		"Local shell work such as building, restarting services, launching local deploy/dev processes, checking readiness, inspecting running revisions, and running acceptance/eval commands is part of the job when the required access is already available.",
		"Before marking a required item `waiting_external`, verify that the blocker is truly outside your available permissions, credentials, or reachable environment.",
		"If a required proof step depends on a long-running local process, confirm that the live process matches current `HEAD`; if it does not, rebuild/restart or relaunch it yourself before evaluating.",
		"Do not stop at intermediate states such as \"implementation complete\", \"ready for eval\", or \"awaiting external verification\" while an actionable required item remains.",
		"If the only remaining gap is proof or verification that you can execute yourself, run it now instead of waiting for another cycle.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
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

func TestRenderSubagentProtocolIncludesOptimizerDoctrineInResearchMode(t *testing.T) {
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
		"Treat the current implementation as evidence, not a boundary.",
		"For any non-trivial task, identify the root cause, bottleneck, or design flaw before patching symptoms.",
		"Do not assume the current module boundaries are correct.",
		"Compare the local patch path, the compatibility-preserving path, and the architecture-level path.",
		"If a deeper path materially improves the goal, report it clearly instead of silently following the old boundary.",
		"When a better architecture path is justified, emit dispatchable slices or report it clearly",
		"Do not wait for user approval when you can recommend a clearly superior path to the master.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolEncouragesBetterArchitecturePathsInDevelopMode(t *testing.T) {
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
		"Do not assume the current module boundaries are correct",
		"If a deeper path materially improves the goal",
		"report it clearly instead of silently following the old boundary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered develop protocol missing %q", want)
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
		"goalx save demo && goalx debate --from demo",
		"goalx save demo && goalx implement --from demo",
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
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeDevelop,
		Engines:               goalx.BuiltinEngines,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		AcceptancePath:        "/tmp/acceptance.md",
		AcceptanceStatePath:   "/tmp/acceptance.json",
		GoalContractPath:      "/tmp/goal-contract.json",
		MasterJournalPath:     "/tmp/master.jsonl",
		StatusPath:            "/tmp/status.json",
		CoordinationPath:      "/tmp/coordination.json",
		MasterInboxPath:       "/tmp/control/inbox/master.jsonl",
		MasterCursorPath:      "/tmp/control/master-cursor.json",
		ControlRunStatePath:   "/tmp/control/run-state.json",
		ControlEventsPath:     "/tmp/control/events.jsonl",
		ControlRemindersPath:  "/tmp/control/reminders.json",
		ControlDeliveriesPath: "/tmp/control/deliveries.json",
		CompletionProofPath:   "/tmp/proof/completion.json",
		EngineCommand:         "claude --model claude-opus-4-6 --permission-mode auto",
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
		"inbox/master.jsonl",
		"master-cursor.json",
		"run-state.json",
		"events.jsonl",
		"reminders.json",
		"deliveries.json",
		"proof/completion.json",
		"dispatchable_slices",
		"goalx add --run demo --mode research",
		"goalx tell --run demo session-N",
		"goalx park --run demo session-N",
		"goalx resume --run demo session-N",
		"temporary research session",
		"Research-mode sessions produce evidence and reports, not mergeable code changes.",
		"Check the coordination digest version each control cycle.",
		"Default to current repo state, control files, runtime state, and latest session outputs.",
		"Only reread older journal history when the current state is ambiguous",
		"You may reorder, delegate, or temporarily postpone required work within the current goal",
		"you may not declare the goal complete while any required item remains unfinished, blocked, or merely scheduled for later",
		"If any required item is uncovered, that is a scheduling bug.",
		"If parallel capacity exists and independent required work remains, dispatch it now instead of waiting.",
		"Treat configured `parallel` as initial fan-out guidance, not a permanent ceiling;",
		"waiting_external",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh guidance before launching another session.",
		"Improvement backlog",
		"short-lived information gathering",
		"if you are running on Claude Code, you may use Claude's native subagents inside the master session",
		"`goalx add` remains the durable path",
		"Treat narrowed causes as hypotheses until a failing regression test or decisive counter-evidence confirms them.",
		"If an urgent required item is active and you are not directly coding it yourself, dispatch or resume a worker quickly instead of carrying passive master ownership across repeated control cycles.",
		"Keep detailed hypotheses, traces, and path comparisons in journals, not the coordination digest.",
		"Avoid sync-only liveness narration.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolOmitsOldSyncOnlyLivenessGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeDevelop,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "best"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptancePath:      "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalContractPath:    "/tmp/goal-contract.json",
		CoordinationPath:    "/tmp/coordination.json",
		StatusPath:          "/tmp/status.json",
		MasterJournalPath:   "/tmp/master.jsonl",
		EngineCommand:       "codex exec",
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, unwanted := range []string{
		"Update `/tmp/status.json` and log to `/tmp/master.jsonl`.",
		"record root cause, local path, compatibility-preserving path, architecture path, chosen path, and why it was chosen in the coordination digest",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol unexpectedly contains %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolIncludesOptimizationDoctrine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "maximize cross-platform discovery throughput",
		RunName:             "demo",
		Mode:                goalx.ModeDevelop,
		Master:              goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptancePath:      "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalContractPath:    "/tmp/goal-contract.json",
		CoordinationPath:    "/tmp/coordination.json",
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
		"Treat every goal as a system optimization problem",
		"Existing implementation boundaries are evidence, not boundaries.",
		"identify the root cause or bottleneck first",
		"local patch path",
		"compatibility-preserving improvement path",
		"architecture-level redesign path",
		"Prefer the highest expected-value path feasible within budget and risk.",
		"Do not over-engineer for elegance alone.",
		"Treat narrowed causes as hypotheses until a failing regression test or decisive counter-evidence confirms them.",
		"keep it short: current problem, chosen path, and one-line reason",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
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
