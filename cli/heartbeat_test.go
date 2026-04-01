package cli

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeRuntimeHostIntervalUsesDefaultFloor(t *testing.T) {
	seconds, warning := normalizeRuntimeHostInterval(20 * time.Second)
	if seconds != 300 {
		t.Fatalf("seconds = %d, want 300", seconds)
	}
	if !strings.Contains(warning, "using default 300s") {
		t.Fatalf("warning = %q, want default 300s note", warning)
	}
}

func TestNormalizeRuntimeHostIntervalKeepsConfiguredValue(t *testing.T) {
	seconds, warning := normalizeRuntimeHostInterval(5 * time.Minute)
	if seconds != 300 {
		t.Fatalf("seconds = %d, want 300", seconds)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}
