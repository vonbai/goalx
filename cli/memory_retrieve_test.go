package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
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

	query, err := BuildMemoryQuery(runDir)
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

func TestBuildMemoryContextAndDomainPackIncludeSuccessPrior(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	if err := os.WriteFile(filepath.Join(repo, "AGENTS.md"), []byte("repo policy"), 0o644); err != nil {
		t.Fatalf("WriteFile AGENTS.md: %v", err)
	}
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindSuccessPrior: {
			{
				ID:                "mem-success-prior-b",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "frontend product goals require critique and finisher proof before closeout",
				Selectors:         map[string]string{"project_id": goalx.ProjectID(repo)},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T00:00:00Z",
				UpdatedAt:         "2026-03-31T00:00:00Z",
			},
			{
				ID:                "mem-success-prior-a",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "operator runs require explicit finisher proof before closeout",
				Selectors:         map[string]string{"project_id": goalx.ProjectID(repo)},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T00:00:00Z",
				UpdatedAt:         "2026-03-31T00:00:00Z",
			},
		},
	})

	if err := RefreshRunMemoryContext(runDir); err != nil {
		t.Fatalf("RefreshRunMemoryContext: %v", err)
	}

	context, err := LoadMemoryContextFile(MemoryContextPath(runDir))
	if err != nil {
		t.Fatalf("LoadMemoryContextFile: %v", err)
	}
	if context == nil || len(context.SuccessPriors) != 2 {
		t.Fatalf("memory context = %+v, want two success priors", context)
	}

	sources, err := buildBootstrapCompilerSources(repo, runDir)
	if err != nil {
		t.Fatalf("buildBootstrapCompilerSources: %v", err)
	}
	pack, err := compileBootstrapDomainPack(cfg, meta, sources)
	if err != nil {
		t.Fatalf("compileBootstrapDomainPack: %v", err)
	}
	wantPriorIDs := []string{"mem-success-prior-a", "mem-success-prior-b"}
	if !reflect.DeepEqual(pack.PriorEntryIDs, wantPriorIDs) {
		t.Fatalf("domain pack prior_entry_ids = %+v, want %+v", pack.PriorEntryIDs, wantPriorIDs)
	}
	if pack.Slots.RepoPolicy.Source != "AGENTS.md" {
		t.Fatalf("repo_policy slot = %+v, want AGENTS.md source", pack.Slots.RepoPolicy)
	}
	if !reflect.DeepEqual(pack.Slots.LearnedSuccessPriors.EntryIDs, wantPriorIDs) {
		t.Fatalf("learned_success_priors slot = %+v, want %+v", pack.Slots.LearnedSuccessPriors.EntryIDs, wantPriorIDs)
	}
	if pack.Slots.RunContext.Source != "control/memory-context.json" {
		t.Fatalf("run_context slot = %+v, want memory context source", pack.Slots.RunContext)
	}
}

func TestRetrieveMemorySuccessPriorPenalizesContradictedEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindSuccessPrior: {
			{
				ID:                "mem-success-a",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "operator runs require finisher proof",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T10:00:00Z",
				UpdatedAt:         "2026-03-31T10:00:00Z",
			},
			{
				ID:                "mem-success-b",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "operator runs require explicit finisher proof",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T10:00:00Z",
				UpdatedAt:         "2026-03-31T10:00:00Z",
			},
		},
	})
	if err := AppendMemoryPriorGovernanceEvent(MemoryPriorGovernanceEvent{
		EntryID:    "mem-success-a",
		Kind:       MemoryPriorGovernanceKindContradicted,
		RecordedAt: "2026-03-31T11:00:00Z",
	}); err != nil {
		t.Fatalf("AppendMemoryPriorGovernanceEvent: %v", err)
	}

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) < 2 || entries[0].ID != "mem-success-b" {
		t.Fatalf("retrieve order = %+v, want non-contradicted prior first", entries)
	}
}

func TestRetrieveMemorySuccessPriorPrefersReinforcedEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindSuccessPrior: {
			{
				ID:                "mem-success-a",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "operator runs require finisher proof",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T10:00:00Z",
				UpdatedAt:         "2026-03-31T10:00:00Z",
			},
			{
				ID:                "mem-success-b",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "operator runs require explicit finisher proof",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T10:00:00Z",
				UpdatedAt:         "2026-03-31T10:00:00Z",
			},
		},
	})
	if err := AppendMemoryPriorGovernanceEvent(MemoryPriorGovernanceEvent{
		EntryID:    "mem-success-b",
		Kind:       MemoryPriorGovernanceKindReinforced,
		RecordedAt: "2026-03-31T11:00:00Z",
	}); err != nil {
		t.Fatalf("AppendMemoryPriorGovernanceEvent: %v", err)
	}

	entries, err := RetrieveMemory(MemoryQuery{ProjectID: "demo"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(entries) < 2 || entries[0].ID != "mem-success-b" {
		t.Fatalf("retrieve order = %+v, want reinforced prior first", entries)
	}
}
