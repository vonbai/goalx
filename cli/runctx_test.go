package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestResolveRunPrefersFocusedRun(t *testing.T) {
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

	if err := SaveProjectRegistry(projectRoot, &ProjectRegistry{
		Version:    1,
		FocusedRun: runName,
		ActiveRuns: map[string]ProjectRunRef{
			"alpha": {Name: "alpha", State: "active"},
			"beta":  {Name: "beta", State: "active"},
		},
	}); err != nil {
		t.Fatalf("save registry: %v", err)
	}

	rc, err := ResolveRun(projectRoot, "")
	if err != nil {
		t.Fatalf("ResolveRun: %v", err)
	}
	if rc.Name != runName {
		t.Fatalf("ResolveRun name = %q, want %q", rc.Name, runName)
	}
}
