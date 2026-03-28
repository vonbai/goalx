package cli

import (
	"encoding/json"
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
		"--research-role", "claude-code/opus",
		"--develop-role", "codex/fast",
		"--master-effort", "high",
		"--research-effort", "medium",
		"--develop-effort", "low",
		"--dimension", "depth,adversarial",
		"--effort", "minimal",
		"--context", "README.md,docs/arch.md",
		"--budget", "15m",
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
	if opts.ResearchRole != "claude-code/opus" {
		t.Fatalf("research-role = %q", opts.ResearchRole)
	}
	if opts.DevelopRole != "codex/fast" {
		t.Fatalf("develop-role = %q", opts.DevelopRole)
	}
	if opts.Effort != goalx.EffortMinimal {
		t.Fatalf("effort = %q, want %q", opts.Effort, goalx.EffortMinimal)
	}
	if opts.MasterEffort != goalx.EffortHigh {
		t.Fatalf("master-effort = %q, want %q", opts.MasterEffort, goalx.EffortHigh)
	}
	if opts.ResearchEffort != goalx.EffortMedium {
		t.Fatalf("research-effort = %q, want %q", opts.ResearchEffort, goalx.EffortMedium)
	}
	if opts.DevelopEffort != goalx.EffortLow {
		t.Fatalf("develop-effort = %q, want %q", opts.DevelopEffort, goalx.EffortLow)
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
	}
	for _, args := range tests {
		if _, err := parsePhaseOptions("debate", args); err == nil {
			t.Fatalf("parsePhaseOptions(%#v) unexpectedly succeeded", args)
		}
	}
}

func TestMergeNextConfigIntoPhaseOptionsIgnoresLegacySelectionFields(t *testing.T) {
	var nc nextConfigJSON
	if err := json.Unmarshal([]byte(`{
		"parallel": 3,
		"objective": "continue",
		"context": ["README.md"],
		"dimensions": ["depth", "adversarial"],
		"preset": "claude",
		"engine": "codex",
		"model": "fast",
		"mode": "develop",
		"master_engine": "claude-code",
		"master_model": "opus",
		"route_role": "research",
		"route_profile": "research_deep",
		"effort": "high",
		"master_effort": "high"
	}`), &nc); err != nil {
		t.Fatalf("unmarshal next_config: %v", err)
	}

	opts := mergeNextConfigIntoPhaseOptions(phaseOptions{}, &nc, goalx.ModeResearch)

	if opts.Parallel != 3 {
		t.Fatalf("parallel = %d, want 3", opts.Parallel)
	}
	if opts.Objective != "continue" {
		t.Fatalf("objective = %q, want continue", opts.Objective)
	}
	if len(opts.ContextPaths) != 1 || opts.ContextPaths[0] != "README.md" {
		t.Fatalf("context = %#v, want [README.md]", opts.ContextPaths)
	}
	if len(opts.Dimensions) != 2 || opts.Dimensions[0] != "depth" || opts.Dimensions[1] != "adversarial" {
		t.Fatalf("dimensions = %#v, want [depth adversarial]", opts.Dimensions)
	}
	if opts.Master != "" || opts.MasterEffort != "" {
		t.Fatalf("master override = %q/%q, want empty", opts.Master, opts.MasterEffort)
	}
	if opts.ResearchRole != "" || opts.DevelopRole != "" {
		t.Fatalf("role overrides = %q/%q, want empty", opts.ResearchRole, opts.DevelopRole)
	}
	if opts.Effort != "" {
		t.Fatalf("effort = %q, want empty", opts.Effort)
	}
}
