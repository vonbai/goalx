package cli

import (
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func TestParsePhaseOptions(t *testing.T) {
	opts, err := parsePhaseOptions("debate", []string{
		"--from", "research-a",
		"--objective", "debate findings",
		"--parallel", "3",
		"--master", "codex/best",
		"--worker", "codex/fast",
		"--master-effort", "high",
		"--worker-effort", "low",
		"--dimension", "depth,adversarial",
		"--effort", "minimal",
		"--context", "README.md,docs/arch.md",
		"--budget", "15m",
		"--readonly",
		"--write-config",
	})
	if err != nil {
		t.Fatalf("parsePhaseOptions: %v", err)
	}
	if opts.From != "research-a" {
		t.Fatalf("from = %q", opts.From)
	}
	if opts.Parallel != 3 {
		t.Fatalf("parallel = %d", opts.Parallel)
	}
	if opts.Master != "codex/best" {
		t.Fatalf("master = %q", opts.Master)
	}
	if opts.Worker != "codex/fast" {
		t.Fatalf("worker = %q", opts.Worker)
	}
	if opts.Effort != goalx.EffortMinimal {
		t.Fatalf("effort = %q, want %q", opts.Effort, goalx.EffortMinimal)
	}
	if opts.MasterEffort != goalx.EffortHigh {
		t.Fatalf("master-effort = %q, want %q", opts.MasterEffort, goalx.EffortHigh)
	}
	if opts.WorkerEffort != goalx.EffortLow {
		t.Fatalf("worker-effort = %q, want %q", opts.WorkerEffort, goalx.EffortLow)
	}
	if len(opts.Dimensions) != 2 || opts.Dimensions[0] != "depth" || opts.Dimensions[1] != "adversarial" {
		t.Fatalf("dimensions = %#v, want [depth adversarial]", opts.Dimensions)
	}
	if !opts.BudgetSet {
		t.Fatal("BudgetSet = false, want true")
	}
	if opts.Budget != 15*time.Minute {
		t.Fatalf("budget = %v, want 15m", opts.Budget)
	}
	if !opts.Readonly {
		t.Fatal("readonly = false, want true")
	}
	if !opts.WriteConfig {
		t.Fatal("write-config = false, want true")
	}
}

func TestParsePhaseOptionsRequiresFrom(t *testing.T) {
	if _, err := parsePhaseOptions("debate", nil); err == nil {
		t.Fatal("expected missing --from error")
	}
}

func TestParsePhaseOptionsRejectsRemovedLegacySelectionFlags(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"--from", "research-a", "--preset", "codex"},
		{"--from", "research-a", "--route-role", "research"},
		{"--from", "research-a", "--route-profile", "research_deep"},
		{"--from", "research-a", "--research-role", "claude-code/opus"},
		{"--from", "research-a", "--develop-role", "codex/fast"},
		{"--from", "research-a", "--research-effort", "high"},
		{"--from", "research-a", "--develop-effort", "medium"},
	}
	for _, args := range tests {
		if _, err := parsePhaseOptions("debate", args); err == nil {
			t.Fatalf("parsePhaseOptions(%#v) unexpectedly succeeded", args)
		}
	}
}
