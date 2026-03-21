package cli

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeHeartbeatIntervalUsesDefaultFloor(t *testing.T) {
	seconds, warning := normalizeHeartbeatInterval(20 * time.Second)
	if seconds != 300 {
		t.Fatalf("seconds = %d, want 300", seconds)
	}
	if !strings.Contains(warning, "using default 300s") {
		t.Fatalf("warning = %q, want default 300s note", warning)
	}
}

func TestNormalizeHeartbeatIntervalKeepsConfiguredValue(t *testing.T) {
	seconds, warning := normalizeHeartbeatInterval(5 * time.Minute)
	if seconds != 300 {
		t.Fatalf("seconds = %d, want 300", seconds)
	}
	if warning != "" {
		t.Fatalf("warning = %q, want empty", warning)
	}
}

func TestHeartbeatCommand(t *testing.T) {
	got := HeartbeatCommand("goalx-demo", 45)
	// Smart heartbeat: idle detection + force after 3 skips
	if !strings.Contains(got, "sleep 45") {
		t.Fatalf("missing sleep interval in %q", got)
	}
	if !strings.Contains(got, "goalx-demo:master") {
		t.Fatalf("missing tmux target in %q", got)
	}
	if !strings.Contains(got, "SKIP") {
		t.Fatalf("missing skip counter in %q", got)
	}
	if !strings.Contains(got, "grep -qE") {
		t.Fatalf("missing idle detection in %q", got)
	}
}
