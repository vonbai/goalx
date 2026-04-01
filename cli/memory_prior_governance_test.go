package cli

import (
	"testing"
)

func TestAppendAndLoadMemoryPriorGovernanceEvents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	events := []MemoryPriorGovernanceEvent{
		{EntryID: "mem-success-1", Kind: MemoryPriorGovernanceKindReplayedValid, RecordedAt: "2026-03-31T10:00:00Z"},
		{EntryID: "mem-success-1", Kind: MemoryPriorGovernanceKindReinforced, RecordedAt: "2026-03-31T10:05:00Z"},
		{EntryID: "mem-success-1", Kind: MemoryPriorGovernanceKindContradicted, RecordedAt: "2026-03-31T10:10:00Z"},
		{EntryID: "mem-success-1", Kind: MemoryPriorGovernanceKindSuperseded, ReplacementID: "mem-success-2", RecordedAt: "2026-03-31T10:15:00Z"},
	}
	for _, event := range events {
		if err := AppendMemoryPriorGovernanceEvent(event); err != nil {
			t.Fatalf("AppendMemoryPriorGovernanceEvent(%s): %v", event.Kind, err)
		}
	}

	loaded, err := LoadMemoryPriorGovernanceEvents()
	if err != nil {
		t.Fatalf("LoadMemoryPriorGovernanceEvents: %v", err)
	}
	if len(loaded) != 4 {
		t.Fatalf("governance events len = %d, want 4", len(loaded))
	}
	if loaded[3].ReplacementID != "mem-success-2" {
		t.Fatalf("replacement_id = %q, want mem-success-2", loaded[3].ReplacementID)
	}
}

func TestContradictMemoryEntryIncrementsMetadataWithoutDeletingPrior(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindSuccessPrior: {
			{
				ID:                "mem-success-1",
				Kind:              MemoryKindSuccessPrior,
				Statement:         "operator runs require finisher proof",
				Selectors:         map[string]string{"project_id": "demo"},
				VerificationState: "repeated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-31T10:00:00Z",
				UpdatedAt:         "2026-03-31T10:00:00Z",
			},
		},
	})

	if err := ContradictMemoryEntry("mem-success-1", "run-3", []MemoryEvidence{{Kind: "summary", Path: "/tmp/summary.md"}}); err != nil {
		t.Fatalf("ContradictMemoryEntry: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindSuccessPrior)
	if len(entries) != 1 {
		t.Fatalf("success prior entries = %+v, want one surviving entry", entries)
	}
	if entries[0].ContradictedCount != 1 {
		t.Fatalf("contradicted_count = %d, want 1", entries[0].ContradictedCount)
	}

	events, err := LoadMemoryPriorGovernanceEvents()
	if err != nil {
		t.Fatalf("LoadMemoryPriorGovernanceEvents: %v", err)
	}
	if len(events) != 1 || events[0].Kind != MemoryPriorGovernanceKindContradicted {
		t.Fatalf("governance events = %+v, want contradicted event", events)
	}
}
