package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestBuildTargetPresenceFactsReportsMasterAndSessionPresence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}

	facts, err := BuildTargetPresenceFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name))
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}

	masterFacts := facts["master"]
	if masterFacts.State != TargetPresencePresent {
		t.Fatalf("master state = %q, want %q", masterFacts.State, TargetPresencePresent)
	}
	if !masterFacts.SessionExists || !masterFacts.WindowExists || !masterFacts.PaneExists {
		t.Fatalf("master presence incomplete: %+v", masterFacts)
	}

	sessionFacts := facts["session-1"]
	if sessionFacts.State != TargetPresencePresent {
		t.Fatalf("session state = %q, want %q", sessionFacts.State, TargetPresencePresent)
	}
	if !sessionFacts.SessionExists || !sessionFacts.WindowExists || !sessionFacts.PaneExists {
		t.Fatalf("session presence incomplete: %+v", sessionFacts)
	}

	sidecarFacts := facts["sidecar"]
	if sidecarFacts.State != TargetPresencePresent {
		t.Fatalf("sidecar state = %q, want %q", sidecarFacts.State, TargetPresencePresent)
	}
	if !sidecarFacts.LeasePresent || !sidecarFacts.LeaseHealthy || !sidecarFacts.ProcessPIDAlive {
		t.Fatalf("sidecar presence incomplete: %+v", sidecarFacts)
	}
}

func TestBuildTargetPresenceFactsReportsMissingMasterWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}

	facts, err := BuildTargetPresenceFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name))
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}

	masterFacts := facts["master"]
	if masterFacts.State != TargetPresenceWindowMissing {
		t.Fatalf("master state = %q, want %q", masterFacts.State, TargetPresenceWindowMissing)
	}
	if !masterFacts.SessionExists || masterFacts.WindowExists || masterFacts.PaneExists {
		t.Fatalf("master missing-window facts wrong: %+v", masterFacts)
	}
}

func TestBuildTargetPresenceFactsReportsMissingSessionWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}

	facts, err := BuildTargetPresenceFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name))
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}

	sessionFacts := facts["session-1"]
	if sessionFacts.State != TargetPresenceWindowMissing {
		t.Fatalf("session state = %q, want %q", sessionFacts.State, TargetPresenceWindowMissing)
	}
	if !sessionFacts.SessionExists || sessionFacts.WindowExists || sessionFacts.PaneExists {
		t.Fatalf("session missing-window facts wrong: %+v", sessionFacts)
	}
}

func TestBuildTargetPresenceFactsTreatsParkedSessionAsNotMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-1",
		State: "parked",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	coord, err := EnsureCoordinationState(runDir, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureCoordinationState: %v", err)
	}
	coord.Sessions["session-1"] = CoordinationSession{State: "parked", Scope: "reusable slice"}
	coord.Version++
	if err := SaveCoordinationState(CoordinationPath(runDir), coord); err != nil {
		t.Fatalf("SaveCoordinationState: %v", err)
	}

	facts, err := BuildTargetPresenceFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name))
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}

	sessionFacts := facts["session-1"]
	if sessionFacts.State != TargetPresenceParked {
		t.Fatalf("session state = %q, want %q", sessionFacts.State, TargetPresenceParked)
	}
	if sessionFacts.WindowExpected {
		t.Fatalf("parked session should not expect a window: %+v", sessionFacts)
	}
	if !sessionFacts.SessionExists || sessionFacts.WindowExists || sessionFacts.PaneExists {
		t.Fatalf("parked session presence wrong: %+v", sessionFacts)
	}
}

func TestBuildTargetPresenceFactsReportsTmuxSessionMissingForTmuxTargets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, false, "", "")
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}

	facts, err := BuildTargetPresenceFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name))
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}

	for _, target := range []string{"master", "session-1"} {
		got := facts[target]
		if got.State != TargetPresenceSessionMissing {
			t.Fatalf("%s state = %q, want %q", target, got.State, TargetPresenceSessionMissing)
		}
		if got.SessionExists || got.WindowExists || got.PaneExists {
			t.Fatalf("%s missing-session facts wrong: %+v", target, got)
		}
	}
}

func TestBuildTargetPresenceFactsReportsMissingSidecarLease(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, _ := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")
	if err := os.Remove(ControlLeasePath(runDir, "sidecar")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("remove sidecar lease: %v", err)
	}

	facts, err := BuildTargetPresenceFacts(runDir, goalx.TmuxSessionName(repo, cfg.Name))
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}

	sidecarFacts := facts["sidecar"]
	if sidecarFacts.State != TargetPresenceLeaseExpired {
		t.Fatalf("sidecar state = %q, want %q", sidecarFacts.State, TargetPresenceLeaseExpired)
	}
}

func TestBuildTargetPresenceFactsUsesSessionWidePaneEnumeration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}

	sessionName := goalx.TmuxSessionName(repo, cfg.Name)
	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf 'master\nsession-1\n'
    exit 0
    ;;
  list-panes)
    if [ "$2" = "-a" ]; then
      printf '%s\t%%0\tmaster\n%s\t%%1\tsession-1\n' "$TMUX_SESSION_NAME" "$TMUX_SESSION_NAME"
      exit 0
    fi
    printf '%%1\tsession-1\n'
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_SESSION_NAME", sessionName)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	facts, err := BuildTargetPresenceFacts(runDir, sessionName)
	if err != nil {
		t.Fatalf("BuildTargetPresenceFacts: %v", err)
	}
	for _, target := range []string{"master", "session-1"} {
		if got := facts[target].State; got != TargetPresencePresent {
			t.Fatalf("%s state = %q, want %q", target, got, TargetPresencePresent)
		}
	}
}

func TestRefreshActivityFactsPersistsMasterWindowMissingExplicitly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, _ := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")

	if err := refreshActivityFacts(runDir, repo, cfg.Name); err != nil {
		t.Fatalf("refreshActivityFacts: %v", err)
	}

	raw, err := os.ReadFile(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("Read activity: %v", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("Unmarshal activity: %v", err)
	}
	targets, ok := snapshot["targets"].(map[string]any)
	if !ok {
		t.Fatalf("activity snapshot missing targets: %+v", snapshot)
	}
	masterFacts, ok := targets["master"].(map[string]any)
	if !ok {
		t.Fatalf("activity snapshot missing master target: %+v", targets)
	}
	if masterFacts["window"] != "master" || masterFacts["state"] != TargetPresenceWindowMissing {
		t.Fatalf("master facts = %+v, want explicit missing-master state", masterFacts)
	}
}

func TestRefreshActivityFactsPersistsSessionWindowMissingExplicitly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, _ := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")

	if err := refreshActivityFacts(runDir, repo, cfg.Name); err != nil {
		t.Fatalf("refreshActivityFacts: %v", err)
	}

	raw, err := os.ReadFile(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("Read activity: %v", err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatalf("Unmarshal activity: %v", err)
	}
	targets, ok := snapshot["targets"].(map[string]any)
	if !ok {
		t.Fatalf("activity snapshot missing targets: %+v", snapshot)
	}
	sessionFacts, ok := targets["session-1"].(map[string]any)
	if !ok {
		t.Fatalf("activity snapshot missing session-1 target: %+v", targets)
	}
	if sessionFacts["window"] != "session-1" || sessionFacts["state"] != TargetPresenceWindowMissing {
		t.Fatalf("session-1 facts = %+v, want explicit missing-session state", sessionFacts)
	}
}

func TestLoadDerivedRunStateMarksMasterMissingAsStranded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")

	state, err := loadDerivedRunState(repo, runDir)
	if err != nil {
		t.Fatalf("loadDerivedRunState: %v", err)
	}
	if state.Status != "stranded" {
		t.Fatalf("derived status = %q, want stranded", state.Status)
	}
	if state.LifecycleState != "active" {
		t.Fatalf("lifecycle state = %q, want active", state.LifecycleState)
	}
	if state.Name != cfg.Name {
		t.Fatalf("derived name = %q, want %q", state.Name, cfg.Name)
	}
}

func TestLoadDerivedRunStateMarksSidecarMissingAsStranded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, _, _ := writeTargetPresenceFixture(t)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		t.Fatalf("ExpireControlLease sidecar: %v", err)
	}
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")

	state, err := loadDerivedRunState(repo, runDir)
	if err != nil {
		t.Fatalf("loadDerivedRunState: %v", err)
	}
	if state.Status != "stranded" {
		t.Fatalf("derived status = %q, want stranded", state.Status)
	}
}

func TestLoadDerivedRunStateMarksSessionMissingAsDegraded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, _, meta := writeTargetPresenceFixture(t)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")

	state, err := loadDerivedRunState(repo, runDir)
	if err != nil {
		t.Fatalf("loadDerivedRunState: %v", err)
	}
	if state.Status != "degraded" {
		t.Fatalf("derived status = %q, want degraded", state.Status)
	}
}

func TestStatusReportsStrandedRunAndMissingActors(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, _ := writeTargetPresenceFixture(t)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		t.Fatalf("ExpireControlLease sidecar: %v", err)
	}
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()
	captureAgentPane = func(target string) (string, error) {
		return "❯ prompt\n", nil
	}

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})
	for _, want := range []string{"run_status=stranded", "master window missing", "sidecar missing"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveReportsSessionWindowMissingExplicitly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")
	origCapture := captureAgentPane
	defer func() { captureAgentPane = origCapture }()
	captureAgentPane = func(target string) (string, error) {
		return "❯ prompt\n", nil
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})
	if !strings.Contains(out, "session-1 window missing") {
		t.Fatalf("observe output missing explicit session loss:\n%s", out)
	}
}

func TestResolveDefaultRunNameTreatsStrandedRunAsOpen(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := RenewControlLease(runDir, "sidecar", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease sidecar: %v", err)
	}
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")
	if _, err := loadDerivedRunState(repo, runDir); err != nil {
		t.Fatalf("loadDerivedRunState: %v", err)
	}

	otherCfg := &goalx.Config{
		Name:      "inactive-run",
		Mode:      goalx.ModeDevelop,
		Objective: "do not pick me",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	otherRunDir := writeRunSpecFixture(t, repo, otherCfg)
	if err := SaveControlRunState(ControlRunStatePath(otherRunDir), &ControlRunState{Version: 1, LifecycleState: "stopped"}); err != nil {
		t.Fatalf("SaveControlRunState stopped: %v", err)
	}

	got, err := ResolveDefaultRunName(repo)
	if err != nil {
		t.Fatalf("ResolveDefaultRunName: %v", err)
	}
	if got != cfg.Name {
		t.Fatalf("ResolveDefaultRunName = %q, want stranded run %q", got, cfg.Name)
	}
}

func TestQueueMasterWakeReminderSkipsWhenMasterWindowMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, _ := writeTargetPresenceFixture(t)
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")

	if err := queueMasterWakeReminder(runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg.Master.Engine); err != nil {
		t.Fatalf("queueMasterWakeReminder: %v", err)
	}
	reminders, err := LoadControlReminders(ControlRemindersPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if len(reminders.Items) != 0 {
		t.Fatalf("master wake reminder queued despite missing master window: %+v", reminders.Items)
	}
}

func TestDeliverTellSkipsMasterNudgeWhenMasterWindowMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, _ := writeTargetPresenceFixture(t)
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := RegisterActiveRun(repo, cfg); err != nil {
		t.Fatalf("RegisterActiveRun: %v", err)
	}
	installFakePresenceTmux(t, true, "session-1", "%1\\tsession-1\\n")

	called := 0
	origNudge := sendAgentNudgeDetailed
	defer func() { sendAgentNudgeDetailed = origNudge }()
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		called++
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
	}

	if _, _, err := deliverTell(repo, cfg.Name, "master", "urgent redirect", true, sendAgentNudgeDetailed); err != nil {
		t.Fatalf("deliverTell: %v", err)
	}
	if called != 0 {
		t.Fatalf("master nudge sent despite missing master window: %d", called)
	}
}

func TestRunSidecarTickDoesNotKillCompactingMasterWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeTargetPresenceFixture(t)
	logPath := filepath.Join(t.TempDir(), "tmux.log")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte(loadTransportFixture(t, "codex_compacting")), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	writeFakeSidecarTmux(t, logPath, "")

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()
	sendAgentNudge = func(target, engine string) error { return nil }

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "kill-window -t "+goalx.TmuxSessionName(repo, cfg.Name)+":master") {
		t.Fatalf("compacting master should not be killed:\n%s", string(logData))
	}
}

func writeTargetPresenceFixture(t *testing.T) (string, string, *goalx.Config, *RunMetadata) {
	t.Helper()

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "presence-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "codex", "gpt-5.4", goalx.TargetConfig{}, goalx.LocalValidationConfig{})
	return repo, runDir, cfg, meta
}
