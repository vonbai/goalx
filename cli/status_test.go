package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestStatusHelpDoesNotResolveRun(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Status(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Status --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx status [NAME] [session-N]") {
		t.Fatalf("status help output = %q", out)
	}
}

func TestStatusShowsControlQueueAndLeaseSummary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	cfg := goalx.Config{
		Name:      "status-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, &cfg)
	if err := SaveProjectRegistry(repo, &ProjectRegistry{
		Version:    1,
		FocusedRun: cfg.Name,
		ActiveRuns: map[string]ProjectRunRef{
			cfg.Name: {Name: cfg.Name, State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveProjectRegistry: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:   1,
		Objective: cfg.Objective,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, cfg.Name, repo)
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, &cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	for i := 0; i < 3; i++ {
		if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "do work"); err != nil {
			t.Fatalf("AppendMasterInboxMessage: %v", err)
		}
	}
	if err := SaveMasterCursorState(MasterCursorPath(runDir), &MasterCursorState{LastSeenID: 1}); err != nil {
		t.Fatalf("SaveMasterCursorState: %v", err)
	}
	if err := RenewControlLease(runDir, "master", "run_status", 1, time.Minute, "tmux", 1234); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		t.Fatalf("ExpireControlLease sidecar: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "active",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := RenewControlLease(runDir, "session-1", "run_status", 1, time.Minute, "tmux", 2222); err != nil {
		t.Fatalf("RenewControlLease session-1: %v", err)
	}
	if err := SaveControlReminders(ControlRemindersPath(runDir), &ControlReminders{
		Version: 1,
		Items: []ControlReminder{
			{ReminderID: "rem-1", DedupeKey: "master-wake", Reason: "control-cycle", Target: "gx-demo:master"},
			{ReminderID: "rem-2", DedupeKey: "acked", Reason: "control-cycle", Target: "gx-demo:master", ResolvedAt: "2026-03-23T00:00:00Z"},
		},
	}); err != nil {
		t.Fatalf("SaveControlReminders: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "master-wake", Status: "failed", Target: "gx-demo:master"},
			{DeliveryID: "del-2", DedupeKey: "tell:1", Status: "sent", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Run: status-run",
		"Control:",
		"run_id=" + meta.RunID,
		"epoch=1",
		"charter=ok",
		"run_status=active",
		"unread_inbox=2",
		"master_lease=healthy",
		"sidecar_lease=expired",
		"reminders_due=1",
		"deliveries_failed=1",
		"LEASE",
		"session-1",
		"healthy",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusPrefersCanonicalControlFactsOverStaleActivitySnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "fresh work"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", 1234); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		t.Fatalf("ExpireControlLease sidecar: %v", err)
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
			{DeliveryID: "del-1", DedupeKey: "master-wake", Status: "failed", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}
	if err := SaveActivitySnapshot(runDir, &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Run: ActivityRunInfo{
			ProjectID:   goalx.ProjectID(repo),
			RunName:     cfg.Name,
			RunID:       meta.RunID,
			Epoch:       meta.Epoch,
			TmuxSession: goalx.TmuxSessionName(repo, cfg.Name),
		},
		Queue: ActivityQueue{
			MasterUnread:     7,
			RemindersDue:     3,
			DeliveriesFailed: 2,
		},
		Actors: map[string]ActivityActor{
			"master":  {Lease: "expired"},
			"sidecar": {Lease: "healthy"},
		},
	}); err != nil {
		t.Fatalf("SaveActivitySnapshot: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"run_id=" + meta.RunID,
		"epoch=1",
		"unread_inbox=1",
		"master_lease=healthy",
		"sidecar_lease=expired",
		"reminders_due=1",
		"deliveries_failed=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusDoesNotReviveStaleActivityUnreadWhenCanonicalQueueIsZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "fresh work"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}
	if err := SaveMasterCursorState(MasterCursorPath(runDir), &MasterCursorState{LastSeenID: 1}); err != nil {
		t.Fatalf("SaveMasterCursorState: %v", err)
	}
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", 1234); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		t.Fatalf("ExpireControlLease sidecar: %v", err)
	}
	if err := SaveActivitySnapshot(runDir, &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Run: ActivityRunInfo{
			ProjectID:   goalx.ProjectID(repo),
			RunName:     cfg.Name,
			RunID:       meta.RunID,
			Epoch:       meta.Epoch,
			TmuxSession: goalx.TmuxSessionName(repo, cfg.Name),
		},
		Queue: ActivityQueue{
			MasterUnread:     7,
			RemindersDue:     3,
			DeliveriesFailed: 2,
		},
	}); err != nil {
		t.Fatalf("SaveActivitySnapshot: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"run_id=" + meta.RunID,
		"epoch=1",
		"unread_inbox=0",
		"reminders_due=0",
		"deliveries_failed=0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsSessionTransportFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "build_fast", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	identity.RouteRole = "develop"
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "idle",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				TransportState:    "buffered",
				InputContainsWake: true,
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"transport=buffered",
		"input_wake=true",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsProviderDialogFactsForMasterAndSession(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "build_fast", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "idle",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"master": {
				TransportState:        "sent",
				ProviderDialogVisible: true,
				ProviderDialogKind:    "permission_prompt",
				ProviderDialogHint:    "Needs your permission",
			},
			"session-1": {
				TransportState:        "buffered",
				ProviderDialogVisible: true,
				ProviderDialogKind:    "auth_prompt",
				ProviderDialogHint:    "Please authenticate in browser",
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"dialog=permission_prompt",
		`dialog_hint="Needs your permission"`,
		"dialog=auth_prompt",
		`dialog_hint="Please authenticate in browser"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsSessionLaunchFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "build_fast", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	identity.RouteRole = "develop"
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "active",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte("{\"round\":1,\"status\":\"active\",\"desc\":\"working\"}\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"mode=develop",
		"engine=codex/gpt-5.4-mini",
		"effort=high/xhigh",
		"route=develop/build_fast",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusPrefersLatestJournalStateOverStaleRuntimeActive(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	cfg := goalx.Config{
		Name:      "status-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, &cfg)
	if err := SaveProjectRegistry(repo, &ProjectRegistry{
		Version:    1,
		FocusedRun: cfg.Name,
		ActiveRuns: map[string]ProjectRunRef{
			cfg.Name: {Name: cfg.Name, State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveProjectRegistry: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{Version: 1, ProjectRoot: repo, RunID: "run_status", RootRunID: "run_status", Epoch: 1, ProtocolVersion: 2}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, cfg.Name, repo)
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "active",
		Mode:  string(goalx.ModeResearch),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":2,"desc":"awaiting master","status":"idle"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	if !strings.Contains(out, "session-1  2           idle") {
		t.Fatalf("status output did not surface latest journal idle state:\n%s", out)
	}
}

func TestStatusShowsSessionQueueFacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":2,"desc":"awaiting master","status":"idle"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "session-wake:session-1", Status: "sent", Target: "gx-demo:session-1", AttemptedAt: "2026-03-25T00:00:00Z", AcceptedAt: "2026-03-25T00:00:01Z"},
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
				Engine:                "codex",
				TransportState:        "sent",
				LastSubmitAttemptAt:   "2026-03-25T00:00:00Z",
				LastTransportAcceptAt: "2026-03-25T00:00:01Z",
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"idle",
		"unread=1",
		"cursor=0/1",
		"submit_at=2026-03-25T00:00:00Z",
		"transport=sent",
		"accepted_at=2026-03-25T00:00:01Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}
