package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveShowsRunRuntimeStateAndProjectStatusCache(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	runState := `{"version":1,"run":"guidance-run","mode":"develop","active":true,"updated_at":"2026-03-25T00:00:00Z"}`
	if err := os.WriteFile(RunRuntimeStatePath(runDir), []byte(runState), 0o644); err != nil {
		t.Fatalf("write run runtime state: %v", err)
	}
	projectStatus := `{"phase":"working","recommendation":"keep going","acceptance_met":false}`
	if err := os.WriteFile(ProjectStatusCachePath(repo), []byte(projectStatus), 0o644); err != nil {
		t.Fatalf("write project status cache: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### Run runtime state",
		runState,
		"### Project status cache",
		projectStatus,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "### Status (from master)") {
		t.Fatalf("observe output still uses stale status heading:\n%s", out)
	}
}

func TestObserveShowsSessionQueueFacts(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("session pane\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "session-wake:session-1", Status: "sent", Target: "gx-demo:session-1", AttemptedAt: "2026-03-25T00:00:00Z"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### session-1",
		"Queue: unread=1 cursor=0/1",
		"last_nudge_at=2026-03-25T00:00:00Z",
		"last_delivery=sent",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}
