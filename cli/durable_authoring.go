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

type goalAuthoringBody struct {
	Required []GoalItem `json:"required,omitempty"`
	Optional []GoalItem `json:"optional,omitempty"`
}

type acceptanceAuthoringBody struct {
	GoalVersion int               `json:"goal_version,omitempty"`
	Checks      []AcceptanceCheck `json:"checks,omitempty"`
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
		case DurableSurfaceGoal:
			state, err := parseGoalAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveGoalState(path, state)
		case DurableSurfaceAcceptance:
			state, err := parseAcceptanceAuthoringBody(body)
			if err != nil {
				return err
			}
			return SaveAcceptanceState(path, state)
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
	case DurableSurfaceGoalLog:
		return parseOpaqueJSONObjectBody(bodyData)
	case DurableSurfaceExperiments:
		return compileExperimentAuthoringBody(kind, bodyData, eventAt)
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

func parseGoalAuthoringBody(data []byte) (*GoalState, error) {
	var body goalAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceGoal, err)
	}
	state := &GoalState{
		Version:  1,
		Required: body.Required,
		Optional: body.Optional,
	}
	if err := validateGoalStateInput(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceGoal, err)
	}
	normalizeGoalState(state)
	return state, nil
}

func parseAcceptanceAuthoringBody(data []byte) (*AcceptanceState, error) {
	var body acceptanceAuthoringBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAcceptance, err)
	}
	state := &AcceptanceState{
		Version:     2,
		GoalVersion: body.GoalVersion,
		Checks:      body.Checks,
		LastResult:  AcceptanceResult{},
	}
	if err := validateAcceptanceState(state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceAcceptance, err)
	}
	normalizeAcceptanceState(state)
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
