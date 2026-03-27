package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
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
		"parallel_opportunity",
		"should_dispatch",
		"serial_bottleneck",
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("activity snapshot should not contain %q: %s", unwanted, text)
		}
	}
}

func TestBuildActivitySnapshotIncludesCoverageFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
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
	installGuidanceFakeTmux(t, []string{"session-1", "session-2"})

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "first open item", State: goalItemStateOpen},
			{ID: "req-2", Text: "second open item", State: goalItemStateOpen},
			{ID: "req-3", Text: "claimed item", State: goalItemStateClaimed, EvidencePaths: []string{"/tmp/evidence.txt"}},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Owners: map[string]string{
			"req-1": "session-9",
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "idle", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-1: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-2", State: "parked", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-2: %v", err)
	}

	snapshot, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot: %v", err)
	}

	if !snapshot.Coverage.OwnersPresent {
		t.Fatal("coverage owners_present = false, want true")
	}
	if got, want := snapshot.Coverage.OpenRequiredIDs, []string{"req-1", "req-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage open_required_ids = %v, want %v", got, want)
	}
	if got, want := snapshot.Coverage.OwnedOpenIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage owned_open_ids = %v, want %v", got, want)
	}
	if got, want := snapshot.Coverage.UnmappedOpenIDs, []string{"req-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage unmapped_open_ids = %v, want %v", got, want)
	}
	if got, want := snapshot.Coverage.OwnerSessionMissingIDs, []string{"req-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage owner_session_missing_ids = %v, want %v", got, want)
	}
	if got, want := snapshot.Coverage.IdleReusableSessions, []string{"session-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage idle_reusable_sessions = %v, want %v", got, want)
	}
	if got, want := snapshot.Coverage.ParkedReusableSessions, []string{"session-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage parked_reusable_sessions = %v, want %v", got, want)
	}
}

func TestBuildActivitySnapshotSerializesCoverageUnknownExplicitly(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	snapshot, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot: %v", err)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, want := range []string{
		`"owners_present":false`,
		`"open_required_ids":["req-1"]`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("activity snapshot missing %q: %s", want, text)
		}
	}
}

func TestBuildActivitySnapshotIncludesSessionQueueFacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("session\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "session-wake:session-1", Status: "sent", Target: "gx-demo:session-1", AttemptedAt: "2026-03-25T00:00:00Z", AcceptedAt: "2026-03-25T00:00:00Z"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				Target:                "session-1",
				Window:                "session-1",
				Engine:                "claude-code",
				TransportState:        "sent",
				QueuedMessageVisible:  true,
				LastSubmitAttemptAt:   "2026-03-25T00:00:00Z",
				LastTransportAcceptAt: "2026-03-25T00:00:00Z",
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	snapshot, err := BuildActivitySnapshot(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildActivitySnapshot: %v", err)
	}

	session := snapshot.Sessions["session-1"]
	if session.InboxLastID != 1 || session.CursorLastSeenID != 0 || session.Unread != 1 {
		t.Fatalf("unexpected session queue facts: %+v", session)
	}
	if session.TransportState != "sent" || !session.QueuedMessageVisible {
		t.Fatalf("unexpected session transport facts: %+v", session)
	}
	if session.LastSubmitAttemptAt != "2026-03-25T00:00:00Z" || session.LastTransportAcceptAt != "2026-03-25T00:00:00Z" {
		t.Fatalf("unexpected session transport timestamps: %+v", session)
	}
}
