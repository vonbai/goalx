package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestMainSupportsResultCommand(t *testing.T) {
	pkgDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	binDir := t.TempDir()
	binPath := filepath.Join(binDir, "goalx-test")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = pkgDir
	build.Env = append(os.Environ(), "HOME="+t.TempDir())
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, string(out))
	}

	projectRoot := t.TempDir()
	runDir := filepath.Join(projectRoot, ".goalx", "runs", "demo-run")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name: "demo-run",
		Mode: goalx.ModeResearch,
		Target: goalx.TargetConfig{
			Files: []string{"report.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("# summary from result\n"), 0o644); err != nil {
		t.Fatalf("write summary.md: %v", err)
	}

	cmd := exec.Command(binPath, "result", "demo-run")
	cmd.Dir = projectRoot
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goalx result: %v\n%s", err, string(out))
	}

	if !strings.Contains(string(out), "# summary from result") {
		t.Fatalf("result output missing summary:\n%s", string(out))
	}
}
