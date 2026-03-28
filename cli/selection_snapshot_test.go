package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
)

type testSelectionSnapshot struct {
	Version           int                            `json:"version"`
	ExplicitSelection bool                           `json:"explicit_selection,omitempty"`
	Policy            goalx.EffectiveSelectionPolicy `json:"policy"`
	Master            goalx.MasterConfig             `json:"master"`
	Research          goalx.SessionConfig            `json:"research"`
	Develop           goalx.SessionConfig            `json:"develop"`
}

func testSelectionSnapshotPath(runDir string) string {
	return filepath.Join(runDir, "selection-policy.json")
}

func writeSelectionSnapshotFixture(t *testing.T, runDir string, snapshot testSelectionSnapshot) {
	t.Helper()

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		t.Fatalf("marshal selection snapshot: %v", err)
	}
	if err := os.WriteFile(testSelectionSnapshotPath(runDir), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write selection snapshot: %v", err)
	}
}

func readSelectionSnapshotFixture(t *testing.T, runDir string) testSelectionSnapshot {
	t.Helper()

	data, err := os.ReadFile(testSelectionSnapshotPath(runDir))
	if err != nil {
		t.Fatalf("read selection snapshot: %v", err)
	}
	var snapshot testSelectionSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatalf("unmarshal selection snapshot: %v", err)
	}
	return snapshot
}
