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
		"run_status=stranded",
		"unread_inbox=2",
		"master_lease=healthy",
		"sidecar_lease=expired",
		"Targets: sidecar missing (lease_expired)",
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

func TestStatusPrintsObjectiveIntegritySummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), &ObjectiveContract{
		Version:       1,
		ObjectiveHash: "sha256:demo",
		State:         objectiveContractStateLocked,
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-goal",
				Text:             "ship the outcome",
				Kind:             objectiveClauseKindDelivery,
				SourceExcerpt:    "ship the outcome",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
			{
				ID:               "ucl-accept",
				Text:             "verify the outcome",
				Kind:             objectiveClauseKindVerification,
				SourceExcerpt:    "verify the outcome",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceAcceptance},
			},
		},
	}); err != nil {
		t.Fatalf("SaveObjectiveContract: %v", err)
	}
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship the outcome", Source: goalItemSourceUser, Role: goalItemRoleOutcome, Covers: []string{"ucl-goal"}, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(runDir), &AcceptanceState{
		Version:     2,
		GoalVersion: 1,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Label: "verify", Command: "printf ok", Covers: []string{"ucl-accept"}, State: acceptanceCheckStateActive},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})
	for _, want := range []string{
		"Objective: contract_state=locked",
		"goal_coverage=1/1",
		"acceptance_coverage=1/1",
		"integrity_ok=true",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsMemoryContextPresenceFact(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:                "mem_status_memory",
				Kind:              MemoryKindFact,
				Statement:         "provider is cloudflare",
				Selectors:         map[string]string{"project_id": goalx.ProjectID(repo)},
				VerificationState: "validated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-27T00:00:00Z",
				UpdatedAt:         "2026-03-27T00:00:00Z",
			},
		},
	})

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Memory:",
		"query_present=true",
		"context_present=true",
		"built_at=",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "recommended") {
		t.Fatalf("status output should stay factual:\n%s", out)
	}

	if _, err := os.Stat(MemoryQueryPath(runDir)); err != nil {
		t.Fatalf("memory query path missing: %v", err)
	}
	if _, err := os.Stat(MemoryContextPath(runDir)); err != nil {
		t.Fatalf("memory context path missing: %v", err)
	}
}

func TestStatusShowsSessionTransportFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("❯ [[GOALX_WAKE_CHECK_INBOX]]\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
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

func TestStatusShowsBudgetFactsAndExhaustionAdvisory(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	startedAt := time.Now().UTC().Add(-90 * time.Minute).Truncate(time.Second)
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       cfg.Name,
		Mode:      string(cfg.Mode),
		Active:    true,
		StartedAt: startedAt.Format(time.RFC3339),
		UpdatedAt: startedAt.Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Budget: max_duration=1h0m0s",
		"deadline_at=",
		"exhausted=true",
		"Budget exhausted:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusDoesNotSurfaceAckSessionAsSessionLifecycleState(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
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
	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte("{\"round\":1,\"status\":\"ack-session\",\"desc\":\"read inbox\"}\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "session-1") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			t.Fatalf("session line too short: %q", line)
		}
		if fields[2] != "active" {
			t.Fatalf("session lifecycle status = %q, want active in line %q", fields[2], line)
		}
		return
	}
	t.Fatalf("status output missing session-1 line:\n%s", out)
}

func TestStatusShowsProviderDialogFactsForMasterAndSession(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Master.Engine = "claude-code"
	cfg.Master.Model = "opus"
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("queued messages\nNeeds your permission\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("❯ [[GOALX_WAKE_CHECK_INBOX]]\nPlease authenticate in browser\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
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
				TransportState:        "buffered",
				ProviderDialogVisible: true,
				ProviderDialogKind:    "stale_dialog",
				ProviderDialogHint:    "stale dialog",
			},
			"session-1": {
				TransportState:        "sent",
				ProviderDialogVisible: true,
				ProviderDialogKind:    "stale_dialog",
				ProviderDialogHint:    "stale dialog",
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
		"provider_capability=tui",
		"provider_native=skills,plugins,mcp",
		"provider_limit=claude_root_no_bypass",
		"provider_native=skills,mcp",
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

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
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
	if _, err := EnsureRuntimeState(runDir, &cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
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
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Messages to be submitted after next tool call\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := os.WriteFile(JournalPath(runDir, "session-1"), []byte(`{"round":2,"desc":"awaiting master","status":"idle"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "take the next slice"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "session-wake:session-1", Status: "accepted", Target: "gx-demo:session-1", AttemptedAt: "2026-03-25T00:00:00Z", AcceptedAt: "2026-03-25T00:00:01Z", TransportState: "queued"},
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
				TransportState:        "buffered_input",
				LastSubmitAttemptAt:   "1999-01-01T00:00:00Z",
				LastTransportAcceptAt: "1999-01-01T00:00:01Z",
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
		"transport=queued",
		"accepted_at=2026-03-25T00:00:01Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusWarnsAboutEvolveManagementGapsAndMissingCloseoutArtifacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"review","required_remaining":0,"active_sessions":[],"updated_at":"2026-03-28T10:10:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`)

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"review_without_managed_stop:",
		"frontier_state=active",
		"open_candidate_count=1",
		"Closeout artifacts missing:",
		"required_remaining=0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Potential evolve stall:") {
		t.Fatalf("status output should not use legacy evolve stall advisory:\n%s", out)
	}
}

func TestStatusWarnsAboutMissingStopOrDispatchInEvolve(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"working","required_remaining":1,"active_sessions":[],"updated_at":"2026-03-28T10:10:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`)

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"missing_stop_or_dispatch:",
		"frontier_state=active",
		"open_candidate_count=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusWarnsAboutUnclosedAbandonedCandidateInEvolve(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"review","required_remaining":1,"active_sessions":[],"updated_at":"2026-03-28T10:10:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.integrated","at":"2026-03-28T10:05:00Z","actor":"master","body":{"integration_id":"int-1","result_experiment_id":"exp-2","source_experiment_ids":["exp-1"],"method":"keep","recorded_at":"2026-03-28T10:05:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"evolve.stopped","at":"2026-03-28T10:06:00Z","actor":"master","body":{"reason_code":"diminishing_returns","reason":"winner is already clear","stopped_at":"2026-03-28T10:06:00Z"}}`)
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:             1,
		CurrentExperimentID: "exp-2",
		CurrentBranch:       "goalx/guidance-run/root",
		CurrentCommit:       "abc123",
		UpdatedAt:           "2026-03-28T10:06:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"unclosed_abandoned_candidate:",
		"best_experiment_id=exp-2",
		"open_candidate_ids=exp-1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusOmitsEvolveManagementAdvisoriesOutsideEvolve(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"review","required_remaining":1,"active_sessions":[],"updated_at":"2026-03-28T10:10:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`)

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, blocked := range []string{
		"missing_stop_or_dispatch:",
		"review_without_managed_stop:",
		"unclosed_abandoned_candidate:",
	} {
		if strings.Contains(out, blocked) {
			t.Fatalf("status output unexpectedly exposed evolve advisory %q:\n%s", blocked, out)
		}
	}
}

func TestStatusShowsEvolveSummaryOnlyForEvolveRuns(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`)
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:02:00Z","actor":"master","body":{"experiment_id":"exp-2","created_at":"2026-03-28T10:02:00Z"}}`)
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:             1,
		CurrentExperimentID: "exp-1",
		CurrentBranch:       "goalx/guidance-run/root",
		CurrentCommit:       "abc123",
		UpdatedAt:           "2026-03-28T10:02:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Evolve:",
		"frontier_state=active",
		"best_experiment_id=exp-1",
		"open_candidate_count=2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}

	repo2, _, cfg2, _ := writeGuidanceRunFixture(t)
	out2 := captureStdout(t, func() {
		if err := Status(repo2, []string{"--run", cfg2.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})
	if strings.Contains(out2, "Evolve:") {
		t.Fatalf("status output unexpectedly exposed evolve summary outside evolve:\n%s", out2)
	}
}

func TestStatusShowsExperimentLineageFacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(ExperimentsLogPath(runDir), []byte(
		"{\"version\":1,\"kind\":\"experiment.created\",\"at\":\"2026-03-28T10:00:00Z\",\"actor\":\"goalx\",\"body\":{\"experiment_id\":\"exp-1\",\"created_at\":\"2026-03-28T10:00:00Z\"}}\n"+
			"{\"version\":1,\"kind\":\"experiment.integrated\",\"at\":\"2026-03-28T10:05:00Z\",\"actor\":\"goalx\",\"body\":{\"integration_id\":\"int-1\",\"result_experiment_id\":\"exp-2\",\"source_experiment_ids\":[\"exp-1\"],\"method\":\"keep\",\"recorded_at\":\"2026-03-28T10:05:00Z\"}}\n"), 0o644); err != nil {
		t.Fatalf("write experiments log: %v", err)
	}
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:                 1,
		CurrentExperimentID:     "exp-2",
		CurrentBranch:           "goalx/guidance-run/root",
		CurrentCommit:           "abc123",
		LastIntegrationID:       "int-1",
		LastMethod:              "keep",
		LastSourceExperimentIDs: []string{"exp-1"},
		UpdatedAt:               "2026-03-28T10:05:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Experiments:",
		"current=exp-2",
		"entries=2",
		"last_record_at=2026-03-28T10:05:00Z",
		"last_method=keep",
		"sources=exp-1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsCoverageUnknownWhenOwnersMissing(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "ship feature", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Coverage:",
		"coverage=unknown",
		"open_required=req-1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsExplicitCoverageFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "first open item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "second open item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Owners: map[string]string{
			"req-1": "session-9",
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-4", State: "idle", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-5", State: "parked", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-5: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Coverage:",
		"open_required=req-1,req-2",
		"unmapped_open=req-2",
		"owner_session_missing=req-1",
		"idle_reusable=session-4",
		"parked_reusable=session-5",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusWarnsAboutExplicitCoverageGapOutsideEvolve(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"working","required_remaining":2,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "first open item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "second open item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Owners: map[string]string{
			"req-1": "session-9",
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-4", State: "idle", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"Coverage facts:",
		"unmapped_open=req-2",
		"owner_session_missing=req-1",
		"reusable_sessions=session-4",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsBlockedOwnerAttentionFacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master prompt\n❯\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("session prompt\n❯\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})
	for _, holder := range []string{"master", "sidecar", "session-1"} {
		if err := RenewControlLease(runDir, holder, meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
			t.Fatalf("RenewControlLease %s: %v", holder, err)
		}
	}
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{{ID: "req-1", Text: "blocked item", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen}},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Owners:  map[string]string{"req-1": "session-1"},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 24},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "session-1", "tell", "master", "continue batch 2", false); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TransportTargetFacts{
			"session-1": {Target: "session-1", Window: "session-1", Engine: "codex", TransportState: string(TUIStateIdlePrompt)},
			"master":    {Target: "master", Window: "master", Engine: "codex", TransportState: string(TUIStateIdlePrompt)},
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
		"Attention:",
		"session-1:transport_blocked",
		"### advisories",
		"Target attention:",
		"Owner attention:",
		"req-1 owner=session-1 state=transport_blocked",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsForkedSessionWorktreeLineage(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedForkedWorktreeLineageFixture(t, repo, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	session1Capture := filepath.Join(t.TempDir(), "session-1-pane.txt")
	session2Capture := filepath.Join(t.TempDir(), "session-2-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master prompt\n❯\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(session1Capture, []byte("session-1 prompt\n❯\n"), 0o644); err != nil {
		t.Fatalf("write session-1 capture: %v", err)
	}
	if err := os.WriteFile(session2Capture, []byte("session-2 prompt\n❯\n"), 0o644); err != nil {
		t.Fatalf("write session-2 capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", session1Capture)
	t.Setenv("TMUX_SESSION2_CAPTURE", session2Capture)
	installGuidanceFakeTmux(t, []string{"session-1", "session-2"})
	for _, holder := range []string{"master", "sidecar", "session-1", "session-2"} {
		if err := RenewControlLease(runDir, holder, meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
			t.Fatalf("RenewControlLease %s: %v", holder, err)
		}
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	for _, want := range []string{
		"Run worktree: branch=goalx/" + cfg.Name + "/root parent=source-root",
		"branch=goalx/" + cfg.Name + "/1 parent=run-root",
		"branch=goalx/" + cfg.Name + "/2 parent=session-1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusWarnsAboutBlockedOwnerAttention(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("❯\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{{ID: "req-1", Text: "ship UI polish", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen}},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Owners:  map[string]string{"req-1": "session-1"},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if _, err := AppendControlInboxMessage(runDir, "session-1", "develop", "master", "polish the source detail page"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 24},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "session-wake:session-1", Status: "accepted", Target: "gx-demo:session-1", AttemptedAt: time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339), AcceptedAt: time.Now().UTC().Add(-20 * time.Minute).Format(time.RFC3339), TransportState: string(TUIStateQueued)},
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
		"Owner attention:",
		"req-1 owner=session-1",
		"state=transport_blocked",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusShowsActiveIdleSessionAttention(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("idle prompt\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "idle", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 2},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TransportTargetFacts{
			"master":    {Target: "master", Window: "master", Engine: "codex", TransportState: string(TUIStateIdlePrompt)},
			"session-1": {Target: "session-1", Window: "session-1", Engine: "codex", TransportState: string(TUIStateIdlePrompt)},
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
		"Attention:",
		"session-1:active_idle",
		"Target attention:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestStatusPrefersRuntimeStateOverStaleCoordinationState(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "parked", Scope: "stale parked scope"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:       "session-1",
		State:      "active",
		Mode:       string(goalx.ModeDevelop),
		OwnerScope: "live active scope",
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	if strings.Contains(out, "parked: stale parked scope") {
		t.Fatalf("status should ignore stale coordination lifecycle summary:\n%s", out)
	}
	if !strings.Contains(out, "session-1  1           active") {
		t.Fatalf("status output should keep runtime-owned active lifecycle:\n%s", out)
	}
}

func TestStatusShowsSharedRunRootWorktreeSummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("shared session pane\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "shared slice", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortMedium, "medium", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
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

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})

	if !strings.Contains(out, "shared run-root worktree") {
		t.Fatalf("status output missing shared run-root worktree summary:\n%s", out)
	}
}
