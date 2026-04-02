package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type QualityDebt struct {
	SuccessDimensionUnowned      []string `json:"success_dimension_unowned,omitempty"`
	ProofPlanGap                 []string `json:"proof_plan_gap,omitempty"`
	CriticGateMissing            bool     `json:"critic_gate_missing,omitempty"`
	FinisherGateMissing          bool     `json:"finisher_gate_missing,omitempty"`
	OnlyCorrectnessEvidence      bool     `json:"only_correctness_evidence_present,omitempty"`
	DomainPackMissing            bool     `json:"domain_pack_missing_for_nontrivial_run,omitempty"`
	RequiredEvidenceStale        []string `json:"required_evidence_stale,omitempty"`
	RequiredCognitionUnsatisfied []string `json:"required_cognition_unsatisfied,omitempty"`
	ImpactResolutionUnknown      bool     `json:"impact_resolution_unknown,omitempty"`
}

func BuildQualityDebt(runDir string) (*QualityDebt, error) {
	successModel, err := LoadSuccessModel(SuccessModelPath(runDir))
	if err != nil {
		return nil, err
	}
	goalState, err := LoadCanonicalGoalState(runDir)
	if err != nil {
		return nil, err
	}
	coordination, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil {
		return nil, err
	}
	proofPlan, err := LoadProofPlan(ProofPlanPath(runDir))
	if err != nil {
		return nil, err
	}
	workflowPlan, err := LoadWorkflowPlan(WorkflowPlanPath(runDir))
	if err != nil {
		return nil, err
	}
	assurancePlan, err := LoadAssurancePlan(AssurancePlanPath(runDir))
	if err != nil {
		return nil, err
	}
	freshnessState, err := LoadFreshnessState(FreshnessStatePath(runDir))
	if err != nil {
		return nil, err
	}
	impactState, err := LoadImpactState(ImpactStatePath(runDir))
	if err != nil {
		return nil, err
	}
	cognitionState, err := LoadCognitionState(CognitionStatePath(runDir))
	if err != nil {
		return nil, err
	}

	debt := &QualityDebt{}
	if successModel != nil {
		for _, dimension := range successModel.Dimensions {
			if !dimension.Required {
				continue
			}
			if !dimensionOwned(dimension.ID, goalState, coordination) {
				debt.SuccessDimensionUnowned = append(debt.SuccessDimensionUnowned, dimension.ID)
			}
		}
	}
	sort.Strings(debt.SuccessDimensionUnowned)

	for _, item := range requiredProofItems(proofPlan) {
		if !proofItemSatisfied(runDir, item, nil) {
			debt.ProofPlanGap = append(debt.ProofPlanGap, item.ID)
		}
	}
	sort.Strings(debt.ProofPlanGap)

	builderEvidence := hasBuilderEvidence(runDir, nil)
	if workflowRequiresRole(workflowPlan, "critic") && builderEvidence && !sessionRoleKindPresent(runDir, "critic") {
		debt.CriticGateMissing = true
	}
	if workflowRequiresRole(workflowPlan, "finisher") && builderEvidence && !finisherEvidencePresent(runDir) && !sessionRoleKindPresent(runDir, "finisher") {
		debt.FinisherGateMissing = true
	}
	if builderEvidence && !nonCorrectnessEvidencePresent(runDir) {
		debt.OnlyCorrectnessEvidence = true
	}
	if !fileExists(DomainPackPath(runDir)) {
		if successModel != nil {
			debt.DomainPackMissing = true
		}
	}
	for _, scenario := range requiredCloseoutScenarios(assurancePlan) {
		if evidenceFreshnessForScenario(freshnessState, scenario.ID) == freshnessStateStale {
			debt.RequiredEvidenceStale = append(debt.RequiredEvidenceStale, scenario.ID)
		}
		if !scenarioCognitionTierSatisfied(cognitionState, scenario.GatePolicy.RequiredCognitionTier) {
			debt.RequiredCognitionUnsatisfied = append(debt.RequiredCognitionUnsatisfied, scenario.ID)
		}
	}
	sort.Strings(debt.RequiredEvidenceStale)
	sort.Strings(debt.RequiredCognitionUnsatisfied)
	if impactState == nil || strings.TrimSpace(impactState.ResolverKind) == "none" {
		if assurancePlan != nil && len(assurancePlan.Scenarios) > 0 {
			debt.ImpactResolutionUnknown = true
		}
	}

	if successModel == nil && assurancePlan == nil && freshnessState == nil && impactState == nil && cognitionState == nil {
		return nil, nil
	}
	if debt.Zero() {
		return &QualityDebt{}, nil
	}
	return debt, nil
}

func (d *QualityDebt) Zero() bool {
	if d == nil {
		return true
	}
	return len(d.SuccessDimensionUnowned) == 0 &&
		len(d.ProofPlanGap) == 0 &&
		!d.CriticGateMissing &&
		!d.FinisherGateMissing &&
		!d.OnlyCorrectnessEvidence &&
		!d.DomainPackMissing &&
		len(d.RequiredEvidenceStale) == 0 &&
		len(d.RequiredCognitionUnsatisfied) == 0 &&
		!d.ImpactResolutionUnknown
}

func dimensionOwned(dimensionID string, goalState *GoalState, coordination *CoordinationState) bool {
	dimensionID = strings.TrimSpace(dimensionID)
	if dimensionID == "" {
		return true
	}
	if dimensionID == "dim-objective" {
		if goalState != nil && len(goalState.Required) > 0 {
			return true
		}
		if coordination != nil && len(coordination.Required) > 0 {
			return true
		}
		return false
	}
	if coordination != nil {
		if item, ok := coordination.Required[dimensionID]; ok {
			if strings.TrimSpace(item.Owner) != "" {
				return true
			}
		}
	}
	return false
}

func requiredProofItems(plan *ProofPlan) []ProofPlanItem {
	if plan == nil || len(plan.Items) == 0 {
		return nil
	}
	out := make([]ProofPlanItem, 0, len(plan.Items))
	for _, item := range plan.Items {
		if item.Required {
			out = append(out, item)
		}
	}
	return out
}

func proofItemSatisfied(runDir string, item ProofPlanItem, acceptance *AcceptanceState) bool {
	switch strings.TrimSpace(item.SourceSurface) {
	case "assurance", "acceptance":
		return acceptanceEvidencePresent(runDir, acceptance)
	case "summary":
		return fileExists(SummaryPath(runDir))
	case "completion_proof", "completion-proof":
		return fileExists(CompletionStatePath(runDir))
	case "report":
		return reportsPresent(runDir)
	case "artifact", "run_artifacts":
		return runArtifactsPresent(runDir)
	default:
		return false
	}
}

func hasBuilderEvidence(runDir string, acceptance *AcceptanceState) bool {
	return acceptanceEvidencePresent(runDir, acceptance) || nonCorrectnessEvidencePresent(runDir)
}

func acceptanceEvidencePresent(runDir string, state *AcceptanceState) bool {
	if state == nil {
		return evidenceLogPresent(runDir)
	}
	if strings.TrimSpace(state.LastResult.CheckedAt) != "" {
		return true
	}
	return len(state.Checks) > 0 || evidenceLogPresent(runDir)
}

func nonCorrectnessEvidencePresent(runDir string) bool {
	return finisherEvidencePresent(runDir) || reportsPresent(runDir) || runArtifactsPresent(runDir)
}

func finisherEvidencePresent(runDir string) bool {
	return fileExists(SummaryPath(runDir)) || fileExists(CompletionStatePath(runDir))
}

func reportsPresent(runDir string) bool {
	entries, err := os.ReadDir(ReportsDir(runDir))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			return true
		}
	}
	return false
}

func runArtifactsPresent(runDir string) bool {
	manifest, err := LoadArtifacts(ArtifactsPath(runDir))
	if err != nil || manifest == nil {
		return false
	}
	for _, session := range manifest.Sessions {
		if len(session.Artifacts) > 0 {
			return true
		}
	}
	return false
}

func workflowRequiresRole(plan *WorkflowPlan, role string) bool {
	if plan == nil {
		return false
	}
	role = strings.ToLower(strings.TrimSpace(role))
	for _, required := range plan.RequiredRoles {
		if !required.Required {
			continue
		}
		if strings.ToLower(strings.TrimSpace(required.ID)) == role {
			return true
		}
	}
	for _, gate := range plan.Gates {
		if strings.Contains(strings.ToLower(strings.TrimSpace(gate)), role+"_") {
			return true
		}
	}
	return false
}

func sessionRoleKindPresent(runDir, needle string) bool {
	entries, err := os.ReadDir(filepath.Join(runDir, "sessions"))
	if err != nil {
		return false
	}
	needle = strings.ToLower(strings.TrimSpace(needle))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, entry.Name()))
		if err != nil || identity == nil {
			continue
		}
		if strings.Contains(strings.ToLower(strings.TrimSpace(identity.RoleKind)), needle) {
			return true
		}
	}
	return false
}

func evidenceLogPresent(runDir string) bool {
	events, err := LoadEvidenceLog(EvidenceLogPath(runDir))
	if err != nil {
		return false
	}
	return len(events) > 0
}

func requiredCloseoutScenarios(plan *AssurancePlan) []AssuranceScenario {
	if plan == nil {
		return nil
	}
	out := make([]AssuranceScenario, 0, len(plan.Scenarios))
	for _, scenario := range plan.Scenarios {
		if strings.TrimSpace(scenario.GatePolicy.Closeout) == "required" {
			out = append(out, scenario)
		}
	}
	return out
}

func evidenceFreshnessForScenario(state *FreshnessState, scenarioID string) string {
	if state == nil {
		return ""
	}
	for _, item := range state.Evidence {
		if item.ScenarioID == scenarioID {
			return item.State
		}
	}
	return ""
}

func scenarioCognitionTierSatisfied(state *CognitionState, tier string) bool {
	switch strings.TrimSpace(tier) {
	case "", "none", "repo-native":
		return true
	case "graph":
		if state == nil {
			return false
		}
		for _, scope := range state.Scopes {
			for _, provider := range scope.Providers {
				if provider.Name == "gitnexus" && provider.Available {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}
