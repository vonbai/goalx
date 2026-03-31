package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildMemorySelectorIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:        "mem-1",
				Kind:      MemoryKindFact,
				Statement: "prod deploy runs on ops-3",
				Selectors: map[string]string{"project_id": "demo", "environment": "prod", "service": "deploy"},
			},
		},
		MemoryKindPitfall: {
			{
				ID:        "mem-2",
				Kind:      MemoryKindPitfall,
				Statement: "postgres checks must run from scheduler",
				Selectors: map[string]string{"project_id": "demo", "host": "ops-3", "service": "postgres"},
			},
		},
	})

	index, err := BuildMemorySelectorIndex()
	if err != nil {
		t.Fatalf("BuildMemorySelectorIndex: %v", err)
	}
	if len(index.EntryIDs) != 2 {
		t.Fatalf("selector entry_ids len = %d, want 2", len(index.EntryIDs))
	}
	mem1 := index.EntryIDs["mem-1"]
	mem2 := index.EntryIDs["mem-2"]
	assertUint32PostingList(t, index.Postings["project_id:demo"], []uint32{mem1, mem2})
	assertUint32PostingList(t, index.Postings["environment:prod"], []uint32{mem1})
	assertUint32PostingList(t, index.Postings["host:ops-3"], []uint32{mem2})
	assertUint32PostingList(t, index.Postings["kind:fact"], []uint32{mem1})
	assertUint32PostingList(t, index.Postings["kind:pitfall"], []uint32{mem2})
}

func TestBuildMemorySelectorIndexUsesStableEntryIDs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{ID: "mem-b", Kind: MemoryKindFact, Statement: "b entry", Selectors: map[string]string{"project_id": "demo"}},
			{ID: "mem-a", Kind: MemoryKindFact, Statement: "a entry", Selectors: map[string]string{"project_id": "demo"}},
		},
	})

	first, err := BuildMemorySelectorIndex()
	if err != nil {
		t.Fatalf("BuildMemorySelectorIndex(first): %v", err)
	}

	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{ID: "mem-a", Kind: MemoryKindFact, Statement: "a entry", Selectors: map[string]string{"project_id": "demo"}},
			{ID: "mem-b", Kind: MemoryKindFact, Statement: "b entry", Selectors: map[string]string{"project_id": "demo"}},
		},
	})

	second, err := BuildMemorySelectorIndex()
	if err != nil {
		t.Fatalf("BuildMemorySelectorIndex(second): %v", err)
	}
	if first.EntryIDs["mem-a"] != second.EntryIDs["mem-a"] || first.EntryIDs["mem-b"] != second.EntryIDs["mem-b"] {
		t.Fatalf("stable ids changed: first=%v second=%v", first.EntryIDs, second.EntryIDs)
	}
}

func TestBuildMemoryTokenIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindProcedure: {
			{
				ID:        "mem-1",
				Kind:      MemoryKindProcedure,
				Statement: "Deploy over SSH; deploy after login.",
				Selectors: map[string]string{"project_id": "demo", "tool": "ssh"},
			},
		},
	})

	index, err := BuildMemoryTokenIndex()
	if err != nil {
		t.Fatalf("BuildMemoryTokenIndex: %v", err)
	}
	mem1 := index.EntryIDs["mem-1"]
	assertUint32PostingList(t, index.Postings["deploy"], []uint32{mem1})
	assertUint32PostingList(t, index.Postings["ssh"], []uint32{mem1})
	if index.DocumentFrequency["deploy"] != 1 {
		t.Fatalf("document_frequency[deploy] = %d, want 1", index.DocumentFrequency["deploy"])
	}
	if _, ok := index.Postings["ssh;"]; ok {
		t.Fatalf("token index should strip punctuation, got raw token postings: %+v", index.Postings)
	}
}

func TestBuildMemoryTrustIndex(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:                "mem-1",
				Kind:              MemoryKindFact,
				Statement:         "prod deploy runs on ops-3",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "validated",
				Confidence:        "grounded",
				ValidFrom:         "2026-03-27T00:00:00Z",
				UpdatedAt:         "2026-03-27T01:00:00Z",
			},
		},
		MemoryKindPitfall: {
			{
				ID:                "mem-2",
				Kind:              MemoryKindPitfall,
				Statement:         "ssh quoting breaks nested commands",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "repeated",
				Confidence:        "heuristic",
				ValidFrom:         "2026-03-20T00:00:00Z",
				UpdatedAt:         "2026-03-26T01:00:00Z",
				SupersededBy:      "mem-3",
			},
		},
	})

	index, err := BuildMemoryTrustIndex()
	if err != nil {
		t.Fatalf("BuildMemoryTrustIndex: %v", err)
	}
	record1, ok := index.Records["mem-1"]
	if !ok {
		t.Fatalf("trust record for mem-1 missing: %+v", index.Records)
	}
	if record1.VerificationState != "validated" || record1.Confidence != "grounded" {
		t.Fatalf("trust record 1 = %+v", record1)
	}
	record2, ok := index.Records["mem-2"]
	if !ok {
		t.Fatalf("trust record for mem-2 missing: %+v", index.Records)
	}
	if record2.SupersededBy != "mem-3" || record2.UpdatedAt != "2026-03-26T01:00:00Z" {
		t.Fatalf("trust record 2 = %+v", record2)
	}
}

func TestBuildMemorySelectorIndexIncludesSuccessPrior(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindSuccessPrior: {
			{
				ID:        "mem-success-prior",
				Kind:      MemoryKindSuccessPrior,
				Statement: "frontend product goals require critique and finisher proof before closeout",
				Selectors: map[string]string{"project_id": "demo", "intent": "worker"},
			},
		},
	})

	index, err := BuildMemorySelectorIndex()
	if err != nil {
		t.Fatalf("BuildMemorySelectorIndex: %v", err)
	}
	entryID := index.EntryIDs["mem-success-prior"]
	assertUint32PostingList(t, index.Postings["kind:success_prior"], []uint32{entryID})
	assertUint32PostingList(t, index.Postings["project_id:demo"], []uint32{entryID})
}

func TestRebuildMemoryIndexesIgnoresRunLocalFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:        "mem-1",
				Kind:      MemoryKindFact,
				Statement: "prod deploy runs on ops-3",
				Selectors: map[string]string{"project_id": "demo"},
			},
		},
	})

	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	if err := os.WriteFile(MemorySeedsPath(runDir), []byte("{\"id\":\"seed-noise\"}\n"), 0o644); err != nil {
		t.Fatalf("write memory-seeds: %v", err)
	}
	if err := os.WriteFile(MemoryQueryPath(runDir), []byte("{\"project_id\":\"noise\"}\n"), 0o644); err != nil {
		t.Fatalf("write memory-query: %v", err)
	}
	if err := os.WriteFile(MemoryContextPath(runDir), []byte("{\"facts\":[\"noise\"]}\n"), 0o644); err != nil {
		t.Fatalf("write memory-context: %v", err)
	}

	if err := RebuildMemoryIndexes(); err != nil {
		t.Fatalf("RebuildMemoryIndexes: %v", err)
	}

	selectorIndex, err := loadMemorySelectorIndex(filepath.Join(MemoryIndexesDir(), "selectors.json"))
	if err != nil {
		t.Fatalf("load selector index: %v", err)
	}
	if len(selectorIndex.EntryIDs) != 1 {
		t.Fatalf("selector entry_ids len = %d, want 1", len(selectorIndex.EntryIDs))
	}
	if _, ok := selectorIndex.Postings["project_id:noise"]; ok {
		t.Fatalf("selector index should ignore run-local files: %+v", selectorIndex.Postings)
	}

	stats, err := loadMemoryIndexStats(filepath.Join(MemoryIndexesDir(), "stats.json"))
	if err != nil {
		t.Fatalf("load memory stats: %v", err)
	}
	if stats.EntryCount != 1 {
		t.Fatalf("memory stats entry_count = %d, want 1", stats.EntryCount)
	}
}

func writeCanonicalMemoryEntries(t *testing.T, byKind map[MemoryKind][]MemoryEntry) {
	t.Helper()

	for _, kind := range []MemoryKind{
		MemoryKindFact,
		MemoryKindProcedure,
		MemoryKindPitfall,
		MemoryKindSecretRef,
		MemoryKindSuccessPrior,
	} {
		entries := byKind[kind]
		lines := make([]byte, 0)
		for _, entry := range entries {
			data, err := json.Marshal(entry)
			if err != nil {
				t.Fatalf("marshal %s entry: %v", kind, err)
			}
			lines = append(lines, data...)
			lines = append(lines, '\n')
		}
		if err := os.WriteFile(MemoryEntryPath(kind), lines, 0o644); err != nil {
			t.Fatalf("write %s entries: %v", kind, err)
		}
	}
}

func assertUint32PostingList(t *testing.T, got, want []uint32) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("posting list len = %d, want %d (got=%v want=%v)", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("posting list = %v, want %v", got, want)
		}
	}
}
