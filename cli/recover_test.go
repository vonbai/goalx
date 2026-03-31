package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestRecoverRelaunchesStoppedRunInPlace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath, stateDir := installRecoverFakeTmux(t, false)
	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := os.MkdirAll(runWT, 0o755); err != nil {
		t.Fatalf("mkdir run worktree: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:        1,
		LifecycleState: "stopped",
		UpdatedAt:      "2026-03-29T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(goalx.ModeWorker),
		Active:    false,
		StartedAt: "2026-03-29T00:00:00Z",
		StoppedAt: "2026-03-29T00:10:00Z",
		UpdatedAt: "2026-03-29T00:10:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	var gotSidecarProjectRoot, gotSidecarRunName string
	var gotSidecarInterval time.Duration
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error {
		gotSidecarProjectRoot, gotSidecarRunName, gotSidecarInterval = projectRoot, runName, interval
		return nil
	}

	if err := Recover(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunRuntimeState: %v", err)
	}
	if runtimeState == nil || !runtimeState.Active || runtimeState.StoppedAt != "" {
		t.Fatalf("runtime state after recover = %+v, want active with cleared stopped_at", runtimeState)
	}

	controlState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if controlState == nil || controlState.LifecycleState != "active" {
		t.Fatalf("control state after recover = %+v, want lifecycle_state active", controlState)
	}

	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	if reg.FocusedRun != runName {
		t.Fatalf("focused run = %q, want %q", reg.FocusedRun, runName)
	}
	if _, ok := reg.ActiveRuns[runName]; !ok {
		t.Fatalf("active runs = %#v, want %q registered active", reg.ActiveRuns, runName)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"new-session -d -s " + goalx.TmuxSessionName(repo, runName) + " -n master",
		filepath.Join(runDir, "master.md"),
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}

	if gotSidecarProjectRoot != repo || gotSidecarRunName != runName || gotSidecarInterval <= 0 {
		t.Fatalf("launchRunSidecar got (%q, %q, %v), want (%q, %q, >0)", gotSidecarProjectRoot, gotSidecarRunName, gotSidecarInterval, repo, runName)
	}

	if _, err := os.Stat(filepath.Join(stateDir, "session_"+goalx.TmuxSessionName(repo, runName))); err != nil {
		t.Fatalf("tmux session marker missing after recover: %v", err)
	}
}

func TestRecoverRejectsAlreadyActiveRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	_, stateDir := installRecoverFakeTmux(t, true)
	runName, runDir := writeLifecycleRunFixture(t, repo)
	tmuxSession := goalx.TmuxSessionName(repo, runName)
	if err := os.WriteFile(filepath.Join(stateDir, "session_"+tmuxSession), nil, 0o644); err != nil {
		t.Fatalf("seed tmux session marker: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:        1,
		LifecycleState: "active",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	err := Recover(repo, []string{"--run", runName})
	if err == nil || !strings.Contains(err.Error(), "already active") {
		t.Fatalf("Recover error = %v, want already active rejection", err)
	}
}

func TestRecoverRequiresExistingRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := t.TempDir()
	if err := Recover(repo, []string{"--run", "missing"}); err == nil || !strings.Contains(err.Error(), "run not found") {
		t.Fatalf("Recover error = %v, want run not found", err)
	}
}

func TestRecoverPromotesSuccessPriorBeforeRelaunch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	_, _ = installRecoverFakeTmux(t, false)
	runName, runDir := writeLifecycleRunFixture(t, repo)
	runWT := RunWorktreePath(runDir)
	if err := os.MkdirAll(runWT, 0o755); err != nil {
		t.Fatalf("mkdir run worktree: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:        1,
		LifecycleState: "stopped",
		UpdatedAt:      "2026-03-29T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(goalx.ModeWorker),
		Active:    false,
		StartedAt: "2026-03-29T00:00:00Z",
		StoppedAt: "2026-03-29T00:10:00Z",
		UpdatedAt: "2026-03-29T00:10:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	now := time.Date(2026, time.March, 31, 14, 0, 0, 0, time.UTC)
	if err := writeProposalShard(now, []MemoryProposal{
		{
			ID:        "prop_success_prior_recover",
			State:     "proposed",
			Kind:      MemoryKindSuccessPrior,
			Statement: "recover should relaunch with the latest success prior snapshot",
			Selectors: map[string]string{"project_id": goalx.ProjectID(repo)},
			Evidence: []MemoryEvidence{
				{Kind: "intervention_log", Path: "/tmp/intervention-log.jsonl"},
				{Kind: "summary", Path: "/tmp/summary.md"},
			},
			SourceRuns: []string{"run-1", "run-2"},
			CreatedAt:  "2026-03-31T14:00:00Z",
			UpdatedAt:  "2026-03-31T14:00:00Z",
		},
	}); err != nil {
		t.Fatalf("writeProposalShard: %v", err)
	}

	origLaunchSidecar := launchRunSidecar
	defer func() { launchRunSidecar = origLaunchSidecar }()
	launchRunSidecar = func(projectRoot, runName string, interval time.Duration) error { return nil }

	if err := Recover(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Recover: %v", err)
	}

	entries := loadCanonicalEntriesByKind(t, MemoryKindSuccessPrior)
	if len(entries) != 1 {
		t.Fatalf("success prior entries = %+v, want one promoted entry", entries)
	}

	pack, err := LoadDomainPack(DomainPackPath(runDir))
	if err != nil {
		t.Fatalf("LoadDomainPack: %v", err)
	}
	if pack == nil || len(pack.PriorEntryIDs) != 1 {
		t.Fatalf("domain pack = %+v, want one prior entry id", pack)
	}
	if !slices.Contains(pack.Signals, "success_prior_present") {
		t.Fatalf("domain pack signals = %v, want success_prior_present", pack.Signals)
	}
}

func installRecoverFakeTmux(t *testing.T, existingSession bool) (string, string) {
	t.Helper()

	fakeBin := t.TempDir()
	stateDir := t.TempDir()
	logPath := filepath.Join(stateDir, "tmux.log")
	script := `#!/bin/sh
set -eu
state="${GOALX_FAKE_TMUX_STATE:?}"
log="$state/tmux.log"
cmd="$1"
shift
echo "$cmd $*" >> "$log"
case "$cmd" in
  has-session)
    target=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-t" ]; then
        shift
        target="$1"
        break
      fi
      shift
    done
    if [ -f "$state/session_$target" ]; then
      exit 0
    fi
    exit 1
    ;;
  new-session)
    name=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-s" ]; then
        shift
        name="$1"
        break
      fi
      shift
    done
    : > "$state/session_$name"
    printf 'master\n' > "$state/windows_$name"
    exit 0
    ;;
  new-window)
    target=""
    window=""
    while [ "$#" -gt 0 ]; do
      case "$1" in
        -t)
          shift
          target="$1"
          ;;
        -n)
          shift
          window="$1"
          ;;
      esac
      shift || true
    done
    : > "$state/session_$target"
    printf '%s\n' "$window" >> "$state/windows_$target"
    exit 0
    ;;
  list-windows)
    target=""
    while [ "$#" -gt 0 ]; do
      if [ "$1" = "-t" ]; then
        shift
        target="$1"
        break
      fi
      shift
    done
    if [ -f "$state/windows_$target" ]; then
      cat "$state/windows_$target"
    fi
    exit 0
    ;;
  list-panes)
    printf '4321\n'
    exit 0
    ;;
  kill-window)
    exit 0
    ;;
  set-environment)
    exit 0
    ;;
  capture-pane)
    printf 'pane output\n'
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	if existingSession {
		t.Setenv("GOALX_FAKE_TMUX_EXISTING", "1")
	}
	t.Setenv("GOALX_FAKE_TMUX_STATE", stateDir)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath, stateDir
}
