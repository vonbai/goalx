package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestRenderSubagentProtocolIncludesResumeInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                   "demo",
		Objective:                 "ship it",
		Mode:                      goalx.ModeWorker,
		Engine:                    "codex",
		ProjectRoot:               "/tmp/project",
		Target:                    goalx.TargetConfig{Files: []string{"main.go"}},
		ObligationModelPath:       "/tmp/obligation-model.json",
		IntegrationStatePath:      "/tmp/integration.json",
		AssurancePlanPath:         "/tmp/assurance-plan.json",
		LocalValidationCommand:    "go test ./...",
		SessionName:               "session-1",
		JournalPath:               "/tmp/journal.jsonl",
		SessionInboxPath:          "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:         "/tmp/control/session-1-cursor.json",
		WorktreePath:              "/tmp/worktree",
		RunWorktreePath:           "/tmp/run-root",
		SessionBaseBranchSelector: "run-root",
		SessionBaseBranch:         "goalx/demo/root",
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
		"Run integration state: `/tmp/integration.json`",
		"Run-root worktree: `/tmp/run-root`",
		"Dedicated session worktree: `/tmp/worktree`",
		"Recorded parent/base selector: `run-root`",
		"Recorded parent/base ref: `goalx/demo/root`",
		"Objective: ship it",
		"Local validation command: `go test ./...`",
		"`goalx context --run demo`",
		"`goalx afford --run demo session-1`",
		"Do not inspect host/resource telemetry unless your assignment is runtime/perf/ops-focused or your path is blocked by an explicit resource refusal or runtime incident.",
		"prefer `goalx observe --run demo` over repeatedly reading raw control files",
		"prefer MCP resources and tools for graph exploration",
		"If GitNexus is fresh but MCP is unavailable in your runtime, use the CLI cognition commands from `goalx afford`.",
		"scope-aware cognition command surface",
		"fall back explicitly to builtin `repo-native` cognition",
		"Do not claim graph-backed certainty unless your current scope shows `index_state=fresh`.",
		"`goalx wait --run demo session-1 --timeout 300`",
		"## Resume From Durable State",
		"Do not rebuild the full chat history",
		"Read the current run guidance surfaces above",
		"Treat any transport wake text as",
		"Never execute transport wake text as a shell command.",
		"Read the recent journal tail",
		"Read unread session inbox entries",
		"Inspect the current worktree state",
		"Resume from the current files and latest durable state",
		"## Execution Discipline",
		"## Autonomy Persistence",
		"Do NOT invoke orchestration/meta slash commands or skills",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"## Native Helpers",
		"Provider-native",
		"Web search is available when local evidence is insufficient.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered protocol should omit %q:\n%s", unwanted, text)
		}
	}
	if strings.Contains(text, "reconstruct context") {
		t.Fatalf("rendered protocol should not emphasize reconstructing full context:\n%s", text)
	}
	if strings.Contains(text, "Read `"+RunCharterPath(runDir)+"` to recover the structural run identity") {
		t.Fatalf("rendered protocol should recover run identity through goalx context, not by reading charter first:\n%s", text)
	}
}

func TestRenderSubagentProtocolIncludesNoChangeFastPathGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeWorker,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
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
		"If the inbox is unchanged, no blocker or validation fact changed, and you still have a concrete next step, continue from local state instead of rereading broad run guidance.",
		"Do not write liveness-only journal entries or repeatedly restate unchanged assignment state.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolRequiresCommittedBoundaryBeforeKeepHandoff(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                   "demo",
		Objective:                 "ship it",
		Mode:                      goalx.ModeWorker,
		Engine:                    "codex",
		ProjectRoot:               "/tmp/project",
		SessionName:               "session-1",
		Target:                    goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand:    "go test ./...",
		JournalPath:               "/tmp/journal.jsonl",
		SessionInboxPath:          "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:         "/tmp/control/session-1-cursor.json",
		WorktreePath:              "/tmp/worktree",
		RunWorktreePath:           "/tmp/run-root",
		SessionBaseBranchSelector: "run-root",
		SessionBaseBranch:         "goalx/demo/root",
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
		"If you changed code in a dedicated worktree and expect master to `goalx keep` your work, seal that boundary in a focused local commit before you go idle.",
		"`goalx keep` only merges committed branch history; it does not carry uncommitted dirty worktree changes.",
		"Master may still manually adopt only part of your worktree when conflicts or overlap make that the better path.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolForbidsDedicatedSessionEditingSourceRoot(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                   "demo",
		Objective:                 "ship it",
		Mode:                      goalx.ModeWorker,
		Engine:                    "codex",
		ProjectRoot:               "/tmp/source-root",
		SessionName:               "session-1",
		JournalPath:               "/tmp/journal.jsonl",
		SessionInboxPath:          "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:         "/tmp/control/session-1-cursor.json",
		WorktreePath:              "/tmp/session-1",
		RunWorktreePath:           "/tmp/run-root",
		SessionBaseBranchSelector: "run-root",
		SessionBaseBranch:         "goalx/demo/root",
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
		"Your default edit boundary is the dedicated session worktree above.",
		"Do not edit the source root or run-root worktree from a dedicated session unless the master explicitly redirects you to inspect or integrate there.",
		"If you discover accidental edits outside your assigned worktree, stop, record the boundary violation in the journal, and migrate or revert those edits before continuing.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolTightensWakeLoopInboxHandling(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeWorker,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
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
	want := `If wakes keep arriving while unread inbox or cursor lag remains, do not keep assuming "no new message". Re-read the inbox and acknowledge the latest processed entry before continuing.`
	if !strings.Contains(text, want) {
		t.Fatalf("rendered protocol missing %q:\n%s", want, text)
	}
}

func TestRenderSubagentProtocolOmitsProviderNativeCapabilityGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
		"## Execution Discipline",
		"## Autonomy Persistence",
		"When this protocol tells you to read a file, run a command, append a journal entry, acknowledge inbox, verify, commit, or wait, perform the corresponding tool action in this turn.",
		"If a concrete next step exists inside your current assignment, execute it before idling.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"## Native Helpers",
		"You are running in Claude Code.",
		"Native subagents are transient helpers inside this session.",
		"Provider-native",
		"Web search is available when local evidence is insufficient.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesGenericExecutionGuidanceForCodex(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
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
		"## Execution Discipline",
		"## Autonomy Persistence",
		"When this protocol tells you to read a file, run a command, append a journal entry, acknowledge inbox, verify, commit, or wait, perform the corresponding tool action in this turn.",
		"Do not end a turn after saying you will do something next. Execute the next tool call now.",
		"If a concrete next step exists inside your current assignment, execute it before idling.",
		"re-check `/tmp/control/inbox/session-1.jsonl` before idling",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"## Native Helpers",
		"You are running in Codex CLI.",
		"This engine only starts native subagents when you explicitly invoke them.",
		"Provider-native",
		"Web search is available when local evidence is insufficient.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered codex protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesExecutionDisciplineForClaude(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
		"## Execution Discipline",
		"When this protocol tells you to read a file, run a command, append a journal entry, acknowledge inbox, verify, commit, or wait, perform the corresponding tool action in this turn.",
		"Do not end a turn after saying you will do something next. Execute the next tool call now.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered claude protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesClaudeAutonomyPersistenceGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
		"## Autonomy Persistence",
		"If a concrete next step exists inside your current assignment, execute it before idling.",
		"Do not ask master to confirm local method choices inside assigned scope. Act, record, continue.",
		"Context compaction is routine. Recover from inbox, journal, and current files instead of stopping early.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesAutonomyPersistenceGuidanceForCodex(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
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
		"## Autonomy Persistence",
		"If a concrete next step exists inside your current assignment, execute it before idling.",
		"Do not ask master to confirm local method choices inside assigned scope. Act, record, continue.",
		"Context compaction is routine. Recover from inbox, journal, and current files instead of stopping early.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered codex protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolDropsRedundantResearchOnlyDoneLine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
	unwanted := "Do not declare yourself done. The master decides when to stop."
	if strings.Contains(text, unwanted) {
		t.Fatalf("rendered protocol should omit %q:\n%s", unwanted, text)
	}
}

func TestRenderSubagentProtocolIncludesOptimizerDoctrineInWorkerCodeSlices(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "optimize discovery pipeline",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
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

func TestRenderSubagentProtocolKeepsWorkerMethodologyConciseForAnalysisSlices(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
	modeSection := sectionBetween(string(out), "## Worker Contract", "## Context")
	for _, want := range []string{
		"You are a worker session. The assignment may ask for code, reports, or a mixed slice.",
		"Quantify what you can",
		"If the slice is analysis-heavy, produce evidence-backed findings",
		"## Key Findings",
		"## Recommendation",
		"## Priority Fix List (if applicable)",
		"dispatchable_slices",
		"directly adopt",
	} {
		if !strings.Contains(modeSection, want) {
			t.Fatalf("worker contract section missing %q:\n%s", want, modeSection)
		}
	}
	if got := nonEmptyLineCount(modeSection); got > 25 {
		t.Fatalf("worker contract section has %d non-empty lines, want <= 25:\n%s", got, modeSection)
	}
}

func TestRenderSubagentProtocolUsesGenericWorkerGuidanceNotHardBans(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
		t.Fatalf("worker protocol should use guidance, not a hard source-code ban:\n%s", text)
	}
	if !strings.Contains(text, "The assignment may ask for code, reports, or a mixed slice.") {
		t.Fatalf("worker guidance missing generic assignment wording:\n%s", text)
	}
}

func TestRenderSubagentProtocolDeclaresReadonlyBoundary(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
	for _, want := range []string{
		"Declared target files/paths: `report.md`",
		"Declared readonly paths: `.`",
		"Do not edit those paths.",
		"This session has a declared readonly boundary.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolDeclaresReadonlyBoundaryToMaster(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "investigate auth",
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Target:              goalx.TargetConfig{Files: []string{"report.md"}, Readonly: []string{"."}},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		StatusPath:          "/tmp/status.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		ReportsDir:          "/tmp/run/reports",
		EngineCommand:       "codex exec",
		RunWorktreePath:     "/tmp/run-root",
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
		"Declared target files/paths: `report.md`",
		"Declared readonly paths: `.`",
		"Treat the readonly boundary as a run-level execution contract for dispatch, reuse, and takeover.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolKeepsWorkerMethodologyConciseForCodeSlices(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
		Context:                goalx.ContextConfig{Files: []string{"/tmp/context.md"}},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	modeSection := sectionBetween(string(out), "## Worker Contract", "## Context")
	for _, want := range []string{
		"one coherent capability slice at a time",
		"If the slice is code-heavy, your changes will be reviewed. Every change must be justified and minimal.",
		"If a local validation command is configured, run it before handing work back:",
		"Keep changes minimal and correct. Do not add unrelated improvements, but do not cut corners on the change you are making.",
		"Respect file ownership from the current inbox assignment.",
		"If the inbox assignment names an allowed edit boundary, stay inside it.",
		"go test ./...",
	} {
		if !strings.Contains(modeSection, want) {
			t.Fatalf("worker contract section missing %q:\n%s", want, modeSection)
		}
	}
	if strings.Contains(modeSection, "avoid gold-plating") {
		t.Fatalf("worker contract section should replace legacy gold-plating guidance:\n%s", modeSection)
	}
	if got := nonEmptyLineCount(modeSection); got > 25 {
		t.Fatalf("worker contract section has %d non-empty lines, want <= 25:\n%s", got, modeSection)
	}
}

func TestRenderMasterProtocolRefinesReviewRoutingAndDepthCap(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeWorker,
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		SummaryPath: "/tmp/summary.md",
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
		"Prefer a different engine/model path for short review, validation, and adversarial checks.",
		"Launch a durable review session only when the review itself needs multi-step durable ownership, worktree isolation, or mergeable output.",
		"Default to one independent review round per implementation path.",
		"If a review finds new decisive evidence, redirect or take over; otherwise arbitrate instead of spinning review/fix/re-review loops.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolAddsConditionalRacingPattern(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeWorker,
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		SummaryPath: "/tmp/summary.md",
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
		"When 2-3 paths are plausibly competitive and independent, prefer racing them in isolated worktrees instead of serial speculation.",
		"Race only when the paths are worktree-safe, touch separable areas, and can be evaluated independently.",
		"Compare the resulting branches with `goalx diff`, then `goalx keep` or manually adopt the winner.",
		"Do not race shared config/schema changes, API contract changes, or naming/cross-cutting refactors that require one coherent decision.",
		"When uncertainty is material, do not rely on a single path or perspective if an independent cross-check is cost-effective.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolRequiresConcreteParallelAssignmentBoundaries(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeWorker,
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		SummaryPath: "/tmp/summary.md",
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
		"When dispatching parallel worker slices, make the inbox brief concrete: required outcome, allowed edit boundary, validation signal, and whether the branch should be kept, partially adopted, or treated as disposable exploration.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesQualityJournalAndSelfCheck(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
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
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		Target:              goalx.TargetConfig{Files: []string{"report.md"}},
		SessionName:         "session-1",
		JournalPath:         "/tmp/journal.jsonl",
		SessionInboxPath:    "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:   "/tmp/control/session-1-cursor.json",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
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
		"obligation-model.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolMakesGoalBoundaryExplicit(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
		ObligationModelPath:    "/tmp/obligation-model.json",
		AssurancePlanPath:      "/tmp/assurance-plan.json",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/worktree-1", Mode: goalx.ModeWorker},
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
		"Obligation boundary: `/tmp/obligation-model.json`",
		"The obligation boundary defines what must be true before the overall objective can be considered complete.",
		"Your current assignment defines what to do next, not what counts as full completion.",
		"Required obligations are the canonical mutable obligations for the overall run.",
		"Your assignment is the decomposition; the obligation boundary is not.",
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

func TestRenderSubagentProtocolPrefersObligationBoundaryWhenPresent(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
		ObligationModelPath:    "/tmp/obligation-model.json",
		AssurancePlanPath:      "/tmp/assurance-plan.json",
		EvidenceLogPath:        "/tmp/evidence-log.jsonl",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/worktree-1", Mode: goalx.ModeWorker},
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
		"Obligation boundary: `/tmp/obligation-model.json`",
		"The obligation boundary defines what must be true before the overall objective can be considered complete.",
		"Required obligations are the canonical mutable obligations for the overall run.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "Goal boundary") {
		t.Fatalf("rendered protocol should prefer obligation boundary wording:\n%s", text)
	}
}

func TestRenderSubagentProtocolEscalatesProofOnlyAssignmentsAgainstMissingOutcome(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		SessionName:         "session-1",
		Target:              goalx.TargetConfig{Files: []string{"main.go"}},
		JournalPath:         "/tmp/journal.jsonl",
		SessionInboxPath:    "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:   "/tmp/control/session-1-cursor.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		EvidenceLogPath:     "/tmp/evidence-log.jsonl",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/worktree-1", Mode: goalx.ModeWorker},
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
		"If your assignment is only gathering proof while the underlying outcome or enabler still appears unmet, record that risk in the journal or dispatchable_slices for the master.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesObligationBoundaryChecklistInstructions(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeWorker,
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		EvidenceLogPath:     "/tmp/evidence-log.jsonl",
		ObligationModelPath: "/tmp/obligation-model.json",
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
		"### Intent Guidance",
		"No explicit intent was provided. Choose the best execution path yourself.",
		"## Operations",
		"run-charter.json",
		"obligation boundary",
		"control/identity-fence.json",
		"assurance-plan.json",
		"evidence-log.jsonl",
		"obligation-model.json",
		"obligation-log.jsonl",
		"`goalx schema obligation-model`",
		"`goalx schema assurance-plan`",
		"Do not invent obligation fields from memory or from older docs.",
		"goalx verify --run demo",
		"goalx add --run demo",
		"goalx afford --run demo master",
		"prefer MCP resources and tools for graph exploration",
		"If GitNexus is fresh but MCP is unavailable in the current runtime, use the CLI cognition commands from `goalx afford`.",
		"prefer its `query`, `context`, and `impact` commands",
		"fall back explicitly to builtin `repo-native` cognition",
		"Do not claim graph-backed certainty unless the current scope shows `index_state=fresh`.",
		"canonical command surface",
		"orchestrator",
		"check evidence density, clear evidence, and actionability of findings",
		"If any required item is uncovered, that is a scheduling bug.",
		"If independent capacity exists and required work remains, dispatch it now instead of waiting.",
		"An external blocker does not silence dispatch",
		"If a required item stays stuck, reassign it, split it, or take it over yourself.",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh inbox instructions before launching another session.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"Required items start as `open`.",
		"Move an item to `claimed`",
		"`waived` only counts",
		"`waiting_external`",
		"An open required item with",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered protocol should route goal item field semantics through schema authority, found %q:\n%s", unwanted, text)
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

func TestRenderMasterProtocolRequiresBoundaryDesignBeforeFirstDispatch(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeWorker,
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		ObjectiveContractPath: "/tmp/objective-contract.json",
		AssurancePlanPath:     "/tmp/assurance-plan.json",
		EvidenceLogPath:       "/tmp/evidence-log.jsonl",
		CoordinationPath:      "/tmp/coordination.json",
		ObligationModelPath:   "/tmp/obligation-model.json",
		ObligationLogPath:     "/tmp/obligation-log.jsonl",
		MasterCursorPath:      "/tmp/master-cursor.json",
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
		"Before the first `goalx add` or `goalx tell`, finish the initial boundary design: draft and lock `objective-contract`, replace `obligation-model`, append the first `obligation-log` decision, synchronize `assurance-plan`, write `coordination`, and inspect `success-model`, `proof-plan`, `workflow-plan`, `domain-pack`, `compiler-input`, and `compiler-report` for this run.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolPrefersObligationBoundaryAuthoringWhenPresent(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeWorker,
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		ObjectiveContractPath: "/tmp/objective-contract.json",
		ObligationModelPath:   "/tmp/obligation-model.json",
		ObligationLogPath:     "/tmp/obligation-log.jsonl",
		AssurancePlanPath:     "/tmp/assurance-plan.json",
		EvidenceLogPath:       "/tmp/evidence-log.jsonl",
		CoordinationPath:      "/tmp/coordination.json",
		MasterCursorPath:      "/tmp/master-cursor.json",
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
		"`goalx schema obligation-log`",
		"replace `obligation-model`",
		"`/tmp/obligation-model.json` is the only mutable completion boundary.",
		"goalx durable write obligation-model --run demo --body-file /abs/path.json",
		"goalx durable write obligation-log --run demo --kind decision --actor master --body-file /abs/path.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "goalx durable write goal --run demo --body-file /abs/path.json") {
		t.Fatalf("rendered master protocol should not prefer goal durable-write wording when obligation-model is present:\n%s", text)
	}
}

func TestRenderMasterProtocolRequiresObjectiveContractIntegrity(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeWorker,
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		ObjectiveContractPath: "/tmp/objective-contract.json",
		AssurancePlanPath:     "/tmp/assurance-plan.json",
		EvidenceLogPath:       "/tmp/evidence-log.jsonl",
		ObligationModelPath:   "/tmp/obligation-model.json",
		ObligationLogPath:     "/tmp/obligation-log.jsonl",
		CoordinationPath:      "/tmp/coordination.json",
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
		"`/tmp/objective-contract.json`",
		"`objective-contract.json` is the immutable extracted user-clause contract for this run.",
		"After `objective-contract` is locked, user-originated clauses may not be narrowed, downgraded to optional-only coverage, or removed from assurance coverage without explicit user approval.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolRequiresBoundaryShapeComparisonBeforeDispatch(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"Before first dispatch on a non-trivial goal, compare at least a shallow user-restatement boundary and an obligation-grammar boundary, then record the choice in `obligation-log`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolForbidsProofOnlyBoundaryCollapse(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"Do not turn `obligation-model` into a proof plan. In mixed delivery runs, `proof` obligations support `outcome` and `enabler` obligations; they do not replace them.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesOptimizerDoctrine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:        "optimize pipeline discovery",
		RunName:          "demo",
		Mode:             goalx.ModeWorker,
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
		"Do not ask the user to choose between implementation paths unless the choice materially changes scope, risk, assurance strategy, or irreversible cost.",
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
		Mode:                  goalx.ModeWorker,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		AssurancePlanPath:     "/tmp/assurance-plan.json",
		ObligationModelPath:   "/tmp/obligation-model.json",
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
		"Local shell work such as building, restarting services, launching local deploy/dev processes, checking readiness, inspecting running revisions, and running assurance/eval commands is part of the job when the required access is already available.",
		"Before recording a required item as externally blocked, verify that the blocker is truly outside your available permissions, credentials, or reachable environment.",
		"If a required proof step depends on a long-running local process, confirm that the live process matches current `HEAD`; if it does not, rebuild/restart or relaunch it yourself before evaluating.",
		"Do not stop at intermediate states such as \"implementation complete\", \"ready for eval\", or \"awaiting external verification\" while an actionable required item remains.",
		"If the only remaining gap is proof or verification that you can execute yourself, run it now instead of waiting for another cycle.",
		"If a better path becomes clear during execution, update the obligation boundary, switch, and continue without waiting for a user tell.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
}

func TestRenderMasterProtocolIncludesIntentAndWorkerLaunchGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "audit auth",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"### Intent Guidance",
		"No explicit intent was provided. Choose the best execution path yourself.",
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
		"### Intent Guidance",
		"No explicit intent was provided. Choose the best execution path yourself.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	if strings.Contains(text, "The user specified **auto** mode.") {
		t.Fatalf("rendered master protocol should not expose legacy mode wording:\n%s", text)
	}
}

func TestRenderSubagentProtocolIncludesOptimizerDoctrineInWorkerAnalysisSlices(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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

func TestRenderSubagentProtocolEncouragesBetterArchitecturePathsInWorkerCodeSlices(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeWorker,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
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
			t.Fatalf("rendered worker protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolTreatsAutoModeAsWorker(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeAuto,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if !strings.Contains(text, "## Worker Contract") {
		t.Fatalf("auto subagent protocol should render worker guidance:\n%s", text)
	}
	if strings.Contains(text, "## Mode:") {
		t.Fatalf("subagent protocol should not expose legacy mode sections:\n%s", text)
	}
}

func TestRenderSubagentProtocolUsesRuntimeDimensionsAndReportsDir(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "investigate auth",
		Mode:              goalx.ModeWorker,
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
		"Mode":      goalx.ModeWorker,
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
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
		Engines:       goalx.BuiltinEngines,
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
		"## Resources",
		"## Objective",
		"- Run: demo",
		"goalx save demo && goalx run --from demo --intent debate",
		"Replace `goalx run --from demo --intent debate` with `goalx run --from demo --intent implement` or `goalx run --from demo --intent explore`",
		"goalx stop --run demo",
		"## Status",
		"`goalx status` is the compact exception-biased control snapshot.",
		"Use `goalx observe` only when you are actively diagnosing transport, runtime pressure, OOM events, or another ambiguous control incident.",
		"Do not read raw resource telemetry every control cycle.",
		"You drive the transition.",
		"Inspect the canonical status contract with `goalx schema status`",
		"Do NOT just write a recommendation and wait.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	if strings.Contains(text, "Set `keep_session` when a develop-mode session should be merged") {
		t.Fatalf("rendered master protocol should not define status fields inline:\n%s", text)
	}
}

func TestRenderMasterProtocolIncludesGoalxWaitLoopGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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

func TestRenderMasterProtocolClarifiesKeepUsesRecordedParentBoundary(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Mode:        goalx.ModeWorker,
		SummaryPath: "/tmp/summary.md",
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
		"`goalx keep --run demo session-N` only merges committed session branch history relative to that session's recorded parent/base ref.",
		"Do not assume every worker branch is rooted directly on the run worktree.",
		"If a worker session still has dirty uncommitted files, require a focused local commit before `goalx keep`, or inspect/take over the work yourself.",
		"If `goalx keep` is not the right fit because there are conflicts, only part of the session result should survive, or the run root/master already changed in overlapping areas, inspect the session worktree directly and integrate the right subset yourself.",
		"After you finish a manual run-root integration, record it with `goalx integrate --run demo --method partial_adopt --from session-N,session-M`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesNoChangeFastPathGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "best"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		CompletionProofPath: "/tmp/proof/completion.json",
		StatusPath:          "/tmp/status.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		CoordinationPath:    "/tmp/coordination.json",
		MasterInboxPath:     "/tmp/control/inbox/master.jsonl",
		RunStatePath:        "/tmp/state/run.json",
		SessionsStatePath:   "/tmp/state/sessions.json",
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
	for _, want := range []string{
		"If inbox state is unchanged, no target crossed a stale/health threshold, no coordination/coverage fact changed, and the active owner is still within grace, treat that control cycle as a no-change fast path.",
		"If `required_remaining` is 0 but `/tmp/summary.md` or `/tmp/proof/completion.json` is missing, that control cycle is **not** a no-change fast path.",
		"If `/tmp/status.json` disagrees with `/tmp/obligation-model.json` about required remaining work, treat `/tmp/obligation-model.json` as canonical and repair `/tmp/status.json` before you idle or close out.",
		"An active session with `active_idle`, `transport_blocked`, `progress_blocked`, or `ownership_risky` facts is **not** a no-change fast path.",
		"Do not use `goalx status` as a default heartbeat.",
		"Do not repeatedly restate unchanged authoritative files, health summaries, or stale-threshold reasoning.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolBindsRequiredFrontierFactsToImmediateIntervention(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:          "demo",
		Objective:        "ship it",
		Mode:             goalx.ModeWorker,
		Engine:           "codex",
		ProjectRoot:      "/tmp/project",
		StatusPath:       "/tmp/status.json",
		CoordinationPath: "/tmp/coordination.json",
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
		"A required item with frontier facts such as `unmapped_required`, `session_owner_missing`, `master_orphaned`, or `premature_blocked` is **not** a no-change fast path.",
		"If `premature_blocked` appears, the required item is not durably blocked yet. Keep probing reachable machine surfaces or dispatch or take over the next concrete lane now.",
		"If `master_orphaned` appears, resolve it in the current control cycle: dispatch or resume a worker, take the work over directly, or durably update the required frontier before you wait.",
		"After any `keep`, `integrate`, or frontier-changing redirect, refresh `/tmp/status.json` and `/tmp/coordination.json` before you idle or return to `goalx wait`.",
		"If reusable worker capacity exists but open required work stays serialized onto one execution lane, either dispatch or reassign more work now, or durably record why this control cycle stays serial.",
		"If a required item's execution lane is blocked or risky, resolve it in the current control cycle: inspect directly (including shell/tmux if needed), redirect, park+replace, or take the work over yourself.",
		"If an active session becomes `active_idle`, treat that as \"worker result or next-step handoff is waiting on you\" and review, redirect, keep, or take over in the current control cycle.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesReportsAndCompletionGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "audit auth",
		RunName:             "demo",
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		StatusPath:          "/tmp/status.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		ReportsDir:          "/tmp/run/reports",
		DimensionsPath:      "/tmp/control/dimensions.json",
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
		"`/tmp/obligation-model.json` required items are the canonical mutable obligations and definition of done.",
		"Do not use required obligations as implementation tasks or temporary decomposition.",
		"Keep execution decomposition in coordination, inbox, journals, and session briefs instead of rewriting obligations.",
		"Only move a required obligation toward closeout when decisive evidence says the outcome itself is satisfied or the user explicitly approved removing it from scope.",
		"Missing user credentials, external approval, or real-world publish access does not justify treating a required outcome as end-to-end verified.",
		"If a session produced valuable work outside the original items, that work matters.",
		"If a session shows no journal output for 15+ minutes while its lease is healthy",
		"Run `goalx verify --run demo` when you need fresh assurance evidence.",
		"goalx dimension [--run NAME] <session-N|all> --set depth,adversarial",
		"goalx dimension [--run NAME] <session-N> --add creative",
		"goalx dimension [--run NAME] <session-N> --remove depth",
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
	if strings.Contains(text, "Goal items are your working decomposition, not the definition of done.") {
		t.Fatalf("rendered master protocol should not describe obligations as decomposition:\n%s", text)
	}
}

func TestRenderMasterProtocolOmitsDuplicatedColdTablesButKeepsDispatchGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "audit auth",
		RunName:             "demo",
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		StatusPath:          "/tmp/status.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/wt-1"},
		},
		Engines: map[string]goalx.EngineConfig{
			"codex": {Description: "Fast code editing", Models: map[string]string{"best": "gpt-5.4"}},
		},
		Preferences: goalx.PreferencesConfig{
			Worker: goalx.PreferencePolicy{Guidance: "speed"},
		},
		DimensionsPath: "/tmp/control/dimensions.json",
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
		"### Preferences",
		"| Worker | speed |",
		"Prefer policy-based session launches.",
		"Explicit `--engine/--model` is an override.",
		"goalx dimension [--run NAME] <session-N|all> --set depth,adversarial",
		"goalx dimension [--run NAME] <session-N> --add creative",
		"goalx dimension [--run NAME] <session-N> --remove depth",
		"/tmp/control/dimensions.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"### Session Roster",
		"### Engines",
		"### Effort Levels",
		"### Routing Profiles",
		"### Routing Rules",
		"| Dimension | Guidance |",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit duplicated section %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolIncludesWaitSafetyNetGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
	if !strings.Contains(text, "Provider stop hooks and similar wake mechanisms are only safety nets. Your normal idle path should still be `goalx wait`.") {
		t.Fatalf("rendered master protocol missing wait safety-net guidance:\n%s", text)
	}
}

func TestRenderMasterProtocolIncludesMixedModeCoordinationGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeWorker,
		Engines:               goalx.BuiltinEngines,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		AssurancePlanPath:     "/tmp/assurance-plan.json",
		ObligationModelPath:   "/tmp/obligation-model.json",
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
		"goalx add --run demo --effort high",
		"goalx add --run demo --engine ENGINE --model MODEL --effort LEVEL",
		"goalx afford --run demo master",
		"canonical command surface",
		"Check the coordination digest version each control cycle.",
		"Default to current repo state, control files, runtime state, and latest session outputs.",
		"Only reread older journal history when the current state is ambiguous",
		"You may reorder, delegate, or temporarily postpone required work within the current obligation boundary",
		"you may not declare the objective complete while any required obligation remains unfinished, blocked, or merely scheduled for later",
		"If any required item is uncovered, that is a scheduling bug.",
		"If you use `/tmp/proof/completion.json`, treat it as an agent-owned closeout/evidence surface.",
		"If independent capacity exists and required work remains, dispatch it now instead of waiting.",
		"An external blocker does not silence dispatch",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh inbox instructions before launching another session.",
		"Improvement backlog",
		"Treat the root master session as the run's control authority, not its default execution surface.",
		"Choose the execution surface with the lowest volatility that can still produce the required outcome and proof.",
		"Use short, reversible, low-volatility probes to improve routing or judgment.",
		"If a short probe becomes attention-binding, interaction-heavy, externally gated, or difficult to recover from, move ownership off the root master.",
		"Treat narrowed causes as hypotheses until a failing regression test or decisive evidence confirms them.",
		"If an urgent required item is active and you are not directly coding it yourself, dispatch or resume a worker quickly instead of carrying passive master ownership across repeated control cycles.",
		"Keep detailed hypotheses, traces, and path comparisons in journals, not the coordination digest.",
		"Avoid sync-only liveness narration.",
		"Sessions without dedicated worktrees share the run worktree.",
		"Use `goalx add --worktree` for parallel isolation.",
		"Explicit `--engine/--model` bypasses the current selection policy.",
		"Run `git status --short` before you say \"ready for commit\" or \"ready for closeout\".",
		"closeout is not complete until `/tmp/summary.md` and `/tmp/proof/completion.json` exist",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"### Native Helpers",
		"Provider-native",
		"Web search is available when local evidence is insufficient.",
		"This engine only starts native subagents when you explicitly invoke them.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit %q:\n%s", unwanted, text)
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
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "best"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
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
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
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

func TestRenderMasterProtocolIncludesCurrentTimeAndEvolveIntentFacts(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:            "ship it",
		RunName:              "demo",
		Mode:                 goalx.ModeAuto,
		Intent:               runIntentEvolve,
		CurrentTime:          "2026-03-27T08:00:00Z",
		RunStartedAt:         "2026-03-27T06:00:00Z",
		ExperimentsLogPath:   "/tmp/experiments.jsonl",
		IntegrationStatePath: "/tmp/integration.json",
		Master:               goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:          "ar-demo",
		SummaryPath:          "/tmp/summary.md",
		AssurancePlanPath:    "/tmp/assurance-plan.json",
		ObligationModelPath:  "/tmp/obligation-model.json",
		StatusPath:           "/tmp/status.json",
		CoordinationPath:     "/tmp/coordination.json",
		EngineCommand:        "codex --model gpt-5.4",
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
		"Current time (UTC): 2026-03-27T08:00:00Z",
		"Run started at (UTC): 2026-03-27T06:00:00Z",
		"Intent: evolve",
		"This run was launched with explicit `evolve` intent.",
		"Experiment ledger: `/tmp/experiments.jsonl`",
		"Current integrated path: `/tmp/integration.json`",
		"`goalx durable write experiments --run demo --kind experiment.created --actor master --body-file /abs/path.json`",
		"Do not hand-author the durable-log envelope.",
		"`required_remaining == 0` only means the current required baseline is covered.",
		"Do not enter review or idle just because required items are covered.",
		"Before you enter review or idle in `evolve`, either dispatch the next experiment, integrate a winning path, or append `evolve.stopped`.",
		"Run an explicit iteration frontier: choose the highest-value next experiment or frontier, execute one or more independent experiments when warranted, review evidence, record the result, then continue, pivot, or consolidate.",
		"A frontier may contain one experiment or multiple independent experiments when paths are worktree-safe and separately verifiable.",
		"Record a factual blocker that is truly outside your current permissions, credentials, or reachable environment.",
		"Close rejected, abandoned, or superseded paths with `experiment.closed`.",
		"Append `evolve.stopped` when you intentionally close the current frontier.",
		"If you need the current frontier snapshot after relaunch, read `goalx context --run demo` or `goalx afford --run demo` instead of inventing a second agent-written ledger.",
		"`goalx add --run demo --worktree --base-branch session-N`",
		"Valid stop reasons include budget exhaustion",
		"user redirection",
		"diminishing returns relative to cost, risk, and remaining upside",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"record a factual stop reason in the experiment ledger",
		"Record why the current path no longer justifies more budget or risk.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit legacy evolve wording %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolIncludesBudgetExhaustionGracefulStopDoctrine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeAuto,
		Intent:              runIntentDeliver,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		ActivityPath:        "/tmp/activity.json",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		StatusPath:          "/tmp/status.json",
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
		"do not dispatch more work",
		"Inspect current outputs",
		"keep/adopt if warranted",
		"save if continuation or artifact preservation matters",
		"stop explicitly",
		"do not auto-drop",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolOmitsStaticBudgetLiteral(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:    "ship it",
		RunName:      "demo",
		Mode:         goalx.ModeAuto,
		Intent:       runIntentDeliver,
		Master:       goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		ActivityPath: "/tmp/activity.json",
		Budget:       goalx.BudgetConfig{MaxDuration: 8 * time.Hour},
		SummaryPath:  "/tmp/summary.md",
		StatusPath:   "/tmp/status.json",
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "Budget: 8h0m0s") {
		t.Fatalf("rendered master protocol should omit static budget literal:\n%s", text)
	}
	if !strings.Contains(text, "current run facts show exhausted budget") {
		t.Fatalf("rendered master protocol missing budget-facts doctrine:\n%s", text)
	}
}

func TestRenderSubagentProtocolOmitsBudgetSection(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeWorker,
		Engine:            "codex",
		SessionName:       "session-1",
		SessionInboxPath:  "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath: "/tmp/control/session-1-cursor.json",
		JournalPath:       "/tmp/journal.jsonl",
		Budget:            goalx.BudgetConfig{MaxDuration: 8 * time.Hour, MaxRounds: 5},
	}

	if err := RenderSubagentProtocol(data, runDir, 0); err != nil {
		t.Fatalf("RenderSubagentProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "program-1.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	for _, unwanted := range []string{"## Budget", "Max rounds:", "Max duration:"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered subagent protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolIncludesExploreIntentFacts(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "understand the regression and compare alternatives",
		RunName:             "demo",
		Mode:                goalx.ModeAuto,
		Intent:              runIntentExplore,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		StatusPath:          "/tmp/status.json",
		CoordinationPath:    "/tmp/coordination.json",
		EngineCommand:       "codex --model gpt-5.4",
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
		"Intent: explore",
		"This run was launched with explicit `explore` intent.",
		"Treat this as an evidence-first run.",
		"Start by expanding evidence, validating blind spots, comparing alternatives, and producing reports or dispatchable slices before committing to implementation.",
		"Do not treat early implementation as the default first move.",
		"Only shift into implementation when evidence from this run makes a concrete path clearly justified, and record why.",
		"If the user's goal is investigation-first, do not close out with code alone; preserve findings, alternatives, and remaining decision points in the final summary.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesDebateIntentFacts(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "challenge prior audit findings and converge on a fix list",
		RunName:             "demo",
		Mode:                goalx.ModeAuto,
		Intent:              runIntentDebate,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		StatusPath:          "/tmp/status.json",
		CoordinationPath:    "/tmp/coordination.json",
		EngineCommand:       "codex --model gpt-5.4",
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
		"Intent: debate",
		"This run was launched with explicit `debate` intent.",
		"Treat this as an adversarial review of prior findings, not a fresh greenfield implementation run.",
		"Start from the saved evidence and reports, identify the strongest disagreements, and drive them to an explicit consensus or decisive rejection.",
		"Do not let `debate` collapse into parallel monologues.",
		"Do not shift into implementation until the debated path, objection handling, and recommended next action are durable and explicit.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesImplementIntentFacts(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "implement the agreed remediation plan",
		RunName:             "demo",
		Mode:                goalx.ModeAuto,
		Intent:              runIntentImplement,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		StatusPath:          "/tmp/status.json",
		CoordinationPath:    "/tmp/coordination.json",
		EngineCommand:       "codex --model gpt-5.4",
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
		"Intent: implement",
		"This run was launched with explicit `implement` intent.",
		"Treat prior reports, debate output, and saved evidence as the input contract for implementation.",
		"Default toward concrete implementation, validation, review, and integration slices instead of reopening broad exploration.",
		"If the saved evidence conflicts with the current repo, runtime, or obligation boundary, resolve the contradiction quickly and record it before continuing.",
		"Do not treat prior recommendations as self-authenticating; verify they still hold in the current repo before you commit on them.",
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
		Mode:                  goalx.ModeWorker,
		Master:                goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession:           "ar-demo",
		SummaryPath:           "/tmp/summary.md",
		AssurancePlanPath:     "/tmp/assurance-plan.json",
		ObligationModelPath:   "/tmp/obligation-model.json",
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
		"If a session is stale for 15+ minutes while its lease is healthy, inspect it in the current control cycle instead of waiting passively.",
		"Reconfirm the immutable run objective from `goalx context --run demo` before declaring completion.",
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

func TestRenderMasterProtocolUsesInspectFirstStaleEscalation(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeWorker,
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		SummaryPath: "/tmp/summary.md",
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
		"If a session is stale for 15+ minutes while its lease is healthy, inspect it in the current control cycle instead of waiting passively.",
		"If stale facts persist after inspection or follow-up recheck, or transport/pane facts show a real blockage, park it and replace it.",
		"Long model waits or test runs are not by themselves proof that ownership failed.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolRequiresActionOnBlockedOwnerFacts(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		ObligationModelPath: "/tmp/obligation-model.json",
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
		"If a required item stays stuck, reassign it, split it, or take it over yourself.",
		"Treat provider dialogs and similar interruptions as path-failure or transport-incident facts.",
		"Record them, recover, reroute, and continue.",
		"Prefer reroute, delegation, replacement, or relaunch over waiting. Use direct shell/tmux intervention only as emergency transport work.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolRequiresInboxRecheckOnRepeatedWake(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeWorker,
		Engine:            "codex",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
		Target:            goalx.TargetConfig{Files: []string{"main.go"}},
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
		"Treat any transport wake text as \"read the session inbox now\".",
		"After a transport wake or `goalx wait` return, read `",
		"Acknowledge the latest processed inbox entry:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesExplicitCoverageOwnershipGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:           "ship it",
		RunName:             "demo",
		Mode:                goalx.ModeWorker,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		ObligationModelPath: "/tmp/obligation-model.json",
		CoordinationPath:    "/tmp/coordination.json",
		StatusPath:          "/tmp/status.json",
		MasterJournalPath:   "/tmp/master.jsonl",
		EngineCommand:       "codex --model gpt-5.4",
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
		"When durable ownership becomes explicit, inspect `goalx schema coordination` and write `coordination` through `goalx durable write coordination --run demo --body-file /abs/path.json`.",
		"If `/tmp/coordination.json` shows uncovered current required work, required outcomes must not remain silently unmapped.",
		"When explicit coverage facts show uncovered required work and reusable capacity exists, either dispatch or reassign it now, or record why this control cycle stays serial.",
		"Do not infer ownership from journals or `owner_scope`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"The framework decides next action.",
		"The framework infers ownership from journals.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolOmitsProviderNativeCapabilityGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"Treat the root master session as the run's control authority, not its default execution surface.",
		"Choose the execution surface with the lowest volatility that can still produce the required outcome and proof.",
		"Use short, reversible, low-volatility probes to improve routing or judgment.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"### Native Helpers",
		"Claude Code native subagents are available in this session.",
		"Provider-native",
		"Web search is available when local evidence is insufficient.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolDefinesRootMasterAsControlPlane(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"Treat the root master session as the run's control authority, not its default execution surface.",
		"Choose the execution surface with the lowest volatility that can still produce the required outcome and proof.",
		"If a short probe becomes attention-binding, interaction-heavy, externally gated, or difficult to recover from, move ownership off the root master.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolDropsCapabilityFirstRoutingHint(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
	unwanted := "If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow."
	if strings.Contains(text, unwanted) {
		t.Fatalf("rendered master protocol should omit capability-first routing hint %q:\n%s", unwanted, text)
	}
}

func TestRenderMasterProtocolTreatsProviderDialogsAsIncidents(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"Treat provider dialogs and similar interruptions as path-failure or transport-incident facts.",
		"Record them, recover, reroute, and continue.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolOmitsProviderNativeOwnedSurfaceLanguage(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:           "demo",
		Objective:         "ship it",
		Mode:              goalx.ModeWorker,
		Engine:            "claude-code",
		ProjectRoot:       "/tmp/project",
		SessionName:       "session-1",
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
	for _, unwanted := range []string{
		"Provider-native tools are allowed inside your owned execution surface when they materially help.",
		"If provider-gated or volatile execution blocks progress, surface it quickly through the journal or `dispatchable_slices` instead of waiting silently.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered session protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolIncludesAutonomyPersistenceGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
		"If the obligation boundary is clear and a concrete next action exists, continue acting. Method uncertainty is not intent uncertainty.",
		"Context compaction is normal. Recover from durable state and continue; do not hand off or close out early because context was trimmed.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesGenericExecutionGuidanceForCodex(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
		Master:        goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
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
		"**Execution rule**: Treat action verbs in this protocol as instructions to execute the corresponding tool action in this control cycle. Stating intent is not action.",
		"Choose the execution surface with the lowest volatility that can still produce the required outcome and proof.",
		"If the obligation boundary is clear and a concrete next action exists, continue acting. Method uncertainty is not intent uncertainty.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		"### Native Helpers",
		"Codex CLI native subagents are available in this session.",
		"This engine only starts native subagents when you explicitly invoke them.",
		"Provider-native",
		"Web search is available when local evidence is insufficient.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolIncludesAutonomyPersistenceGuidanceForCodex(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
		Master:        goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
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
		"If the obligation boundary is clear and a concrete next action exists, continue acting. Method uncertainty is not intent uncertainty.",
		"Context compaction is normal. Recover from durable state and continue; do not hand off or close out early because context was trimmed.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered codex master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesExecutionDisciplineForClaude(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeWorker,
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
	want := "**Execution rule**: Treat action verbs in this protocol as instructions to execute the corresponding tool action in this control cycle. Stating intent is not action."
	if !strings.Contains(text, want) {
		t.Fatalf("rendered claude master protocol missing %q:\n%s", want, text)
	}
}

func TestRenderMasterProtocolReferencesGoalxContextAndAfford(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeWorker,
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
		"`goalx afford --run demo master`",
		"`/tmp/control/context-index.json`",
		"`/tmp/control/activity.json`",
		"`/tmp/control/affordances.md`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesSelectionFacts(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:             "ship it",
		RunName:               "demo",
		Mode:                  goalx.ModeWorker,
		Master:                goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Roles:                 goalx.RoleDefaultsConfig{Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini"}},
		SelectionSnapshotPath: "/tmp/run/selection-policy.json",
		SelectionPolicy:       goalx.EffectiveSelectionPolicy{MasterCandidates: []string{"codex/gpt-5.4", "claude-code/opus"}, WorkerCandidates: []string{"codex/gpt-5.4-mini", "claude-code/opus"}, DisabledTargets: []string{"claude-code/sonnet"}},
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
		"Selection snapshot: `/tmp/run/selection-policy.json`",
		"Bootstrap master target: `codex/gpt-5.4`",
		"Worker default target: `codex/gpt-5.4-mini`",
		"Master candidates: `codex/gpt-5.4, claude-code/opus`",
		"Disabled targets: `claude-code/sonnet`",
		"Treat these as factual candidate pools and bans, not hidden framework judgment.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolUsesContextInsteadOfCharterForStartup(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:        "ship it",
		RunName:          "demo",
		Mode:             goalx.ModeWorker,
		Master:           goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		MasterCursorPath: "/tmp/control/master-cursor.json",
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	if strings.Contains(text, "Read `"+RunCharterPath(runDir)+"` first.") {
		t.Fatalf("rendered master protocol should use goalx context as startup identity surface:\n%s", text)
	}
	if !strings.Contains(text, "`goalx context --run demo`") {
		t.Fatalf("rendered master protocol missing goalx context startup surface:\n%s", text)
	}
}

func TestRenderMasterProtocolRoutesSessionDispatchThroughTell(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective: "ship it",
		RunName:   "demo",
		Mode:      goalx.ModeWorker,
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
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
		"`goalx tell --run demo session-N \"...\"`",
		"Do not write `control/inbox/session-N.jsonl` directly.",
		"`goalx attach --run demo session-N`",
		"Treat `status=sent` in `",
		"as transport success: the input was submitted to the target engine.",
		"`status=buffered` means the wake text is still sitting in the target input buffer.",
		"`queued_message_visible=true` means the provider queue accepted the transport.",
		"Do not take over or reassign immediately just because the session cursor has not advanced yet after a `sent` delivery.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolUsesAckInboxForMasterCursorAdvance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:        "ship it",
		RunName:          "demo",
		Mode:             goalx.ModeWorker,
		Master:           goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		MasterCursorPath: "/tmp/control/master-cursor.json",
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
		"`goalx ack-inbox --run demo master`",
		"instead of editing `",
		"master-cursor.json",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolStatusRecordIsFactsOnly(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:   "ship it",
		RunName:     "demo",
		Mode:        goalx.ModeWorker,
		StatusPath:  "/tmp/status.json",
		Master:      goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		TmuxSession: "ar-demo",
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
		`"recommendation"`,
		`"acceptance_met"`,
		`"acceptance_status"`,
		`"goal_satisfied"`,
		`"phase":"working|review|complete"`,
		`"user_approved":false`,
		"`keep_session`",
		"`default_command`",
		"`effective_command`",
		"`goal_version`",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit judgment field %q:\n%s", unwanted, text)
		}
	}
	for _, want := range []string{
		"`goalx schema obligation-model`",
		"`goalx schema assurance-plan`",
		"`goalx schema coordination`",
		"`goalx schema status`",
		"`goalx schema obligation-log`",
		"`goalx schema experiments`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesSuccessModelCloseoutDoctrine(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		ObligationModelPath: "/tmp/obligation-model.json",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		EvidenceLogPath:     "/tmp/evidence-log.jsonl",
		StatusPath:          "/tmp/status.json",
		CoordinationPath:    "/tmp/coordination.json",
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
		"`goalx schema success-model`",
		"`goalx schema proof-plan`",
		"`goalx schema workflow-plan`",
		"`goalx schema domain-pack`",
		"`goalx schema intervention-log`",
		"Builder-only correctness is insufficient by default.",
		"treat them as closeout surfaces: required dimensions must stay owned, required proof items must be present, and critic/finisher gates must be satisfied before finalization.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesPriorPromotionBoundary(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		ObligationModelPath: "/tmp/obligation-model.json",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		StatusPath:          "/tmp/status.json",
	}

	if err := RenderMasterProtocol(data, runDir); err != nil {
		t.Fatalf("RenderMasterProtocol: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read rendered protocol: %v", err)
	}
	text := string(out)
	want := "Validated success-delta lessons may become priors only through the memory proposal and promotion path; do not hand-edit canonical priors."
	if !strings.Contains(text, want) {
		t.Fatalf("rendered master protocol missing prior promotion boundary %q:\n%s", want, text)
	}
}

func TestRenderMasterProtocolIncludesCompilerReportGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		ObligationModelPath: "/tmp/obligation-model.json",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		StatusPath:          "/tmp/status.json",
		CoordinationPath:    "/tmp/coordination.json",
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
		"`goalx schema compiler-input`",
		"`goalx schema compiler-report`",
		"inspect `success-model`, `proof-plan`, `workflow-plan`, `domain-pack`, `compiler-input`, and `compiler-report`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesCriticFinisherAndPriorGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:             "demo",
		Objective:           "ship it",
		Mode:                goalx.ModeWorker,
		Engine:              "codex",
		ProjectRoot:         "/tmp/project",
		SessionName:         "session-1",
		SessionInboxPath:    "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:   "/tmp/control/session-1-cursor.json",
		JournalPath:         "/tmp/journal.jsonl",
		ObligationModelPath: "/tmp/obligation-model.json",
		AssurancePlanPath:   "/tmp/assurance-plan.json",
		WorktreePath:        "/tmp/worktree",
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
		"Success model: `",
		"Proof plan: `",
		"Workflow plan: `",
		"Domain pack: `",
		"If your assignment or session role acts as `builder`, `critic`, or `finisher`, treat that as a real workflow gate:",
		"builder output alone does not close a non-trivial run.",
		"it may later promote through the memory proposal path instead of direct canonical-memory edits.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered subagent protocol missing %q:\n%s", want, text)
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
