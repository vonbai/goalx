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

type WorkflowPlan struct {
	Version       int                       `json:"version"`
	CompiledAt    string                    `json:"compiled_at,omitempty"`
	RequiredRoles []WorkflowRoleRequirement `json:"required_roles"`
	Gates         []string                  `json:"gates"`
}

type WorkflowRoleRequirement struct {
	ID       string `json:"id"`
	Required bool   `json:"required,omitempty"`
}

func WorkflowPlanPath(runDir string) string {
	return filepath.Join(runDir, "workflow-plan.json")
}

func LoadWorkflowPlan(path string) (*WorkflowPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	plan, err := parseWorkflowPlan(data)
	if err != nil {
		return nil, fmt.Errorf("parse workflow plan: %w", err)
	}
	return plan, nil
}

func SaveWorkflowPlan(path string, plan *WorkflowPlan) error {
	if plan == nil {
		return fmt.Errorf("workflow plan is nil")
	}
	if err := validateWorkflowPlanInput(plan); err != nil {
		return err
	}
	normalizeWorkflowPlan(plan)
	if plan.CompiledAt == "" {
		plan.CompiledAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseWorkflowPlan(data []byte) (*WorkflowPlan, error) {
	var plan WorkflowPlan
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceWorkflowPlan, fmt.Errorf("workflow plan is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceWorkflowPlan, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceWorkflowPlan, err)
	}
	if err := validateWorkflowPlanInput(&plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceWorkflowPlan, err)
	}
	normalizeWorkflowPlan(&plan)
	return &plan, nil
}

func validateWorkflowPlanInput(plan *WorkflowPlan) error {
	if plan == nil {
		return fmt.Errorf("workflow plan is nil")
	}
	if plan.Version <= 0 {
		return fmt.Errorf("workflow plan version must be positive")
	}
	if len(plan.RequiredRoles) == 0 {
		return fmt.Errorf("workflow plan required_roles are required")
	}
	seenRoles := make(map[string]struct{}, len(plan.RequiredRoles))
	for _, role := range plan.RequiredRoles {
		if strings.TrimSpace(role.ID) == "" {
			return fmt.Errorf("workflow plan required role id is required")
		}
		if _, ok := seenRoles[role.ID]; ok {
			return fmt.Errorf("duplicate workflow plan role id %q", role.ID)
		}
		seenRoles[role.ID] = struct{}{}
	}
	if len(compactStrings(plan.Gates)) == 0 {
		return fmt.Errorf("workflow plan gates are required")
	}
	return nil
}

func normalizeWorkflowPlan(plan *WorkflowPlan) {
	if plan.Version <= 0 {
		plan.Version = 1
	}
	plan.CompiledAt = strings.TrimSpace(plan.CompiledAt)
	if plan.RequiredRoles == nil {
		plan.RequiredRoles = []WorkflowRoleRequirement{}
	}
	for i := range plan.RequiredRoles {
		plan.RequiredRoles[i].ID = strings.TrimSpace(plan.RequiredRoles[i].ID)
	}
	plan.Gates = compactStrings(plan.Gates)
}
