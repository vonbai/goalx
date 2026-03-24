package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestEnsureArtifactsManifestCreatesEmptyRunManifest(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	manifest, err := EnsureArtifactsManifest(runDir)
	if err != nil {
		t.Fatalf("EnsureArtifactsManifest: %v", err)
	}
	if manifest.Run != "demo" {
		t.Fatalf("manifest run = %q, want demo", manifest.Run)
	}
	if len(manifest.Sessions) != 0 {
		t.Fatalf("manifest sessions = %d, want 0", len(manifest.Sessions))
	}

	data, err := os.ReadFile(ArtifactsPath(runDir))
	if err != nil {
		t.Fatalf("read artifacts manifest: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty artifacts manifest")
	}
}

func TestRegisterSessionArtifactUpsertsArtifact(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if _, err := EnsureArtifactsManifest(runDir); err != nil {
		t.Fatalf("EnsureArtifactsManifest: %v", err)
	}

	initial := ArtifactMeta{
		Kind:        "report",
		Path:        "/tmp/report-v1.md",
		RelPath:     "report.md",
		DurableName: "session-1-report.md",
	}
	if err := RegisterSessionArtifact(runDir, "session-1", initial); err != nil {
		t.Fatalf("RegisterSessionArtifact initial: %v", err)
	}

	updated := initial
	updated.Path = "/tmp/report-v2.md"
	if err := RegisterSessionArtifact(runDir, "session-1", updated); err != nil {
		t.Fatalf("RegisterSessionArtifact update: %v", err)
	}

	manifest, err := LoadArtifacts(ArtifactsPath(runDir))
	if err != nil {
		t.Fatalf("LoadArtifacts: %v", err)
	}
	if len(manifest.Sessions) != 1 {
		t.Fatalf("manifest sessions = %d, want 1", len(manifest.Sessions))
	}
	got := manifest.Sessions[0].Artifacts
	if len(got) != 1 {
		t.Fatalf("artifact count = %d, want 1", len(got))
	}
	if got[0].Path != updated.Path {
		t.Fatalf("artifact path = %q, want %q", got[0].Path, updated.Path)
	}
}

func TestCopyManifestToSavedRun(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	saveDir := filepath.Join(t.TempDir(), "saved")
	for _, dir := range []string{runDir, saveDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	if _, err := EnsureArtifactsManifest(runDir); err != nil {
		t.Fatalf("EnsureArtifactsManifest: %v", err)
	}
	if err := RegisterSessionArtifact(runDir, "session-1", ArtifactMeta{
		Kind:        "report",
		Path:        "/tmp/report.md",
		RelPath:     "report.md",
		DurableName: "session-1-report.md",
	}); err != nil {
		t.Fatalf("RegisterSessionArtifact: %v", err)
	}

	if err := CopyArtifactsManifest(runDir, saveDir); err != nil {
		t.Fatalf("CopyArtifactsManifest: %v", err)
	}

	manifest, err := LoadArtifacts(filepath.Join(saveDir, "artifacts.json"))
	if err != nil {
		t.Fatalf("LoadArtifacts saved: %v", err)
	}
	if len(manifest.Sessions) != 1 {
		t.Fatalf("saved manifest sessions = %d, want 1", len(manifest.Sessions))
	}
}

func TestBuildRunArtifactsManifestPrefersRunReportsDir(t *testing.T) {
	runName := "demo"
	runDir := filepath.Join(t.TempDir(), runName)
	reportsDir := filepath.Join(runDir, "reports")
	wtPath := WorktreePath(runDir, runName, 1)
	for _, dir := range []string{reportsDir, wtPath} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	seedSaveRunProvenance(t, t.TempDir(), runDir, runName, "collect reports")
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "", goalx.TargetConfig{Files: []string{"report.md"}}, goalx.HarnessConfig{})

	runReport := filepath.Join(reportsDir, "session-1-report.md")
	if err := os.WriteFile(runReport, []byte("run report\n"), 0o644); err != nil {
		t.Fatalf("write run report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "report.md"), []byte("legacy report\n"), 0o644); err != nil {
		t.Fatalf("write worktree report: %v", err)
	}

	manifest := buildRunArtifactsManifest(runDir, &goalx.Config{Name: runName})
	artifact := FindSessionArtifact(manifest, "session-1", "report")
	if artifact == nil {
		t.Fatalf("expected report artifact in manifest: %#v", manifest)
	}
	if artifact.Path != runReport {
		t.Fatalf("artifact path = %q, want %q", artifact.Path, runReport)
	}
	if artifact.DurableName != "session-1-report.md" {
		t.Fatalf("artifact durable name = %q, want session-1-report.md", artifact.DurableName)
	}
}

func TestBuildRunArtifactsManifestFallsBackToSessionWorktree(t *testing.T) {
	runName := "demo"
	runDir := filepath.Join(t.TempDir(), runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	seedSaveRunProvenance(t, t.TempDir(), runDir, runName, "collect reports")
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "", goalx.TargetConfig{Files: []string{"report.md"}}, goalx.HarnessConfig{})

	legacyReport := filepath.Join(wtPath, "report.md")
	if err := os.WriteFile(legacyReport, []byte("legacy report\n"), 0o644); err != nil {
		t.Fatalf("write worktree report: %v", err)
	}

	manifest := buildRunArtifactsManifest(runDir, &goalx.Config{Name: runName})
	artifact := FindSessionArtifact(manifest, "session-1", "report")
	if artifact == nil {
		t.Fatalf("expected report artifact in manifest: %#v", manifest)
	}
	if artifact.Path != legacyReport {
		t.Fatalf("artifact path = %q, want %q", artifact.Path, legacyReport)
	}
}
