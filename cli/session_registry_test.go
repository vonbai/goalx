package cli

import "testing"

func TestNextSessionIndexStartsAtOneWithNoJournals(t *testing.T) {
	runDir := t.TempDir()

	got, err := nextSessionIndex(runDir)
	if err != nil {
		t.Fatalf("nextSessionIndex: %v", err)
	}
	if got != 1 {
		t.Fatalf("nextSessionIndex = %d, want 1", got)
	}
}
