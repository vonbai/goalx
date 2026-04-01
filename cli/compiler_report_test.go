package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveCompilerReportRoundTrip(t *testing.T) {
	path := CompilerReportPath(t.TempDir())
	report := &CompilerReport{
		Version:         1,
		CompiledAt:      "2026-03-31T08:00:00Z",
		CompilerVersion: "compiler-v2",
		AvailableSourceSlots: []CompilerReportSlot{
			{Slot: CompilerInputSlotRepoPolicy, Refs: []string{"AGENTS.md"}},
			{Slot: CompilerInputSlotLearnedSuccessPriors, Refs: []string{"mem-success-1"}},
		},
		SelectedPriorRefs: []string{"mem-success-1"},
		RejectedPriors: []CompilerRejectedPrior{
			{Ref: "mem-success-2", ReasonCode: CompilerReasonNoSelectorMatch},
		},
		OutputSources: []CompilerOutputSource{
			{Output: "success-model.dimension:dim-objective", SourceSlot: CompilerInputSlotRepoPolicy, Refs: []string{"AGENTS.md"}},
		},
	}

	if err := SaveCompilerReport(path, report); err != nil {
		t.Fatalf("SaveCompilerReport: %v", err)
	}
	loaded, err := LoadCompilerReport(path)
	if err != nil {
		t.Fatalf("LoadCompilerReport: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadCompilerReport returned nil report")
	}
	if len(loaded.RejectedPriors) != 1 || loaded.RejectedPriors[0].ReasonCode != CompilerReasonNoSelectorMatch {
		t.Fatalf("rejected_priors = %+v, want no_selector_match", loaded.RejectedPriors)
	}
}

func TestLoadCompilerReportRejectsUnknownFields(t *testing.T) {
	path := CompilerReportPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "available_source_slots": [{"slot": "repo_policy", "refs": ["AGENTS.md"]}],
  "unexpected": true
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCompilerReport(path)
	if err == nil {
		t.Fatal("LoadCompilerReport should reject unknown fields")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadCompilerReport error = %v, want unknown field hint", err)
	}
}

func TestLoadCompilerReportRejectsInvalidReasonCode(t *testing.T) {
	path := CompilerReportPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "rejected_priors": [{"ref": "mem-success-2", "reason_code": "mystery"}]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCompilerReport(path)
	if err == nil {
		t.Fatal("LoadCompilerReport should reject invalid reason_code")
	}
	if !strings.Contains(err.Error(), "reason_code") {
		t.Fatalf("LoadCompilerReport error = %v, want reason_code hint", err)
	}
}

func TestCompileBootstrapCompilerReportIncludesProtocolCompositionOutput(t *testing.T) {
	report := compileBootstrapCompilerReport(&bootstrapCompilerSources{
		SourceSlots: []CompilerInputSlot{
			{Slot: CompilerInputSlotRepoPolicy, Refs: []string{"AGENTS.md"}},
		},
	})
	if report == nil {
		t.Fatal("compileBootstrapCompilerReport returned nil")
	}
	found := false
	for _, output := range report.OutputSources {
		if output.Output == "protocol-composition" && output.SourceSlot == CompilerInputSlotRepoPolicy {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("output_sources = %+v, want protocol-composition from repo_policy", report.OutputSources)
	}
}
