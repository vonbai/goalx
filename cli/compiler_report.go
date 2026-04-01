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
	CompilerReasonNoSelectorMatch = "no_selector_match"
	CompilerReasonContradicted    = "contradicted"
	CompilerReasonSuperseded      = "superseded"
	CompilerReasonLowerPriority   = "lower_priority"
)

type CompilerReport struct {
	Version              int                     `json:"version"`
	CompiledAt           string                  `json:"compiled_at,omitempty"`
	CompilerVersion      string                  `json:"compiler_version,omitempty"`
	AvailableSourceSlots []CompilerReportSlot    `json:"available_source_slots,omitempty"`
	SelectedPriorRefs    []string                `json:"selected_prior_refs,omitempty"`
	RejectedPriors       []CompilerRejectedPrior `json:"rejected_priors,omitempty"`
	OutputSources        []CompilerOutputSource  `json:"output_sources,omitempty"`
}

type CompilerReportSlot struct {
	Slot string   `json:"slot"`
	Refs []string `json:"refs,omitempty"`
}

type CompilerRejectedPrior struct {
	Ref        string `json:"ref"`
	ReasonCode string `json:"reason_code"`
}

type CompilerOutputSource struct {
	Output     string   `json:"output"`
	SourceSlot string   `json:"source_slot"`
	Refs       []string `json:"refs,omitempty"`
}

func CompilerReportPath(runDir string) string {
	return filepath.Join(runDir, "compiler-report.json")
}

func LoadCompilerReport(path string) (*CompilerReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	report, err := parseCompilerReport(data)
	if err != nil {
		return nil, fmt.Errorf("parse compiler report: %w", err)
	}
	return report, nil
}

func SaveCompilerReport(path string, report *CompilerReport) error {
	if report == nil {
		return fmt.Errorf("compiler report is nil")
	}
	if err := validateCompilerReport(report); err != nil {
		return err
	}
	normalizeCompilerReport(report)
	if report.CompiledAt == "" {
		report.CompiledAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseCompilerReport(data []byte) (*CompilerReport, error) {
	var report CompilerReport
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceCompilerReport, fmt.Errorf("compiler report is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerReport, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerReport, err)
	}
	if err := validateCompilerReport(&report); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerReport, err)
	}
	normalizeCompilerReport(&report)
	return &report, nil
}

func validateCompilerReport(report *CompilerReport) error {
	if report == nil {
		return fmt.Errorf("compiler report is nil")
	}
	if report.Version <= 0 {
		return fmt.Errorf("compiler report version must be positive")
	}
	seenSlots := make(map[string]struct{}, len(report.AvailableSourceSlots))
	for _, slot := range report.AvailableSourceSlots {
		name := strings.TrimSpace(slot.Slot)
		if !isValidCompilerInputSlot(name) {
			return fmt.Errorf("compiler report slot %q is invalid", slot.Slot)
		}
		if _, ok := seenSlots[name]; ok {
			return fmt.Errorf("duplicate compiler report slot %q", name)
		}
		seenSlots[name] = struct{}{}
	}
	for _, prior := range report.RejectedPriors {
		if strings.TrimSpace(prior.Ref) == "" {
			return fmt.Errorf("compiler report rejected_prior ref is required")
		}
		if !isValidCompilerReasonCode(prior.ReasonCode) {
			return fmt.Errorf("compiler report rejected_prior reason_code %q is invalid", prior.ReasonCode)
		}
	}
	for _, source := range report.OutputSources {
		if strings.TrimSpace(source.Output) == "" {
			return fmt.Errorf("compiler report output_sources output is required")
		}
		if !isValidCompilerInputSlot(source.SourceSlot) {
			return fmt.Errorf("compiler report output_sources source_slot %q is invalid", source.SourceSlot)
		}
	}
	return nil
}

func normalizeCompilerReport(report *CompilerReport) {
	if report.Version <= 0 {
		report.Version = 1
	}
	report.CompiledAt = strings.TrimSpace(report.CompiledAt)
	report.CompilerVersion = strings.TrimSpace(report.CompilerVersion)
	if report.AvailableSourceSlots == nil {
		report.AvailableSourceSlots = []CompilerReportSlot{}
	}
	for i := range report.AvailableSourceSlots {
		report.AvailableSourceSlots[i].Slot = strings.TrimSpace(report.AvailableSourceSlots[i].Slot)
		report.AvailableSourceSlots[i].Refs = compactStrings(report.AvailableSourceSlots[i].Refs)
	}
	report.SelectedPriorRefs = compactStrings(report.SelectedPriorRefs)
	if report.RejectedPriors == nil {
		report.RejectedPriors = []CompilerRejectedPrior{}
	}
	for i := range report.RejectedPriors {
		report.RejectedPriors[i].Ref = strings.TrimSpace(report.RejectedPriors[i].Ref)
		report.RejectedPriors[i].ReasonCode = strings.TrimSpace(report.RejectedPriors[i].ReasonCode)
	}
	if report.OutputSources == nil {
		report.OutputSources = []CompilerOutputSource{}
	}
	for i := range report.OutputSources {
		report.OutputSources[i].Output = strings.TrimSpace(report.OutputSources[i].Output)
		report.OutputSources[i].SourceSlot = strings.TrimSpace(report.OutputSources[i].SourceSlot)
		report.OutputSources[i].Refs = compactStrings(report.OutputSources[i].Refs)
	}
}

func isValidCompilerReasonCode(reason string) bool {
	switch strings.TrimSpace(reason) {
	case CompilerReasonNoSelectorMatch, CompilerReasonContradicted, CompilerReasonSuperseded, CompilerReasonLowerPriority:
		return true
	default:
		return false
	}
}
