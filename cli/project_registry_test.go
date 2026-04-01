package cli

import (
	"fmt"
	"sync"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestProjectRegistryFocusedRunFallsBackToRunScopedTruth(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	activeCfg := &goalx.Config{
		Name:      "alpha",
		Mode:      goalx.ModeWorker,
		Objective: "ship alpha",
	}
	activeRun := writeRunSpecFixture(t, repo, activeCfg)
	if err := SaveControlRunState(ControlRunStatePath(activeRun), &ControlRunState{
		Version:         1,
		GoalState:       "open",
		ContinuityState: "running",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := RenewControlLease(activeRun, "runtime-host", "run_alpha", 1, time.Minute, "process", 4242); err != nil {
		t.Fatalf("RenewControlLease: %v", err)
	}

	if err := SaveProjectRegistry(repo, &ProjectRegistry{
		Version:    1,
		FocusedRun: "ghost",
		ActiveRuns: map[string]ProjectRunRef{
			"ghost": {Name: "ghost", State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveProjectRegistry: %v", err)
	}

	got, err := ResolveDefaultRunName(repo)
	if err != nil {
		t.Fatalf("ResolveDefaultRunName: %v", err)
	}
	if got != "alpha" {
		t.Fatalf("ResolveDefaultRunName = %q, want alpha", got)
	}
}

func TestRegisterActiveRunPreservesConcurrentEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	configs := make([]*goalx.Config, 0, 8)
	for i := 0; i < 8; i++ {
		configs = append(configs, &goalx.Config{
			Name:      fmt.Sprintf("run-%d", i),
			Mode:      goalx.ModeWorker,
			Objective: fmt.Sprintf("ship run %d", i),
		})
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, cfg := range configs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := RegisterActiveRun(repo, cfg); err != nil {
				t.Errorf("RegisterActiveRun(%s): %v", cfg.Name, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	reg, err := LoadProjectRegistry(repo)
	if err != nil {
		t.Fatalf("LoadProjectRegistry: %v", err)
	}
	for _, cfg := range configs {
		if _, ok := reg.ActiveRuns[cfg.Name]; !ok {
			t.Fatalf("missing active run %s in %#v", cfg.Name, reg.ActiveRuns)
		}
	}
}
