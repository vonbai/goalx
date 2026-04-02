package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type DurableMutation struct {
	Surface DurableSurfaceName
	Kind    string
	Actor   string
	At      string
	Body    json.RawMessage
}

type objectiveContractAuthoringBody struct {
	ObjectiveHash string            `json:"objective_hash"`
	State         string            `json:"state"`
	Clauses       []ObjectiveClause `json:"clauses"`
}

type obligationModelAuthoringBody struct {
	ObjectiveContractHash string           `json:"objective_contract_hash"`
	Required              []ObligationItem `json:"required"`
	Optional              []ObligationItem `json:"optional,omitempty"`
	Guardrails            []ObligationItem `json:"guardrails,omitempty"`
}

type assurancePlanAuthoringBody struct {
	ObligationRefs []string            `json:"obligation_refs,omitempty"`
	Scenarios      []AssuranceScenario `json:"scenarios"`
}

type cognitionStateAuthoringBody struct {
	Scopes []CognitionScopeState `json:"scopes"`
}

type successModelAuthoringBody struct {
	ObjectiveContractHash string             `json:"objective_contract_hash"`
	ObligationModelHash   string             `json:"obligation_model_hash"`
	Dimensions            []SuccessDimension `json:"dimensions"`
	AntiGoals             []SuccessAntiGoal  `json:"anti_goals,omitempty"`
	CloseoutRequirements  []string           `json:"closeout_requirements,omitempty"`
}

type proofPlanAuthoringBody struct {
	Items []ProofPlanItem `json:"items"`
}

type workflowPlanAuthoringBody struct {
	RequiredRoles []WorkflowRoleRequirement `json:"required_roles"`
	Gates         []string                  `json:"gates"`
}

type domainPackAuthoringBody struct {
	Domain        string          `json:"domain"`
	Signals       []string        `json:"signals,omitempty"`
	Slots         DomainPackSlots `json:"slots,omitempty"`
	PriorEntryIDs []string        `json:"prior_entry_ids,omitempty"`
}

type compilerInputAuthoringBody struct {
	ObjectiveContractRef string              `json:"objective_contract_ref"`
	ObligationModelRef   string              `json:"obligation_model_ref"`
	MemoryQueryRef       string              `json:"memory_query_ref,omitempty"`
	MemoryContextRef     string              `json:"memory_context_ref,omitempty"`
	PolicySourceRefs     []string            `json:"policy_source_refs,omitempty"`
	SelectedPriorRefs    []string            `json:"selected_prior_refs,omitempty"`
	SourceSlots          []CompilerInputSlot `json:"source_slots,omitempty"`
}

type compilerReportAuthoringBody struct {
	AvailableSourceSlots []CompilerReportSlot    `json:"available_source_slots,omitempty"`
	SelectedPriorRefs    []string                `json:"selected_prior_refs,omitempty"`
	RejectedPriors       []CompilerRejectedPrior `json:"rejected_priors,omitempty"`
	OutputSources        []CompilerOutputSource  `json:"output_sources,omitempty"`
}

type impactStateAuthoringBody struct {
	Scope            string   `json:"scope"`
	BaselineRevision string   `json:"baseline_revision"`
	HeadRevision     string   `json:"head_revision"`
	ResolverKind     string   `json:"resolver_kind"`
	ChangedFiles     []string `json:"changed_files,omitempty"`
	ChangedSymbols   []string `json:"changed_symbols,omitempty"`
	ChangedProcesses []string `json:"changed_processes,omitempty"`
}

type freshnessStateAuthoringBody struct {
	Cognition []CognitionFreshnessItem `json:"cognition,omitempty"`
	Evidence  []EvidenceFreshnessItem  `json:"evidence,omitempty"`
}

type resourceStateAuthoringBody struct {
	Host           *ResourceHostFacts   `json:"host,omitempty"`
	PSI            *ResourcePSIFacts    `json:"psi,omitempty"`
	Cgroup         *ResourceCgroupFacts `json:"cgroup,omitempty"`
	GoalxProcesses *GoalXProcessFacts   `json:"goalx_processes,omitempty"`
	HeadroomBytes  int64                `json:"headroom_bytes"`
	State          string               `json:"state"`
	Reasons        []string             `json:"reasons,omitempty"`
}

type coordinationAuthoringBody struct {
	PlanSummary   []string                            `json:"plan_summary,omitempty"`
	Required      map[string]CoordinationRequiredItem `json:"required,omitempty"`
	Sessions      map[string]CoordinationSession      `json:"sessions,omitempty"`
	Decision      *CoordinationDecision               `json:"decision,omitempty"`
	OpenQuestions []string                            `json:"open_questions,omitempty"`
}

type statusAuthoringBody struct {
	Phase             string   `json:"phase"`
	RequiredRemaining *int     `json:"required_remaining"`
	OpenRequiredIDs   []string `json:"open_required_ids,omitempty"`
	ActiveSessions    []string `json:"active_sessions,omitempty"`
	KeepSession       string   `json:"keep_session,omitempty"`
	LastVerifiedAt    string   `json:"last_verified_at,omitempty"`
}

type experimentCreatedAuthoringBody struct {
	ExperimentID     string `json:"experiment_id"`
	Session          string `json:"session,omitempty"`
	Branch           string `json:"branch,omitempty"`
	Worktree         string `json:"worktree,omitempty"`
	Intent           string `json:"intent,omitempty"`
	BaseRef          string `json:"base_ref,omitempty"`
	BaseExperimentID string `json:"base_experiment_id,omitempty"`
}

type experimentIntegratedAuthoringBody struct {
	IntegrationID       string   `json:"integration_id"`
	ResultExperimentID  string   `json:"result_experiment_id"`
	SourceExperimentIDs []string `json:"source_experiment_ids"`
	Method              string   `json:"method"`
	ResultBranch        string   `json:"result_branch,omitempty"`
	ResultCommit        string   `json:"result_commit,omitempty"`
}

type experimentClosedAuthoringBody struct {
	ExperimentID            string `json:"experiment_id"`
	Disposition             string `json:"disposition"`
	Reason                  string `json:"reason"`
	ReplacementExperimentID string `json:"replacement_experiment_id,omitempty"`
}

type evolveStoppedAuthoringBody struct {
	ReasonCode       string `json:"reason_code"`
	Reason           string `json:"reason"`
	BestExperimentID string `json:"best_experiment_id,omitempty"`
}

func ApplyDurableMutation(runDir string, mutation DurableMutation) error {
	spec, err := LookupDurableSurface(string(mutation.Surface))
	if err != nil {
		return err
	}
	if spec.Class == DurableSurfaceClassArtifact {
		return fmt.Errorf("surface %q is not machine-consumed", spec.Name)
	}
	if len(bytes.TrimSpace(mutation.Body)) == 0 {
		return fmt.Errorf("durable authoring body is empty")
	}
	switch spec.Class {
	case DurableSurfaceClassStructuredState:
		if strings.TrimSpace(mutation.Kind) != "" {
			return fmt.Errorf("structured surface %q does not accept kind", spec.Name)
		}
		if strings.TrimSpace(mutation.Actor) != "" {
			return fmt.Errorf("structured surface %q does not accept actor", spec.Name)
		}
		if strings.TrimSpace(mutation.At) != "" {
			return fmt.Errorf("structured surface %q does not accept explicit timestamp", spec.Name)
		}
		return applyDurableStructuredMutation(runDir, spec, mutation.Body)
	case DurableSurfaceClassEventLog:
		return applyDurableEventMutation(runDir, spec, mutation)
	default:
		return fmt.Errorf("surface %q is not machine-consumed", spec.Name)
	}
}

func applyDurableStructuredMutation(runDir string, spec DurableSurfaceSpec, body []byte) error {
	path := spec.Path(runDir)
	return withExclusiveFileLock(path, func() error {
		switch spec.Name {
		case DurableSurfaceObjectiveContract:
			contract, err := parseObjectiveContractAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveObjectiveContract(path, contract)
		case DurableSurfaceObligationModel:
			model, err := parseObligationModelAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveObligationModel(path, model)
		case DurableSurfaceAssurancePlan:
			plan, err := parseAssurancePlanAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveAssurancePlan(path, plan)
		case DurableSurfaceCognitionState:
			state, err := parseCognitionStateAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveCognitionState(path, state)
		case DurableSurfaceSuccessModel:
			model, err := parseSuccessModelAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveSuccessModel(path, model)
		case DurableSurfaceProofPlan:
			plan, err := parseProofPlanAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveProofPlan(path, plan)
		case DurableSurfaceWorkflowPlan:
			plan, err := parseWorkflowPlanAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveWorkflowPlan(path, plan)
		case DurableSurfaceDomainPack:
			pack, err := parseDomainPackAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveDomainPack(path, pack)
		case DurableSurfaceCompilerInput:
			input, err := parseCompilerInputAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveCompilerInput(path, input)
		case DurableSurfaceCompilerReport:
			report, err := parseCompilerReportAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveCompilerReport(path, report)
		case DurableSurfaceImpactState:
			state, err := parseImpactStateAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveImpactState(path, state)
		case DurableSurfaceFreshnessState:
			state, err := parseFreshnessStateAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveFreshnessState(path, state)
		case DurableSurfaceResourceState:
			state, err := parseResourceStateAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveResourceState(path, state)
		case DurableSurfaceCoordination:
			state, err := parseCoordinationAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveCoordinationState(path, state)
		case DurableSurfaceStatus:
			record, err := parseStatusAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveRunStatusRecord(path, record)
		default:
			return fmt.Errorf("surface %q is not machine-consumed", spec.Name)
		}
	})
}

func applyDurableEventMutation(runDir string, spec DurableSurfaceSpec, mutation DurableMutation) error {
	path := spec.Path(runDir)
	line, err := buildDurableEventLine(spec.Name, mutation)
	if err != nil {
		return err
	}
	return withExclusiveFileLock(path, func() error {
		existing, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		var buf bytes.Buffer
		if len(existing) > 0 {
			buf.Write(existing)
			if existing[len(existing)-1] != '\n' {
				buf.WriteByte('\n')
			}
		}
		buf.Write(line)
		buf.WriteByte('\n')
		return writeFileAtomic(path, buf.Bytes(), 0o644)
	})
}

func buildDurableEventLine(surface DurableSurfaceName, mutation DurableMutation) ([]byte, error) {
	kind := strings.TrimSpace(mutation.Kind)
	if kind == "" {
		return nil, fmt.Errorf("event-log surface %q requires --kind", surface)
	}
	actor := strings.TrimSpace(mutation.Actor)
	if actor == "" {
		return nil, fmt.Errorf("event-log surface %q requires --actor", surface)
	}
	recordedAt := strings.TrimSpace(mutation.At)
	if recordedAt == "" {
		recordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	body, err := compileDurableEventBody(surface, kind, mutation.Body, recordedAt)
	if err != nil {
		return nil, durableSchemaHintError(surface, err)
	}
	event := DurableLogEvent{
		Version: 1,
		Kind:    kind,
		At:      recordedAt,
		Actor:   actor,
		Body:    body,
	}
	if err := validateDurableLogEvent(event, surface); err != nil {
		return nil, durableSchemaHintError(surface, err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func compileDurableEventBody(surface DurableSurfaceName, kind string, bodyData []byte, eventAt string) (json.RawMessage, error) {
	switch surface {
	case DurableSurfaceObligationLog:
		return parseOpaqueJSONObjectBody(bodyData)
	case DurableSurfaceEvidenceLog:
		body, err := parseEvidenceEventBody(bodyData)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(encoded), nil
	case DurableSurfaceExperiments:
		return compileExperimentAuthoringBody(kind, bodyData, eventAt)
	case DurableSurfaceInterventionLog:
		body, err := parseInterventionEventBody(bodyData)
		if err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(encoded), nil
	default:
		return nil, fmt.Errorf("surface %q does not support event-log authoring", surface)
	}
}

func parseOpaqueJSONObjectBody(data []byte) (json.RawMessage, error) {
	var body map[string]json.RawMessage
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func parseObjectiveContractAuthoringBody(data []byte) (*ObjectiveContract, error) {
	var body objectiveContractAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObjectiveContract, err)
	}
	contract := &ObjectiveContract{
		Version:       1,
		ObjectiveHash: body.ObjectiveHash,
		State:         body.State,
		Clauses:       body.Clauses,
	}
	if err := validateObjectiveContractInput(contract); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObjectiveContract, err)
	}
	normalizeObjectiveContract(contract)
	return contract, nil
}

func parseObligationModelAuthoringBody(data []byte) (*ObligationModel, error) {
	var body obligationModelAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	model := &ObligationModel{
		Version:               1,
		ObjectiveContractHash: body.ObjectiveContractHash,
		Required:              body.Required,
		Optional:              body.Optional,
		Guardrails:            body.Guardrails,
	}
	if err := validateObligationModelInput(model); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	normalizeObligationModel(model)
	return model, nil
}

func parseAssurancePlanAuthoringBody(data []byte) (*AssurancePlan, error) {
	var body assurancePlanAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAssurancePlan, err)
	}
	plan := &AssurancePlan{
		Version:        1,
		ObligationRefs: body.ObligationRefs,
		Scenarios:      body.Scenarios,
	}
	if err := validateAssurancePlanInput(plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAssurancePlan, err)
	}
	normalizeAssurancePlan(plan)
	return plan, nil
}

func parseCognitionStateAuthoringBody(data []byte) (*CognitionState, error) {
	var body cognitionStateAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCognitionState, err)
	}
	state := &CognitionState{
		Version: 1,
		Scopes:  body.Scopes,
	}
	if err := validateCognitionStateInput(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCognitionState, err)
	}
	normalizeCognitionState(state)
	return state, nil
}

func parseSuccessModelAuthoringBody(data []byte) (*SuccessModel, error) {
	var body successModelAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceSuccessModel, err)
	}
	model := &SuccessModel{
		Version:               1,
		ObjectiveContractHash: body.ObjectiveContractHash,
		ObligationModelHash:   body.ObligationModelHash,
		Dimensions:            body.Dimensions,
		AntiGoals:             body.AntiGoals,
		CloseoutRequirements:  body.CloseoutRequirements,
	}
	if err := validateSuccessModelInput(model); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceSuccessModel, err)
	}
	normalizeSuccessModel(model)
	return model, nil
}

func parseProofPlanAuthoringBody(data []byte) (*ProofPlan, error) {
	var body proofPlanAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceProofPlan, err)
	}
	plan := &ProofPlan{
		Version: 1,
		Items:   body.Items,
	}
	if err := validateProofPlanInput(plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceProofPlan, err)
	}
	normalizeProofPlan(plan)
	return plan, nil
}

func parseWorkflowPlanAuthoringBody(data []byte) (*WorkflowPlan, error) {
	var body workflowPlanAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceWorkflowPlan, err)
	}
	plan := &WorkflowPlan{
		Version:       1,
		RequiredRoles: body.RequiredRoles,
		Gates:         body.Gates,
	}
	if err := validateWorkflowPlanInput(plan); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceWorkflowPlan, err)
	}
	normalizeWorkflowPlan(plan)
	return plan, nil
}

func parseDomainPackAuthoringBody(data []byte) (*DomainPack, error) {
	var body domainPackAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceDomainPack, err)
	}
	pack := &DomainPack{
		Version:       1,
		Domain:        body.Domain,
		Signals:       body.Signals,
		Slots:         body.Slots,
		PriorEntryIDs: body.PriorEntryIDs,
	}
	if err := validateDomainPackInput(pack); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceDomainPack, err)
	}
	normalizeDomainPack(pack)
	return pack, nil
}

func parseCompilerInputAuthoringBody(data []byte) (*CompilerInput, error) {
	var body compilerInputAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerInput, err)
	}
	input := &CompilerInput{
		Version:              1,
		ObjectiveContractRef: body.ObjectiveContractRef,
		ObligationModelRef:   body.ObligationModelRef,
		MemoryQueryRef:       body.MemoryQueryRef,
		MemoryContextRef:     body.MemoryContextRef,
		PolicySourceRefs:     body.PolicySourceRefs,
		SelectedPriorRefs:    body.SelectedPriorRefs,
		SourceSlots:          body.SourceSlots,
	}
	if err := validateCompilerInput(input); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerInput, err)
	}
	normalizeCompilerInput(input)
	return input, nil
}

func parseCompilerReportAuthoringBody(data []byte) (*CompilerReport, error) {
	var body compilerReportAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerReport, err)
	}
	report := &CompilerReport{
		Version:              1,
		AvailableSourceSlots: body.AvailableSourceSlots,
		SelectedPriorRefs:    body.SelectedPriorRefs,
		RejectedPriors:       body.RejectedPriors,
		OutputSources:        body.OutputSources,
	}
	if err := validateCompilerReport(report); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCompilerReport, err)
	}
	normalizeCompilerReport(report)
	return report, nil
}

func parseImpactStateAuthoringBody(data []byte) (*ImpactState, error) {
	var body impactStateAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceImpactState, err)
	}
	state := &ImpactState{
		Version:          1,
		Scope:            body.Scope,
		BaselineRevision: body.BaselineRevision,
		HeadRevision:     body.HeadRevision,
		ResolverKind:     body.ResolverKind,
		ChangedFiles:     body.ChangedFiles,
		ChangedSymbols:   body.ChangedSymbols,
		ChangedProcesses: body.ChangedProcesses,
	}
	if err := validateImpactStateInput(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceImpactState, err)
	}
	normalizeImpactState(state)
	return state, nil
}

func parseFreshnessStateAuthoringBody(data []byte) (*FreshnessState, error) {
	var body freshnessStateAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceFreshnessState, err)
	}
	state := &FreshnessState{
		Version:   1,
		Cognition: body.Cognition,
		Evidence:  body.Evidence,
	}
	if err := validateFreshnessStateInput(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceFreshnessState, err)
	}
	normalizeFreshnessState(state)
	return state, nil
}

func parseResourceStateAuthoringBody(data []byte) (*ResourceState, error) {
	var body resourceStateAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceResourceState, err)
	}
	state := &ResourceState{
		Version:        1,
		Host:           body.Host,
		PSI:            body.PSI,
		Cgroup:         body.Cgroup,
		GoalxProcesses: body.GoalxProcesses,
		HeadroomBytes:  body.HeadroomBytes,
		State:          body.State,
		Reasons:        body.Reasons,
	}
	if err := validateResourceStateInput(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceResourceState, err)
	}
	normalizeResourceState(state)
	return state, nil
}

func parseCoordinationAuthoringBody(data []byte) (*CoordinationState, error) {
	var body coordinationAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCoordination, err)
	}
	state := &CoordinationState{
		Version:       1,
		PlanSummary:   body.PlanSummary,
		Required:      body.Required,
		Sessions:      body.Sessions,
		Decision:      body.Decision,
		OpenQuestions: body.OpenQuestions,
	}
	if err := validateCoordinationState(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCoordination, err)
	}
	normalizeCoordinationState(state)
	return state, nil
}

func parseStatusAuthoringBody(data []byte) (*RunStatusRecord, error) {
	var body statusAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceStatus, err)
	}
	record := &RunStatusRecord{
		Version:           1,
		Phase:             body.Phase,
		RequiredRemaining: body.RequiredRemaining,
		OpenRequiredIDs:   body.OpenRequiredIDs,
		ActiveSessions:    body.ActiveSessions,
		KeepSession:       body.KeepSession,
		LastVerifiedAt:    body.LastVerifiedAt,
		UpdatedAt:         time.Now().UTC().Format(time.RFC3339),
	}
	if err := validateRunStatusRecord(record); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceStatus, err)
	}
	return record, nil
}

func compileExperimentAuthoringBody(kind string, data []byte, eventAt string) (json.RawMessage, error) {
	switch strings.TrimSpace(kind) {
	case "experiment.created":
		var body experimentCreatedAuthoringBody
		if err := decodeStrictJSON(data, &body); err != nil {
			return nil, err
		}
		record := ExperimentCreatedBody{
			ExperimentID:     body.ExperimentID,
			Session:          body.Session,
			Branch:           body.Branch,
			Worktree:         body.Worktree,
			Intent:           body.Intent,
			BaseRef:          body.BaseRef,
			BaseExperimentID: body.BaseExperimentID,
			CreatedAt:        eventAt,
		}
		return marshalValidatedExperimentBody(kind, record)
	case "experiment.integrated":
		var body experimentIntegratedAuthoringBody
		if err := decodeStrictJSON(data, &body); err != nil {
			return nil, err
		}
		record := ExperimentIntegratedBody{
			IntegrationID:       body.IntegrationID,
			ResultExperimentID:  body.ResultExperimentID,
			SourceExperimentIDs: body.SourceExperimentIDs,
			Method:              body.Method,
			ResultBranch:        body.ResultBranch,
			ResultCommit:        body.ResultCommit,
			RecordedAt:          eventAt,
		}
		return marshalValidatedExperimentBody(kind, record)
	case "experiment.closed":
		var body experimentClosedAuthoringBody
		if err := decodeStrictJSON(data, &body); err != nil {
			return nil, err
		}
		record := ExperimentClosedBody{
			ExperimentID:            body.ExperimentID,
			Disposition:             body.Disposition,
			Reason:                  body.Reason,
			ClosedAt:                eventAt,
			ReplacementExperimentID: body.ReplacementExperimentID,
		}
		return marshalValidatedExperimentBody(kind, record)
	case "evolve.stopped":
		var body evolveStoppedAuthoringBody
		if err := decodeStrictJSON(data, &body); err != nil {
			return nil, err
		}
		record := EvolveStoppedBody{
			ReasonCode:       body.ReasonCode,
			Reason:           body.Reason,
			BestExperimentID: body.BestExperimentID,
			StoppedAt:        eventAt,
		}
		return marshalValidatedExperimentBody(kind, record)
	default:
		return nil, fmt.Errorf("invalid durable log kind %q for %s", kind, DurableSurfaceExperiments)
	}
}

func marshalValidatedExperimentBody(kind string, body any) (json.RawMessage, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	raw := json.RawMessage(encoded)
	if err := validateExperimentLogBody(kind, raw); err != nil {
		return nil, err
	}
	return raw, nil
}
