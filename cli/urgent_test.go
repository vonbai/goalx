package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func writeFakeRuntimeHostTmux(t *testing.T, logPath string, extra string) string {
	t.Helper()
	fakeBin := t.TempDir()
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf '%s\n' master
    if [ -n "$TMUX_SESSION1_CAPTURE" ]; then
      printf '%s\n' session-1
    fi
    exit 0
    ;;
  list-panes)
    printf '%%0\tmaster\n'
    if [ -n "$TMUX_SESSION1_CAPTURE" ]; then
      printf '%%1\tsession-1\n'
    fi
    exit 0
    ;;
  capture-pane)
    target=""
    while [ $# -gt 0 ]; do
      if [ "$1" = "-t" ]; then
        target="$2"
        shift 2
        continue
      fi
      shift
    done
    case "$target" in
      *:master) cat "$TMUX_MASTER_CAPTURE" ;;
      *:session-1) cat "$TMUX_SESSION1_CAPTURE" ;;
    esac
    exit 0
    ;;
` + extra + `
esac
exit 0
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return tmuxPath
}

func TestHasUrgentUnreadReturnsTrueForUnreadUrgentMessage(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "drop everything and triage", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	if !hasUrgentUnread(runDir) {
		t.Fatal("hasUrgentUnread = false, want true")
	}
}

func TestHasUrgentUnreadReturnsFalseWhenNoUrgent(t *testing.T) {
	runDir := t.TempDir()
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "normal priority message"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}

	if hasUrgentUnread(runDir) {
		t.Fatal("hasUrgentUnread = true, want false")
	}
}

func TestRunRuntimeHostTickUsesWakeSubmitForPromptCapableUrgentMaster(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "drop everything and triage", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "tmux.log")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte(loadTransportFixture(t, "codex_idle_prompt")), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	writeFakeRuntimeHostTmux(t, logPath, "")

	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	var gotTarget, gotEngine string
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		gotTarget, gotEngine = target, engine
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
	if gotTarget != wantTarget || gotEngine != "codex" {
		t.Fatalf("sendAgentNudge target=%q engine=%q, want %q codex", gotTarget, gotEngine, wantTarget)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "send-keys -t "+wantTarget+" Escape") {
		t.Fatalf("tmux log should not contain an urgent Escape for prompt-capable master:\n%s", string(logData))
	}
	recovery, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportRecovery: %v", err)
	}
	if recovery.Targets["master"].LastWakeSubmitAt == "" {
		t.Fatalf("master recovery missing last_wake_submit_at: %+v", recovery.Targets["master"])
	}
}

func TestRunRuntimeHostTickDoesNotInterruptActiveWorkingMasterOnUrgentUnread(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "urgent redirect", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "tmux.log")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte(loadTransportFixture(t, "codex_working")), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	writeFakeRuntimeHostTmux(t, logPath, "")

	orig := sendAgentNudge
	defer func() { sendAgentNudge = orig }()
	sendAgentNudge = func(target, engine string) error { return nil }

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
	if strings.Contains(string(logData), "send-keys -t "+wantTarget+" Escape") {
		t.Fatalf("tmux log should not contain an eager urgent Escape for active-working master:\n%s", string(logData))
	}
}

func TestRunRuntimeHostTickWritesTargetScopedRecoveryForUrgentSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := EnsureSessionControl(runDir, "session-1"); err != nil {
		t.Fatalf("EnsureSessionControl: %v", err)
	}
	identity, err := NewSessionIdentity(runDir, "session-1", "develop", goalx.ModeWorker, "codex", "gpt-5.4", goalx.EffortHigh, "high", "", goalx.TargetConfig{})
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "session-1", "tell", "user", "urgent redirect", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "tmux.log")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte(loadTransportFixture(t, "codex_idle_prompt")), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte(loadTransportFixture(t, "codex_idle_prompt")), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	writeFakeRuntimeHostTmux(t, logPath, "")

	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
	}

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	recovery, err := LoadTransportRecovery(TransportRecoveryPath(runDir))
	if err != nil {
		t.Fatalf("LoadTransportRecovery: %v", err)
	}
	target := recovery.Targets["session-1"]
	if target.Target != "session-1" {
		t.Fatalf("recovery target = %+v, want session-1 entry", target)
	}
	if target.LastWakeSubmitAt == "" {
		t.Fatalf("last_wake_submit_at empty: %+v", target)
	}
	if target.UrgentEscalationAttempts != 0 {
		t.Fatalf("urgent escalation attempts = %d, want 0 for prompt-capable urgent session wake", target.UrgentEscalationAttempts)
	}
}

func TestRelaunchMasterRecreatesWindow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	if _, err := EnsureRunMetadata(runDir, repo, cfg.Objective); err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	if err := os.MkdirAll(RunWorktreePath(runDir), 0o755); err != nil {
		t.Fatalf("mkdir run worktree: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
exit 0
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := relaunchMaster(repo, runDir, goalx.TmuxSessionName(repo, cfg.Name), cfg); err != nil {
		t.Fatalf("relaunchMaster: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantSession := goalx.TmuxSessionName(repo, cfg.Name)
		for _, want := range []string{
			"kill-window -t " + wantSession + ":master",
			"new-window -t " + wantSession + " -n master -c " + RunWorktreePath(runDir),
			"target-runner --run",
			filepath.Join(runDir, "master.md"),
		} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}
}

func TestRunRuntimeHostTickDoesNotRelaunchMasterForUrgentUnreadAlone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if _, err := appendControlInboxMessage(runDir, "master", "tell", "user", "drop everything and triage", true); err != nil {
		t.Fatalf("appendControlInboxMessage: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "tmux.log")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte(loadTransportFixture(t, "codex_idle_prompt")), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	writeFakeRuntimeHostTmux(t, logPath, "")

	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
	}

	for tick := 1; tick <= 3; tick++ {
		if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
			t.Fatalf("runRuntimeHostTick #%d: %v", tick, err)
		}
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
	for _, unwanted := range []string{
		"send-keys -t " + wantTarget + " Escape",
		"kill-window -t " + wantTarget,
		"new-window -t " + goalx.TmuxSessionName(repo, cfg.Name) + " -n master",
	} {
		if strings.Contains(logText, unwanted) {
			t.Fatalf("tmux log should not contain %q:\n%s", unwanted, logText)
		}
	}
}

func TestRunRuntimeHostTickDoesNotInterruptProviderDialogWithoutUnread(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "tmux.log")
	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("Authentication required\nPlease authenticate in browser to continue\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	writeFakeRuntimeHostTmux(t, logPath, "")

	for tick := 1; tick <= 3; tick++ {
		if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, 2*time.Minute, 4242); err != nil {
			t.Fatalf("runRuntimeHostTick #%d: %v", tick, err)
		}
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	wantTarget := goalx.TmuxSessionName(repo, cfg.Name) + ":master"
	for _, unwanted := range []string{
		"send-keys -t " + wantTarget + " Escape",
		"kill-window -t " + wantTarget,
		"new-window -t " + goalx.TmuxSessionName(repo, cfg.Name) + " -n master",
	} {
		if strings.Contains(logText, unwanted) {
			t.Fatalf("tmux log should not contain %q:\n%s", unwanted, logText)
		}
	}
}
