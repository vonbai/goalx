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
		RunName:                   "demo",
		Objective:                 "ship it",
		Mode:                      goalx.ModeDevelop,
		Engine:                    "codex",
		ProjectRoot:               "/tmp/project",
		Target:                    goalx.TargetConfig{Files: []string{"main.go"}},
		GoalPath:                  "/tmp/goal.json",
		IntegrationStatePath:      "/tmp/integration.json",
		AcceptanceStatePath:       "/tmp/acceptance.json",
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
		"The runtime truth is the provider-native interactive TUI.",
		"Provider-native skills, plugins, and MCP tools are allowed when they materially help.",
		"If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
		"Do NOT invoke orchestration/meta slash commands or skills",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
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
		Mode:              goalx.ModeDevelop,
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
		Mode:                      goalx.ModeDevelop,
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
		Mode:                      goalx.ModeDevelop,
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
		Mode:              goalx.ModeDevelop,
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

func TestProviderCapabilitiesMarksOnlyCodexForActionExecutionReminder(t *testing.T) {
	codexCaps := providerCapabilities("codex")
	if !codexCaps.ActionExecutionReminder {
		t.Fatalf("codex capabilities should enable action execution reminder: %+v", codexCaps)
	}

	claudeCaps := providerCapabilities("claude-code")
	if claudeCaps.ActionExecutionReminder {
		t.Fatalf("claude capabilities should not enable action execution reminder: %+v", claudeCaps)
	}
}

func TestProviderCapabilitiesMarksOnlyClaudeForAutonomyPersistenceReminder(t *testing.T) {
	claudeCaps := providerCapabilities("claude-code")
	if !claudeCaps.AutonomyPersistenceReminder {
		t.Fatalf("claude capabilities should enable autonomy persistence reminder: %+v", claudeCaps)
	}

	codexCaps := providerCapabilities("codex")
	if codexCaps.AutonomyPersistenceReminder {
		t.Fatalf("codex capabilities should not enable autonomy persistence reminder: %+v", codexCaps)
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
		"## Native Helpers",
		"You are running in Claude Code.",
		"Native subagents are transient helpers inside this session.",
		"short parallel reading, review, doc/API checks, or adversarial cross-checks",
		"Summarize every native-helper result back into this session's journal, report, or `dispatchable_slices` before you continue.",
		"The runtime truth is the provider-native interactive TUI.",
		"Provider-native skills, plugins, and MCP tools are allowed when they materially help.",
		"If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
		"Web search is available when local evidence is insufficient.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolIncludesCodexGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeDevelop,
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
		"## Native Helpers",
		"You are running in Codex CLI.",
		"Native subagents are transient helpers inside this session.",
		"This engine only starts native subagents when you explicitly invoke them.",
		"## Execution Discipline",
		"When this protocol tells you to read a file, run a command, append a journal entry, acknowledge inbox, verify, commit, or wait, perform the corresponding tool action in this turn.",
		"Do not end a turn after saying you will do something next. Execute the next tool call now.",
		"Do not give them durable ownership.",
		"Summarize every native-helper result back into this session's journal, report, or `dispatchable_slices` before you continue.",
		"The runtime truth is the provider-native interactive TUI.",
		"Provider-native skills, plugins, and MCP tools are allowed when they materially help.",
		"If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
		"re-check `/tmp/control/inbox/session-1.jsonl` before idling",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q", want)
		}
	}
	for _, unwanted := range []string{
		"Agent tool",
		"WebSearch/WebFetch",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered codex protocol should not inherit claude-only agent guidance %q", unwanted)
		}
	}
}

func TestRenderSubagentProtocolOmitsExecutionDisciplineForClaude(t *testing.T) {
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
	for _, unwanted := range []string{
		"## Execution Discipline",
		"When this protocol tells you to read a file, run a command, append a journal entry, acknowledge inbox, verify, commit, or wait, perform the corresponding tool action in this turn.",
		"Do not end a turn after saying you will do something next. Execute the next tool call now.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered claude protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderSubagentProtocolIncludesClaudeAutonomyPersistenceGuidance(t *testing.T) {
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

func TestRenderSubagentProtocolOmitsClaudeAutonomyPersistenceGuidanceForCodex(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeDevelop,
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
	for _, unwanted := range []string{
		"## Autonomy Persistence",
		"If a concrete next step exists inside your current assignment, execute it before idling.",
		"Do not ask master to confirm local method choices inside assigned scope. Act, record, continue.",
		"Context compaction is routine. Recover from inbox, journal, and current files instead of stopping early.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered codex protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderSubagentProtocolDropsRedundantResearchOnlyDoneLine(t *testing.T) {
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
	unwanted := "Do not declare yourself done. The master decides when to stop."
	if strings.Contains(text, unwanted) {
		t.Fatalf("rendered protocol should omit %q:\n%s", unwanted, text)
	}
}

func TestRenderSubagentProtocolIncludesOptimizerDoctrineInDevelopMode(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:                "demo",
		Objective:              "optimize discovery pipeline",
		Mode:                   goalx.ModeDevelop,
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
		"Produce evidence-backed findings until the master stops you",
		"Quantify what you can",
		"Every major finding must include counter-evidence or failed alternative explanations.",
		"## Key Findings",
		"## Recommendation",
		"## Priority Fix List (if applicable)",
		"dispatchable_slices",
		"directly adopt",
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
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeDevelop,
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
	modeSection := sectionBetween(string(out), "## Mode: Develop", "## Context")
	for _, want := range []string{
		"one coherent capability slice at a time",
		"Your code will be reviewed. Every change must be justified and minimal.",
		"If a local validation command is configured, run it before handing work back:",
		"Keep changes minimal and correct. Do not add unrelated improvements, but do not cut corners on the change you are making.",
		"Respect file ownership from the current inbox assignment.",
		"If the inbox assignment names an allowed edit boundary, stay inside it.",
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

func TestRenderMasterProtocolRefinesReviewRoutingAndDepthCap(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeDevelop,
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
		"Prefer native helpers or a different engine/model path for short review, validation, and adversarial checks.",
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
		Mode:        goalx.ModeDevelop,
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
		Mode:        goalx.ModeDevelop,
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
		"When dispatching parallel develop work, make the inbox brief concrete: required outcome, allowed edit boundary, validation signal, and whether the branch should be kept, partially adopted, or treated as disposable exploration.",
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
		Mode:                   goalx.ModeDevelop,
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
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeDevelop,
		Engine:                 "codex",
		ProjectRoot:            "/tmp/project",
		SessionName:            "session-1",
		Target:                 goalx.TargetConfig{Files: []string{"main.go"}},
		LocalValidationCommand: "go test ./...",
		JournalPath:            "/tmp/journal.jsonl",
		SessionInboxPath:       "/tmp/control/inbox/session-1.jsonl",
		SessionCursorPath:      "/tmp/control/session-1-cursor.json",
		GoalPath:               "/tmp/goal.json",
		AcceptanceStatePath:    "/tmp/acceptance.json",
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
		"Required goal items are the canonical current-goal obligations for the overall run.",
		"Your assignment is the decomposition; the goal boundary is not.",
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
		"goalx afford --run demo master",
		"canonical command surface",
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
		RunName:                "demo",
		Objective:              "ship it",
		Mode:                   goalx.ModeDevelop,
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
			t.Fatalf("rendered develop protocol missing %q", want)
		}
	}
}

func TestRenderSubagentProtocolTreatsAutoModeAsDevelop(t *testing.T) {
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
		"goalx save demo && goalx run --from demo --intent debate",
		"Replace `goalx run --from demo --intent debate` with `goalx run --from demo --intent implement` or `goalx run --from demo --intent explore`",
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

func TestRenderMasterProtocolClarifiesKeepUsesRecordedParentBoundary(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Mode:        goalx.ModeDevelop,
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
		"Do not assume every develop session branch is rooted directly on the run worktree.",
		"If a develop session still has dirty uncommitted files, require a focused local commit before `goalx keep`, or inspect/take over the work yourself.",
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
		Objective:         "ship it",
		RunName:           "demo",
		Mode:              goalx.ModeDevelop,
		Master:            goalx.MasterConfig{Engine: "codex", Model: "best"},
		TmuxSession:       "ar-demo",
		SummaryPath:       "/tmp/summary.md",
		StatusPath:        "/tmp/status.json",
		CoordinationPath:  "/tmp/coordination.json",
		MasterInboxPath:   "/tmp/control/inbox/master.jsonl",
		RunStatePath:      "/tmp/state/run.json",
		SessionsStatePath: "/tmp/state/sessions.json",
		EngineCommand:     "codex exec",
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
		"An active session with `active_idle`, `transport_blocked`, `progress_blocked`, or `ownership_risky` facts is **not** a no-change fast path.",
		"Do not use `goalx status` as a default heartbeat.",
		"Do not repeatedly restate unchanged authoritative files, health summaries, or stale-threshold reasoning.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolBindsBlockedOwnersToImmediateIntervention(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeDevelop,
		Engine:      "codex",
		ProjectRoot: "/tmp/project",
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
		"An open required item with `owner_attention`, `owner_blocked`, or `owner_risky` facts is **not** a no-change fast path.",
		"If a required item owner is blocked or risky, resolve it in the current control cycle: inspect directly (including shell/tmux if needed), redirect, park+replace, or take the work over yourself.",
		"If an active session becomes `active_idle`, treat that as \"worker result or next-step handoff is waiting on you\" and review, redirect, keep, or take over in the current control cycle.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolIncludesReportsAndResearchCompletionGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:      "audit auth",
		RunName:        "demo",
		Mode:           goalx.ModeResearch,
		Master:         goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:    "ar-demo",
		SummaryPath:    "/tmp/summary.md",
		StatusPath:     "/tmp/status.json",
		GoalPath:       "/tmp/goal.json",
		ReportsDir:     "/tmp/run/reports",
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
		"`/tmp/goal.json` required items are the canonical current-goal obligations and definition of done.",
		"Do not use required goal items as implementation tasks or temporary decomposition.",
		"Keep execution decomposition in coordination, inbox, journals, and session briefs instead of rewriting goal items.",
		"Move an item to `claimed` only when decisive evidence says the goal outcome itself is satisfied and ready for closeout review.",
		"Missing user credentials, external approval, or real-world publish access does not justify claiming a required item as end-to-end verified.",
		"If a session produced valuable work outside the original items, that work matters.",
		"If a session shows no journal output for 15+ minutes while its lease is healthy",
		"Research runs usually close out through reports and goal updates; develop runs usually close out through reviewed code, verification evidence, and `goalx keep`.",
		"Run `goalx verify --run demo` when you need fresh acceptance evidence.",
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
		t.Fatalf("rendered master protocol should not describe goal items as decomposition:\n%s", text)
	}
}

func TestRenderMasterProtocolOmitsDuplicatedColdTablesButKeepsDispatchGuidance(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:   "audit auth",
		RunName:     "demo",
		Mode:        goalx.ModeResearch,
		Master:      goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession: "ar-demo",
		SummaryPath: "/tmp/summary.md",
		StatusPath:  "/tmp/status.json",
		GoalPath:    "/tmp/goal.json",
		Sessions: []SessionData{
			{Name: "session-1", WorktreePath: "/tmp/wt-1"},
		},
		Engines: map[string]goalx.EngineConfig{
			"codex": {Description: "Fast code editing", Models: map[string]string{"best": "gpt-5.4"}},
		},
		Preferences: goalx.PreferencesConfig{
			Research: goalx.PreferencePolicy{Guidance: "multi-perspective"},
			Develop:  goalx.PreferencePolicy{Guidance: "speed"},
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
		"| Research | multi-perspective |",
		"| Develop | speed |",
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
		"goalx add --run demo --mode research --effort high",
		"goalx add --run demo --mode develop --effort medium",
		"goalx add --run demo --mode research --engine ENGINE --model MODEL --effort LEVEL",
		"goalx afford --run demo master",
		"canonical command surface",
		"temporary research session",
		"Research-mode sessions produce evidence and reports, not mergeable code changes.",
		"Check the coordination digest version each control cycle.",
		"Default to current repo state, control files, runtime state, and latest session outputs.",
		"Only reread older journal history when the current state is ambiguous",
		"You may reorder, delegate, or temporarily postpone required work within the current goal",
		"you may not declare the goal complete while any required item remains unfinished, blocked, or merely scheduled for later",
		"If any required item is uncovered, that is a scheduling bug.",
		"If you use `/tmp/proof/completion.json`, treat it as an agent-owned closeout/evidence surface.",
		"If parallel capacity exists and independent required work remains, dispatch it now instead of waiting.",
		"Treat configured `parallel` as initial fan-out guidance, not a permanent ceiling;",
		"waiting_external",
		"Do not wait on one session if other independent required work can proceed.",
		"Prefer reusing a parked or idle session with fresh inbox instructions before launching another session.",
		"Improvement backlog",
		"Native subagents are transient helpers inside the current master session.",
		"Do not give them durable ownership.",
		"If a slice needs worktree ownership, pause/resume, replace, keep, or mergeable output, use `goalx add`.",
		"Summarize every native-helper result back into durable GoalX state before moving on.",
		"The runtime truth is the provider-native interactive TUI.",
		"Provider-native skills, plugins, and MCP tools are allowed when they materially help.",
		"If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
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
		AcceptanceStatePath:  "/tmp/acceptance.json",
		GoalPath:             "/tmp/goal.json",
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
		"`goalx durable append experiments --run demo --file /abs/path.jsonl`",
		"`version`, `kind`, `at`, `actor`, `body`",
		"`required_remaining == 0` only means the current required baseline is covered.",
		"Do not enter review or idle just because required items are covered.",
		"Before you enter review or idle in `evolve`, either dispatch the next experiment, integrate a winning path, or record a factual stop reason in the experiment ledger.",
		"Run an explicit iteration frontier: choose the highest-value next experiment or frontier, execute one or more independent experiments when warranted, review evidence, record the result, then continue, pivot, or consolidate.",
		"A frontier may contain one experiment or multiple independent experiments when paths are worktree-safe and separately verifiable.",
		"Record a factual blocker that is truly outside your current permissions, credentials, or reachable environment.",
		"Record why the current path no longer justifies more budget or risk.",
		"`goalx add --run demo --worktree --base-branch session-N`",
		"Stop the iteration frontier when budget is exhausted",
		"the user redirects or stops the run",
		"recent experiments show diminishing returns",
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
		Mode:        goalx.ModeDevelop,
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
		RunName:     "demo",
		Objective:   "ship it",
		Mode:        goalx.ModeDevelop,
		Engine:      "codex",
		ProjectRoot: "/tmp/project",
		GoalPath:    "/tmp/goal.json",
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
		"Prefer direct recovery (shell/tmux intervention, local fallback path, or relaunch) over waiting.",
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
		Mode:              goalx.ModeDevelop,
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
		Mode:                goalx.ModeDevelop,
		Master:              goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		TmuxSession:         "ar-demo",
		SummaryPath:         "/tmp/summary.md",
		AcceptanceStatePath: "/tmp/acceptance.json",
		GoalPath:            "/tmp/goal.json",
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
		"When durable ownership becomes explicit, replace `coordination` through `goalx durable replace coordination --run demo --file /abs/path.json` and record `owners` entries mapping `req-*` required items to owner tokens.",
		"If `/tmp/coordination.json` has a non-empty `owners` map, open required items must not remain silently unmapped.",
		"When explicit coverage facts show uncovered open work and reusable capacity exists, either dispatch or reassign it now, or record why this control cycle stays serial.",
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
		"The runtime truth is the provider-native interactive TUI.",
		"Provider-native skills, plugins, and MCP tools are allowed when they materially help.",
		"If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
		"Claude Code native subagents are available in this session.",
		"Native subagents are transient helpers inside the current master session.",
		"Do not give them durable ownership.",
		"If a slice needs worktree ownership, pause/resume, replace, keep, or mergeable output, use `goalx add`.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "under 30 seconds") {
		t.Fatalf("rendered master protocol should not keep time-threshold native-subagent wording:\n%s", text)
	}
}

func TestRenderMasterProtocolIncludesClaudeAutonomyPersistenceGuidance(t *testing.T) {
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
		"If helper work becomes multi-step, durable, worktree-bound, or expected to produce mergeable output, move it into `goalx add` instead of extending native-helper ownership.",
		"If the goal boundary is clear and a concrete next action exists, continue acting. Method uncertainty is not intent uncertainty.",
		"Context compaction is normal. Recover from durable state and continue; do not hand off or close out early because context was trimmed.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolMakesCodexNativeSubagentExplicitAskBoundaryVisible(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeDevelop,
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
		"Codex CLI native subagents are available in this session.",
		"**Execution rule**: Treat action verbs in this protocol as instructions to execute the corresponding tool action in this control cycle. Stating intent is not action.",
		"This engine only starts native subagents when you explicitly invoke them.",
		"The runtime truth is the provider-native interactive TUI.",
		"Provider-native skills, plugins, and MCP tools are allowed when they materially help.",
		"If the user or master explicitly names a provider-native capability and it is visible, use it before the default flow.",
		"If the named capability is unavailable, report that immediately.",
		"Do not treat skill presence as the selection standard.",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered master protocol missing %q:\n%s", want, text)
		}
	}
}

func TestRenderMasterProtocolOmitsClaudeAutonomyPersistenceGuidanceForCodex(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:     "ship it",
		RunName:       "demo",
		Mode:          goalx.ModeDevelop,
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
	for _, unwanted := range []string{
		"If helper work becomes multi-step, durable, worktree-bound, or expected to produce mergeable output, move it into `goalx add` instead of extending native-helper ownership.",
		"If the goal boundary is clear and a concrete next action exists, continue acting. Method uncertainty is not intent uncertainty.",
		"Context compaction is normal. Recover from durable state and continue; do not hand off or close out early because context was trimmed.",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered codex master protocol should omit %q:\n%s", unwanted, text)
		}
	}
}

func TestRenderMasterProtocolOmitsExecutionDisciplineForClaude(t *testing.T) {
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
	unwanted := "**Execution rule**: Treat action verbs in this protocol as instructions to execute the corresponding tool action in this control cycle. Stating intent is not action."
	if strings.Contains(text, unwanted) {
		t.Fatalf("rendered claude master protocol should omit %q:\n%s", unwanted, text)
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
		Mode:                  goalx.ModeDevelop,
		Master:                goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
		Roles:                 goalx.RoleDefaultsConfig{Research: goalx.SessionConfig{Engine: "claude-code", Model: "opus"}, Develop: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini"}},
		SelectionSnapshotPath: "/tmp/run/selection-policy.json",
		SelectionPolicy:       goalx.EffectiveSelectionPolicy{MasterCandidates: []string{"codex/gpt-5.4", "claude-code/opus"}, ResearchCandidates: []string{"claude-code/opus"}, DevelopCandidates: []string{"codex/gpt-5.4-mini"}, DisabledTargets: []string{"claude-code/sonnet"}},
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
		"Research default target: `claude-code/opus`",
		"Develop default target: `codex/gpt-5.4-mini`",
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
		Objective: "ship it",
		RunName:   "demo",
		Mode:      goalx.ModeDevelop,
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
		Mode:      goalx.ModeDevelop,
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

func TestRenderMasterProtocolStatusRecordIsFactsOnly(t *testing.T) {
	runDir := t.TempDir()
	data := ProtocolData{
		Objective:   "ship it",
		RunName:     "demo",
		Mode:        goalx.ModeDevelop,
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
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered master protocol should omit judgment field %q:\n%s", unwanted, text)
		}
	}
	for _, want := range []string{
		`"version":1`,
		`"phase":"working|review|complete"`,
		`"required_remaining":0`,
		`"keep_session":"session-N"`,
		`"updated_at":"`,
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
