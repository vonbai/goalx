package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func loadTransportFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "transport", name+".txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transport fixture %s: %v", name, err)
	}
	return string(data)
}

func TestInspectTransportTargetClassifiesCanonicalStatesFromFixtures(t *testing.T) {
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()

	tests := []struct {
		name    string
		engine  string
		fixture string
		want    string
	}{
		{name: "codex idle", engine: "codex", fixture: "codex_idle_prompt", want: "idle_prompt"},
		{name: "codex buffered", engine: "codex", fixture: "codex_buffered_input", want: "buffered_input"},
		{name: "codex queued", engine: "codex", fixture: "codex_queued", want: "queued"},
		{name: "codex working", engine: "codex", fixture: "codex_working", want: "working"},
		{name: "codex compacting", engine: "codex", fixture: "codex_compacting", want: "compacting"},
		{name: "codex interrupted", engine: "codex", fixture: "codex_interrupted", want: "interrupted"},
		{name: "codex provider dialog", engine: "codex", fixture: "codex_provider_dialog", want: "provider_dialog"},
		{name: "codex trust prompt", engine: "codex", fixture: "codex_trust_prompt", want: "provider_dialog"},
		{name: "claude idle", engine: "claude-code", fixture: "claude_idle_prompt", want: "idle_prompt"},
		{name: "claude buffered", engine: "claude-code", fixture: "claude_buffered_input", want: "buffered_input"},
		{name: "claude queued", engine: "claude-code", fixture: "claude_queued", want: "queued"},
		{name: "claude working", engine: "claude-code", fixture: "claude_working", want: "working"},
		{name: "claude provider dialog", engine: "claude-code", fixture: "claude_provider_dialog", want: "provider_dialog"},
		{name: "claude trust prompt", engine: "claude-code", fixture: "claude_trust_prompt", want: "provider_dialog"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureAgentPane = func(target string) (string, error) {
				return loadTransportFixture(t, tt.fixture), nil
			}
			got := inspectTransportTarget("gx-demo:master", "master", "master", tt.engine)
			if got.TransportState != tt.want {
				t.Fatalf("transport_state = %q, want %q: %+v", got.TransportState, tt.want, got)
			}
		})
	}
}

func TestInspectTransportTargetDetectsWorkspaceTrustPromptAsProviderDialog(t *testing.T) {
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()

	tests := []struct {
		name    string
		engine  string
		fixture string
	}{
		{name: "codex", engine: "codex", fixture: "codex_trust_prompt"},
		{name: "claude", engine: "claude-code", fixture: "claude_trust_prompt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureAgentPane = func(target string) (string, error) {
				return loadTransportFixture(t, tt.fixture), nil
			}
			got := inspectTransportTarget("gx-demo:master", "master", "master", tt.engine)
			if !got.ProviderDialogVisible {
				t.Fatalf("provider_dialog_visible = false: %+v", got)
			}
			if got.ProviderDialogKind != "trust_prompt" {
				t.Fatalf("provider_dialog_kind = %q, want trust_prompt", got.ProviderDialogKind)
			}
			if got.TransportState != "provider_dialog" {
				t.Fatalf("transport_state = %q, want provider_dialog", got.TransportState)
			}
		})
	}
}

func TestInspectTransportTargetDetectsClaudePermissionChoicePromptAsProviderDialog(t *testing.T) {
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()

	captureAgentPane = func(target string) (string, error) {
		return "Tool needs your permission\nAllow for this project\nYes, don't ask again\n", nil
	}
	got := inspectTransportTarget("gx-demo:master", "master", "master", "claude-code")
	if !got.ProviderDialogVisible {
		t.Fatalf("provider_dialog_visible = false: %+v", got)
	}
	if got.ProviderDialogKind != "permission_prompt" {
		t.Fatalf("provider_dialog_kind = %q, want permission_prompt", got.ProviderDialogKind)
	}
	if got.TransportState != "provider_dialog" {
		t.Fatalf("transport_state = %q, want provider_dialog", got.TransportState)
	}
}

func TestInspectTransportTargetRejectsMixedWakeInputAsBuffered(t *testing.T) {
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()

	captureAgentPane = func(target string) (string, error) {
		return "› draft [[GOALX_WAKE_CHECK_INBOX]] and other user text", nil
	}
	got := inspectTransportTarget("gx-demo:master", "master", "master", "codex")
	if got.TransportState != "unknown" {
		t.Fatalf("transport_state = %q, want unknown for mixed foreground input: %+v", got.TransportState, got)
	}
}

func TestInspectTransportTargetClassifiesBlankPaneExplicitly(t *testing.T) {
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()

	captureAgentPane = func(target string) (string, error) {
		return "   \n\t", nil
	}
	got := inspectTransportTarget("gx-demo:master", "master", "master", "codex")
	if got.TransportState != "blank" {
		t.Fatalf("transport_state = %q, want blank: %+v", got.TransportState, got)
	}
	if got.ProviderDialogVisible {
		t.Fatalf("provider_dialog_visible = true, want false: %+v", got)
	}
}

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

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
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
	if got.TransportState != "buffered_input" {
		t.Fatalf("transport_state = %q, want buffered_input", got.TransportState)
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

	identity, err := NewSessionIdentity(runDir, "session-1", "research", goalx.ModeResearch, "claude-code", "opus", goalx.EffortHigh, "high", "", goalx.TargetConfig{})
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
	if got.TransportState != "queued" {
		t.Fatalf("transport_state = %q, want queued", got.TransportState)
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
	if got.TransportState != "queued" {
		t.Fatalf("transport_state = %q, want queued", got.TransportState)
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
				TransportState:    "buffered_input",
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
	if got == nil || got.Targets["session-1"].TransportState != "buffered_input" {
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

func TestBuildTransportFactsClassifiesBlankPaneExplicitly(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("  \n\t"), 0o644); err != nil {
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
	if got.TransportState != "blank" {
		t.Fatalf("transport_state = %q, want blank", got.TransportState)
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

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
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

	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeDevelop, "codex", "gpt-5.4", goalx.EffortHigh, "xhigh", "", goalx.TargetConfig{})
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

	identity, err := NewSessionIdentity(runDir, "session-1", "research", goalx.ModeResearch, "claude-code", "opus", goalx.EffortHigh, "high", "", goalx.TargetConfig{})
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
				TransportState:        "buffered_input",
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
	for _, want := range []string{"transport=buffered_input", "input_wake=true", "queued=true", "dialog=auth_prompt", `dialog_hint="Please authenticate in browser"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("summary missing %q: %s", want, got)
		}
	}
}

func TestTransportTargetFactsSummaryFormatsBlank(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveTransportFacts(runDir, &TransportFacts{
		Version: 1,
		Targets: map[string]TransportTargetFacts{
			"session-1": {
				TransportState: "blank",
			},
		},
	}); err != nil {
		t.Fatalf("SaveTransportFacts: %v", err)
	}
	if got := transportTargetFactsSummary(runDir, "session-1"); got != "transport=blank" {
		t.Fatalf("summary = %q, want transport=blank", got)
	}
}
