package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestReviewUsesConfiguredResearchTargetFile(t *testing.T) {
	projectRoot, wtPath := seedReviewResearchRun(t, []string{"notes.md"})

	want := "custom report body"
	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Review(projectRoot, []string{"--run", "demo"}); err != nil {
			t.Fatalf("Review: %v", err)
		}
	})

	if !strings.Contains(out, want) {
		t.Fatalf("review output missing configured target file contents:\n%s", out)
	}
}

func TestReviewFallsBackToReportWhenTargetIsDot(t *testing.T) {
	projectRoot, wtPath := seedReviewResearchRun(t, []string{"."})

	want := "fallback report body"
	if err := os.WriteFile(filepath.Join(wtPath, "report.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write report.md: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Review(projectRoot, []string{"--run", "demo"}); err != nil {
			t.Fatalf("Review: %v", err)
		}
	})

	if !strings.Contains(out, want) {
		t.Fatalf("review output missing fallback report.md contents:\n%s", out)
	}
}

func TestReviewFallsBackToReportWhenConfiguredTargetMissing(t *testing.T) {
	projectRoot, wtPath := seedReviewResearchRun(t, []string{"missing.md"})

	want := "report fallback for missing target"
	if err := os.WriteFile(filepath.Join(wtPath, "report.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write report.md: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Review(projectRoot, []string{"--run", "demo"}); err != nil {
			t.Fatalf("Review: %v", err)
		}
	})

	if !strings.Contains(out, want) {
		t.Fatalf("review output missing fallback report.md contents:\n%s", out)
	}
}

func TestReviewFallsBackToUntrackedMarkdownReport(t *testing.T) {
	projectRoot, wtPath := seedReviewResearchRun(t, []string{"missing.md"})

	runReviewGit(t, wtPath, "init")
	runReviewGit(t, wtPath, "config", "user.name", "Test User")
	runReviewGit(t, wtPath, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(wtPath, "README.txt"), []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write README.txt: %v", err)
	}
	runReviewGit(t, wtPath, "add", "README.txt")
	runReviewGit(t, wtPath, "commit", "-m", "seed")

	want := "untracked markdown fallback"
	if err := os.WriteFile(filepath.Join(wtPath, "findings.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write findings.md: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Review(projectRoot, []string{"--run", "demo"}); err != nil {
			t.Fatalf("Review: %v", err)
		}
	})

	if !strings.Contains(out, want) {
		t.Fatalf("review output missing fallback untracked markdown contents:\n%s", out)
	}
}

func TestReviewUsesManifestDeclaredReportInMixedModeRun(t *testing.T) {
	projectRoot, wtPath := seedReviewResearchRun(t, []string{"missing.md"})
	runDir := goalx.RunDir(projectRoot, "demo")

	cfg, err := goalx.LoadYAML[goalx.Config](RunSpecPath(runDir))
	if err != nil {
		t.Fatalf("load run config: %v", err)
	}
	cfg.Mode = goalx.ModeWorker
	cfg.Target = goalx.TargetConfig{Files: []string{"src/"}}
	cfg.LocalValidation = goalx.LocalValidationConfig{Command: "go test ./..."}
	cfg.Sessions = []goalx.SessionConfig{
		{
			Hint:            "investigate",
			Mode:            goalx.ModeWorker,
			Target:          &goalx.TargetConfig{Files: []string{"missing.md"}, Readonly: []string{"."}},
			LocalValidation: &goalx.LocalValidationConfig{Command: "test -s missing.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run config: %v", err)
	}

	want := "manifest-only report body"
	reportPath := filepath.Join(wtPath, "analysis.txt")
	if err := os.WriteFile(reportPath, []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write analysis.txt: %v", err)
	}
	if err := RegisterSessionArtifact(runDir, "session-1", ArtifactMeta{
		Kind:        "report",
		Path:        reportPath,
		RelPath:     "analysis.txt",
		DurableName: "session-1-report.md",
	}); err != nil {
		t.Fatalf("RegisterSessionArtifact: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Review(projectRoot, []string{"--run", "demo"}); err != nil {
			t.Fatalf("Review: %v", err)
		}
	})

	if !strings.Contains(out, want) {
		t.Fatalf("review output missing manifest report contents:\n%s", out)
	}
}

func seedReviewResearchRun(t *testing.T, targetFiles []string) (string, string) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	for _, dir := range []string{
		filepath.Join(runDir, "journals"),
		wtPath,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target: goalx.TargetConfig{
			Files: targetFiles,
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "codex", goalx.TargetConfig{Files: targetFiles}, goalx.LocalValidationConfig{})
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session journal: %v", err)
	}

	return projectRoot, wtPath
}

func runReviewGit(t *testing.T, repo string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, string(out))
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return buf.String()
}
