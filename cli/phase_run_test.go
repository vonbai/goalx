package cli

import (
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildPhaseResolveRequestLeavesTargetAndLocalValidationUnsetWhenUnconfigured(t *testing.T) {
	source := &savedPhaseSource{
		Run:      "research-a",
		Mode:     goalx.ModeResearch,
		Parallel: 1,
	}

	req, err := buildPhaseResolveRequest(t.TempDir(), "implement", goalx.ModeDevelop, source, goalx.Config{}, phaseOptions{})
	if err != nil {
		t.Fatalf("buildPhaseResolveRequest: %v", err)
	}
	if req.TargetOverride != nil {
		t.Fatalf("TargetOverride = %#v, want nil", req.TargetOverride)
	}
	if req.LocalValidationOverride != nil {
		t.Fatalf("LocalValidationOverride = %#v, want nil", req.LocalValidationOverride)
	}
}

func TestResolvePhaseConfigLeavesBudgetUnlimitedByDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
target:
  files: ["."]
local_validation:
  command: go test ./...
`)

	source := &savedPhaseSource{
		Run:      "research-a",
		Mode:     goalx.ModeResearch,
		Parallel: 1,
	}

	resolved, err := resolvePhaseConfig(projectRoot, "debate", goalx.ModeResearch, source, phaseOptions{})
	if err != nil {
		t.Fatalf("resolvePhaseConfig: %v", err)
	}
	cfg := &resolved.Config
	if cfg.Budget.MaxDuration != 0 {
		t.Fatalf("budget = %v, want unlimited (0)", cfg.Budget.MaxDuration)
	}
}
