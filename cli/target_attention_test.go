package cli

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestBuildTargetAttentionFactsMarksUnreadCursorLaggedIdleSessionBlocked(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

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
	if err := SaveMasterCursorState(SessionCursorPath(runDir, "session-1"), &MasterCursorState{
		LastSeenID: 0,
		UpdatedAt:  time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveMasterCursorState: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				Target:         "session-1",
				Window:         "session-1",
				Engine:         "codex",
				TransportState: string(TUIStateIdlePrompt),
				LastSampleAt:   time.Now().UTC().Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"session-1": {Target: "session-1", Kind: "session", State: TargetPresencePresent},
		},
		Sessions: map[string]ActivitySession{
			"session-1": {LastOutputChangeAt: time.Now().Add(-24 * time.Minute).UTC().Format(time.RFC3339)},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	got := attention["session-1"]
	if got.AttentionState != TargetAttentionTransportBlocked {
		t.Fatalf("attention_state = %q, want %q (%+v)", got.AttentionState, TargetAttentionTransportBlocked, got)
	}
	if got.Unread != 1 || got.CursorLag != 1 {
		t.Fatalf("queue facts wrong: %+v", got)
	}
	if got.JournalStaleMinutes != 24 {
		t.Fatalf("journal stale = %d, want 24", got.JournalStaleMinutes)
	}
	_ = repo
}

func TestBuildTargetAttentionFactsMarksAcceptedWorkingSessionBlockedAfterGrace(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 3},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "session-1", "tell", "master", "next slice", false); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				Target:                "session-1",
				Window:                "session-1",
				Engine:                "codex",
				TransportState:        string(TUIStateWorking),
				LastSampleAt:          time.Now().UTC().Format(time.RFC3339),
				LastTransportAcceptAt: time.Now().Add(-16 * time.Minute).UTC().Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"session-1": {Target: "session-1", Kind: "session", State: TargetPresencePresent},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	got := attention["session-1"]
	if got.AttentionState != TargetAttentionTransportBlocked {
		t.Fatalf("attention_state = %q, want %q (%+v)", got.AttentionState, TargetAttentionTransportBlocked, got)
	}
	if !got.DeliveryGraceExpired {
		t.Fatalf("delivery grace should be expired: %+v", got)
	}
}

func TestBuildTargetAttentionFactsMarksActiveIdleOwnerForMasterFollowUp(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "idle", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 3},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				Target:         "session-1",
				Window:         "session-1",
				Engine:         "codex",
				TransportState: string(TUIStateIdlePrompt),
				LastSampleAt:   time.Now().UTC().Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"session-1": {Target: "session-1", State: TargetPresencePresent},
		},
		Sessions: map[string]ActivitySession{
			"session-1": {LastOutputChangeAt: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339)},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	if got := attention["session-1"].AttentionState; got != TargetAttentionActiveIdle {
		t.Fatalf("attention_state = %q, want %q", got, TargetAttentionActiveIdle)
	}
}

func TestBuildTargetAttentionFactsDoesNotMarkAcceptedWorkingOwnerActiveIdle(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "idle", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 3},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				Target:                "session-1",
				Window:                "session-1",
				Engine:                "codex",
				TransportState:        string(TUIStateWorking),
				LastSampleAt:          time.Now().UTC().Format(time.RFC3339),
				LastTransportAcceptAt: time.Now().UTC().Format(time.RFC3339),
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"session-1": {Target: "session-1", State: TargetPresencePresent},
		},
		Sessions: map[string]ActivitySession{
			"session-1": {LastOutputChangeAt: time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	if got := attention["session-1"].AttentionState; got != TargetAttentionHealthy {
		t.Fatalf("attention_state = %q, want %q", got, TargetAttentionHealthy)
	}
}

func TestBuildTargetAttentionFactsIgnoresParkedReusableSessions(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "parked"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "parked", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 90},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}

	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"session-1": {Target: "session-1", State: TargetPresenceParked},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	if got := attention["session-1"].AttentionState; got != TargetAttentionHealthy {
		t.Fatalf("attention_state = %q, want %q", got, TargetAttentionHealthy)
	}
}

func TestBuildTargetAttentionFactsIgnoresStoppedSessions(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "stopped", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true},
		Sessions: map[string]LivenessEntry{
			"session-1": {Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 90},
		},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}

	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"session-1": {Target: "session-1", State: TargetPresenceInactive},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	if got := attention["session-1"].AttentionState; got != TargetAttentionHealthy {
		t.Fatalf("attention_state = %q, want %q", got, TargetAttentionHealthy)
	}
}

func TestBuildTargetAttentionFactsDoesNotMarkMasterBlockedWhenPaneIsStillChanging(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)

	if err := SaveLivenessState(runDir, &LivenessState{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Master:    LivenessEntry{Lease: "healthy", PIDAlive: true, HasWorktree: true, JournalStaleMinutes: 25},
	}); err != nil {
		t.Fatalf("SaveLivenessState: %v", err)
	}
	snapshot := &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets: map[string]TargetPresenceFacts{
			"master": {Target: "master", State: TargetPresencePresent},
		},
		Actors: map[string]ActivityActor{
			"master": {LastOutputChangeAt: time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)},
		},
	}

	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	if got := attention["master"].AttentionState; got == TargetAttentionProgressBlocked {
		t.Fatalf("attention_state = %q, want not progress_blocked", got)
	}
}

func TestBuildTargetAttentionFactsTreatsFreshWorktreeProgressAsHealthy(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Sessions: map[string]CoordinationSession{
			"session-1": {State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-1", State: "active", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
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

	payload := map[string]any{
		"version":    1,
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"targets": map[string]any{
			"session-1": map[string]any{
				"target": "session-1",
				"state":  TargetPresencePresent,
			},
		},
		"sessions": map[string]any{
			"session-1": map[string]any{
				"dirty_files":             9,
				"last_output_change_at":   time.Now().Add(-24 * time.Minute).UTC().Format(time.RFC3339),
				"last_worktree_change_at": time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339),
			},
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal activity payload: %v", err)
	}
	if err := os.WriteFile(ActivityPath(runDir), append(data, '\n'), 0o644); err != nil {
		t.Fatalf("write activity payload: %v", err)
	}

	snapshot, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	attention, err := BuildTargetAttentionFacts(runDir, snapshot)
	if err != nil {
		t.Fatalf("BuildTargetAttentionFacts: %v", err)
	}
	if got := attention["session-1"].AttentionState; got != TargetAttentionHealthy {
		t.Fatalf("attention_state = %q, want %q", got, TargetAttentionHealthy)
	}
}

func TestBuildRequiredCoverageDoesNotCollapseTargetAttentionIntoFrontierFacts(t *testing.T) {
	_, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-2", State: "done", Mode: string(goalx.ModeWorker)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-2: %v", err)
	}

	if err := writeBoundaryFixture(t, runDir, &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "blocked owner", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "risky owner", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-1",
				ExecutionState: coordinationRequiredExecutionStateActive,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceAvailable,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
			"req-2": {
				Owner:          "session-2",
				ExecutionState: coordinationRequiredExecutionStateActive,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceAvailable,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := SaveActivitySnapshot(runDir, &ActivitySnapshot{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Attention: map[string]TargetAttentionFacts{
			"session-1": {Target: "session-1", AttentionState: TargetAttentionTransportBlocked},
			"session-2": {Target: "session-2", AttentionState: TargetAttentionOwnershipRisky},
		},
	}); err != nil {
		t.Fatalf("SaveActivitySnapshot: %v", err)
	}

	coverage, err := BuildRequiredCoverage(runDir)
	if err != nil {
		t.Fatalf("BuildRequiredCoverage: %v", err)
	}
	if len(coverage.MappedRequiredIDs) != 2 {
		t.Fatalf("mapped_required_ids = %v, want [req-1 req-2]", coverage.MappedRequiredIDs)
	}
	if len(coverage.BlockedRequiredIDs) != 0 || len(coverage.PrematureBlockedRequiredIDs) != 0 || len(coverage.MasterOrphanedRequiredIDs) != 0 || len(coverage.SessionOwnerMissingIDs) != 0 {
		t.Fatalf("unexpected frontier gaps in coverage: %+v", coverage)
	}
}
