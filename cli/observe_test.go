package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestObserveShowsRunRuntimeStateAndRunStatusRecord(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	runState := `{"version":1,"run":"guidance-run","mode":"develop","active":true,"updated_at":"2026-03-25T00:00:00Z"}`
	if err := os.WriteFile(RunRuntimeStatePath(runDir), []byte(runState), 0o644); err != nil {
		t.Fatalf("write run runtime state: %v", err)
	}
	runStatus := `{"phase":"working","required_remaining":2,"active_sessions":["session-1"]}`
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
				TransportState:        "buffered",
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
		"transport=sent",
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

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"Launch: mode=develop engine=codex/gpt-5.4-mini effort=high/xhigh route=develop/build_fast",
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

	identity, err := NewSessionIdentity(runDir, "session-1", "research", goalx.ModeResearch, "claude-code", "opus", goalx.EffortHigh, "high", "", "", goalx.TargetConfig{})
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
				Status:         "sent",
				Target:         "gx-demo:session-1",
				AttemptedAt:    "2026-03-25T00:00:01Z",
				AcceptedAt:     "2026-03-25T00:00:02Z",
				SubmitMode:     "payload_enter",
				TransportState: "sent",
			},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				TransportState: "buffered",
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
		"Transport: state=sent",
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

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4-mini", goalx.EffortHigh, "xhigh", "", "", goalx.TargetConfig{})
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
		"provider_capability=tui",
		"provider_native=skills,plugins,mcp",
		"provider_limit=claude_root_no_bypass",
		"Queue: unread=0 cursor=0/0 transport=sent dialog=permission_prompt",
		`dialog_hint="Needs your permission"`,
		"provider_dialog_visible=true provider_dialog_kind=permission_prompt",
		"### session-1",
		"provider_native=skills,mcp",
		"Queue: unread=0 cursor=0/0 transport=buffered dialog=auth_prompt",
		`dialog_hint="Please authenticate in browser"`,
		"provider_dialog_visible=true provider_dialog_kind=auth_prompt",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveWarnsAboutPotentialEvolveStallAndMissingCloseoutArtifacts(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), []byte(`{"phase":"review","required_remaining":0,"active_sessions":[]}`), 0o644); err != nil {
		t.Fatalf("write status record: %v", err)
	}
	if err := os.WriteFile(EvolutionLogPath(runDir), []byte("{\"trial\":1}\n"), 0o644); err != nil {
		t.Fatalf("write evolution log: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})

	for _, want := range []string{
		"### advisories",
		"Potential evolve stall:",
		"phase=review",
		"active_sessions=0",
		"evolution_entries=1",
		"summary_exists=false",
		"completion_proof_exists=false",
		"Closeout artifacts missing:",
		"required_remaining=0",
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
			{ID: "req-1", Text: "first open item", State: goalItemStateOpen},
			{ID: "req-2", Text: "second open item", State: goalItemStateOpen},
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
		"unmapped_open=req-2",
		"owner_session_missing=req-1",
		"idle_reusable=session-4",
		"parked_reusable=session-5",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}
