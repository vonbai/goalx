package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuildActivitySnapshotAggregatesControlFacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master is coordinating\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("session is editing\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "do work"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}
	if err := SaveControlReminders(ControlRemindersPath(runDir), &ControlReminders{
		Version: 1,
		Items: []ControlReminder{
			{ReminderID: "rem-1", DedupeKey: "master-wake", Reason: "control-cycle", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlReminders: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "master-wake", Status: "failed", Target: "gx-demo:master", AttemptedAt: time.Now().UTC().Format(time.RFC3339)},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 1},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 2},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if err := SaveWorktreeSnapshot(runDir, &WorktreeSnapshot{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Root:      WorktreeDiffStat{DirtyFiles: 1, Insertions: 5},
		Sessions: map[string]WorktreeDiffStat{
			"session-1": {DirtyFiles: 2, Insertions: 3, Deletions: 1},
		},
	}); err != nil {
		t.Fatalf("SaveWorktreeSnapshot: %v", err)
	}

	snapshot, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot: %v", err)
	}

	if snapshot.Run.RunID != meta.RunID {
		t.Fatalf("run_id = %q, want %q", snapshot.Run.RunID, meta.RunID)
	}
	if snapshot.Queue.MasterUnread != 1 {
		t.Fatalf("master_unread = %d, want 1", snapshot.Queue.MasterUnread)
	}
	if snapshot.Queue.DeliveriesFailed != 1 {
		t.Fatalf("deliveries_failed = %d, want 1", snapshot.Queue.DeliveriesFailed)
	}
	if snapshot.Actors["master"].Lease != "healthy" {
		t.Fatalf("master lease = %q, want healthy", snapshot.Actors["master"].Lease)
	}
	if !snapshot.Actors["master"].PanePresent {
		t.Fatal("master pane should be present")
	}
	if snapshot.Actors["master"].PaneHash == "" || snapshot.Actors["master"].LastOutputChangeAt == "" {
		t.Fatalf("master pane facts missing: %+v", snapshot.Actors["master"])
	}
	if snapshot.Sessions["session-1"].DirtyFiles != 2 {
		t.Fatalf("session-1 dirty_files = %d, want 2", snapshot.Sessions["session-1"].DirtyFiles)
	}
	if snapshot.Root.DirtyFiles != 1 {
		t.Fatalf("root dirty_files = %d, want 1", snapshot.Root.DirtyFiles)
	}
}

func TestBuildActivitySnapshotTracksPaneHashChanges(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("first pane content\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	first, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot first: %v", err)
	}
	if err := SaveActivitySnapshot(runDir, first); err != nil {
		t.Fatalf("SaveActivitySnapshot: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(masterCapture, []byte("second pane content\n"), 0o644); err != nil {
		t.Fatalf("rewrite master capture: %v", err)
	}

	second, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot second: %v", err)
	}

	if first.Actors["master"].PaneHash == second.Actors["master"].PaneHash {
		t.Fatalf("pane hash did not change: %q", second.Actors["master"].PaneHash)
	}
	if first.Actors["master"].LastOutputChangeAt == second.Actors["master"].LastOutputChangeAt {
		t.Fatalf("last_output_change_at did not change: first=%q second=%q", first.Actors["master"].LastOutputChangeAt, second.Actors["master"].LastOutputChangeAt)
	}
}

func TestActivitySnapshotContainsNoJudgmentFields(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master is waiting\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	snapshot, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot: %v", err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, unwanted := range []string{
		"recommendation",
		"recommended",
		"next_step",
		"should_verify",
		"done",
		"stuck",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("activity snapshot should not contain %q: %s", unwanted, text)
		}
	}
}
