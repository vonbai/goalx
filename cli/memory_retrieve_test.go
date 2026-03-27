package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildMemoryQueryFromRunFacts(t *testing.T) {
	repo, runDir, _, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	projectID := goalx.ProjectID(repo)
	if err := os.MkdirAll(MemoryProjectsDir(), 0o755); err != nil {
		t.Fatalf("mkdir memory projects dir: %v", err)
	}
	profile := map[string]string{
		"environment": "staging",
		"service":     "deploy",
		"provider":    "cloudflare",
		"tool":        "docker-compose",
	}
	data, err := json.Marshal(profile)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(MemoryProjectsDir(), projectID+".json"), data, 0o644); err != nil {
		t.Fatalf("write profile: %v", err)
	}

	query, err := BuildMemoryQuery(repo, "guidance-run", runDir)
	if err != nil {
		t.Fatalf("BuildMemoryQuery: %v", err)
	}
	if query.ProjectID != projectID {
		t.Fatalf("project_id = %q, want %q", query.ProjectID, projectID)
	}
	if query.Intent != runIntentEvolve {
		t.Fatalf("intent = %q, want %q", query.Intent, runIntentEvolve)
	}
	if query.Environment != "staging" || query.Service != "deploy" || query.Provider != "cloudflare" || query.Tool != "docker-compose" {
		t.Fatalf("query profile fields = %+v", query)
	}
}

func TestRetrieveMemoryPrefersProjectSpecificScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:        "mem-project",
				Kind:      MemoryKindFact,
				Statement: "project deploy runs through compose",
				Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
			},
			{
				ID:        "mem-infra",
				Kind:      MemoryKindFact,
				Statement: "shared deploy infrastructure",
				Selectors: map[string]string{"infra_group": "shared", "service": "deploy"},
			},
		},
	})

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo", InfraGroup: "shared", Service: "deploy"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) == 0 || entries[0].ID != "mem-project" {
		t.Fatalf("retrieve order = %+v, want project-specific first", entries)
	}
}

func TestRetrieveMemoryFallsBackToInfraScope(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:        "mem-infra",
				Kind:      MemoryKindFact,
				Statement: "shared scheduler reaches postgres",
				Selectors: map[string]string{"infra_group": "shared", "service": "postgres"},
			},
		},
	})

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo", InfraGroup: "shared", Service: "postgres"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "mem-infra" {
		t.Fatalf("retrieve result = %+v, want infra fallback entry", entries)
	}
}

func TestRetrieveMemoryUsesLexicalFallbackAfterSelectorRecall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindProcedure: {
			{
				ID:        "mem-postgres",
				Kind:      MemoryKindProcedure,
				Statement: "check postgres from scheduler container first",
				Selectors: map[string]string{"project_id": "demo"},
			},
			{
				ID:        "mem-deploy",
				Kind:      MemoryKindProcedure,
				Statement: "check deploy path before release",
				Selectors: map[string]string{"project_id": "demo"},
			},
		},
	})

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo", Service: "postgres"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) == 0 || entries[0].ID != "mem-postgres" {
		t.Fatalf("retrieve order = %+v, want lexical postgres match first", entries)
	}
}

func TestRetrieveMemoryPrefersFresherEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:        "mem-old",
				Kind:      MemoryKindFact,
				Statement: "deploy host is ops-3",
				Selectors: map[string]string{"project_id": "demo"},
				UpdatedAt: "2026-03-20T00:00:00Z",
			},
			{
				ID:        "mem-new",
				Kind:      MemoryKindFact,
				Statement: "deploy host is ops-7",
				Selectors: map[string]string{"project_id": "demo"},
				UpdatedAt: "2026-03-27T00:00:00Z",
			},
		},
	})

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) < 2 || entries[0].ID != "mem-new" {
		t.Fatalf("retrieve order = %+v, want fresher entry first", entries)
	}
}

func TestRetrieveMemoryExcludesSupersededEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:           "mem-old",
				Kind:         MemoryKindFact,
				Statement:    "deploy host is ops-3",
				Selectors:    map[string]string{"project_id": "demo", "service": "deploy"},
				SupersededBy: "mem-new",
			},
			{
				ID:        "mem-new",
				Kind:      MemoryKindFact,
				Statement: "deploy host is ops-7",
				Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
			},
		},
	})

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo", Service: "deploy"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "mem-new" {
		t.Fatalf("retrieve result = %+v, want only active replacement", entries)
	}
}

func TestBuildMemoryContextBoundsCategorySizes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	entries := make([]MemoryEntry, 0, 5)
	for i := 0; i < 5; i++ {
		entries = append(entries, MemoryEntry{
			ID:        "mem-fact-" + string(rune('a'+i)),
			Kind:      MemoryKindFact,
			Statement: "fact entry " + string(rune('a'+i)),
			Selectors: map[string]string{"project_id": "demo"},
		})
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: entries,
	})

	context, err := BuildMemoryContext(MemoryQuery{ProjectID: "demo"})
	if err != nil {
		t.Fatalf("BuildMemoryContext: %v", err)
	}
	if len(context.Facts) != memoryContextCategoryLimit {
		t.Fatalf("facts len = %d, want %d", len(context.Facts), memoryContextCategoryLimit)
	}
}
