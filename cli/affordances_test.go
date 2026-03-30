package cli

import (
	"os"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildAffordancesIncludesRunScopedCommands(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	commands := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		commands = append(commands, item.Command)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"goalx status --run guidance-run",
		"goalx observe --run guidance-run",
		"goalx context --run guidance-run",
		"goalx afford --run guidance-run",
		"goalx verify --run guidance-run",
		"goalx schema status",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("affordance commands missing %q:\n%s", want, joined)
		}
	}
}

func TestBuildAffordancesIncludesCloseoutAndAcceptancePaths(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	var closeoutItem *AffordanceItem
	for i := range doc.Items {
		if doc.Items[i].ID == "closeout" {
			closeoutItem = &doc.Items[i]
			break
		}
	}
	if closeoutItem == nil {
		t.Fatal("closeout affordance missing")
	}
	joinedPaths := strings.Join(closeoutItem.Paths, "\n")
	for _, want := range []string{
		AcceptanceStatePath(runDir),
		RunStatusPath(runDir),
		SummaryPath(runDir),
		CompletionStatePath(runDir),
	} {
		if !strings.Contains(joinedPaths, want) {
			t.Fatalf("closeout affordance missing path %q:\n%s", want, joinedPaths)
		}
	}
}

func TestBuildAffordancesRouteDurableInspectionThroughSchemaCommand(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	var found bool
	for _, item := range doc.Items {
		if item.ID != "durable-write-state" && item.ID != "durable-write-event" {
			continue
		}
		found = true
		if !strings.Contains(item.Summary, "Inspect the contract with `goalx schema <surface>` first.") {
			t.Fatalf("%s summary = %q", item.ID, item.Summary)
		}
		for _, unwanted := range []string{"canonical JSON shape", "canonical JSONL envelope"} {
			if strings.Contains(item.Summary, unwanted) {
				t.Fatalf("%s summary should not define schema authority inline: %q", item.ID, item.Summary)
			}
		}
	}
	if !found {
		t.Fatal("durable write affordance not found")
	}
}

func TestBuildAffordancesExposeTransportFactsPath(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	found := false
	for _, item := range doc.Items {
		if item.ID != "status" && item.ID != "observe" {
			continue
		}
		for _, path := range item.Paths {
			if path == TransportFactsPath(runDir) {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("transport facts path not exposed in status/observe affordances: %+v", doc.Items)
	}
}

func TestBuildAffordancesExposeCanonicalExperimentPaths(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	var foundExperiments bool
	var foundIntegration bool
	for _, item := range doc.Items {
		for _, path := range item.Paths {
			if path == ExperimentsLogPath(runDir) {
				foundExperiments = true
			}
			if path == IntegrationStatePath(runDir) {
				foundIntegration = true
			}
		}
	}
	if !foundExperiments || !foundIntegration {
		t.Fatalf("canonical experiment paths missing from affordances: experiments=%v integration=%v items=%+v", foundExperiments, foundIntegration, doc.Items)
	}
}

func TestRenderAffordancesMarkdownUsesCurrentRunPaths(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "master")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}
	text := RenderAffordancesMarkdown(doc)

	for _, want := range []string{
		"# GoalX Affordances",
		"goalx context --run guidance-run",
		"goalx afford --run guidance-run master",
		runDir,
		ControlDir(runDir),
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("markdown affordances missing %q:\n%s", want, text)
		}
	}
}

func TestAffordancesAvoidRecommendationLanguage(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}
	text := RenderAffordancesMarkdown(doc)
	for _, unwanted := range []string{
		"you should",
		"recommended next step",
		"best action",
		"must now",
	} {
		if strings.Contains(strings.ToLower(text), unwanted) {
			t.Fatalf("affordance markdown should avoid %q:\n%s", unwanted, text)
		}
	}
}

func TestBuildAffordancesDoesNotDefaultTargetToMaster(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}
	if doc.Target != "" {
		t.Fatalf("target = %q, want empty target", doc.Target)
	}
	for _, item := range doc.Items {
		if item.ID == "tell" && strings.Contains(item.Command, " master ") {
			t.Fatalf("tell affordance should not default to master:\n%s", item.Command)
		}
	}
}

func TestBuildAffordancesIncludesSessionTellAndAttachCommands(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	commands := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		commands = append(commands, item.Command)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		`goalx tell --run guidance-run session-N "message"`,
		`goalx attach --run guidance-run session-N`,
		`goalx add --run guidance-run --mode research --effort high --worktree "sub-goal"`,
		`goalx add --run guidance-run --mode develop --effort medium --worktree "sub-goal"`,
		`goalx add --run guidance-run --mode research --engine ENGINE --model MODEL --effort LEVEL --worktree "sub-goal"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("affordance commands missing %q:\n%s", want, joined)
		}
	}
}

func TestBuildAffordancesIncludesKeepCommands(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	commands := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		commands = append(commands, item.Command)
	}
	joined := strings.Join(commands, "\n")
	for _, want := range []string{
		"goalx keep --run guidance-run session-N",
		"goalx integrate --run guidance-run --method partial_adopt --from session-1,session-2",
		"goalx keep --run guidance-run",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("affordance commands missing %q:\n%s", want, joined)
		}
	}
}

func TestBuildAffordancesIncludesEvolveExperimentCommandsAndFacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(ExperimentsLogPath(runDir), []byte(
		"{\"version\":1,\"kind\":\"experiment.created\",\"at\":\"2026-03-28T10:00:00Z\",\"actor\":\"goalx\",\"body\":{\"experiment_id\":\"exp-1\",\"created_at\":\"2026-03-28T10:00:00Z\"}}\n"+
			"{\"version\":1,\"kind\":\"experiment.integrated\",\"at\":\"2026-03-28T10:05:00Z\",\"actor\":\"goalx\",\"body\":{\"integration_id\":\"int-1\",\"result_experiment_id\":\"exp-2\",\"source_experiment_ids\":[\"exp-1\"],\"method\":\"keep\",\"recorded_at\":\"2026-03-28T10:05:00Z\"}}\n"), 0o644); err != nil {
		t.Fatalf("write experiments log: %v", err)
	}
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:                 1,
		CurrentExperimentID:     "exp-2",
		CurrentBranch:           "goalx/guidance-run/root",
		CurrentCommit:           "abc123",
		LastIntegrationID:       "int-1",
		LastMethod:              "keep",
		LastSourceExperimentIDs: []string{"exp-1"},
		UpdatedAt:               "2026-03-28T10:05:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}
	if err := RefreshEvolveFacts(runDir); err != nil {
		t.Fatalf("RefreshEvolveFacts: %v", err)
	}

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	commands := make([]string, 0, len(doc.Items))
	facts := make([]string, 0, len(doc.Items))
	paths := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		commands = append(commands, item.Command)
		facts = append(facts, strings.Join(item.Facts, "\n"))
		paths = append(paths, strings.Join(item.Paths, "\n"))
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, want := range []string{
		"goalx diff --run guidance-run session-1 session-2",
		`goalx add --run guidance-run --mode develop --worktree --base-branch session-N "follow-on direction"`,
		"goalx durable write experiments --run guidance-run --kind experiment.closed --actor master --body-file /abs/path.experiment-closed.json",
		"goalx durable write experiments --run guidance-run --kind evolve.stopped --actor master --body-file /abs/path.evolve-stopped.json",
	} {
		if !strings.Contains(joinedCommands, want) {
			t.Fatalf("affordance commands missing %q:\n%s", want, joinedCommands)
		}
	}
	joinedFacts := strings.Join(facts, "\n")
	for _, want := range []string{
		"Current integrated experiment: `exp-2`.",
		"Experiment entries: `2`.",
		"Last experiment record: `2026-03-28T10:05:00Z`.",
		"Last integration method: `keep`.",
		"Last integration sources: `exp-1`.",
		"Frontier state: `active`.",
		"Best experiment: `exp-2`.",
		"Open candidate count: `1`.",
	} {
		if !strings.Contains(joinedFacts, want) {
			t.Fatalf("affordance facts missing %q:\n%s", want, joinedFacts)
		}
	}
	if !strings.Contains(strings.Join(paths, "\n"), EvolveFactsPath(runDir)) {
		t.Fatalf("affordance paths missing evolve facts path:\n%s", strings.Join(paths, "\n"))
	}
}

func TestBuildAffordancesOmitsEvolveManagementItemsOutsideEvolve(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	commands := make([]string, 0, len(doc.Items))
	paths := make([]string, 0, len(doc.Items))
	for _, item := range doc.Items {
		commands = append(commands, item.Command)
		paths = append(paths, strings.Join(item.Paths, "\n"))
	}
	joinedCommands := strings.Join(commands, "\n")
	for _, blocked := range []string{
		"goalx durable write experiments --run guidance-run --kind experiment.closed --actor master --body-file /abs/path.experiment-closed.json",
		"goalx durable write experiments --run guidance-run --kind evolve.stopped --actor master --body-file /abs/path.evolve-stopped.json",
	} {
		if strings.Contains(joinedCommands, blocked) {
			t.Fatalf("affordances unexpectedly exposed evolve command %q:\n%s", blocked, joinedCommands)
		}
	}
	if strings.Contains(strings.Join(paths, "\n"), EvolveFactsPath(runDir)) {
		t.Fatalf("affordances unexpectedly exposed evolve facts path outside evolve:\n%s", strings.Join(paths, "\n"))
	}
}

func TestBuildAffordancesIncludesProviderRuntimeFactsForClaudeTargets(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	sessionName := "session-1"
	if err := EnsureSessionControl(runDir, sessionName); err != nil {
		t.Fatalf("EnsureSessionControl: %v", err)
	}
	identity := &SessionIdentity{
		Version:         1,
		SessionName:     sessionName,
		ExperimentID:    "exp_guidance_claude_target_1",
		RoleKind:        "research",
		Mode:            "research",
		Engine:          "claude-code",
		Model:           "opus",
		OriginCharterID: meta.CharterID,
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, sessionName), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	doc, err := BuildAffordances(repo, cfg.Name, runDir, sessionName)
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	found := false
	for _, item := range doc.Items {
		if item.ID != "provider-runtime" {
			continue
		}
		summaryOK := strings.Contains(item.Summary, "Provider runtime and bootstrap facts for `session-1` (`claude-code`).")
		runtimeOK := false
		boundaryOK := false
		rootGuardOK := false
		pathOK := false
		for _, fact := range item.Facts {
			if strings.Contains(fact, "GoalX canonical provider runtime is tmux + interactive TUI.") {
				runtimeOK = true
			}
			if strings.Contains(fact, "GoalX provider runtime does not change durable ownership boundaries.") {
				boundaryOK = true
			}
			if strings.Contains(fact, "Claude root sessions cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions.") {
				rootGuardOK = true
			}
		}
		for _, path := range item.Paths {
			if path == ContextIndexPath(runDir) {
				pathOK = true
				break
			}
		}
		found = summaryOK && runtimeOK && boundaryOK && rootGuardOK && pathOK
	}
	if !found {
		t.Fatalf("provider facts affordance missing for claude target: %+v", doc.Items)
	}
}

func TestBuildAffordancesIncludesSelectionFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	writeSelectionSnapshotFixture(t, runDir, testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			DisabledEngines:    []string{"aider"},
			MasterCandidates:   []string{"codex/gpt-5.4", "claude-code/opus"},
			ResearchCandidates: []string{"claude-code/opus"},
			DevelopCandidates:  []string{"codex/gpt-5.4-mini"},
		},
		Master:   goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
		Research: goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
		Develop:  goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini", Effort: goalx.EffortMedium},
	})

	doc, err := BuildAffordances(repo, cfg.Name, runDir, "master")
	if err != nil {
		t.Fatalf("BuildAffordances: %v", err)
	}

	found := false
	for _, item := range doc.Items {
		if item.ID != "selection-facts" {
			continue
		}
		found = strings.Contains(item.Summary, "Selection candidate pools and disabled targets") &&
			strings.Contains(strings.Join(item.Facts, "\n"), "Master candidates: `codex/gpt-5.4, claude-code/opus`") &&
			strings.Contains(strings.Join(item.Facts, "\n"), "Disabled engines: `aider`") &&
			len(item.Paths) == 1 && item.Paths[0] == SelectionSnapshotPath(runDir)
	}
	if !found {
		t.Fatalf("selection facts affordance missing: %+v", doc.Items)
	}
}
