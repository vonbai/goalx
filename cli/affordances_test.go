package cli

import (
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
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("affordance commands missing %q:\n%s", want, joined)
		}
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

func TestBuildAffordancesIncludesProviderFactsForClaudeTargets(t *testing.T) {
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
		if item.ID != "provider-facts" {
			continue
		}
		summaryOK := strings.Contains(item.Summary, "Provider-native capability facts for `session-1` (`claude-code`).")
		runtimeOK := false
		claudeCapabilityOK := false
		rootGuardOK := false
		pathOK := false
		for _, fact := range item.Facts {
			if strings.Contains(fact, "GoalX canonical provider runtime is tmux + interactive TUI.") {
				runtimeOK = true
			}
			if strings.Contains(fact, "Interactive Claude sessions can use installed skills, plugins, and MCP servers from the native TUI.") {
				claudeCapabilityOK = true
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
		found = summaryOK && runtimeOK && claudeCapabilityOK && rootGuardOK && pathOK
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
