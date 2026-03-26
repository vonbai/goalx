package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildTransportFactsMarksCodexBufferedWake(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("› [[GOALX_WAKE_CHECK_INBOX]]\n  gpt-5.4 xhigh\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortHigh, "xhigh", "", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine)
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["session-1"]
	if !got.InputContainsWake {
		t.Fatalf("input_contains_wake = false, want true: %+v", got)
	}
	if got.TransportState != "buffered" {
		t.Fatalf("transport_state = %q, want buffered", got.TransportState)
	}
}

func TestBuildTransportFactsMarksClaudeQueuedWakeAsSent(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("❯ [[GOALX_WAKE_CHECK_INBOX]]\nPress up to edit queued messages\n"), 0o644); err != nil {
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

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine)
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["session-1"]
	if !got.QueuedMessageVisible {
		t.Fatalf("queued_message_visible = false, want true: %+v", got)
	}
	if got.TransportState != "sent" {
		t.Fatalf("transport_state = %q, want sent", got.TransportState)
	}
}

func TestBuildTransportFactsMarksCodexQueuedWakeAsSent(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("• Working (24s)\n• Messages to be submitted after next tool call\n  ↳ [[GOALX_WAKE_CHECK_INBOX]]\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), "codex")
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["master"]
	if !got.QueuedMessageVisible {
		t.Fatalf("queued_message_visible = false, want true: %+v", got)
	}
	if got.TransportState != "sent" {
		t.Fatalf("transport_state = %q, want sent", got.TransportState)
	}
}

func TestSaveAndLoadTransportFactsRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	want := &TransportFacts{
		Version:   1,
		CheckedAt: "2026-03-25T00:00:00Z",
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				Target:            "session-1",
				Engine:            "codex",
				InputContainsWake: true,
				TransportState:    "buffered",
			},
		},
	}
	if err := SaveTransportFacts(runDir, want); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}
	got, err := LoadTransportFacts(TransportFactsPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportFacts: %v", err)
	}
	if got == nil || got.Targets["session-1"].TransportState != "buffered" {
		t.Fatalf("round trip facts = %+v", got)
	}
}

func TestBuildTransportFactsDetectsProviderDialog(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("Authentication required\nPlease authenticate in browser to continue\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", masterCapture)
	installGuidanceFakeTmux(t, nil)

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), "codex")
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["master"]
	if !got.ProviderDialogVisible {
		t.Fatalf("provider_dialog_visible = false, want true: %+v", got)
	}
	if got.ProviderDialogKind != "auth_prompt" {
		t.Fatalf("provider_dialog_kind = %q, want auth_prompt", got.ProviderDialogKind)
	}
	if !strings.Contains(got.ProviderDialogHint, "Please authenticate in browser") {
		t.Fatalf("provider_dialog_hint = %q, want auth line", got.ProviderDialogHint)
	}
}

func TestBuildTransportFactsDetectsCodexSkillChooserDialog(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Use /skills to list available skills\nChoose a skill to continue\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortHigh, "xhigh", "", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine)
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["session-1"]
	if !got.ProviderDialogVisible {
		t.Fatalf("provider_dialog_visible = false, want true: %+v", got)
	}
	if got.ProviderDialogKind != "skill_ui" {
		t.Fatalf("provider_dialog_kind = %q, want skill_ui", got.ProviderDialogKind)
	}
	if !strings.Contains(got.ProviderDialogHint, "Choose a skill") {
		t.Fatalf("provider_dialog_hint = %q, want skill chooser line", got.ProviderDialogHint)
	}
}

func TestBuildTransportFactsDoesNotTreatPassiveSkillsHintAsDialog(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Use /skills to list available skills\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortHigh, "xhigh", "", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine)
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["session-1"]
	if got.ProviderDialogVisible {
		t.Fatalf("provider_dialog_visible = true, want false: %+v", got)
	}
	if got.ProviderDialogKind != "" || got.ProviderDialogHint != "" {
		t.Fatalf("provider dialog details = %+v, want empty", got)
	}
}

func TestBuildTransportFactsDetectsCapacityPickerDialog(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Choose a model to continue\nModel capacity picker\n"), 0o644); err != nil {
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

	facts, err := BuildTransportFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine)
	if err != nil {
		t.Fatalf("BuildTransportFacts: %v", err)
	}
	got := facts.Targets["session-1"]
	if !got.ProviderDialogVisible {
		t.Fatalf("provider_dialog_visible = false, want true: %+v", got)
	}
	if got.ProviderDialogKind != "capacity_picker" {
		t.Fatalf("provider_dialog_kind = %q, want capacity_picker", got.ProviderDialogKind)
	}
	if !strings.Contains(got.ProviderDialogHint, "capacity picker") {
		t.Fatalf("provider_dialog_hint = %q, want model picker line", got.ProviderDialogHint)
	}
}

func TestTransportTargetFactsSummaryFormatsBufferedQueuedAndDialog(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				TransportState:        "buffered",
				InputContainsWake:     true,
				QueuedMessageVisible:  true,
				ProviderDialogVisible: true,
				ProviderDialogKind:    "auth_prompt",
				ProviderDialogHint:    "Please authenticate in browser",
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}
	got := transportTargetFactsSummary(runDir, "session-1")
	for _, want := range []string{"transport=buffered", "input_wake=true", "queued=true", "dialog=auth_prompt", `dialog_hint="Please authenticate in browser"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q: %s", want, got)
		}
	}
}
