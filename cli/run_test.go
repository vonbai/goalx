package cli

import (
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
		if err := Run(t.TempDir(), []string{"ship it"}, nil); err != nil {
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

func TestRunIntentResearchUsesResearchLaunchMode(t *testing.T) {
	oldLaunch := runLaunchWithOptions
	defer func() { runLaunchWithOptions = oldLaunch }()

	calls := 0
	runLaunchWithOptions = func(projectRoot string, opts launchOptions) error {
		calls++
		if projectRoot == "" {
			t.Fatal("projectRoot should not be empty")
		}
		if opts.Objective != "audit auth" {
			t.Fatalf("objective = %q, want audit auth", opts.Objective)
		}
		if opts.Mode != goalx.ModeResearch {
			t.Fatalf("mode = %q, want %q", opts.Mode, goalx.ModeResearch)
		}
		return nil
	}

	if err := Run(t.TempDir(), []string{"audit auth", "--intent", "research"}, nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("launch calls = %d, want 1", calls)
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
		if err := Run(t.TempDir(), []string{"ship auth", "--intent", "evolve"}, nil); err != nil {
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

func TestRunIntentDebateUsesPhasePath(t *testing.T) {
	oldDebate := runDebateWithNextConfig
	defer func() { runDebateWithNextConfig = oldDebate }()

	calls := 0
	expectedNC := &nextConfigJSON{Parallel: 3}
	runDebateWithNextConfig = func(projectRoot string, args []string, nc *nextConfigJSON) error {
		calls++
		if projectRoot == "" {
			t.Fatal("projectRoot should not be empty")
		}
		if nc != expectedNC {
			t.Fatalf("next config = %#v, want %#v", nc, expectedNC)
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

	if err := Run(t.TempDir(), []string{"--from", "research-a", "--intent", "debate", "--write-config"}, expectedNC); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls != 1 {
		t.Fatalf("debate calls = %d, want 1", calls)
	}
}

func TestRunRejectsUnknownIntent(t *testing.T) {
	err := Run(t.TempDir(), []string{"ship it", "--intent", "mystery"}, nil)
	if err == nil || !strings.Contains(err.Error(), `unknown --intent "mystery"`) {
		t.Fatalf("Run error = %v, want unknown intent", err)
	}
}

func TestRunHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Run(t.TempDir(), []string{"--help"}, nil); err != nil {
			t.Fatalf("Run --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx run") {
		t.Fatalf("run help missing usage:\n%s", out)
	}
	if strings.Contains(out, "legacy command names remain temporary aliases") {
		t.Fatalf("run help should not mention legacy aliases:\n%s", out)
	}
}

func TestDebateRoutesThroughRunEntrypoint(t *testing.T) {
	oldRun := runEntrypoint
	defer func() { runEntrypoint = oldRun }()

	expectedNC := &nextConfigJSON{Context: []string{"README.md"}}
	runEntrypoint = func(_ string, args []string, nc *nextConfigJSON) error {
		if nc != expectedNC {
			t.Fatalf("next config = %#v, want %#v", nc, expectedNC)
		}
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

	if err := Debate(t.TempDir(), []string{"--from", "research-a"}, expectedNC); err != nil {
		t.Fatalf("Debate: %v", err)
	}
}
