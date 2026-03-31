package cli

import (
	"testing"
)

func TestDurableSurfaceRegistryResolvesKnownSurfaces(t *testing.T) {
	runDir := t.TempDir()
	cases := []struct {
		name      string
		class     DurableSurfaceClass
		writeMode DurableSurfaceWriteMode
		strict    bool
		wantPath  string
	}{
		{name: "objective-contract", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: ObjectiveContractPath(runDir)},
		{name: "goal", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: GoalPath(runDir)},
		{name: "acceptance", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: AcceptanceStatePath(runDir)},
		{name: "coordination", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: CoordinationPath(runDir)},
		{name: "status", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: RunStatusPath(runDir)},
		{name: "success-model", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: SuccessModelPath(runDir)},
		{name: "proof-plan", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: ProofPlanPath(runDir)},
		{name: "workflow-plan", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: WorkflowPlanPath(runDir)},
		{name: "domain-pack", class: DurableSurfaceClassStructuredState, writeMode: DurableSurfaceWriteModeReplace, strict: true, wantPath: DomainPackPath(runDir)},
		{name: "goal-log", class: DurableSurfaceClassEventLog, writeMode: DurableSurfaceWriteModeAppend, strict: true, wantPath: GoalLogPath(runDir)},
		{name: "experiments", class: DurableSurfaceClassEventLog, writeMode: DurableSurfaceWriteModeAppend, strict: true, wantPath: ExperimentsLogPath(runDir)},
		{name: "intervention-log", class: DurableSurfaceClassEventLog, writeMode: DurableSurfaceWriteModeAppend, strict: true, wantPath: InterventionLogPath(runDir)},
		{name: "summary", class: DurableSurfaceClassArtifact, writeMode: DurableSurfaceWriteModeReplace, strict: false, wantPath: SummaryPath(runDir)},
		{name: "completion-proof", class: DurableSurfaceClassArtifact, writeMode: DurableSurfaceWriteModeReplace, strict: false, wantPath: CompletionStatePath(runDir)},
	}
	for _, tc := range cases {
		spec, err := LookupDurableSurface(tc.name)
		if err != nil {
			t.Fatalf("LookupDurableSurface(%q): %v", tc.name, err)
		}
		if spec.Class != tc.class {
			t.Fatalf("%s class = %s, want %s", tc.name, spec.Class, tc.class)
		}
		if spec.WriteMode != tc.writeMode {
			t.Fatalf("%s write mode = %s, want %s", tc.name, spec.WriteMode, tc.writeMode)
		}
		if spec.Strict != tc.strict {
			t.Fatalf("%s strict = %t, want %t", tc.name, spec.Strict, tc.strict)
		}
		if got := spec.Path(runDir); got != tc.wantPath {
			t.Fatalf("%s path = %s, want %s", tc.name, got, tc.wantPath)
		}
	}
}

func TestDurableSurfaceRegistryRejectsUnknownSurface(t *testing.T) {
	if _, err := LookupDurableSurface("mystery"); err == nil {
		t.Fatal("LookupDurableSurface should reject unknown surface")
	}
}

func TestDurableSurfaceRegistryIncludesSchemaMetadataForAllSurfaces(t *testing.T) {
	for name, spec := range durableSurfaceRegistry {
		if spec.Schema.AuthoringFormat == "" {
			t.Fatalf("%s schema authoring format is empty", name)
		}
		if spec.Schema.StorageFormat == "" {
			t.Fatalf("%s schema storage format is empty", name)
		}
		if spec.Schema.Summary == "" {
			t.Fatalf("%s schema summary is empty", name)
		}
		if spec.Schema.Example == "" {
			t.Fatalf("%s schema example is empty", name)
		}
		if spec.Class != DurableSurfaceClassArtifact && len(spec.Schema.FieldNotes) == 0 {
			t.Fatalf("%s schema field notes are empty", name)
		}
	}
}
