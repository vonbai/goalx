package cli

import (
	"os"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestListShowsDerivedStatusAndCanonicalSelector(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	activeRun := writeRunSpecFixture(t, repo, &goalx.Config{
		Name:      "alpha",
		Mode:      goalx.ModeWorker,
		Objective: "ship alpha",
	})
	if err := SaveControlRunState(ControlRunStatePath(activeRun), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState active: %v", err)
	}
	if err := RenewControlLease(activeRun, "runtime-host", "run_alpha", 1, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease active: %v", err)
	}

	degradedRun := writeRunSpecFixture(t, repo, &goalx.Config{
		Name:      "beta",
		Mode:      goalx.ModeWorker,
		Objective: "audit beta",
	})
	if err := SaveControlRunState(ControlRunStatePath(degradedRun), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState degraded: %v", err)
	}
	installFakePresenceTmux(t, true, "master", "%0\\tmaster\\n")

	out := captureStdout(t, func() {
		if err := List(repo, nil); err != nil {
			t.Fatalf("List: %v", err)
		}
	})

	projectID := goalx.ProjectID(repo)
	for _, want := range []string{
		"SELECTOR",
		projectID + "/alpha",
		projectID + "/beta",
		"alpha",
		"active",
		"beta",
		"degraded",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("list output missing %q:\n%s", want, out)
		}
	}
}

func TestListShowsLaunchingStatusForBootstrapInProgress(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	launchingRun := writeRunSpecFixture(t, repo, &goalx.Config{
		Name:      "gamma",
		Mode:      goalx.ModeWorker,
		Objective: "ship gamma",
	})
	if err := EnsureControlState(launchingRun); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(launchingRun), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState launching: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(launchingRun), &RunRuntimeState{
		Version:   1,
		Run:       "gamma",
		Mode:      string(goalx.ModeWorker),
		Active:    true,
		StartedAt: "2026-03-31T00:00:00Z",
		UpdatedAt: "2026-03-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState launching: %v", err)
	}
	if err := submitControlOperationTarget(launchingRun, RunBootstrapOperationKey(), ControlOperationTarget{
		Kind:              ControlOperationKindRunBootstrap,
		State:             ControlOperationStatePreparing,
		Summary:           "launching master runtime",
		PendingConditions: []string{"master_window_ready"},
	}); err != nil {
		t.Fatalf("submitControlOperationTarget launching: %v", err)
	}

	out := captureStdout(t, func() {
		if err := List(repo, nil); err != nil {
			t.Fatalf("List: %v", err)
		}
	})

	if !strings.Contains(out, "gamma") || !strings.Contains(out, "launching") {
		t.Fatalf("list output missing launching row:\n%s", out)
	}
}
