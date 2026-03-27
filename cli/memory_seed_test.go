package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestAppendMemorySeedFromVerifyResult(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, _, _ := writeReadOnlyRunFixture(t, repo)
	if err := os.WriteFile(AcceptanceEvidencePath(runDir), []byte("gate ok\n"), 0o644); err != nil {
		t.Fatalf("write acceptance evidence: %v", err)
	}
	exitCode := 0
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:          1,
		EffectiveCommand: "printf 'gate ok\\n'",
		LastResult: AcceptanceResult{
			CheckedAt:    "2026-03-27T08:00:00Z",
			Command:      "printf 'gate ok\\n'",
			ExitCode:     &exitCode,
			EvidencePath: AcceptanceEvidencePath(runDir),
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}

	if err := AppendMemorySeedFromVerifyResult(runDir); err != nil {
		t.Fatalf("AppendMemorySeedFromVerifyResult: %v", err)
	}

	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		t.Fatalf("LoadMemorySeeds: %v", err)
	}
	if len(seeds) != 1 {
		t.Fatalf("memory seeds len = %d, want 1", len(seeds))
	}
	seed := seeds[0]
	if seed.Kind != "verify_result" {
		t.Fatalf("seed kind = %q, want verify_result", seed.Kind)
	}
	if seed.Run != runName {
		t.Fatalf("seed run = %q, want %q", seed.Run, runName)
	}
	if seed.CreatedAt != "2026-03-27T08:00:00Z" {
		t.Fatalf("seed created_at = %q, want verify timestamp", seed.CreatedAt)
	}
	if !strings.Contains(seed.Message, "exit_code=0") {
		t.Fatalf("seed message = %q, want exit code detail", seed.Message)
	}
	for _, banned := range []string{"important", "best practice", "recommended"} {
		if strings.Contains(strings.ToLower(seed.Message), banned) {
			t.Fatalf("seed message should stay factual, found %q in %q", banned, seed.Message)
		}
	}
	if len(seed.Evidence) != 2 {
		t.Fatalf("seed evidence len = %d, want 2", len(seed.Evidence))
	}
}

func TestCollectRunMemorySeedsIncludesSavedArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.MkdirAll(ReportsDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir reports dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeResearch,
		Objective: "inspect",
		Target:    goalx.TargetConfig{Files: []string{"notes.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "", cfg.Target, goalx.LocalValidationConfig{})
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ReportsDir(runDir), "repo-summary.md"), []byte("repo report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(AcceptanceEvidencePath(runDir), []byte("gate ok\n"), 0o644); err != nil {
		t.Fatalf("write acceptance evidence: %v", err)
	}
	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	seeds, err := CollectRunMemorySeeds(runDir)
	if err != nil {
		t.Fatalf("CollectRunMemorySeeds: %v", err)
	}
	savedPrefix := SavedRunDir(projectRoot, runName)
	foundSavedArtifact := false
	for _, seed := range seeds {
		for _, evidence := range seed.Evidence {
			if strings.HasPrefix(evidence.Path, savedPrefix) {
				foundSavedArtifact = true
			}
		}
	}
	if !foundSavedArtifact {
		t.Fatalf("expected saved artifact seed evidence under %s, got %+v", savedPrefix, seeds)
	}
}

func TestSidecarRefreshesMemorySeedsWithoutCanonicalMutation(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	payloads := writeCanonicalMemorySentinels(t, home)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Please authenticate in browser to continue\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		t.Fatalf("LoadMemorySeeds: %v", err)
	}
	foundProviderDialog := false
	for _, seed := range seeds {
		if seed.Kind == "provider_dialog_visible" && strings.Contains(seed.Message, "target=session-1") {
			foundProviderDialog = true
		}
	}
	if !foundProviderDialog {
		t.Fatalf("memory seeds missing provider dialog incident: %+v", seeds)
	}
	for path, want := range payloads {
		assertFileUnchanged(t, path, want)
	}
}
