package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestNextDefaultsToAutoFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Next(projectRoot, nil); err != nil {
			t.Fatalf("Next: %v", err)
		}
	})
	if !strings.Contains(out, "goalx auto \"your objective\"") {
		t.Fatalf("next output missing auto-first quickstart:\n%s", out)
	}
	if strings.Contains(out, "goalx init") || strings.Contains(out, "goalx start") {
		t.Fatalf("next output still promotes init/start:\n%s", out)
	}
}

func TestNextPromptsFocusForMultipleActiveRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	if err := SaveProjectRegistry(projectRoot, &ProjectRegistry{
		Version: 1,
		ActiveRuns: map[string]ProjectRunRef{
			"alpha": {Name: "alpha", State: "active"},
			"beta":  {Name: "beta", State: "active"},
		},
	}); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Next(projectRoot, nil); err != nil {
			t.Fatalf("Next: %v", err)
		}
	})
	if !strings.Contains(out, "goalx focus --run NAME") {
		t.Fatalf("next output missing focus guidance:\n%s", out)
	}
}

func TestNextSuggestsObserveForSingleDegradedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	runDir := goalx.RunDir(projectRoot, "alpha")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: alpha\nmode: develop\nobjective: ship alpha\n"), 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Next(projectRoot, nil); err != nil {
			t.Fatalf("Next: %v", err)
		}
	})
	if !strings.Contains(out, "Degraded run: alpha") {
		t.Fatalf("next output missing degraded run header:\n%s", out)
	}
	for _, want := range []string{"goalx observe --run alpha", "goalx status --run alpha"} {
		if !strings.Contains(out, want) {
			t.Fatalf("next output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "goalx attach --run alpha") {
		t.Fatalf("next output should not recommend attach for degraded run:\n%s", out)
	}
}

func TestNextUsesDerivedActiveRunWithoutRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	runDir := goalx.RunDir(projectRoot, "alpha")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: alpha\nmode: develop\nobjective: ship alpha\n"), 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := RenewControlLease(runDir, "sidecar", "run_alpha", 1, time.Minute, "process", 4242); err != nil {
		t.Fatalf("RenewControlLease: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Next(projectRoot, nil); err != nil {
			t.Fatalf("Next: %v", err)
		}
	})
	if !strings.Contains(out, "Active run: alpha") {
		t.Fatalf("next output missing derived active run:\n%s", out)
	}
}

func TestNextSuggestsExplicitPhaseFromSavedResearchRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(SavedRunDir(projectRoot, "research-a"), 0o755); err != nil {
		t.Fatalf("mkdir saved run: %v", err)
	}
	if err := os.WriteFile(filepath.Join(SavedRunDir(projectRoot, "research-a"), "run-spec.yaml"), []byte("name: research-a\nmode: research\nobjective: audit auth\n"), 0o644); err != nil {
		t.Fatalf("write run-spec: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Next(projectRoot, nil); err != nil {
			t.Fatalf("Next: %v", err)
		}
	})
	for _, want := range []string{
		"goalx debate --from research-a",
		"goalx implement --from research-a",
		"goalx explore --from research-a",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("next output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "goalx start") {
		t.Fatalf("next output should not suggest config-first start:\n%s", out)
	}
}

func TestNextFallsBackToLegacySavedRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	legacyRunDir := LegacySavedRunDir(projectRoot, "research-a")
	if err := os.MkdirAll(legacyRunDir, 0o755); err != nil {
		t.Fatalf("mkdir legacy saved run: %v", err)
	}
	if err := os.WriteFile(filepath.Join(legacyRunDir, "run-spec.yaml"), []byte("name: research-a\nmode: research\nobjective: audit auth\n"), 0o644); err != nil {
		t.Fatalf("write legacy run-spec: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Next(projectRoot, nil); err != nil {
			t.Fatalf("Next: %v", err)
		}
	})
	if !strings.Contains(out, "goalx debate --from research-a") {
		t.Fatalf("next output missing legacy saved run guidance:\n%s", out)
	}
}

func TestNextHelpPrintsUsage(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Next(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Next --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx next") {
		t.Fatalf("next help missing usage:\n%s", out)
	}
}
