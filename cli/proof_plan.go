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

type ProofPlan struct {
	Version    int             `json:"version"`
	CompiledAt string          `json:"compiled_at,omitempty"`
	Items      []ProofPlanItem `json:"items"`
}

type ProofPlanItem struct {
	ID               string   `json:"id"`
	CoversDimensions []string `json:"covers_dimensions"`
	Kind             string   `json:"kind"`
	Required         bool     `json:"required,omitempty"`
	SourceSurface    string   `json:"source_surface"`
}

func ProofPlanPath(runDir string) string {
	return filepath.Join(runDir, "proof-plan.json")
}

func LoadProofPlan(path string) (*ProofPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	plan, err := parseProofPlan(data)
	if err != nil {
		return nil, fmt.Errorf("parse proof plan: %w", err)
	}
	return plan, nil
}

func SaveProofPlan(path string, plan *ProofPlan) error {
	if plan == nil {
		return fmt.Errorf("proof plan is nil")
	}
	if err := validateProofPlanInput(plan); err != nil {
		return err
	}
	normalizeProofPlan(plan)
	if plan.CompiledAt == "" {
		plan.CompiledAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseProofPlan(data []byte) (*ProofPlan, error) {
	var plan ProofPlan
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceProofPlan, fmt.Errorf("proof plan is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceProofPlan, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceProofPlan, err)
	}
	if err := validateProofPlanInput(&plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceProofPlan, err)
	}
	normalizeProofPlan(&plan)
	return &plan, nil
}

func validateProofPlanInput(plan *ProofPlan) error {
	if plan == nil {
		return fmt.Errorf("proof plan is nil")
	}
	if plan.Version <= 0 {
		return fmt.Errorf("proof plan version must be positive")
	}
	if len(plan.Items) == 0 {
		return fmt.Errorf("proof plan items are required")
	}
	seen := make(map[string]struct{}, len(plan.Items))
	for _, item := range plan.Items {
		if strings.TrimSpace(item.ID) == "" {
			return fmt.Errorf("proof plan item id is required")
		}
		if len(compactStrings(item.CoversDimensions)) == 0 {
			return fmt.Errorf("proof plan item %s covers_dimensions is required", item.ID)
		}
		if strings.TrimSpace(item.Kind) == "" {
			return fmt.Errorf("proof plan item %s kind is required", item.ID)
		}
		if strings.TrimSpace(item.SourceSurface) == "" {
			return fmt.Errorf("proof plan item %s source_surface is required", item.ID)
		}
		if _, ok := seen[item.ID]; ok {
			return fmt.Errorf("duplicate proof plan item id %q", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	return nil
}

func normalizeProofPlan(plan *ProofPlan) {
	if plan.Version <= 0 {
		plan.Version = 1
	}
	plan.CompiledAt = strings.TrimSpace(plan.CompiledAt)
	if plan.Items == nil {
		plan.Items = []ProofPlanItem{}
	}
	for i := range plan.Items {
		plan.Items[i].ID = strings.TrimSpace(plan.Items[i].ID)
		plan.Items[i].CoversDimensions = compactStrings(plan.Items[i].CoversDimensions)
		plan.Items[i].Kind = strings.TrimSpace(plan.Items[i].Kind)
		plan.Items[i].SourceSurface = strings.TrimSpace(plan.Items[i].SourceSurface)
	}
}
