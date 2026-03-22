package goalx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadJournal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jsonl")
	data := `{"round":1,"commit":"abc","desc":"read code","status":"progress","owner_scope":"auth retry flow"}
{"round":2,"commit":"def","desc":"split file","status":"progress"}
{"round":3,"commit":"ghi","desc":"all tests pass","status":"done","quality":"A"}
`
	os.WriteFile(path, []byte(data), 0644)

	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("len = %d, want 3", len(entries))
	}
	if entries[0].Round != 1 || entries[0].Commit != "abc" {
		t.Errorf("entry[0] = %+v", entries[0])
	}
	if entries[0].OwnerScope != "auth retry flow" {
		t.Errorf("entry[0].OwnerScope = %q, want auth retry flow", entries[0].OwnerScope)
	}
	if entries[2].Status != "done" {
		t.Errorf("entry[2].Status = %q, want done", entries[2].Status)
	}
	if entries[2].Quality != "A" {
		t.Errorf("entry[2].Quality = %q, want A", entries[2].Quality)
	}
}

func TestLoadJournalMissing(t *testing.T) {
	entries, err := LoadJournal("/nonexistent/journal.jsonl")
	if err != nil {
		t.Errorf("expected no error for missing file, got: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for missing file")
	}
}

func TestLoadJournalMaster(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "master.jsonl")
	data := `{"ts":"2026-03-19T10:00:00Z","action":"check","session":"session-1","finding":"15/23 tests pass"}
{"ts":"2026-03-19T10:05:00Z","action":"guide","session":"session-1","guidance":"fix imports"}
`
	os.WriteFile(path, []byte(data), 0644)

	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("len = %d, want 2", len(entries))
	}
	if entries[0].Action != "check" || entries[1].Guidance != "fix imports" {
		t.Errorf("entries = %+v", entries)
	}
}

func TestLoadJournalBlockedEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.jsonl")
	data := `{"round":4,"desc":"live verification blocked","status":"stuck","blocked_by":"await_input response","depends_on":["session-4"],"can_split":true,"suggested_next":"spawn live verification session"}`
	os.WriteFile(path, []byte(data), 0644)

	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if entries[0].BlockedBy != "await_input response" {
		t.Errorf("entries[0].BlockedBy = %q, want await_input response", entries[0].BlockedBy)
	}
	if len(entries[0].DependsOn) != 1 || entries[0].DependsOn[0] != "session-4" {
		t.Errorf("entries[0].DependsOn = %+v", entries[0].DependsOn)
	}
	if !entries[0].CanSplit {
		t.Errorf("entries[0].CanSplit = false, want true")
	}
	if entries[0].SuggestedNext != "spawn live verification session" {
		t.Errorf("entries[0].SuggestedNext = %q", entries[0].SuggestedNext)
	}
}

func TestLoadJournalDispatchableSlices(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "research.jsonl")
	data := `{"round":5,"desc":"research found follow-up slices","status":"progress","dispatchable_slices":[{"title":"split backend retries","why":"keep backend moving","mode":"develop","suggested_owner":"session-2"}]}`
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := LoadJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("len = %d, want 1", len(entries))
	}
	if len(entries[0].DispatchableSlices) != 1 {
		t.Fatalf("DispatchableSlices len = %d, want 1", len(entries[0].DispatchableSlices))
	}
	slice := entries[0].DispatchableSlices[0]
	if slice.Title != "split backend retries" || slice.SuggestedOwner != "session-2" {
		t.Fatalf("slice = %+v", slice)
	}
}

func TestSummary(t *testing.T) {
	entries := []JournalEntry{
		{Round: 1, Desc: "read code", Status: "progress"},
		{Round: 2, Desc: "all pass", Status: "done"},
	}
	s := Summary(entries)
	if s != "round 2: all pass (done)" {
		t.Errorf("Summary = %q", s)
	}
}

func TestSummaryStuckIncludesBlocker(t *testing.T) {
	entries := []JournalEntry{
		{Round: 4, Desc: "live verification blocked", Status: "stuck", BlockedBy: "await_input response"},
	}
	s := Summary(entries)
	if s != "round 4: live verification blocked (stuck: await_input response)" {
		t.Errorf("Summary = %q", s)
	}
}

func TestSummaryEmpty(t *testing.T) {
	s := Summary(nil)
	if s != "no entries" {
		t.Errorf("Summary(nil) = %q", s)
	}
}

func TestSummaryMaster(t *testing.T) {
	entries := []JournalEntry{
		{Action: "check", Session: "session-1", Finding: "15/23 pass"},
	}
	s := Summary(entries)
	if s != "[check] session-1: 15/23 pass" {
		t.Errorf("Summary = %q", s)
	}
}
