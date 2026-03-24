package cli

import (
	"os"
	"strings"
	"testing"
)

func TestEnsureRunMetadataCreatesProtocolV2Identity(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")

	runDir := t.TempDir()
	meta, err := EnsureRunMetadata(runDir, repo, "ship feature")
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if meta.ProtocolVersion != currentProtocolVersion {
		t.Fatalf("protocol version = %d, want %d", meta.ProtocolVersion, currentProtocolVersion)
	}
	if meta.ProtocolVersion != 2 {
		t.Fatalf("protocol version = %d, want 2", meta.ProtocolVersion)
	}
	if meta.RunID == "" {
		t.Fatalf("run id empty")
	}
	if meta.Epoch != 1 {
		t.Fatalf("epoch = %d, want 1", meta.Epoch)
	}
	if meta.RootRunID != meta.RunID {
		t.Fatalf("root run id = %q, want %q", meta.RootRunID, meta.RunID)
	}

	reloaded, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if reloaded == nil {
		t.Fatal("reloaded metadata is nil")
	}
	if reloaded.RunID != meta.RunID {
		t.Fatalf("reloaded run id = %q, want %q", reloaded.RunID, meta.RunID)
	}
	if reloaded.Epoch != meta.Epoch {
		t.Fatalf("reloaded epoch = %d, want %d", reloaded.Epoch, meta.Epoch)
	}
}

func TestSaveRunMetadataPreservesCharterFields(t *testing.T) {
	runDir := t.TempDir()
	meta := &RunMetadata{
		Version:         1,
		Objective:       "ship feature",
		ProjectRoot:     "/tmp/project",
		ProtocolVersion: 2,
		RunID:           "run_abc",
		RootRunID:       "run_root",
		Epoch:           3,
		BaseRevision:    "base",
		PhaseKind:       "research",
		CharterID:       "charter_abc",
		CharterHash:     "sha256:abc",
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	reloaded, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if reloaded.RootRunID != meta.RootRunID {
		t.Fatalf("RootRunID = %q, want %q", reloaded.RootRunID, meta.RootRunID)
	}
	if reloaded.CharterID != meta.CharterID {
		t.Fatalf("CharterID = %q, want %q", reloaded.CharterID, meta.CharterID)
	}
	if reloaded.CharterHash != meta.CharterHash {
		t.Fatalf("CharterHash = %q, want %q", reloaded.CharterHash, meta.CharterHash)
	}
}

func TestLoadRunMetadataLeavesLegacyProtocolUntouched(t *testing.T) {
	runDir := t.TempDir()
	path := RunMetadataPath(runDir)
	before := []byte("{\n  \"version\": 1,\n  \"protocol_version\": 1,\n  \"objective\": \"legacy\"\n}\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}

	meta, err := LoadRunMetadata(path)
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if meta == nil {
		t.Fatal("legacy metadata is nil")
	}
	if meta.ProtocolVersion != 1 {
		t.Fatalf("protocol version = %d, want 1", meta.ProtocolVersion)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read run metadata: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("legacy metadata mutated:\n%s", string(after))
	}
}

func TestEnsureRunMetadataDoesNotImplicitlyUpgradeLegacyProtocol(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")

	runDir := t.TempDir()
	path := RunMetadataPath(runDir)
	before := []byte("{\n  \"version\": 1,\n  \"protocol_version\": 1,\n  \"objective\": \"legacy\",\n  \"project_root\": \"" + repo + "\",\n  \"base_revision\": \"" + strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")) + "\"\n}\n")
	if err := os.WriteFile(path, before, 0o644); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}

	meta, err := EnsureRunMetadata(runDir, repo, "legacy")
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if meta.ProtocolVersion != 1 {
		t.Fatalf("protocol version = %d, want 1", meta.ProtocolVersion)
	}
	if meta.RunID != "" {
		t.Fatalf("run id = %q, want empty", meta.RunID)
	}
	if meta.Epoch != 0 {
		t.Fatalf("epoch = %d, want 0", meta.Epoch)
	}
}
