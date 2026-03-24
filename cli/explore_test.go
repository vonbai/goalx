package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestExploreStartCreatesFreshCharterWithPreservedRootLineage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")
	sourceMeta, sourceCharter := writeSavedPhaseSourceFixture(t, projectRoot, "research-a", "research")
	installPhaseStartFakeTmux(t)
	stubLaunchRunSidecar(t)

	if err := Explore(projectRoot, []string{"--from", "research-a"}); err != nil {
		t.Fatalf("Explore: %v", err)
	}

	assertPhaseRunLineage(t, projectRoot, derivePhaseRunName("research-a", "explore", ""), "explore", "research-a", sourceMeta, sourceCharter)
}

func installPhaseStartFakeTmux(t *testing.T) {
	t.Helper()
	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 1
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func stubLaunchRunSidecar(t *testing.T) {
	t.Helper()
	origLaunchSidecar := launchRunSidecar
	launchRunSidecar = func(projectRoot, runName string, intervalDuration time.Duration) error {
		return nil
	}
	t.Cleanup(func() { launchRunSidecar = origLaunchSidecar })
}

func writeSavedPhaseSourceFixture(t *testing.T, projectRoot, runName, phaseKind string) (*RunMetadata, *RunCharter) {
	t.Helper()
	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "claude",
		Parallel:  2,
		Master: goalx.MasterConfig{
			Engine: "codex",
			Model:  "codex",
		},
	}
	writeSavedRunFixture(t, projectRoot, runName, cfg, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	runDir := SavedRunDir(projectRoot, runName)
	meta, err := EnsureRunMetadata(runDir, projectRoot, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	meta.RunID = "run_source_" + phaseKind
	meta.RootRunID = "run_root_lineage"
	meta.PhaseKind = phaseKind
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	charter, err := NewRunCharter(runDir, runName, meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	digest, err := hashRunCharter(charter)
	if err != nil {
		t.Fatalf("hashRunCharter: %v", err)
	}
	meta.CharterID = charter.CharterID
	meta.CharterHash = digest
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata charter linkage: %v", err)
	}
	return meta, charter
}

func assertPhaseRunLineage(t *testing.T, projectRoot, runName, phaseKind, sourceRun string, sourceMeta *RunMetadata, sourceCharter *RunCharter) {
	t.Helper()
	runDir := goalx.RunDir(projectRoot, runName)
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	charter, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	if err := ValidateRunCharterLinkage(meta, charter); err != nil {
		t.Fatalf("ValidateRunCharterLinkage: %v", err)
	}
	if meta.RootRunID != sourceMeta.RootRunID || charter.RootRunID != sourceMeta.RootRunID {
		t.Fatalf("phase root lineage metadata=%+v charter=%+v source=%+v", meta, charter, sourceMeta)
	}
	if charter.CharterID == sourceCharter.CharterID {
		t.Fatalf("phase charter should be fresh, got reused charter_id %q", charter.CharterID)
	}
	if meta.PhaseKind != phaseKind || charter.PhaseKind != phaseKind {
		t.Fatalf("phase kind metadata=%q charter=%q want %q", meta.PhaseKind, charter.PhaseKind, phaseKind)
	}
	if meta.SourceRun != sourceRun || charter.SourceRun != sourceRun || meta.ParentRun != sourceRun || charter.ParentRun != sourceRun {
		t.Fatalf("phase lineage metadata=%+v charter=%+v sourceRun=%q", meta, charter, sourceRun)
	}
	if meta.SourcePhase != sourceMeta.PhaseKind || charter.SourcePhase != sourceMeta.PhaseKind {
		t.Fatalf("phase source phase metadata=%q charter=%q want %q", meta.SourcePhase, charter.SourcePhase, sourceMeta.PhaseKind)
	}
}
