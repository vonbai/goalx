package cli

import "strings"

type RunCloseoutFacts struct {
	StatusPhase                  string   `json:"status_phase,omitempty"`
	SummaryExists                bool     `json:"summary_exists,omitempty"`
	CompletionExists             bool     `json:"completion_exists,omitempty"`
	MasterUnread                 int      `json:"master_unread,omitempty"`
	RunIntent                    string   `json:"run_intent,omitempty"`
	EvolveFrontierState          string   `json:"evolve_frontier_state,omitempty"`
	EvolveOpenCandidateCount     int      `json:"evolve_open_candidate_count,omitempty"`
	EvolveManagementGap          string   `json:"evolve_management_gap,omitempty"`
	ObjectiveContractPresent     bool     `json:"objective_contract_present,omitempty"`
	ObjectiveContractLocked      bool     `json:"objective_contract_locked,omitempty"`
	ObjectiveIntegrityReady      bool     `json:"objective_integrity_ready,omitempty"`
	ObjectiveIntegrityOK         bool     `json:"objective_integrity_ok,omitempty"`
	MissingGoalClauseIDs         []string `json:"missing_obligation_clause_ids,omitempty"`
	MissingAcceptanceClauseIDs   []string `json:"missing_assurance_clause_ids,omitempty"`
	SuccessPlanePresent          bool     `json:"success_plane_present,omitempty"`
	SuccessModelExists           bool     `json:"success_model_exists,omitempty"`
	ProofPlanExists              bool     `json:"proof_plan_exists,omitempty"`
	WorkflowPlanExists           bool     `json:"workflow_plan_exists,omitempty"`
	QualityDebtZero              bool     `json:"quality_debt_zero,omitempty"`
	QualityDebtPresent           bool     `json:"quality_debt_present,omitempty"`
	CriticGateMissing            bool     `json:"critic_gate_missing,omitempty"`
	FinisherGateMissing          bool     `json:"finisher_gate_missing,omitempty"`
	SuccessDimensionUnowned      []string `json:"success_dimension_unowned,omitempty"`
	ProofPlanGap                 []string `json:"proof_plan_gap,omitempty"`
	RequiredEvidenceStale        []string `json:"required_evidence_stale,omitempty"`
	RequiredCognitionUnsatisfied []string `json:"required_cognition_unsatisfied,omitempty"`
	ImpactResolutionUnknown      bool     `json:"impact_resolution_unknown,omitempty"`
	Complete                     bool     `json:"complete,omitempty"`
}

type RunCloseoutMaintenanceAction string

const (
	RunCloseoutMaintenanceActionNone          RunCloseoutMaintenanceAction = ""
	RunCloseoutMaintenanceActionRecoverMaster RunCloseoutMaintenanceAction = "recover_master"
	RunCloseoutMaintenanceActionFinalize      RunCloseoutMaintenanceAction = "finalize"
)

func BuildRunCloseoutFacts(runDir string) (RunCloseoutFacts, error) {
	status, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		return RunCloseoutFacts{}, err
	}
	facts := RunCloseoutFacts{
		SummaryExists:      fileExists(SummaryPath(runDir)),
		CompletionExists:   fileExists(CompletionStatePath(runDir)),
		MasterUnread:       unreadControlInboxCount(MasterInboxPath(runDir), MasterCursorPath(runDir)),
		SuccessModelExists: fileExists(SuccessModelPath(runDir)),
		ProofPlanExists:    fileExists(ProofPlanPath(runDir)),
		WorkflowPlanExists: fileExists(WorkflowPlanPath(runDir)),
	}
	facts.SuccessPlanePresent = facts.SuccessModelExists || facts.ProofPlanExists || facts.WorkflowPlanExists
	if status != nil {
		facts.StatusPhase = strings.TrimSpace(status.Phase)
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return RunCloseoutFacts{}, err
	}
	if meta != nil {
		facts.RunIntent = strings.TrimSpace(meta.Intent)
	}
	if facts.RunIntent == runIntentEvolve {
		evolveFacts, err := BuildEvolveFacts(runDir)
		if err != nil {
			return RunCloseoutFacts{}, err
		}
		if evolveFacts != nil {
			facts.EvolveFrontierState = strings.TrimSpace(evolveFacts.FrontierState)
			facts.EvolveOpenCandidateCount = evolveFacts.OpenCandidateCount
			facts.EvolveManagementGap = strings.TrimSpace(evolveFacts.ManagementGap)
		}
	}
	integrity, err := BuildObjectiveIntegritySummary(runDir)
	if err != nil {
		return RunCloseoutFacts{}, err
	}
	facts.ObjectiveContractPresent = integrity.ContractPresent
	facts.ObjectiveContractLocked = integrity.ContractLocked
	facts.ObjectiveIntegrityReady = integrity.ReadyForNoShrinkEnforcement()
	facts.ObjectiveIntegrityOK = integrity.IntegrityOK()
	facts.MissingGoalClauseIDs = append([]string(nil), integrity.MissingGoalClauseIDs...)
	facts.MissingAcceptanceClauseIDs = append([]string(nil), integrity.MissingAcceptanceClauseIDs...)
	debt, err := BuildQualityDebt(runDir)
	if err != nil {
		return RunCloseoutFacts{}, err
	}
	if debt != nil {
		facts.QualityDebtPresent = true
		facts.QualityDebtZero = debt.Zero()
		facts.CriticGateMissing = debt.CriticGateMissing
		facts.FinisherGateMissing = debt.FinisherGateMissing
		facts.SuccessDimensionUnowned = append([]string(nil), debt.SuccessDimensionUnowned...)
		facts.ProofPlanGap = append([]string(nil), debt.ProofPlanGap...)
		facts.RequiredEvidenceStale = append([]string(nil), debt.RequiredEvidenceStale...)
		facts.RequiredCognitionUnsatisfied = append([]string(nil), debt.RequiredCognitionUnsatisfied...)
		facts.ImpactResolutionUnknown = debt.ImpactResolutionUnknown
	}
	facts.Complete = facts.StatusPhase == "complete" && facts.SummaryExists && facts.CompletionExists
	return facts, nil
}

func (facts RunCloseoutFacts) ReadyToFinalize() bool {
	return facts.Complete && facts.MasterUnread == 0 && facts.objectiveCloseoutReady() && facts.evolveCloseoutReady()
}

func (facts RunCloseoutFacts) objectiveCloseoutReady() bool {
	if facts.ObjectiveContractPresent && !facts.ObjectiveContractLocked {
		return false
	}
	if facts.ObjectiveContractPresent && !facts.ObjectiveIntegrityOK {
		return false
	}
	if len(facts.RequiredEvidenceStale) > 0 || len(facts.RequiredCognitionUnsatisfied) > 0 {
		return false
	}
	return true
}

func (facts RunCloseoutFacts) needsMasterFollowup() bool {
	if !facts.Complete {
		return false
	}
	if facts.MasterUnread > 0 {
		return true
	}
	return !facts.objectiveCloseoutReady() || !facts.evolveCloseoutReady()
}

func (facts RunCloseoutFacts) MaintenanceAction(master TargetPresenceFacts) RunCloseoutMaintenanceAction {
	if facts.ReadyToFinalize() {
		return RunCloseoutMaintenanceActionFinalize
	}
	if facts.needsMasterFollowup() && targetPresenceMissing(master) {
		return RunCloseoutMaintenanceActionRecoverMaster
	}
	return RunCloseoutMaintenanceActionNone
}

func (facts RunCloseoutFacts) evolveCloseoutReady() bool {
	if strings.TrimSpace(facts.RunIntent) != runIntentEvolve {
		return true
	}
	if strings.TrimSpace(facts.EvolveFrontierState) != EvolveFrontierStopped {
		return false
	}
	if strings.TrimSpace(facts.EvolveManagementGap) != "" {
		return false
	}
	return facts.EvolveOpenCandidateCount == 0
}
