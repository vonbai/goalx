package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRefreshImpactStateWritesRepoNativeChangedFiles(t *testing.T) {
	prev := lookPathFunc
	defer func() { lookPathFunc = prev }()
	lookPathFunc = func(name string) (string, error) {
		switch name {
		case "git":
			return "/usr/bin/git", nil
		default:
			return "", exec.ErrNotFound
		}
	}

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base\n", "base")

	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := CreateWorktree(repo, runWT, "goalx/"+runName+"/root"); err != nil {
		t.Fatalf("CreateWorktree run root: %v", err)
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if meta == nil {
		t.Fatal("run metadata missing")
	}
	meta.BaseRevision = gitOutput(t, repo, "rev-parse", "HEAD")
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runWT, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("write README in run worktree: %v", err)
	}

	if err := RefreshImpactState(runDir, "run-root"); err != nil {
		t.Fatalf("RefreshImpactState: %v", err)
	}

	state, err := LoadImpactState(ImpactStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadImpactState: %v", err)
	}
	if state == nil {
		t.Fatal("impact state missing")
	}
	if state.ResolverKind != "repo-native" {
		t.Fatalf("resolver_kind = %q, want repo-native", state.ResolverKind)
	}
	if len(state.ChangedFiles) == 0 || state.ChangedFiles[0] != "README.md" {
		t.Fatalf("changed_files = %#v, want README.md", state.ChangedFiles)
	}
}
