package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOperatorSurfaceConsistency(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	repoRoot := filepath.Dir(wd)
	files := []string{
		filepath.Join(repoRoot, "README.md"),
		filepath.Join(repoRoot, "skill", "SKILL.md"),
		filepath.Join(repoRoot, "skill", "agents", "openai.yaml"),
		filepath.Join(repoRoot, "skill", "references", "advanced-control.md"),
		filepath.Join(repoRoot, "deploy", "README.md"),
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", path, err)
		}
		text := string(data)
		isDeployReadme := path == filepath.Join(repoRoot, "deploy", "README.md")
		switch filepath.Base(path) {
		case "README.md":
			if !isDeployReadme && strings.Contains(text, "--guided") {
				t.Fatalf("%s should omit removed guided guidance", path)
			}
			if strings.Contains(path, filepath.Join("deploy", "README.md")) {
				if !strings.Contains(text, "goalx schema") {
					t.Fatalf("%s missing goalx schema guidance", path)
				}
			} else if !strings.Contains(text, "goalx schema") {
				t.Fatalf("%s missing goalx schema guidance", path)
			}
			if !isDeployReadme && !strings.Contains(text, "goalx budget") {
				t.Fatalf("%s missing budget control guidance", path)
			}
			if !isDeployReadme && !strings.Contains(text, "--context README.md,docs/architecture") {
				t.Fatalf("%s missing canonical comma-delimited context example", path)
			}
		case "SKILL.md":
			if strings.Contains(text, "--guided") || !strings.Contains(text, "goalx schema") {
				t.Fatalf("%s should omit guided and keep schema guidance", path)
			}
			if !strings.Contains(text, "goalx budget") {
				t.Fatalf("%s missing budget guidance", path)
			}
			if !strings.Contains(text, "goalx context") || !strings.Contains(text, "goalx afford") {
				t.Fatalf("%s missing context/afford guidance", path)
			}
			if strings.Contains(text, "repeat --context") {
				t.Fatalf("%s should not teach repeated --context flags", path)
			}
		case "openai.yaml":
			if !strings.Contains(text, "goalx budget") {
				t.Fatalf("%s missing budget guidance", path)
			}
			if !strings.Contains(text, "goalx context") || !strings.Contains(text, "goalx afford") {
				t.Fatalf("%s missing context/afford guidance", path)
			}
			if strings.Contains(text, "--guided") {
				t.Fatalf("%s should omit removed guided guidance", path)
			}
		case "advanced-control.md":
			if strings.Contains(text, "--guided") {
				t.Fatalf("%s should omit removed guided guidance", path)
			}
			if !strings.Contains(text, "goalx budget") {
				t.Fatalf("%s missing budget guidance", path)
			}
			if strings.Contains(text, "repeat --context") {
				t.Fatalf("%s should not teach repeated --context flags", path)
			}
			if strings.Contains(text, "sidecar") {
				t.Fatalf("%s should not mention removed sidecar control plane", path)
			}
			for _, want := range []string{"goalx list", "goalx afford", "goalx attach", "goalx review", "goalx diff", "goalx archive", "goalx wait"} {
				if !strings.Contains(text, want) {
					t.Fatalf("%s missing public command %q", path, want)
				}
			}
		}
	}
}
