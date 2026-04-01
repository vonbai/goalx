package cli

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestBudgetCommandPrintsCurrentBudgetSummary(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	startedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
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
		if err := Budget(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Budget: %v", err)
		}
	})

	for _, want := range []string{
		"max_duration=1h0m0s",
		"deadline_at=",
		"remaining=",
		"exhausted=false",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("budget output missing %q:\n%s", want, out)
		}
	}
}

func TestBudgetCommandExtendsCurrentTotalBudget(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	if err := Budget(repo, []string{"--run", cfg.Name, "--extend", "2h"}); err != nil {
		t.Fatalf("Budget --extend: %v", err)
	}

	updated, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if updated.Budget.MaxDuration != 3*time.Hour {
		t.Fatalf("budget = %v, want 3h", updated.Budget.MaxDuration)
	}
}

func TestBudgetCommandSetsTotalBudget(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	if err := Budget(repo, []string{"--run", cfg.Name, "--set-total", "10h"}); err != nil {
		t.Fatalf("Budget --set-total: %v", err)
	}

	updated, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if updated.Budget.MaxDuration != 10*time.Hour {
		t.Fatalf("budget = %v, want 10h", updated.Budget.MaxDuration)
	}
}

func TestBudgetCommandClearsBudget(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	if err := Budget(repo, []string{"--run", cfg.Name, "--clear"}); err != nil {
		t.Fatalf("Budget --clear: %v", err)
	}

	updated, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	if updated.Budget.MaxDuration != 0 {
		t.Fatalf("budget = %v, want 0", updated.Budget.MaxDuration)
	}
}

func TestBudgetCommandRejectsInvalidMutationShapes(t *testing.T) {
	repo, _, cfg, _ := writeGuidanceRunFixture(t)

	tests := [][]string{
		{"--run", cfg.Name, "--extend", "0"},
		{"--run", cfg.Name, "--extend", "-1h"},
		{"--run", cfg.Name, "--set-total", "0"},
		{"--run", cfg.Name, "--extend", "1h", "--clear"},
		{"--run", cfg.Name, "--set-total", "1h", "--extend", "1h"},
	}
	for _, args := range tests {
		if err := Budget(repo, args); err == nil {
			t.Fatalf("Budget(%#v) unexpectedly succeeded", args)
		}
	}
}

func TestBudgetCommandRejectsCompletedAndDroppedRuns(t *testing.T) {
	for _, lifecycle := range []string{"completed", "dropped"} {
		t.Run(lifecycle, func(t *testing.T) {
			repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
			state := &ControlRunState{Version: 1}
			if lifecycle == "completed" {
				state.GoalState = "completed"
			} else {
				state.GoalState = "dropped"
			}
			if err := SaveControlRunState(ControlRunStatePath(runDir), state); err != nil {
				t.Fatalf("SaveControlRunState: %v", err)
			}
			if err := Budget(repo, []string{"--run", cfg.Name, "--extend", "1h"}); err == nil {
				t.Fatalf("Budget unexpectedly succeeded for %s run", lifecycle)
			}
		})
	}
}

func TestBudgetCommandRefreshesGuidanceSurfacesAfterMutation(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}
	startedAt := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Second)
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

	if err := Budget(repo, []string{"--run", cfg.Name, "--extend", "2h"}); err != nil {
		t.Fatalf("Budget --extend: %v", err)
	}

	activity, err := LoadActivitySnapshot(ActivityPath(runDir))
	if err != nil {
		t.Fatalf("LoadActivitySnapshot: %v", err)
	}
	if activity == nil || activity.Budget.MaxDurationSeconds != int64((3*time.Hour)/time.Second) {
		t.Fatalf("activity budget = %+v, want 3h", activity)
	}
	index, err := LoadContextIndex(ContextIndexPath(runDir))
	if err != nil {
		t.Fatalf("LoadContextIndex: %v", err)
	}
	if index == nil || index.Budget == nil || index.Budget.MaxDurationSeconds != int64((3*time.Hour)/time.Second) {
		t.Fatalf("context index budget = %+v, want 3h", index)
	}
	if _, err := os.Stat(AffordancesMarkdownPath(runDir)); err != nil {
		t.Fatalf("affordances markdown missing after budget mutation: %v", err)
	}
}
