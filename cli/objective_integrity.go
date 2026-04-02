package cli

import (
	"fmt"
	"sort"
	"strings"
)

type ObjectiveIntegritySummary struct {
	ContractPresent            bool     `json:"contract_present,omitempty"`
	ContractState              string   `json:"contract_state,omitempty"`
	ContractLocked             bool     `json:"contract_locked,omitempty"`
	ClauseCount                int      `json:"clause_count,omitempty"`
	GoalClauseCount            int      `json:"obligation_clause_count,omitempty"`
	AcceptanceClauseCount      int      `json:"assurance_clause_count,omitempty"`
	GoalCoveredCount           int      `json:"obligation_covered_count,omitempty"`
	AcceptanceCoveredCount     int      `json:"assurance_covered_count,omitempty"`
	MissingGoalClauseIDs       []string `json:"missing_obligation_clause_ids,omitempty"`
	MissingAcceptanceClauseIDs []string `json:"missing_assurance_clause_ids,omitempty"`
}

func BuildObjectiveIntegritySummary(runDir string) (ObjectiveIntegritySummary, error) {
	if strings.TrimSpace(runDir) == "" {
		return ObjectiveIntegritySummary{}, nil
	}
	contract, err := LoadObjectiveContract(ObjectiveContractPath(runDir))
	if err != nil {
		return ObjectiveIntegritySummary{}, err
	}
	if contract == nil {
		return ObjectiveIntegritySummary{}, nil
	}
	obligationModel, err := LoadObligationModel(ObligationModelPath(runDir))
	if err != nil {
		return ObjectiveIntegritySummary{}, err
	}
	assurancePlan, err := LoadAssurancePlan(AssurancePlanPath(runDir))
	if err != nil {
		return ObjectiveIntegritySummary{}, err
	}

	goalClauses := objectiveClausesBySurface(contract, objectiveRequiredSurfaceGoal)
	acceptanceClauses := objectiveClausesBySurface(contract, objectiveRequiredSurfaceAcceptance)
	goalCoverage := map[string]int{}
	if obligationModel != nil && obligationModelHasContent(obligationModel) {
		goalCoverage = requiredObligationCoverageCounts(obligationModel)
	}
	acceptanceCoverage := assuranceCoverageCounts(assurancePlan)

	summary := ObjectiveIntegritySummary{
		ContractPresent:       true,
		ContractState:         strings.TrimSpace(contract.State),
		ContractLocked:        strings.TrimSpace(contract.State) == objectiveContractStateLocked,
		ClauseCount:           len(contract.Clauses),
		GoalClauseCount:       len(goalClauses),
		AcceptanceClauseCount: len(acceptanceClauses),
	}
	for clauseID := range goalClauses {
		if goalCoverage[clauseID] > 0 {
			summary.GoalCoveredCount++
			continue
		}
		summary.MissingGoalClauseIDs = append(summary.MissingGoalClauseIDs, clauseID)
	}
	for clauseID := range acceptanceClauses {
		if acceptanceCoverage[clauseID] > 0 {
			summary.AcceptanceCoveredCount++
			continue
		}
		summary.MissingAcceptanceClauseIDs = append(summary.MissingAcceptanceClauseIDs, clauseID)
	}
	sort.Strings(summary.MissingGoalClauseIDs)
	sort.Strings(summary.MissingAcceptanceClauseIDs)
	return summary, nil
}

func refreshBoundaryEstablishmentOperation(runDir string) error {
	summary, err := BuildObjectiveIntegritySummary(runDir)
	if err != nil {
		return err
	}
	if !summary.ContractPresent {
		return clearControlOperationTarget(runDir, BoundaryEstablishmentOperationKey())
	}
	pendingConditions := make([]string, 0, 3)
	if !summary.ContractLocked {
		pendingConditions = append(pendingConditions, "objective_contract_locked")
	}
	if len(summary.MissingGoalClauseIDs) > 0 {
		pendingConditions = append(pendingConditions, "obligation_required_coverage_ready")
	}
	if len(summary.MissingAcceptanceClauseIDs) > 0 {
		pendingConditions = append(pendingConditions, "assurance_required_coverage_ready")
	}
	target := ControlOperationTarget{
		Kind:              ControlOperationKindBoundaryEstablishment,
		PendingConditions: pendingConditions,
	}
	if len(pendingConditions) == 0 {
		target.State = ControlOperationStateCommitted
		target.Summary = "boundary establishment committed"
	} else {
		target.State = ControlOperationStateAwaitingAgent
		switch {
		case !summary.ContractLocked:
			target.Summary = "objective contract still draft"
		case len(summary.MissingGoalClauseIDs) > 0 || len(summary.MissingAcceptanceClauseIDs) > 0:
			target.Summary = "required boundary coverage still incomplete"
		default:
			target.Summary = "boundary establishment still in progress"
		}
	}
	return submitControlOperationTarget(runDir, BoundaryEstablishmentOperationKey(), target)
}

func (summary ObjectiveIntegritySummary) ReadyForNoShrinkEnforcement() bool {
	if !summary.ContractPresent {
		return false
	}
	return summary.ContractLocked
}

func (summary ObjectiveIntegritySummary) IntegrityOK() bool {
	if !summary.ReadyForNoShrinkEnforcement() {
		return false
	}
	return len(summary.MissingGoalClauseIDs) == 0 && len(summary.MissingAcceptanceClauseIDs) == 0
}

func validateGoalStateIntegrity(runDir string, state *GoalState) error {
	if strings.TrimSpace(runDir) == "" || state == nil {
		return nil
	}
	contract, err := LoadObjectiveContract(ObjectiveContractPath(runDir))
	if err != nil {
		return err
	}
	if contract == nil || contract.State != objectiveContractStateLocked {
		return nil
	}
	return validateLockedObjectiveGoalCoverage(contract, state)
}

func validateLockedObjectiveGoalCoverage(contract *ObjectiveContract, state *GoalState) error {
	if contract == nil || state == nil {
		return nil
	}
	goalClauses := objectiveClausesBySurface(contract, objectiveRequiredSurfaceGoal)
	if len(goalClauses) == 0 {
		return nil
	}

	requiredCoverage := requiredGoalCoverageCounts(state)
	for _, item := range state.Required {
		if len(requiredGoalCoverageIDs(item)) == 0 {
			return fmt.Errorf("required goal item %s is missing covers", item.ID)
		}
		for _, clauseID := range requiredGoalCoverageIDs(item) {
			if _, ok := goalClauses[clauseID]; !ok {
				return fmt.Errorf("goal item %s references unknown objective clause %q", item.ID, clauseID)
			}
		}
	}
	for _, item := range state.Optional {
		for _, clauseID := range requiredGoalCoverageIDs(item) {
			if _, ok := goalClauses[clauseID]; !ok {
				return fmt.Errorf("goal item %s references unknown objective clause %q", item.ID, clauseID)
			}
		}
	}
	for clauseID := range goalClauses {
		if requiredCoverage[clauseID] == 0 {
			return fmt.Errorf("objective clause %s requires required goal coverage", clauseID)
		}
	}
	return nil
}

func validateAssurancePlanIntegrity(runDir string, plan *AssurancePlan) error {
	if strings.TrimSpace(runDir) == "" || plan == nil {
		return nil
	}
	contract, err := LoadObjectiveContract(ObjectiveContractPath(runDir))
	if err != nil {
		return err
	}
	if contract == nil || contract.State != objectiveContractStateLocked {
		return nil
	}
	return validateLockedObjectiveAssuranceCoverage(contract, plan)
}

func validateObligationModelIntegrity(runDir string, model *ObligationModel) error {
	if strings.TrimSpace(runDir) == "" || model == nil {
		return nil
	}
	contract, err := LoadObjectiveContract(ObjectiveContractPath(runDir))
	if err != nil {
		return err
	}
	if contract == nil || contract.State != objectiveContractStateLocked {
		return nil
	}
	return validateLockedObjectiveObligationCoverage(contract, model)
}

func validateLockedObjectiveObligationCoverage(contract *ObjectiveContract, model *ObligationModel) error {
	if contract == nil || model == nil {
		return nil
	}
	goalClauses := objectiveClausesBySurface(contract, objectiveRequiredSurfaceGoal)
	if len(goalClauses) == 0 {
		return nil
	}
	coverage := requiredObligationCoverageCounts(model)
	for _, item := range model.Required {
		if len(compactStrings(item.CoversClauses)) == 0 {
			return fmt.Errorf("required obligation %s is missing covers_clauses", item.ID)
		}
		for _, clauseID := range compactStrings(item.CoversClauses) {
			if _, ok := goalClauses[clauseID]; !ok {
				return fmt.Errorf("obligation %s references unknown objective clause %q", item.ID, clauseID)
			}
		}
	}
	for _, item := range append(append([]ObligationItem{}, model.Optional...), model.Guardrails...) {
		for _, clauseID := range compactStrings(item.CoversClauses) {
			if _, ok := goalClauses[clauseID]; !ok {
				return fmt.Errorf("obligation %s references unknown objective clause %q", item.ID, clauseID)
			}
		}
	}
	for clauseID := range goalClauses {
		if coverage[clauseID] == 0 {
			return fmt.Errorf("objective clause %s requires required obligation coverage", clauseID)
		}
	}
	return nil
}

func validateLockedObjectiveAssuranceCoverage(contract *ObjectiveContract, plan *AssurancePlan) error {
	if contract == nil || plan == nil {
		return nil
	}
	acceptanceClauses := objectiveClausesBySurface(contract, objectiveRequiredSurfaceAcceptance)
	if len(acceptanceClauses) == 0 {
		return nil
	}
	coverage := assuranceCoverageCounts(plan)
	for _, scenario := range plan.Scenarios {
		if len(compactStrings(scenario.CoversObligations)) == 0 {
			return fmt.Errorf("assurance scenario %s is missing covers_obligations", scenario.ID)
		}
		for _, clauseID := range compactStrings(scenario.CoversObligations) {
			if _, ok := acceptanceClauses[clauseID]; !ok {
				return fmt.Errorf("assurance scenario %s references unknown objective clause %q", scenario.ID, clauseID)
			}
		}
	}
	for clauseID := range acceptanceClauses {
		if coverage[clauseID] == 0 {
			return fmt.Errorf("objective clause %s requires assurance coverage", clauseID)
		}
	}
	return nil
}

func objectiveClausesBySurface(contract *ObjectiveContract, surface ObjectiveRequiredSurface) map[string]ObjectiveClause {
	if contract == nil {
		return nil
	}
	clauses := make(map[string]ObjectiveClause)
	for _, clause := range contract.Clauses {
		for _, requiredSurface := range clause.RequiredSurfaces {
			if requiredSurface == surface {
				clauses[clause.ID] = clause
				break
			}
		}
	}
	return clauses
}

func requiredGoalCoverageCounts(state *GoalState) map[string]int {
	coverage := map[string]int{}
	if state == nil {
		return coverage
	}
	for _, item := range state.Required {
		for _, clauseID := range requiredGoalCoverageIDs(item) {
			coverage[clauseID]++
		}
	}
	return coverage
}

func assuranceCoverageCounts(plan *AssurancePlan) map[string]int {
	coverage := map[string]int{}
	if plan == nil {
		return coverage
	}
	for _, scenario := range plan.Scenarios {
		for _, clauseID := range compactStrings(scenario.CoversObligations) {
			coverage[clauseID]++
		}
	}
	return coverage
}

func requiredGoalCoverageIDs(item GoalItem) []string {
	return trimmedGoalCovers(item.Covers)
}

func requiredObligationCoverageCounts(model *ObligationModel) map[string]int {
	coverage := map[string]int{}
	if model == nil {
		return coverage
	}
	for _, item := range model.Required {
		for _, clauseID := range compactStrings(item.CoversClauses) {
			coverage[clauseID]++
		}
	}
	return coverage
}
