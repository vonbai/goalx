package cli

import (
	"fmt"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestRunStartsDeliverPathByDefault(t *testing.T) {
	oldAuto := runAutoWithOptions
	defer func() { runAutoWithOptions = oldAuto }()

	calls := 0
	runAutoWithOptions = func(projectRoot string, opts launchOptions) error {
		calls++
		if projectRoot == "" {
			t.Fatal("projectRoot should not be empty")
		}
		if opts.Objective != "ship it" {
			t.Fatalf("objective = %q, want ship it", opts.Objective)
		}
		if opts.Mode != goalx.ModeAuto {
			t.Fatalf("mode = %q, want %q", opts.Mode, goalx.ModeAuto)
		}
		return nil
	}

	out := captureStdout(t, func() {
		if err := Run(t.TempDir(), []string{"ship it"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if calls != 1 {
		t.Fatalf("auto calls = %d, want 1", calls)
	}
	for _, want := range []string{"Run started.", "goalx status", "goalx observe", "goalx attach"} {
		if !strings.Contains(out, want) {
			t.Fatalf("run output missing %q:\n%s", want, out)
		}
	}
}

func TestRunIntentEvolveUsesAutoLaunchMode(t *testing.T) {
	oldAuto := runAutoWithOptions
	defer func() { runAutoWithOptions = oldAuto }()

	calls := 0
	runAutoWithOptions = func(projectRoot string, opts launchOptions) error {
		calls++
		if projectRoot == "" {
			t.Fatal("projectRoot should not be empty")
		}
		if opts.Objective != "ship auth" {
			t.Fatalf("objective = %q, want ship auth", opts.Objective)
		}
		if opts.Mode != goalx.ModeAuto {
			t.Fatalf("mode = %q, want %q", opts.Mode, goalx.ModeAuto)
		}
		if opts.Intent != runIntentEvolve {
			t.Fatalf("intent = %q, want %q", opts.Intent, runIntentEvolve)
		}
		return nil
	}

	out := captureStdout(t, func() {
		if err := Run(t.TempDir(), []string{"ship auth", "--intent", "evolve"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if calls != 1 {
		t.Fatalf("auto calls = %d, want 1", calls)
	}
	if !strings.Contains(out, "Run started.") {
		t.Fatalf("run output missing start summary:\n%s", out)
	}
}

func TestRunIntentExploreUsesAutoLaunchModeWithoutFrom(t *testing.T) {
	oldAuto := runAutoWithOptions
	oldExplore := runExploreIntent
	defer func() {
		runAutoWithOptions = oldAuto
		runExploreIntent = oldExplore
	}()

	autoCalls := 0
	phaseCalls := 0
	runAutoWithOptions = func(projectRoot string, opts launchOptions) error {
		autoCalls++
		if projectRoot == "" {
			t.Fatal("projectRoot should not be empty")
		}
		if opts.Objective != "audit auth boundary" {
			t.Fatalf("objective = %q, want audit auth boundary", opts.Objective)
		}
		if opts.Mode != goalx.ModeAuto {
			t.Fatalf("mode = %q, want %q", opts.Mode, goalx.ModeAuto)
		}
		if opts.Intent != runIntentExplore {
			t.Fatalf("intent = %q, want %q", opts.Intent, runIntentExplore)
		}
		if !opts.Readonly {
			t.Fatal("readonly = false, want true")
		}
		return nil
	}
	runExploreIntent = func(projectRoot string, args []string) error {
		phaseCalls++
		return nil
	}

	out := captureStdout(t, func() {
		if err := Run(t.TempDir(), []string{"audit auth boundary", "--intent", "explore", "--readonly"}); err != nil {
			t.Fatalf("Run: %v", err)
		}
	})

	if autoCalls != 1 {
		t.Fatalf("auto calls = %d, want 1", autoCalls)
	}
	if phaseCalls != 0 {
		t.Fatalf("phase calls = %d, want 0", phaseCalls)
	}
	if !strings.Contains(out, "Run started.") {
		t.Fatalf("run output missing start summary:\n%s", out)
	}
}

func TestRunRejectsRemovedGuidedFlag(t *testing.T) {
	oldAuto := runAutoWithOptions
	defer func() { runAutoWithOptions = oldAuto }()

	calls := 0
	runAutoWithOptions = func(projectRoot string, opts launchOptions) error {
		calls++
		return nil
	}

	err := Run(t.TempDir(), []string{"ship it", "--guided"})
	if err == nil {
		t.Fatal("Run unexpectedly succeeded with removed --guided flag")
	}
	if !strings.Contains(err.Error(), `unknown flag "--guided"`) {
		t.Fatalf("Run error = %v, want unknown flag --guided", err)
	}
	if calls != 0 {
		t.Fatalf("auto calls = %d, want 0", calls)
	}
}

func TestRunIntentDebateUsesPhasePath(t *testing.T) {
	oldDebate := runDebateIntent
	defer func() { runDebateIntent = oldDebate }()

	calls := 0
	runDebateIntent = func(projectRoot string, args []string) error {
		calls++
		if projectRoot == "" {
			t.Fatal("projectRoot should not be empty")
		}
		want := []string{"--from", "research-a", "--write-config"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("args = %v, want %v", args, want)
			}
		}
		return nil
	}

	if err := Run(t.TempDir(), []string{"--from", "research-a", "--intent", "debate", "--write-config"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("debate calls = %d, want 1", calls)
	}
}

func TestRunIntentExploreUsesPhasePathWithFrom(t *testing.T) {
	oldExplore := runExploreIntent
	oldAuto := runAutoWithOptions
	defer func() {
		runExploreIntent = oldExplore
		runAutoWithOptions = oldAuto
	}()

	phaseCalls := 0
	autoCalls := 0
	runExploreIntent = func(projectRoot string, args []string) error {
		phaseCalls++
		want := []string{"--from", "audit-a", "--write-config"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("args = %v, want %v", args, want)
			}
		}
		return nil
	}
	runAutoWithOptions = func(projectRoot string, opts launchOptions) error {
		autoCalls++
		return nil
	}

	if err := Run(t.TempDir(), []string{"--from", "audit-a", "--intent", "explore", "--write-config"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if phaseCalls != 1 {
		t.Fatalf("phase calls = %d, want 1", phaseCalls)
	}
	if autoCalls != 0 {
		t.Fatalf("auto calls = %d, want 0", autoCalls)
	}
}

func TestRunIntentDebateRequiresSavedRun(t *testing.T) {
	err := Run(t.TempDir(), []string{"audit auth boundary", "--intent", "debate"})
	if err == nil || !strings.Contains(err.Error(), "--intent debate requires --from RUN") {
		t.Fatalf("Run error = %v, want requires --from RUN", err)
	}
}

func TestRunIntentImplementRequiresSavedRun(t *testing.T) {
	err := Run(t.TempDir(), []string{"ship auth boundary", "--intent", "implement"})
	if err == nil || !strings.Contains(err.Error(), "--intent implement requires --from RUN") {
		t.Fatalf("Run error = %v, want requires --from RUN", err)
	}
}

func TestRunRejectsUnknownIntent(t *testing.T) {
	err := Run(t.TempDir(), []string{"ship it", "--intent", "mystery"})
	if err == nil || !strings.Contains(err.Error(), `unknown --intent "mystery"`) {
		t.Fatalf("Run error = %v, want unknown intent", err)
	}
}

func TestRunRejectsRemovedResearchAndDevelopIntents(t *testing.T) {
	for _, intent := range []string{"research", "develop"} {
		err := Run(t.TempDir(), []string{"ship it", "--intent", intent})
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf(`unknown --intent %q`, intent)) {
			t.Fatalf("Run(%q) error = %v, want unknown intent", intent, err)
		}
	}
}

func TestRunHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Run(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Run --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx run") {
		t.Fatalf("run help missing usage:\n%s", out)
	}
	for _, unwanted := range []string{"research", "develop"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("run help should omit removed intent %q:\n%s", unwanted, out)
		}
	}
	for _, want := range []string{
		`goalx run "objective" [--intent deliver|evolve|explore] [--readonly] [flags]`,
		`goalx run --from RUN --intent debate|implement|explore [--readonly] [flags]`,
		`--readonly`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("run help missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--guided") {
		t.Fatalf("run help should omit removed --guided flag:\n%s", out)
	}
	if strings.Contains(out, "legacy command names remain temporary aliases") {
		t.Fatalf("run help should not mention legacy aliases:\n%s", out)
	}
}

func TestDebateRoutesThroughRunEntrypoint(t *testing.T) {
	oldRun := runEntrypoint
	defer func() { runEntrypoint = oldRun }()

	runEntrypoint = func(_ string, args []string) error {
		want := []string{"--intent", runIntentDebate, "--from", "research-a"}
		if len(args) != len(want) {
			t.Fatalf("args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("args = %v, want %v", args, want)
			}
		}
		return nil
	}

	if err := Debate(t.TempDir(), []string{"--from", "research-a"}); err != nil {
		t.Fatalf("Debate: %v", err)
	}
}
