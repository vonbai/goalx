package cli

import (
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildPhaseResolveRequestLeavesTargetAndHarnessUnsetWhenUnconfigured(t *testing.T) {
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
	if req.HarnessOverride != nil {
		t.Fatalf("HarnessOverride = %#v, want nil", req.HarnessOverride)
	}
}

