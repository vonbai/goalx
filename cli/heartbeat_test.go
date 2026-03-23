package cli

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeSidecarIntervalUsesDefaultFloor(t *testing.T) {
	seconds, warning := normalizeSidecarInterval(20 * time.Second)
	if seconds != 300 {
		t.Fatalf("seconds = %d, want 300", seconds)
	}
	if !strings.Contains(warning, "using default 300s") {
		t.Fatalf("warning = %q, want default 300s note", warning)
	}
}

func TestNormalizeSidecarIntervalKeepsConfiguredValue(t *testing.T) {
	seconds, warning := normalizeSidecarInterval(5 * time.Minute)
	if seconds != 300 {
		t.Fatalf("seconds = %d, want 300", seconds)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}
