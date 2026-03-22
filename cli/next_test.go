package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
