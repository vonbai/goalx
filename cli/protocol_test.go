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
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeDevelop,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		SessionName:       "session-1",
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
		WorktreePath:      "/tmp/worktree",
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
		"Session inbox: `/tmp/control/inbox/session-1.jsonl`",
		"Session cursor: `/tmp/control/session-1-cursor.json`",
		"Objective: ship it",
		"Gate: `go test ./...`",
		"## Resume From Durable State",
		"Do not rebuild the full chat history",
		"Read the recent journal tail",
		"Read unread session inbox entries",
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
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeResearch,
		Engine:            "claude-code",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"report.md"}},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
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
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeDevelop,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
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
		"re-check `/tmp/control/inbox/session-1.jsonl` before idling",
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
		RunName:           "demo",
		Objective:         "optimize discovery pipeline",
		Mode:              goalx.ModeDevelop,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
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
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeResearch,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"report.md"}},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
		Context:           goalx.ContextConfig{Files: []string{"/tmp/context.md"}},
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

func TestRenderSubagentProtocolResearchModeUsesGuidanceNotHardBan(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeResearch,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "DO NOT modify any source code.") {
		t.Fatalf("research mode should use guidance, not a hard source-code ban:\n%s", text)
	}
	if !strings.Contains(text, "Research mode typically focuses on producing reports; code modification controlled by target config.") {
		t.Fatalf("research mode guidance missing updated target-config wording:\n%s", text)
	}
}

func TestRenderSubagentProtocolKeepsDevelopMethodologyConcise(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeDevelop,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
		Context:           goalx.ContextConfig{Files: []string{"/tmp/context.md"}},
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
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeDevelop,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
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
		SessionInboxPath:    "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:   "/tmp/control/session-1-cursor.json",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalPath:            "/tmp/goal.json",
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
		"goal.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolMakesGoalBoundaryExplicit(t *testing.T) {
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
		SessionInboxPath:    "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:   "/tmp/control/session-1-cursor.json",
		GoalPath:            "/tmp/goal.json",
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
		"Goal boundary: `/tmp/goal.json`",
		"The goal boundary defines what must be true before the overall goal can be considered complete.",
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

func TestRenderMasterProtocolIncludesGoalBoundaryChecklistInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeDevelop,
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalPath:            "/tmp/goal.json",
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
		"## Operations",
		"run-charter.json",
		"goal boundary",
		"control/identity-fence.json",
		"state\":\"open|claimed|waived\"",
		"acceptance.json",
		"goal.json",
		"goal-log.jsonl",
		"goalx verify --run demo",
		"goalx add --run demo",
		"goalx tell --run demo session-N",
		"goalx park --run demo session-N",
		"goalx resume --run demo session-N",
		"orchestrator",
		"check evidence density, clear evidence, and actionability of findings",
		"If any required item is uncovered, that is a scheduling bug.",
		"If parallel capacity exists and independent required work remains, dispatch it now instead of waiting.",
		"waiting_external",
		"If a required item stays stuck, reassign it, split it, or take it over yourself.",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh inbox instructions before launching another session.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	if strings.Contains(text, "tmux send-keys -t ar-demo:<window> Enter") {
		t.Fatalf("rendered master protocol should not tell master to hand-send tmux enter nudges:\n%s", text)
	}
	for _, unwanted := range []string{
		"goal contract",
		"counter-evidence",
		"semantic_match",
		"acceptance.md",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should not contain legacy boundary/proof wording %q:\n%s", unwanted, text)
		}
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
		"Treat narrowed causes as hypotheses until a failing regression test or decisive evidence confirms them.",
		"keep it short: current problem, chosen path, and one-line reason",
		"run-charter.json",
		"control/identity-fence.json",
		"Do not ask the user to choose between implementation paths unless the choice materially changes scope, risk, acceptance, or irreversible cost.",
		"Otherwise decide yourself, record why, and execute.",
		"Before you commit on a non-trivial goal, compare at least 2-3 concrete paths first.",
		"If later evidence shows the chosen path is stuck, falsified, or clearly lower-value, switch paths autonomously and record why.",
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
		AcceptanceStatePath:   "/tmp/acceptance.json",
		GoalPath:              "/tmp/goal.json",
		RunStatePath:          "/tmp/state/run.json",
		SessionsStatePath:     "/tmp/state/sessions.json",
		MasterInboxPath:       "/tmp/control/master-inbox.jsonl",
		MasterCursorPath:      "/tmp/control/master-cursor.json",
		ControlRunStatePath:   "/tmp/control/run-state.json",
		LivenessPath:          "/tmp/control/liveness.json",
		WorktreeSnapshotPath:  "/tmp/control/worktree-snapshot.json",
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
		"If a better path becomes clear during execution, update the goal boundary, switch, and continue without waiting for a user tell.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesResearchModeLaunchGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "audit auth",
		RunName:             "demo",
		Mode:                goalx.ModeResearch,
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptanceNotesPath: "/tmp/acceptance.md",
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
		"The user specified **research** mode.",
		"goalx add --run demo",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesAutoModeGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeAuto,
		TmuxSession:   "ar-demo",
		SummaryPath:   "/tmp/summary.md",
		EngineCommand: "claude --model claude-opus-4-6 --permission-mode auto",
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
		"No mode was specified.",
		"Determine the best approach (research, develop, or hybrid)",
		"You have full autonomy to decide.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	if strings.Contains(text, "The user specified **auto** mode.") {
		t.Fatalf("rendered master protocol should not treat auto as a literal requested mode:\n%s", text)
	}
}

func TestRenderSubagentProtocolIncludesOptimizerDoctrineInResearchMode(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeResearch,
		Engine:            "claude-code",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"report.md"}},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
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
		"run-charter.json",
		"session-1/identity.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolEncouragesBetterArchitecturePathsInDevelopMode(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeDevelop,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
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

func TestRenderSubagentProtocolTreatsAutoModeAsDevelop(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeAuto,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
		Harness:           goalx.HarnessConfig{Command: "go test ./..."},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "## Mode: Develop") {
		t.Fatalf("auto subagent protocol should use develop guidance:\n%s", text)
	}
	if strings.Contains(text, "## Mode: Research") {
		t.Fatalf("auto subagent protocol should not use research guidance:\n%s", text)
	}
}

func TestRenderSubagentProtocolUsesRuntimeDimensionsAndReportsDir(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeResearch,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"report.md"}},
		JournalPath:       "/tmp/journal.jsonl",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
		ReportsDir:        "/tmp/run/reports",
		DimensionsPath:    "/tmp/control/dimensions.json",
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
		"Write reports to `/tmp/run/reports`",
		"read `/tmp/control/dimensions.json` for the current dimension assignment",
		"Dimensions can change while the run is active",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"## Your Approach",
		"DiversityHint",
		"--strategy",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered protocol should omit legacy guidance %q:\n%s", unwanted, text)
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
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeDevelop,
		Engines:             goalx.BuiltinEngines,
		Master:              goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptanceNotesPath: "/tmp/acceptance.md",
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
		"## Resources",
		"## Objective",
		"- Run: demo",
		"goalx save demo && goalx debate --from demo",
		"goalx save demo && goalx implement --from demo",
		"goalx stop --run demo",
		"## Status",
		"You drive the transition.",
		"Set `keep_session` when a develop-mode session should be merged",
		"Do NOT just write a recommendation and wait.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesGoalxWaitLoopGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeDevelop,
		Master:        goalx.MasterConfig{Engine: "codex", Model: "best"},
		TmuxSession:   "ar-demo",
		SummaryPath:   "/tmp/summary.md",
		StatusPath:    "/tmp/status.json",
		EngineCommand: "codex exec",
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
		"**Monitoring loop**: Always use `goalx wait --run demo master --timeout 300` between control cycles.",
		"Never use `sleep`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesReportsRoutingAndResearchCompletionGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "audit auth",
		RunName:        "demo",
		Mode:           goalx.ModeResearch,
		Master:         goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		StatusPath:     "/tmp/status.json",
		ReportsDir:     "/tmp/run/reports",
		DimensionsPath: "/tmp/control/dimensions.json",
		Routing: goalx.RoutingTableConfig{
			Profiles: map[string]goalx.ExecutionProfile{
				"deep-research": {Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
			},
			Rules: []goalx.RoutingRule{
				{Role: "research", AnyDimensions: []string{"depth"}, Profile: "deep-research"},
			},
		},
		DimensionsCatalog: map[string]string{
			"depth":    "Depth focus",
			"evidence": "Evidence focus",
		},
		EngineCommand: "codex exec",
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
		"Goal items are your working decomposition, not the definition of done.",
		"If a session produced valuable work outside the original items, that work matters.",
		"If a session shows no journal output for 15+ minutes while its lease is healthy",
		"Research completion: write findings to /tmp/run/reports, update goal items, stop sessions. No acceptance gate or proof manifest required.",
		"goalx dimension [--run NAME] <session-N|all> --set depth,adversarial",
		"goalx dimension [--run NAME] <session-N> --add creative",
		"goalx dimension [--run NAME] <session-N> --remove depth",
		"deep-research",
		"gpt-5.4",
		"high",
		"depth",
		"evidence",
		"/tmp/run/reports",
		"/tmp/control/dimensions.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "--strategy") {
		t.Fatalf("rendered master protocol should omit legacy strategy guidance:\n%s", text)
	}
}

func TestRenderMasterProtocolIncludesClaudeWaitSafetyNetGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeDevelop,
		Master:        goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:   "ar-demo",
		SummaryPath:   "/tmp/summary.md",
		StatusPath:    "/tmp/status.json",
		EngineCommand: "claude --model claude-opus-4-6 --permission-mode auto",
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "Claude Code Stop hook is only a safety net") {
		t.Fatalf("rendered master protocol missing Claude wait safety-net guidance:\n%s", text)
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
		AcceptanceNotesPath:   "/tmp/acceptance.md",
		AcceptanceStatePath:   "/tmp/acceptance.json",
		GoalPath:              "/tmp/goal.json",
		MasterJournalPath:     "/tmp/master.jsonl",
		StatusPath:            "/tmp/status.json",
		CoordinationPath:      "/tmp/coordination.json",
		MasterInboxPath:       "/tmp/control/inbox/master.jsonl",
		MasterCursorPath:      "/tmp/control/master-cursor.json",
		ControlRunStatePath:   "/tmp/control/run-state.json",
		LivenessPath:          "/tmp/control/liveness.json",
		WorktreeSnapshotPath:  "/tmp/control/worktree-snapshot.json",
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
		"liveness.json",
		"worktree-snapshot.json",
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
		"Prefer reusing a parked or idle session with fresh inbox instructions before launching another session.",
		"Improvement backlog",
		"Only use Claude native subagents for quick read-only checks or validation under 30 seconds.",
		"Do not use native subagents for primary research, implementation, or durable ownership.",
		"`goalx add` remains the durable path",
		"Treat narrowed causes as hypotheses until a failing regression test or decisive evidence confirms them.",
		"If an urgent required item is active and you are not directly coding it yourself, dispatch or resume a worker quickly instead of carrying passive master ownership across repeated control cycles.",
		"Keep detailed hypotheses, traces, and path comparisons in journals, not the coordination digest.",
		"Avoid sync-only liveness narration.",
		"Sessions without dedicated worktrees share the run worktree.",
		"Use `goalx add --worktree` for parallel isolation.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	if strings.Contains(text, "events.jsonl") {
		t.Fatalf("rendered master protocol should not reference legacy events log:\n%s", text)
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
		AcceptanceNotesPath: "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalPath:            "/tmp/goal.json",
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
		AcceptanceNotesPath: "/tmp/acceptance.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalPath:            "/tmp/goal.json",
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
		"Treat narrowed causes as hypotheses until a failing regression test or decisive evidence confirms them.",
		"keep it short: current problem, chosen path, and one-line reason",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolUsesCondensedOperatingSections(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeDevelop,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		AcceptanceStatePath:   "/tmp/acceptance.json",
		GoalPath:              "/tmp/goal.json",
		StatusPath:            "/tmp/status.json",
		CoordinationPath:      "/tmp/coordination.json",
		MasterInboxPath:       "/tmp/control/inbox/master.jsonl",
		MasterCursorPath:      "/tmp/control/master-cursor.json",
		ControlRunStatePath:   "/tmp/control/run-state.json",
		RunStatePath:          "/tmp/state/run.json",
		SessionsStatePath:     "/tmp/state/sessions.json",
		LivenessPath:          "/tmp/control/liveness.json",
		WorktreeSnapshotPath:  "/tmp/control/worktree-snapshot.json",
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
		"## Identity",
		"## Objective",
		"## Resources",
		"## Strategy",
		"## Operations",
		"## Completion",
		"## Tools",
		"## Status",
		"Dispatch implementation to GoalX sessions first.",
		"Mechanical work belongs on codex-class workers. Judgment and final arbitration belong on opus-class workers.",
		"Read the inbox every control cycle before making decisions.",
		"If you finish a thinking block without a concrete next action, immediately enter `goalx wait --run demo master --timeout 300`.",
		"If a session is stale for 15+ minutes while its lease is healthy, park it and replace it.",
		"Re-read the charter objective before declaring completion.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"## Current Configuration",
		"## Routing Guidance",
		"## Implementation Strategy",
		"## Your Job",
		"## Rules",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolTightensClaudeNativeSubagentUsage(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeDevelop,
		Master:        goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:   "ar-demo",
		SummaryPath:   "/tmp/summary.md",
		StatusPath:    "/tmp/status.json",
		EngineCommand: "claude --model claude-opus-4-6 --permission-mode auto",
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
		"Only use Claude native subagents for quick read-only checks or validation under 30 seconds.",
		"Do not use native subagents for primary research, implementation, or durable ownership.",
		"If the task needs a worktree, durable tracking, or more than a quick check, use `goalx add`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "short-lived information gathering") {
		t.Fatalf("rendered master protocol should not keep the old permissive native-subagent wording:\n%s", text)
	}
}

func TestRenderMasterProtocolReferencesGoalxContextAndAfford(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeDevelop,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		ContextIndexPath:      "/tmp/control/context-index.json",
		ActivityPath:          "/tmp/control/activity.json",
		AffordancesPath:       "/tmp/control/affordances.md",
		ControlRunStatePath:   "/tmp/control/run-state.json",
		RunStatePath:          "/tmp/state/run.json",
		SessionsStatePath:     "/tmp/state/sessions.json",
		ControlRemindersPath:  "/tmp/control/reminders.json",
		ControlDeliveriesPath: "/tmp/control/deliveries.json",
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
		"`goalx context --run demo`",
		"`goalx afford --run demo`",
		"`/tmp/control/context-index.json`",
		"`/tmp/control/activity.json`",
		"`/tmp/control/affordances.md`",
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
