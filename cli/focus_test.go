package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestFocusSetsFocusedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}

	runName := "beta"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), []byte("name: beta\nmode: develop\nobjective: keep moving\n"), 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}

	reg := &ProjectRegistry{
		Version: 1,
		ActiveRuns: map[string]ProjectRunRef{
			"alpha": {Name: "alpha", State: "active"},
			"beta":  {Name: "beta", State: "active"},
		},
	}
	if err := SaveProjectRegistry(projectRoot, reg); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Focus(projectRoot, []string{"--run", runName}); err != nil {
			t.Fatalf("Focus: %v", err)
		}
	})
	if !strings.Contains(out, "Focused run set to beta") {
		t.Fatalf("focus output = %q", out)
	}

	reg2, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if reg2.FocusedRun != runName {
		t.Fatalf("focused run = %q, want %q", reg2.FocusedRun, runName)
	}

	got, err := ResolveDefaultRunName(projectRoot)
	if err != nil {
		t.Fatalf("ResolveDefaultRunName: %v", err)
	}
	if got != runName {
		t.Fatalf("ResolveDefaultRunName = %q, want %q", got, runName)
	}
}

func TestFocusHelpPrintsUsageWithoutMutatingRegistry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(projectRoot, 0o755); err != nil {
		t.Fatalf("mkdir project root: %v", err)
	}
	if err := SaveProjectRegistry(projectRoot, &ProjectRegistry{
		Version:    1,
		FocusedRun: "alpha",
		ActiveRuns: map[string]ProjectRunRef{
			"alpha": {Name: "alpha", State: "active"},
		},
	}); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Focus(projectRoot, []string{"--help"}); err != nil {
			t.Fatalf("Focus --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx focus --run NAME") {
		t.Fatalf("focus help output = %q", out)
	}

	reg, err := LoadProjectRegistry(projectRoot)
	if err != nil {
		t.Fatalf("load registry: %v", err)
	}
	if reg.FocusedRun != "alpha" {
		t.Fatalf("focused run changed unexpectedly: %#v", reg)
	}
}
