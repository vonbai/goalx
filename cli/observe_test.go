package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestObserveShowsRunRuntimeStateAndRunStatusRecord(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	runState := `{"version":1,"run":"guidance-run","mode":"develop","active":true,"updated_at":"2026-03-25T00:00:00Z"}`
	if err := os.WriteFile(RunRuntimeStatePath(runDir), []byte(runState), 0o644); err != nil {
		t.Fatalf("write run runtime state: %v", err)
	}
	runStatus := `{"version":1,"phase":"working","required_remaining":2,"active_sessions":["session-1"],"updated_at":"2026-03-28T10:00:00Z"}`
	if err := os.WriteFile(RunStatusPath(runDir), []byte(runStatus), 0o644); err != nil {
		t.Fatalf("write run status record: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### Run runtime state",
		runState,
		"### Run status record",
		runStatus,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "### Status (from master)") {
		t.Fatalf("observe output still uses stale status heading:\n%s", out)
	}
}

func TestRefreshDisplayFactsWritesEvolveFactsOnlyForEvolveRun(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	appendExperimentEventForTest(t, runDir, `{"version":1,"kind":"experiment.created","at":"2026-03-29T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-29T10:00:00Z"}}`)

	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	rc := &RunContext{
		Name:        cfg.Name,
		RunDir:      runDir,
		TmuxSession: goalx.TmuxSessionName(repo, cfg.Name),
		ProjectRoot: repo,
		Config:      cfg,
	}
	if err := refreshDisplayFacts(rc); err != nil {
		t.Fatalf("refreshDisplayFacts: %v", err)
	}
	if _, err := os.Stat(EvolveFactsPath(runDir)); err != nil {
		t.Fatalf("expected evolve facts file, stat err = %v", err)
	}

	repo2, runDir2, cfg2, _ := writeGuidanceRunFixture(t)
	if err := os.WriteFile(filepath.Join(fakeBin, "tmux"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("rewrite fake tmux: %v", err)
	}
	rc2 := &RunContext{
		Name:        cfg2.Name,
		RunDir:      runDir2,
		TmuxSession: goalx.TmuxSessionName(repo2, cfg2.Name),
		ProjectRoot: repo2,
		Config:      cfg2,
	}
	if err := refreshDisplayFacts(rc2); err != nil {
		t.Fatalf("refreshDisplayFacts non-evolve: %v", err)
	}
	if _, err := os.Stat(EvolveFactsPath(runDir2)); !os.IsNotExist(err) {
		t.Fatalf("expected no evolve facts for non-evolve run, stat err = %v", err)
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
	if err := os.WriteFile(sessionCapture, []byte("Messages to be submitted after next tool call\n"), 0o644); err != nil {
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
		if err := Observe(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### session-1",
		"Queue: unread=1 cursor=0/1",
		"submit_at=2026-03-25T00:00:00Z",
		"transport=queued",
		"accepted_at=2026-03-25T00:00:01Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsSessionLaunchFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("queued messages\n"), 0o644); err != nil {
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
		State: "active",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Launch: mode=develop engine=codex/gpt-5.4-mini effort=high/xhigh",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsSessionTransportFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("queued messages\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "research", goalx.ModeResearch, "claude-code", "opus", goalx.EffortHigh, "high", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "active",
		Mode:  string(goalx.ModeResearch),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{
				DeliveryID:     "del-1",
				DedupeKey:      "session-wake:session-1",
				Status:         "accepted",
				Target:         "gx-demo:session-1",
				AttemptedAt:    "2026-03-25T00:00:01Z",
				AcceptedAt:     "2026-03-25T00:00:02Z",
				SubmitMode:     "payload_enter",
				TransportState: "queued",
			},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				TransportState: "buffered_input",
				LastSubmitMode: "stale_mode",
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Transport: state=queued",
		"queued_message_visible=true",
		"submit_mode=payload_enter",
		"submit_at=2026-03-25T00:00:01Z",
		"accepted_at=2026-03-25T00:00:02Z",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsProviderDialogFactsForMasterAndSession(t *testing.T) {
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
		State: "active",
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### master",
		"Queue: unread=0 cursor=0/0 transport=provider_dialog dialog=permission_prompt",
		`dialog_hint="Needs your permission"`,
		"provider_dialog_visible=true provider_dialog_kind=permission_prompt",
		"### session-1",
		"Queue: unread=0 cursor=0/0 transport=provider_dialog dialog=auth_prompt",
		`dialog_hint="Please authenticate in browser"`,
		"provider_dialog_visible=true provider_dialog_kind=auth_prompt",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{
		"provider_capability=",
		"provider_native=",
		"provider_limit=",
	} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("observe output should omit %q:\n%s", unwanted, out)
		}
	}
}

func TestObserveShowsMemoryContextPresenceFact(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:                "mem_observe_memory",
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Memory:",
		"query_present=true",
		"context_present=true",
		"built_at=",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(strings.ToLower(out), "recommended") {
		t.Fatalf("observe output should stay factual:\n%s", out)
	}
}

func TestObserveWarnsAboutEvolveManagementGapsAndMissingCloseoutArtifacts(t *testing.T) {
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
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
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Potential evolve stall:") {
		t.Fatalf("observe output should not use legacy evolve stall advisory:\n%s", out)
	}
}

func TestObserveWarnsAboutRunStatusGoalDrift(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{
				ID:     "req-1",
				Text:   "ship feature",
				Source: goalItemSourceUser,
				Role:   goalItemRoleOutcome,
				State:  goalItemStateOpen,
			},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"review","required_remaining":0,"updated_at":"2026-03-28T10:10:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"Status drift:",
		"status_required_remaining=0",
		"goal_required_remaining=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsEvolveSummaryOnlyForEvolveRuns(t *testing.T) {
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### Evolve",
		"frontier_state=active",
		"best_experiment_id=exp-1",
		"open_candidate_count=2",
		"open_candidate_ids=exp-1,exp-2",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}

	repo2, _, cfg2, _ := writeGuidanceRunFixture(t)
	out2 := captureStdout(t, func() {
		if err := Observe(repo2, []string{"--run", cfg2.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})
	if strings.Contains(out2, "### Evolve") {
		t.Fatalf("observe output unexpectedly exposed evolve section outside evolve:\n%s", out2)
	}
}

func TestObserveShowsExperimentLineageFacts(t *testing.T) {
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
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
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsExplicitCoverageFacts(t *testing.T) {
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
	installGuidanceFakeTmux(t, []string{"session-1"})

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
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-9",
				ExecutionState: coordinationRequiredExecutionStateProbing,
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
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-4", State: "idle", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-5", State: "parked", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-5: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Coverage:",
		"open_required=req-1,req-2",
		"unmapped_required=req-2",
		"session_owner_missing=req-1",
		"probing_required=req-1",
		"idle_reusable=session-4",
		"parked_reusable=session-5",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsBlockedRequiredFrontierAdvisory(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n❯\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("session pane\n❯\n"), 0o644); err != nil {
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
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-1",
				ExecutionState: coordinationRequiredExecutionStateProbing,
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Attention:",
		"session-1:transport_blocked",
		"### advisories",
		"Target attention:",
		"Required frontier:",
		"req-1 owner=session-1 execution_state=probing owner_attention=transport_blocked",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsBudgetFactsAndExhaustionAdvisory(t *testing.T) {
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Budget: max_duration=1h0m0s",
		"exhausted=true",
		"Budget exhausted:",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsBlockedRequiredFrontierDetailAdvisory(t *testing.T) {
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
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "session-1",
				ExecutionState: coordinationRequiredExecutionStateProbing,
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Required frontier:",
		"req-1 owner=session-1",
		"execution_state=probing",
		"owner_attention=transport_blocked",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsOperationsBeforeMasterSection(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedDraftObjectiveContractFixture(t, runDir)
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n❯\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)
	if err := SaveControlOperationsState(ControlOperationsPath(runDir), &ControlOperationsState{
		Version: 1,
		Targets: map[string]ControlOperationTarget{
			SessionDispatchOperationKey("session-2"): {
				Kind:              ControlOperationKindSessionDispatch,
				State:             ControlOperationStateHandshaking,
				Summary:           "waiting for first transport frame before publish",
				PendingConditions: []string{"transport_first_frame"},
			},
		},
	}); err != nil {
		t.Fatalf("SaveControlOperationsState: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Operations:",
		"### operations",
		"run.boundary state=awaiting_agent",
		"session-2 state=handshaking",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
	if strings.Index(out, "### operations") > strings.Index(out, "### master") {
		t.Fatalf("operations section should appear before master section:\n%s", out)
	}
}

func TestObserveWarnsAboutMasterOrphanedAndPrematureBlockedFrontierFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"version":1,"phase":"working","required_remaining":2,"active_sessions":[],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Required: []GoalItem{
			{ID: "req-1", Text: "finish integration", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
			{ID: "req-2", Text: "verify remote system", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}
	if err := SaveCoordinationState(CoordinationPath(runDir), &CoordinationState{
		Version: 1,
		Required: map[string]CoordinationRequiredItem{
			"req-1": {
				Owner:          "master",
				ExecutionState: coordinationRequiredExecutionStateProbing,
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceActive,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfacePending,
					WebResearch:    coordinationRequiredSurfacePending,
					ExternalSystem: coordinationRequiredSurfaceNotApplicable,
				},
			},
			"req-2": {
				Owner:          "master",
				ExecutionState: coordinationRequiredExecutionStateBlocked,
				BlockedBy:      "claimed blocker before runtime exhausted",
				Surfaces: CoordinationRequiredSurfaces{
					Repo:           coordinationRequiredSurfaceExhausted,
					Runtime:        coordinationRequiredSurfacePending,
					RunArtifacts:   coordinationRequiredSurfaceExhausted,
					WebResearch:    coordinationRequiredSurfaceExhausted,
					ExternalSystem: coordinationRequiredSurfaceUnreachable,
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{Name: "session-4", State: "idle", Mode: string(goalx.ModeDevelop)}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState session-4: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"Required frontier facts:",
		"master_orphaned=req-1",
		"premature_blocked=req-2",
		"reusable_sessions=session-4",
		"Required frontier:",
		"req-1 owner=master execution_state=probing",
		"req-2 owner=master execution_state=blocked blocked_by=claimed blocker before runtime exhausted",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveShowsSharedRunRootWorktreeSummary(t *testing.T) {
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
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	if !strings.Contains(out, "Worktree: shared run-root worktree") {
		t.Fatalf("observe output missing shared run-root worktree summary:\n%s", out)
	}
}
