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
	want := "while sleep 45; do tmux send-keys -t goalx-demo:master 'Heartbeat: execute check cycle now.' Enter; done"
	if got != want {
		t.Fatalf("HeartbeatCommand() = %q, want %q", got, want)
	}
}
