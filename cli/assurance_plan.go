package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type AssurancePlan struct {
	Version        int                 `json:"version"`
	ObligationRefs []string            `json:"obligation_refs,omitempty"`
	Scenarios      []AssuranceScenario `json:"scenarios"`
	UpdatedAt      string              `json:"updated_at,omitempty"`
}

type AssuranceScenario struct {
	ID                string                         `json:"id"`
	CoversObligations []string                       `json:"covers_obligations"`
	Harness           AssuranceHarness               `json:"harness"`
	Oracle            AssuranceOracle                `json:"oracle"`
	Evidence          []AssuranceEvidenceRequirement `json:"evidence"`
	Touchpoints       AssuranceTouchpoints           `json:"touchpoints,omitempty"`
	GatePolicy        AssuranceGatePolicy            `json:"gate_policy,omitempty"`
}

type AssuranceHarness struct {
	Kind    string `json:"kind"`
	Command string `json:"command,omitempty"`
}

type AssuranceOracle struct {
	Kind             string                 `json:"kind"`
	CheckDefinitions []AssuranceOracleCheck `json:"checks,omitempty"`
}

type AssuranceOracleCheck struct {
	Kind   string `json:"kind"`
	Equals string `json:"equals,omitempty"`
}

type AssuranceEvidenceRequirement struct {
	Kind string `json:"kind"`
}

type AssuranceTouchpoints struct {
	Files     []string `json:"files,omitempty"`
	Symbols   []string `json:"symbols,omitempty"`
	Processes []string `json:"processes,omitempty"`
}

type AssuranceGatePolicy struct {
	VerifyLane            string `json:"verify_lane,omitempty"`
	RequiredCognitionTier string `json:"required_cognition_tier,omitempty"`
	Closeout              string `json:"closeout,omitempty"`
	Merge                 string `json:"merge,omitempty"`
}

func AssurancePlanPath(runDir string) string {
	return filepath.Join(runDir, "assurance-plan.json")
}

func LoadAssurancePlan(path string) (*AssurancePlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	plan, err := parseAssurancePlan(data)
	if err != nil {
		return nil, fmt.Errorf("parse assurance plan: %w", err)
	}
	return plan, nil
}

func SaveAssurancePlan(path string, plan *AssurancePlan) error {
	if plan == nil {
		return fmt.Errorf("assurance plan is nil")
	}
	if err := validateAssurancePlanInput(plan); err != nil {
		return err
	}
	normalizeAssurancePlan(plan)
	if err := validateAssurancePlanIntegrity(filepath.Dir(path), plan); err != nil {
		return err
	}
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, plan)
}

func parseAssurancePlan(data []byte) (*AssurancePlan, error) {
	var plan AssurancePlan
	if err := decodeStrictJSON(data, &plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAssurancePlan, err)
	}
	if err := validateAssurancePlanInput(&plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAssurancePlan, err)
	}
	normalizeAssurancePlan(&plan)
	return &plan, nil
}

func validateAssurancePlanInput(plan *AssurancePlan) error {
	if plan == nil {
		return fmt.Errorf("assurance plan is nil")
	}
	if plan.Version <= 0 {
		return fmt.Errorf("assurance plan version must be positive")
	}
	seenScenarios := map[string]struct{}{}
	for _, scenario := range plan.Scenarios {
		if strings.TrimSpace(scenario.ID) == "" {
			return fmt.Errorf("assurance scenario id is required")
		}
		if len(compactStrings(scenario.CoversObligations)) == 0 {
			return fmt.Errorf("assurance scenario %s covers_obligations is required", scenario.ID)
		}
		if strings.TrimSpace(scenario.Harness.Kind) == "" {
			return fmt.Errorf("assurance scenario %s harness kind is required", scenario.ID)
		}
		if strings.TrimSpace(scenario.Oracle.Kind) == "" {
			return fmt.Errorf("assurance scenario %s oracle kind is required", scenario.ID)
		}
		if len(scenario.Oracle.CheckDefinitions) == 0 {
			return fmt.Errorf("assurance scenario %s oracle checks are required", scenario.ID)
		}
		if len(scenario.Evidence) == 0 {
			return fmt.Errorf("assurance scenario %s evidence is required", scenario.ID)
		}
		if _, ok := seenScenarios[scenario.ID]; ok {
			return fmt.Errorf("duplicate assurance scenario id %q", scenario.ID)
		}
		seenScenarios[scenario.ID] = struct{}{}
		for _, check := range scenario.Oracle.CheckDefinitions {
			if strings.TrimSpace(check.Kind) == "" {
				return fmt.Errorf("assurance scenario %s oracle check kind is required", scenario.ID)
			}
		}
		for _, evidence := range scenario.Evidence {
			if strings.TrimSpace(evidence.Kind) == "" {
				return fmt.Errorf("assurance scenario %s evidence kind is required", scenario.ID)
			}
		}
		if tier := strings.TrimSpace(scenario.GatePolicy.RequiredCognitionTier); tier != "" {
			switch tier {
			case "none", "repo-native", "graph":
			default:
				return fmt.Errorf("assurance scenario %s required_cognition_tier %q is invalid", scenario.ID, scenario.GatePolicy.RequiredCognitionTier)
			}
		}
		if lane := strings.TrimSpace(scenario.GatePolicy.VerifyLane); lane != "" {
			switch lane {
			case "quick", "required", "full":
			default:
				return fmt.Errorf("assurance scenario %s verify_lane %q is invalid", scenario.ID, scenario.GatePolicy.VerifyLane)
			}
		}
	}
	return nil
}

func normalizeAssurancePlan(plan *AssurancePlan) {
	if plan.Version <= 0 {
		plan.Version = 1
	}
	plan.UpdatedAt = strings.TrimSpace(plan.UpdatedAt)
	plan.ObligationRefs = compactStrings(plan.ObligationRefs)
	if plan.Scenarios == nil {
		plan.Scenarios = []AssuranceScenario{}
	}
	for i := range plan.Scenarios {
		scenario := &plan.Scenarios[i]
		scenario.ID = strings.TrimSpace(scenario.ID)
		scenario.CoversObligations = compactStrings(scenario.CoversObligations)
		scenario.Harness.Kind = strings.TrimSpace(scenario.Harness.Kind)
		scenario.Harness.Command = strings.TrimSpace(scenario.Harness.Command)
		scenario.Oracle.Kind = strings.TrimSpace(scenario.Oracle.Kind)
		if scenario.Oracle.CheckDefinitions == nil {
			scenario.Oracle.CheckDefinitions = []AssuranceOracleCheck{}
		}
		for j := range scenario.Oracle.CheckDefinitions {
			scenario.Oracle.CheckDefinitions[j].Kind = strings.TrimSpace(scenario.Oracle.CheckDefinitions[j].Kind)
			scenario.Oracle.CheckDefinitions[j].Equals = strings.TrimSpace(scenario.Oracle.CheckDefinitions[j].Equals)
		}
		if scenario.Evidence == nil {
			scenario.Evidence = []AssuranceEvidenceRequirement{}
		}
		for j := range scenario.Evidence {
			scenario.Evidence[j].Kind = strings.TrimSpace(scenario.Evidence[j].Kind)
		}
		scenario.Touchpoints.Files = compactStrings(scenario.Touchpoints.Files)
		scenario.Touchpoints.Symbols = compactStrings(scenario.Touchpoints.Symbols)
		scenario.Touchpoints.Processes = compactStrings(scenario.Touchpoints.Processes)
		scenario.GatePolicy.VerifyLane = strings.TrimSpace(scenario.GatePolicy.VerifyLane)
		scenario.GatePolicy.RequiredCognitionTier = strings.TrimSpace(scenario.GatePolicy.RequiredCognitionTier)
		scenario.GatePolicy.Closeout = strings.TrimSpace(scenario.GatePolicy.Closeout)
		scenario.GatePolicy.Merge = strings.TrimSpace(scenario.GatePolicy.Merge)
	}
}

func EnsureAssurancePlan(runDir string, acceptanceState *AcceptanceState) (*AssurancePlan, error) {
	path := AssurancePlanPath(runDir)
	plan, err := LoadAssurancePlan(path)
	if err != nil {
		return nil, err
	}
	if plan != nil {
		return plan, nil
	}
	plan = assurancePlanFromAcceptanceState(acceptanceState)
	if err := SaveAssurancePlan(path, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

func assurancePlanFromAcceptanceState(state *AcceptanceState) *AssurancePlan {
	plan := &AssurancePlan{
		Version:        1,
		ObligationRefs: []string{},
		Scenarios:      []AssuranceScenario{},
	}
	if state == nil {
		normalizeAssurancePlan(plan)
		return plan
	}
	normalizeAcceptanceState(state)
	for _, check := range state.Checks {
		if normalizeAcceptanceCheckState(check.State) != acceptanceCheckStateActive {
			continue
		}
		covers := compactStrings(check.Covers)
		if len(covers) == 0 {
			covers = []string{"legacy-acceptance"}
		}
		plan.Scenarios = append(plan.Scenarios, AssuranceScenario{
			ID:                "scenario-" + goalx.Slugify(check.ID),
			CoversObligations: covers,
			Harness: AssuranceHarness{
				Kind:    "cli",
				Command: check.Command,
			},
			Oracle: AssuranceOracle{
				Kind:             "exit_code",
				CheckDefinitions: []AssuranceOracleCheck{{Kind: "exit_code", Equals: "0"}},
			},
			Evidence: []AssuranceEvidenceRequirement{
				{Kind: "stdout"},
			},
			GatePolicy: AssuranceGatePolicy{
				VerifyLane:            "required",
				RequiredCognitionTier: "repo-native",
				Closeout:              "required",
			},
		})
	}
	normalizeAssurancePlan(plan)
	return plan
}
