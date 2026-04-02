package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveImpactStateRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := ImpactStatePath(runDir)
	state := &ImpactState{
		Version:          1,
		Scope:            "run-root",
		BaselineRevision: "abc123",
		HeadRevision:     "def456",
		ResolverKind:     "repo-native",
		ChangedFiles:     []string{"cli/start.go"},
	}

	if err := SaveImpactState(path, state); err != nil {
		t.Fatalf("SaveImpactState: %v", err)
	}
	loaded, err := LoadImpactState(path)
	if err != nil {
		t.Fatalf("LoadImpactState: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadImpactState returned nil state")
	}
	if loaded.ResolverKind != "repo-native" {
		t.Fatalf("resolver_kind = %q, want repo-native", loaded.ResolverKind)
	}
}

func TestLoadImpactStateRejectsMissingResolverKind(t *testing.T) {
	path := ImpactStatePath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "scope": "run-root",
  "baseline_revision": "abc123",
  "head_revision": "def456",
  "changed_files": ["cli/start.go"]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadImpactState(path)
	if err == nil {
		t.Fatal("LoadImpactState should reject missing resolver_kind")
	}
	if !strings.Contains(err.Error(), "resolver_kind") {
		t.Fatalf("LoadImpactState error = %v, want resolver_kind hint", err)
	}
}
