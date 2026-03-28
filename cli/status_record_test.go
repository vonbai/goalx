package cli

import (
	"strings"
	"testing"
)

func TestLoadRunStatusRecordParsesCanonicalShape(t *testing.T) {
	runDir := t.TempDir()
	path := RunStatusPath(runDir)
	data := `{
  "version": 1,
  "phase": "working",
  "required_remaining": 2,
  "open_required_ids": ["req-1", "req-2"],
  "active_sessions": ["session-1"],
  "keep_session": "session-2",
  "last_verified_at": "2026-03-28T10:00:00Z",
  "updated_at": "2026-03-28T10:05:00Z"
}`
	if err := writeFileAtomic(path, []byte(data), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	record, err := LoadRunStatusRecord(path)
	if err != nil {
		t.Fatalf("LoadRunStatusRecord: %v", err)
	}
	if record == nil || record.RequiredRemaining == nil {
		t.Fatal("LoadRunStatusRecord returned nil record or missing required_remaining")
	}
	if record.Version != 1 || record.Phase != runStatusPhaseWorking || *record.RequiredRemaining != 2 {
		t.Fatalf("unexpected record: %#v", record)
	}
}

func TestLoadRunStatusRecordRejectsUnknownFields(t *testing.T) {
	runDir := t.TempDir()
	path := RunStatusPath(runDir)
	if err := writeFileAtomic(path, []byte(`{"version":1,"phase":"working","required_remaining":1,"run":"demo"}`), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	_, err := LoadRunStatusRecord(path)
	if err == nil {
		t.Fatal("LoadRunStatusRecord should fail")
	}
	for _, want := range []string{"unknown field", "goalx schema status"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("LoadRunStatusRecord error = %v, want %q", err, want)
		}
	}
}

func TestLoadRunStatusRecordRejectsMissingVersion(t *testing.T) {
	runDir := t.TempDir()
	path := RunStatusPath(runDir)
	if err := writeFileAtomic(path, []byte(`{"phase":"working","required_remaining":1}`), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}
	_, err := LoadRunStatusRecord(path)
	if err == nil || !strings.Contains(err.Error(), "version must be positive") {
		t.Fatalf("LoadRunStatusRecord error = %v, want version failure", err)
	}
}
