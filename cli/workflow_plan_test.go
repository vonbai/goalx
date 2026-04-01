package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveWorkflowPlanRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := WorkflowPlanPath(runDir)
	plan := &WorkflowPlan{
		Version:    1,
		CompiledAt: "2026-03-31T08:00:00Z",
		RequiredRoles: []WorkflowRoleRequirement{
			{ID: "builder", Required: true},
			{ID: "critic", Required: true},
		},
		Gates: []string{"builder_result_present", "critic_review_present"},
	}

	if err := SaveWorkflowPlan(path, plan); err != nil {
		t.Fatalf("SaveWorkflowPlan: %v", err)
	}
	loaded, err := LoadWorkflowPlan(path)
	if err != nil {
		t.Fatalf("LoadWorkflowPlan: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadWorkflowPlan returned nil plan")
	}
	if len(loaded.RequiredRoles) != 2 {
		t.Fatalf("required_roles = %#v, want two round-tripped roles", loaded.RequiredRoles)
	}
}

func TestLoadWorkflowPlanRejectsRoleWithoutID(t *testing.T) {
	path := WorkflowPlanPath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "required_roles": [
    {
      "required": true
    }
  ],
  "gates": ["builder_result_present"]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadWorkflowPlan(path)
	if err == nil {
		t.Fatal("LoadWorkflowPlan should reject role without id")
	}
	if !strings.Contains(err.Error(), "required role id") {
		t.Fatalf("LoadWorkflowPlan error = %v, want required role id hint", err)
	}
}
