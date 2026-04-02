package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunCharterPathAndRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	if got, want := RunCharterPath(runDir), filepath.Join(runDir, "run-charter.json"); got != want {
		t.Fatalf("RunCharterPath = %q, want %q", got, want)
	}

	meta := &RunMetadata{
		Version:         1,
		ProjectRoot:     "/tmp/project",
		ProtocolVersion: 2,
		RunID:           "run_abc123",
		RootRunID:       "run_root123",
		Epoch:           3,
		BaseRevision:    "base-rev",
		PhaseKind:       "research",
		SourceRun:       "seed-run",
		SourcePhase:     "research",
		ParentRun:       "parent-run",
	}

	charter, err := NewRunCharter(runDir, "knowledge-base", "user objective", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if charter.RunID != meta.RunID {
		t.Fatalf("RunID = %q, want %q", charter.RunID, meta.RunID)
	}
	if charter.RootRunID != meta.RootRunID {
		t.Fatalf("RootRunID = %q, want %q", charter.RootRunID, meta.RootRunID)
	}
	if charter.RunName != "knowledge-base" {
		t.Fatalf("RunName = %q, want %q", charter.RunName, "knowledge-base")
	}
	if charter.Objective != "user objective" {
		t.Fatalf("Objective = %q, want %q", charter.Objective, "user objective")
	}
	if charter.Paths.ObligationModel != ObligationModelPath(runDir) || charter.Paths.AssurancePlan != AssurancePlanPath(runDir) || charter.Paths.Proof != CompletionStatePath(runDir) {
		t.Fatalf("charter paths = %+v", charter.Paths)
	}
	if charter.RoleContracts.Master == nil || charter.RoleContracts.Worker == nil {
		t.Fatalf("charter role contracts missing: %+v", charter.RoleContracts)
	}
	if charter.CharterID == "" {
		t.Fatal("CharterID empty")
	}

	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err == nil {
		t.Fatal("second SaveRunCharter should fail for immutable charter storage")
	}
	reloaded, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	if reloaded == nil {
		t.Fatal("reloaded charter is nil")
	}
	if reloaded.CharterID != charter.CharterID {
		t.Fatalf("CharterID = %q, want %q", reloaded.CharterID, charter.CharterID)
	}
	if reloaded.Paths.ObligationModel != charter.Paths.ObligationModel || reloaded.Paths.AssurancePlan != charter.Paths.AssurancePlan || reloaded.Paths.Proof != charter.Paths.Proof {
		t.Fatalf("reloaded charter paths = %+v, want %+v", reloaded.Paths, charter.Paths)
	}
}

func TestRunCharterHashChangesWithContent(t *testing.T) {
	a := &RunCharter{Version: 1, CharterID: "charter-a", Objective: "one"}
	b := &RunCharter{Version: 1, CharterID: "charter-a", Objective: "two"}

	ha, err := hashRunCharter(a)
	if err != nil {
		t.Fatalf("hashRunCharter(a): %v", err)
	}
	hb, err := hashRunCharter(b)
	if err != nil {
		t.Fatalf("hashRunCharter(b): %v", err)
	}
	if ha == "" || hb == "" {
		t.Fatal("hashes should not be empty")
	}
	if ha == hb {
		t.Fatalf("hashes equal for different charter content: %q", ha)
	}
}

func TestRunCharterRoundTripKeepsReadablePaths(t *testing.T) {
	runDir := t.TempDir()
	meta := &RunMetadata{Version: 1, ProtocolVersion: 2, RunID: "run_1", RootRunID: "run_1", Epoch: 1}
	charter, err := NewRunCharter(runDir, "demo", "demo objective", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	data, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	marshaled, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal charter: %v", err)
	}
	text := string(marshaled)
	for _, want := range []string{"run-charter.json", ObligationModelPath(runDir), AssurancePlanPath(runDir), CompletionStatePath(runDir)} {
		if !strings.Contains(text, want) {
			t.Fatalf("charter JSON missing %q:\n%s", want, text)
		}
	}
}

func TestValidateRunCharterLinkageRejectsIdentityMismatch(t *testing.T) {
	meta := &RunMetadata{
		Version:         1,
		ProjectRoot:     "/tmp/project",
		ProtocolVersion: 2,
		RunID:           "run_abc123",
		RootRunID:       "run_root123",
		Epoch:           3,
		BaseRevision:    "base-rev",
		PhaseKind:       "research",
		SourceRun:       "seed-run",
		SourcePhase:     "research",
		ParentRun:       "parent-run",
	}
	charter, err := NewRunCharter(t.TempDir(), "knowledge-base", "user objective", meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	tests := []struct {
		name   string
		mutate func(*RunCharter)
	}{
		{
			name:   "run id",
			mutate: func(c *RunCharter) { c.RunID = "run_other" },
		},
		{
			name:   "root run id",
			mutate: func(c *RunCharter) { c.RootRunID = "run_other_root" },
		},
		{
			name:   "project root",
			mutate: func(c *RunCharter) { c.ProjectRoot = "/tmp/other-project" },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated := *charter
			tc.mutate(&mutated)
			if err := ValidateRunCharterLinkage(meta, &mutated); err == nil {
				t.Fatalf("ValidateRunCharterLinkage should reject %s mismatch", tc.name)
			}
		})
	}
}
