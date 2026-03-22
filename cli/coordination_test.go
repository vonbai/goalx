package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureCoordinationStateCreatesDigest(t *testing.T) {
	runDir := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	state, err := EnsureCoordinationState(runDir, "audit auth flow")
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	if state.Objective != "audit auth flow" {
		t.Fatalf("Objective = %q, want %q", state.Objective, "audit auth flow")
	}
	if state.Version <= 0 {
		t.Fatalf("Version = %d, want > 0", state.Version)
	}
	if _, err := os.Stat(CoordinationPath(runDir)); err != nil {
		t.Fatalf("coordination path missing: %v", err)
	}
}
