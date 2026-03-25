package cli

import (
	"strings"
	"testing"
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
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("affordance commands missing %q:\n%s", want, joined)
		}
	}
}
