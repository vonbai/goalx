package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestAutoStartsRunWithoutInjectingMode(t *testing.T) {
	oldStart := autoStart
	defer func() {
		autoStart = oldStart
	}()

	startCalls := 0
	autoStart = func(_ string, args []string) error {
		startCalls++
		want := []string{"ship it"}
		if len(args) != len(want) {
			t.Fatalf("start args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("start args = %v, want %v", args, want)
			}
		}
		return nil
	}

	out := captureStdout(t, func() {
		if err := Auto(t.TempDir(), []string{"ship it"}); err != nil {
			t.Fatalf("Auto: %v", err)
		}
	})

	if startCalls != 1 {
		t.Fatalf("start calls = %d, want 1", startCalls)
	}
	for _, want := range []string{
		"Run started.",
		"goalx status",
		"goalx observe",
		"goalx attach",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("auto output missing %q:\n%s", want, out)
		}
	}
}

func TestAutoPreservesExplicitMode(t *testing.T) {
	oldStart := autoStart
	defer func() {
		autoStart = oldStart
	}()

	autoStart = func(_ string, args []string) error {
		want := []string{"ship it", "--develop"}
		if len(args) != len(want) {
			t.Fatalf("start args = %v, want %v", args, want)
		}
		for i := range want {
			if args[i] != want[i] {
				t.Fatalf("start args = %v, want %v", args, want)
			}
		}
		return nil
	}

	if err := Auto(t.TempDir(), []string{"ship it", "--develop"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
}

func TestAutoReturnsInitError(t *testing.T) {
	oldStart := autoStart
	defer func() {
		autoStart = oldStart
	}()

	autoStart = func(string, []string) error { return errors.New("boom") }

	err := Auto(t.TempDir(), []string{"ship it"})
	if err == nil || !strings.Contains(err.Error(), "start: boom") {
		t.Fatalf("Auto error = %v, want start failure", err)
	}
}

func TestAutoReturnsStartError(t *testing.T) {
	oldStart := autoStart
	defer func() {
		autoStart = oldStart
	}()

	autoStart = func(string, []string) error { return errors.New("boom") }

	err := Auto(t.TempDir(), []string{"ship it"})
	if err == nil || !strings.Contains(err.Error(), "start: boom") {
		t.Fatalf("Auto error = %v, want start failure", err)
	}
}

func TestAutoHelpPrintsUsage(t *testing.T) {
	oldStart := autoStart
	defer func() {
		autoStart = oldStart
	}()

	startCalls := 0
	autoStart = func(string, []string) error {
		startCalls++
		return nil
	}

	out := captureStdout(t, func() {
		if err := Auto(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Auto --help: %v", err)
		}
	})

	if startCalls != 0 {
		t.Fatalf("start calls = %d, want 0", startCalls)
	}
	if !strings.Contains(out, "usage: goalx auto") {
		t.Fatalf("auto help missing usage:\n%s", out)
	}
	if !strings.Contains(out, "master decides mode") {
		t.Fatalf("auto help missing auto-mode guidance:\n%s", out)
	}
}

func TestResearchHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Research(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Research --help: %v", err)
		}
	})

	if !strings.Contains(out, "usage: goalx research") {
		t.Fatalf("research help missing usage:\n%s", out)
	}
	if !strings.Contains(out, "--research-role") {
		t.Fatalf("research help missing role defaults:\n%s", out)
	}
}

func TestValidateNextConfigRejectsInvalidFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Parallel:       99,
		Engine:         "unknown-engine",
		BudgetSeconds:  -1,
		DiversityHints: []string{"a", "b"},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Parallel != 10 {
		t.Fatalf("parallel = %d, want 10", got.Parallel)
	}
	if got.Engine != "" {
		t.Fatalf("engine = %q, want empty", got.Engine)
	}
	if got.BudgetSeconds != 0 {
		t.Fatalf("budget_seconds = %d, want 0", got.BudgetSeconds)
	}
}

func TestValidateNextConfigNormalizesExtendedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Mode:          " research ",
		MaxIterations: 7,
		Context:       []string{" docs/plan.md ", " ", "README.md"},
		MasterEngine:  " codex ",
		MasterModel:   " fast ",
		Sessions: []sessionConfigJSON{
			{Hint: " alpha ", Engine: " codex ", Model: " fast "},
			{Hint: " beta ", Engine: " unknown ", Model: " fast "},
		},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Mode != "research" {
		t.Fatalf("mode = %q, want research", got.Mode)
	}
	if got.MaxIterations != 7 {
		t.Fatalf("max_iterations = %d, want 7", got.MaxIterations)
	}
	if len(got.Context) != 2 || got.Context[0] != "docs/plan.md" || got.Context[1] != "README.md" {
		t.Fatalf("context = %#v, want trimmed non-empty paths", got.Context)
	}
	if got.MasterEngine != "codex" || got.MasterModel != "fast" {
		t.Fatalf("master engine/model = %q/%q, want codex/fast", got.MasterEngine, got.MasterModel)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 entries", got.Sessions)
	}
	if got.Sessions[0].Hint != "alpha" || got.Sessions[0].Engine != "codex" || got.Sessions[0].Model != "fast" {
		t.Fatalf("sessions[0] = %#v, want trimmed codex/fast entry", got.Sessions[0])
	}
	if got.Sessions[1].Hint != "beta" || got.Sessions[1].Engine != "" || got.Sessions[1].Model != "" {
		t.Fatalf("sessions[1] = %#v, want invalid engine/model cleared", got.Sessions[1])
	}
}

func TestValidateNextConfigRejectsInvalidExtendedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Mode:          "invalid",
		MaxIterations: 42,
		MasterEngine:  "unknown",
		MasterModel:   "fast",
		Sessions: []sessionConfigJSON{
			{Hint: "x", Engine: "codex", Model: "gpt-5.2"},
			{Hint: "y", Model: "fast"},
		},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Mode != "" {
		t.Fatalf("mode = %q, want empty", got.Mode)
	}
	if got.MaxIterations != 0 {
		t.Fatalf("max_iterations = %d, want 0", got.MaxIterations)
	}
	if got.MasterEngine != "" || got.MasterModel != "" {
		t.Fatalf("master engine/model = %q/%q, want empty", got.MasterEngine, got.MasterModel)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 entries", got.Sessions)
	}
	if got.Sessions[0].Model != "" {
		t.Fatalf("sessions[0].model = %q, want empty", got.Sessions[0].Model)
	}
	if got.Sessions[1].Model != "" {
		t.Fatalf("sessions[1].model = %q, want empty", got.Sessions[1].Model)
	}
}
