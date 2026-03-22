package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestSaveUsesConfiguredResearchTargetFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeResearch,
		Objective: "inspect",
		Parallel:  1,
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}

	want := "saved custom report"
	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedPath := filepath.Join(projectRoot, ".goalx", "runs", runName, "session-1-report.md")
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != want+"\n" {
		t.Fatalf("saved report = %q, want %q", string(got), want+"\n")
	}
}

func TestSaveWritesArtifactsManifestForResearchSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeDevelop,
		Objective: "inspect",
		Sessions: []goalx.SessionConfig{
			{
				Hint:    "investigate",
				Mode:    goalx.ModeResearch,
				Target:  &goalx.TargetConfig{Files: []string{"missing.md"}, Readonly: []string{"."}},
				Harness: &goalx.HarnessConfig{Command: "test -s missing.md"},
			},
		},
		Target:  goalx.TargetConfig{Files: []string{"src/"}},
		Harness: goalx.HarnessConfig{Command: "go test ./..."},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}

	want := "saved manifest report"
	reportPath := filepath.Join(wtPath, "analysis.txt")
	if err := os.WriteFile(reportPath, []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write analysis.txt: %v", err)
	}
	if _, err := EnsureArtifactsManifest(runDir); err != nil {
		t.Fatalf("EnsureArtifactsManifest: %v", err)
	}
	if err := RegisterSessionArtifact(runDir, "session-1", ArtifactMeta{
		Kind:        "report",
		Path:        reportPath,
		RelPath:     "analysis.txt",
		DurableName: "session-1-report.md",
	}); err != nil {
		t.Fatalf("RegisterSessionArtifact: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedPath := filepath.Join(projectRoot, ".goalx", "runs", runName, "session-1-report.md")
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != want+"\n" {
		t.Fatalf("saved report = %q, want %q", string(got), want+"\n")
	}

	manifest, err := LoadArtifacts(filepath.Join(projectRoot, ".goalx", "runs", runName, "artifacts.json"))
	if err != nil {
		t.Fatalf("LoadArtifacts: %v", err)
	}
	if len(manifest.Sessions) != 1 || len(manifest.Sessions[0].Artifacts) != 1 {
		t.Fatalf("saved manifest = %#v, want one research artifact", manifest)
	}
}
