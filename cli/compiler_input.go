package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	CompilerInputSlotRepoPolicy           = "repo_policy"
	CompilerInputSlotLearnedSuccessPriors = "learned_success_priors"
	CompilerInputSlotRunContext           = "run_context"
)

type CompilerInput struct {
	Version              int                 `json:"version"`
	CompiledAt           string              `json:"compiled_at,omitempty"`
	CompilerVersion      string              `json:"compiler_version,omitempty"`
	ObjectiveContractRef string              `json:"objective_contract_ref"`
	GoalRef              string              `json:"goal_ref"`
	MemoryQueryRef       string              `json:"memory_query_ref,omitempty"`
	MemoryContextRef     string              `json:"memory_context_ref,omitempty"`
	PolicySourceRefs     []string            `json:"policy_source_refs,omitempty"`
	SelectedPriorRefs    []string            `json:"selected_prior_refs,omitempty"`
	SourceSlots          []CompilerInputSlot `json:"source_slots,omitempty"`
}

type CompilerInputSlot struct {
	Slot string   `json:"slot"`
	Refs []string `json:"refs"`
}

func CompilerInputPath(runDir string) string {
	return filepath.Join(runDir, "compiler-input.json")
}

func LoadCompilerInput(path string) (*CompilerInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	input, err := parseCompilerInput(data)
	if err != nil {
		return nil, fmt.Errorf("parse compiler input: %w", err)
	}
	return input, nil
}

func SaveCompilerInput(path string, input *CompilerInput) error {
	if input == nil {
		return fmt.Errorf("compiler input is nil")
	}
	if err := validateCompilerInput(input); err != nil {
		return err
	}
	normalizeCompilerInput(input)
	if input.CompiledAt == "" {
		input.CompiledAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseCompilerInput(data []byte) (*CompilerInput, error) {
	var input CompilerInput
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceCompilerInput, fmt.Errorf("compiler input is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerInput, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerInput, err)
	}
	if err := validateCompilerInput(&input); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerInput, err)
	}
	normalizeCompilerInput(&input)
	return &input, nil
}

func validateCompilerInput(input *CompilerInput) error {
	if input == nil {
		return fmt.Errorf("compiler input is nil")
	}
	if input.Version <= 0 {
		return fmt.Errorf("compiler input version must be positive")
	}
	if strings.TrimSpace(input.ObjectiveContractRef) == "" {
		return fmt.Errorf("compiler input objective_contract_ref is required")
	}
	if strings.TrimSpace(input.GoalRef) == "" {
		return fmt.Errorf("compiler input goal_ref is required")
	}
	seenSlots := make(map[string]struct{}, len(input.SourceSlots))
	for _, slot := range input.SourceSlots {
		name := strings.TrimSpace(slot.Slot)
		if !isValidCompilerInputSlot(name) {
			return fmt.Errorf("compiler input slot %q is invalid", slot.Slot)
		}
		if len(compactStrings(slot.Refs)) == 0 {
			return fmt.Errorf("compiler input slot %s refs are required", name)
		}
		if _, ok := seenSlots[name]; ok {
			return fmt.Errorf("duplicate compiler input slot %q", name)
		}
		seenSlots[name] = struct{}{}
	}
	return nil
}

func normalizeCompilerInput(input *CompilerInput) {
	if input.Version <= 0 {
		input.Version = 1
	}
	input.CompiledAt = strings.TrimSpace(input.CompiledAt)
	input.CompilerVersion = strings.TrimSpace(input.CompilerVersion)
	input.ObjectiveContractRef = strings.TrimSpace(input.ObjectiveContractRef)
	input.GoalRef = strings.TrimSpace(input.GoalRef)
	input.MemoryQueryRef = strings.TrimSpace(input.MemoryQueryRef)
	input.MemoryContextRef = strings.TrimSpace(input.MemoryContextRef)
	input.PolicySourceRefs = compactStrings(input.PolicySourceRefs)
	input.SelectedPriorRefs = compactStrings(input.SelectedPriorRefs)
	if input.SourceSlots == nil {
		input.SourceSlots = []CompilerInputSlot{}
	}
	for i := range input.SourceSlots {
		input.SourceSlots[i].Slot = strings.TrimSpace(input.SourceSlots[i].Slot)
		input.SourceSlots[i].Refs = compactStrings(input.SourceSlots[i].Refs)
	}
}

func isValidCompilerInputSlot(slot string) bool {
	switch strings.TrimSpace(slot) {
	case CompilerInputSlotRepoPolicy, CompilerInputSlotLearnedSuccessPriors, CompilerInputSlotRunContext:
		return true
	default:
		return false
	}
}
