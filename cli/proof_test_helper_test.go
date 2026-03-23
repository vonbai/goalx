package cli

import (
	"os"
	"testing"
)

const sharedProofEvidencePath = "/tmp/e2e.txt"

func ensureSharedProofEvidence(t *testing.T) string {
	t.Helper()
	if err := os.WriteFile(sharedProofEvidencePath, []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write shared proof evidence: %v", err)
	}
	return sharedProofEvidencePath
}
