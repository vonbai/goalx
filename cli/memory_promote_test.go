package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPromoteGroundedFactToCanonical(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 27, 14, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:         "prop_fact_1",
			State:      "proposed",
			Kind:       MemoryKindFact,
			Statement:  "host is ops-3",
			Selectors:  map[string]string{"project_id": "demo", "service": "deploy"},
			Evidence:   []MemoryEvidence{{Kind: "report", Path: "/tmp/report.md"}},
			SourceRuns: []string{"run-1"},
			ValidFrom:  "2026-03-27T14:00:00Z",
			CreatedAt:  "2026-03-27T14:00:00Z",
			UpdatedAt:  "2026-03-27T14:00:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := PromoteMemoryProposals(); err != nil {
		t.Fatalf("PromoteMemoryProposals: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindFact)
	if len(entries) != 1 {
		t.Fatalf("fact entries len = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.Statement != "host is ops-3" {
		t.Fatalf("statement = %q, want promoted fact", entry.Statement)
	}
	if entry.VerificationState != "validated" {
		t.Fatalf("verification_state = %q, want validated", entry.VerificationState)
	}
	if entry.Confidence != "grounded" {
		t.Fatalf("confidence = %q, want grounded", entry.Confidence)
	}
	if entry.ValidFrom != "2026-03-27T14:00:00Z" {
		t.Fatalf("valid_from = %q, want proposal timestamp", entry.ValidFrom)
	}
}

func TestProcedureDoesNotPromoteFromSingleUse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 27, 15, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:         "prop_proc_single",
			State:      "proposed",
			Kind:       MemoryKindProcedure,
			Statement:  "run db checks inside scheduler first",
			Selectors:  map[string]string{"project_id": "demo", "service": "postgres"},
			Evidence:   []MemoryEvidence{{Kind: "report", Path: "/tmp/report.md"}},
			SourceRuns: []string{"run-1"},
			CreatedAt:  "2026-03-27T15:00:00Z",
			UpdatedAt:  "2026-03-27T15:00:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := PromoteMemoryProposals(); err != nil {
		t.Fatalf("PromoteMemoryProposals: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindProcedure)
	if len(entries) != 0 {
		t.Fatalf("procedure entries = %+v, want none", entries)
	}
}

func TestProcedurePromotesAfterFailureAndSuccess(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 27, 16, 0, 0, 0, time.UTC)
	proposal := MemoryProposal{
		ID:        "prop_proc_recovery",
		State:     "proposed",
		Kind:      MemoryKindProcedure,
		Statement: "run db checks inside scheduler first",
		Selectors: map[string]string{"project_id": "demo", "service": "postgres"},
		Evidence: []MemoryEvidence{
			{Kind: "verify_failure", Path: "/tmp/failure.txt"},
			{Kind: "verify_recovery", Path: "/tmp/recovery.txt"},
		},
		SourceRuns: []string{"run-1"},
		CreatedAt:  "2026-03-27T16:00:00Z",
		UpdatedAt:  "2026-03-27T16:00:00Z",
	}
	if err := writeProposalShard(now, []MemoryProposal{proposal}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := PromoteMemoryProposals(); err != nil {
		t.Fatalf("PromoteMemoryProposals: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindProcedure)
	if len(entries) != 1 {
		t.Fatalf("procedure entries len = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.VerificationState != "validated" {
		t.Fatalf("verification_state = %q, want validated", entry.VerificationState)
	}
	if entry.Confidence != "grounded" {
		t.Fatalf("confidence = %q, want grounded", entry.Confidence)
	}
}

func TestSuccessPriorPromotesAfterRepeatedRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 31, 10, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:        "prop_success_prior",
			State:     "proposed",
			Kind:      MemoryKindSuccessPrior,
			Statement: "frontend product goals require critique and finisher proof before closeout",
			Selectors: map[string]string{"project_id": "demo", "intent": "worker"},
			Evidence: []MemoryEvidence{
				{Kind: "intervention_log", Path: "/tmp/intervention-log.jsonl"},
				{Kind: "summary", Path: "/tmp/summary.md"},
			},
			SourceRuns: []string{"run-1", "run-2"},
			CreatedAt:  "2026-03-31T10:00:00Z",
			UpdatedAt:  "2026-03-31T10:00:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := PromoteMemoryProposals(); err != nil {
		t.Fatalf("PromoteMemoryProposals: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindSuccessPrior)
	if len(entries) != 1 {
		t.Fatalf("success_prior entries len = %d, want 1", len(entries))
	}
	entry := entries[0]
	if entry.VerificationState != "repeated" {
		t.Fatalf("verification_state = %q, want repeated", entry.VerificationState)
	}
	if entry.Statement != "frontend product goals require critique and finisher proof before closeout" {
		t.Fatalf("statement = %q", entry.Statement)
	}
}

func TestSupersedeMemoryEntryMarksPriorEntryInactive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:                "mem_old",
				Kind:              MemoryKindFact,
				Statement:         "host is ops-3",
				Selectors:         map[string]string{"project_id": "demo", "service": "deploy"},
				VerificationState: "validated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-20T00:00:00Z",
				UpdatedAt:         "2026-03-20T00:00:00Z",
			},
			{
				ID:                "mem_new",
				Kind:              MemoryKindFact,
				Statement:         "host is ops-7",
				Selectors:         map[string]string{"project_id": "demo", "service": "deploy"},
				VerificationState: "validated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-27T00:00:00Z",
				UpdatedAt:         "2026-03-27T00:00:00Z",
			},
		},
	})

	if err := SupersedeMemoryEntry("mem_old", "mem_new"); err != nil {
		t.Fatalf("SupersedeMemoryEntry: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindFact)
	if len(entries) != 2 {
		t.Fatalf("fact entries len = %d, want 2", len(entries))
	}
	oldEntry := findCanonicalEntryByID(entries, "mem_old")
	if oldEntry == nil {
		t.Fatal("old entry missing")
	}
	if oldEntry.SupersededBy != "mem_new" {
		t.Fatalf("superseded_by = %q, want mem_new", oldEntry.SupersededBy)
	}
	if oldEntry.ValidTo == "" {
		t.Fatal("valid_to empty on superseded entry")
	}
	if oldEntry.ContradictedCount != 1 {
		t.Fatalf("contradicted_count = %d, want 1", oldEntry.ContradictedCount)
	}

	retrieved, err := RetrieveMemory(MemoryQuery{ProjectID: "demo", Service: "deploy"})
	if err != nil {
		t.Fatalf("RetrieveMemory: %v", err)
	}
	if len(retrieved) != 1 || retrieved[0].ID != "mem_new" {
		t.Fatalf("retrieved = %+v, want only active successor", retrieved)
	}
}

func TestUsageStatsDoNotPromoteEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 27, 17, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:         "prop_proc_usage_only",
			State:      "proposed",
			Kind:       MemoryKindProcedure,
			Statement:  "run db checks inside scheduler first",
			Selectors:  map[string]string{"project_id": "demo", "service": "postgres"},
			Evidence:   []MemoryEvidence{{Kind: "report", Path: "/tmp/report.md"}},
			SourceRuns: []string{"run-1"},
			CreatedAt:  "2026-03-27T17:00:00Z",
			UpdatedAt:  "2026-03-27T17:00:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	if err := PromoteMemoryProposals(); err != nil {
		t.Fatalf("PromoteMemoryProposals: %v", err)
	}

	if entries := loadCanonicalEntriesByKind(t, MemoryKindProcedure); len(entries) != 0 {
		t.Fatalf("procedure entries after initial promotion = %+v, want none", entries)
	}

	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:                "mem_usage",
				Kind:              MemoryKindFact,
				Statement:         "provider is cloudflare",
				Selectors:         map[string]string{"project_id": "demo", "service": "deploy"},
				VerificationState: "validated",
				Confidence:        "grounded",
				RetrievedCount:    50,
				UsedCount:         20,
				CreatedAt:         "2026-03-27T00:00:00Z",
				UpdatedAt:         "2026-03-27T00:00:00Z",
			},
		},
	})

	if err := PromoteMemoryProposals(); err != nil {
		t.Fatalf("PromoteMemoryProposals(second): %v", err)
	}

	if entries := loadCanonicalEntriesByKind(t, MemoryKindProcedure); len(entries) != 0 {
		t.Fatalf("procedure entries after usage-biased promotion = %+v, want none", entries)
	}
}

func TestPromoteMemoryProposalsWaitsForMemoryStoreLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 27, 19, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:         "prop_fact_locked",
			State:      "proposed",
			Kind:       MemoryKindFact,
			Statement:  "host is ops-3",
			Selectors:  map[string]string{"project_id": "demo", "service": "deploy"},
			Evidence:   []MemoryEvidence{{Kind: "report", Path: "/tmp/report.md"}},
			SourceRuns: []string{"run-1"},
			CreatedAt:  now.Format(time.RFC3339),
			UpdatedAt:  now.Format(time.RFC3339),
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	lockFile, err := os.OpenFile(MemoryLockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("OpenFile lock: %v", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("Flock lock: %v", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	done := make(chan error, 1)
	go func() {
		done <- PromoteMemoryProposals()
	}()

	select {
	case err := <-done:
		t.Fatalf("PromoteMemoryProposals completed before lock release: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("Flock unlock: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("PromoteMemoryProposals(after unlock): %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindFact)
	if len(entries) != 1 || entries[0].ID != stableMemoryEntryID(MemoryKindFact, map[string]string{"project_id": "demo", "service": "deploy"}, "host is ops-3") {
		t.Fatalf("canonical fact entries = %+v, want promoted locked fact", entries)
	}
}

func writeProposalShard(now time.Time, proposals []MemoryProposal) error {
	if err := EnsureMemoryStore(); err != nil {
		return err
	}
	path := MemoryProposalPath(now)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	lines := make([]byte, 0)
	for _, proposal := range proposals {
		data, err := json.Marshal(proposal)
		if err != nil {
			return err
		}
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	return os.WriteFile(path, lines, 0o644)
}

func loadCanonicalEntriesByKind(t *testing.T, kind MemoryKind) []MemoryEntry {
	t.Helper()

	data, err := os.ReadFile(MemoryEntryPath(kind))
	if err != nil {
		t.Fatalf("ReadFile %s: %v", kind, err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 1 && strings.TrimSpace(lines[0]) == "" {
		return nil
	}
	entries := make([]MemoryEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry MemoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("json.Unmarshal %s: %v", kind, err)
		}
		entries = append(entries, entry)
	}
	return entries
}

func findCanonicalEntryByID(entries []MemoryEntry, id string) *MemoryEntry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}
