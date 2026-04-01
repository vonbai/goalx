package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveCompilerInputRoundTrip(t *testing.T) {
	path := CompilerInputPath(t.TempDir())
	input := &CompilerInput{
		Version:              1,
		CompiledAt:           "2026-03-31T08:00:00Z",
		CompilerVersion:      "compiler-v2",
		ObjectiveContractRef: "objective-contract.json",
		GoalRef:              "goal.json",
		MemoryQueryRef:       "control/memory-query.json",
		MemoryContextRef:     "control/memory-context.json",
		PolicySourceRefs:     []string{"AGENTS.md"},
		SelectedPriorRefs:    []string{"mem-success-1"},
		SourceSlots: []CompilerInputSlot{
			{Slot: CompilerInputSlotRepoPolicy, Refs: []string{"AGENTS.md"}},
			{Slot: CompilerInputSlotLearnedSuccessPriors, Refs: []string{"mem-success-1"}},
			{Slot: CompilerInputSlotRunContext, Refs: []string{"control/intervention-log.jsonl"}},
		},
	}

	if err := SaveCompilerInput(path, input); err != nil {
		t.Fatalf("SaveCompilerInput: %v", err)
	}
	loaded, err := LoadCompilerInput(path)
	if err != nil {
		t.Fatalf("LoadCompilerInput: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadCompilerInput returned nil input")
	}
	if loaded.CompilerVersion != "compiler-v2" {
		t.Fatalf("compiler_version = %q, want compiler-v2", loaded.CompilerVersion)
	}
	if len(loaded.SourceSlots) != 3 {
		t.Fatalf("source_slots len = %d, want 3", len(loaded.SourceSlots))
	}
}

func TestLoadCompilerInputRejectsUnknownFields(t *testing.T) {
	path := CompilerInputPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "objective_contract_ref": "objective-contract.json",
  "goal_ref": "goal.json",
  "source_slots": [{"slot": "repo_policy", "refs": ["AGENTS.md"]}],
  "unexpected": true
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCompilerInput(path)
	if err == nil {
		t.Fatal("LoadCompilerInput should reject unknown fields")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("LoadCompilerInput error = %v, want unknown field hint", err)
	}
}

func TestLoadCompilerInputRejectsInvalidSlot(t *testing.T) {
	path := CompilerInputPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "objective_contract_ref": "objective-contract.json",
  "goal_ref": "goal.json",
  "source_slots": [{"slot": "mystery", "refs": ["x"]}]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCompilerInput(path)
	if err == nil {
		t.Fatal("LoadCompilerInput should reject invalid slot")
	}
	if !strings.Contains(err.Error(), "slot") {
		t.Fatalf("LoadCompilerInput error = %v, want slot hint", err)
	}
}
