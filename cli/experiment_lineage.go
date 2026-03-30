package cli

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	experimentIDPrefix  = "exp_"
	integrationIDPrefix = "int_"
)

type ExperimentCreatedBody struct {
	ExperimentID     string `json:"experiment_id"`
	Session          string `json:"session,omitempty"`
	Branch           string `json:"branch,omitempty"`
	Worktree         string `json:"worktree,omitempty"`
	Intent           string `json:"intent,omitempty"`
	BaseRef          string `json:"base_ref,omitempty"`
	BaseExperimentID string `json:"base_experiment_id,omitempty"`
	CreatedAt        string `json:"created_at"`
}

type ExperimentIntegratedBody struct {
	IntegrationID      string   `json:"integration_id"`
	ResultExperimentID string   `json:"result_experiment_id"`
	SourceExperimentIDs []string `json:"source_experiment_ids"`
	Method             string   `json:"method"`
	ResultBranch       string   `json:"result_branch,omitempty"`
	ResultCommit       string   `json:"result_commit,omitempty"`
	RecordedAt         string   `json:"recorded_at"`
}

type ExperimentClosedBody struct {
	ExperimentID          string `json:"experiment_id"`
	Disposition           string `json:"disposition"`
	Reason                string `json:"reason"`
	ClosedAt              string `json:"closed_at"`
	ReplacementExperimentID string `json:"replacement_experiment_id,omitempty"`
}

type EvolveStoppedBody struct {
	ReasonCode      string `json:"reason_code"`
	Reason          string `json:"reason"`
	BestExperimentID string `json:"best_experiment_id,omitempty"`
	StoppedAt       string `json:"stopped_at"`
}

var allowedIntegrationMethods = map[string]struct{}{
	"keep":          {},
	"manual_merge":  {},
	"partial_adopt": {},
	"cherry_pick":   {},
	"consolidate":   {},
}

var allowedExperimentDispositions = map[string]struct{}{
	"rejected":   {},
	"abandoned":  {},
	"superseded": {},
}

var allowedEvolveStopReasonCodes = map[string]struct{}{
	"budget_exhausted":   {},
	"user_redirected":    {},
	"external_blocker":   {},
	"diminishing_returns": {},
	"risk_boundary":      {},
}

func newExperimentID() string {
	return newOpaqueID(experimentIDPrefix)
}

func newIntegrationID() string {
	return newOpaqueID(integrationIDPrefix)
}

func newOpaqueID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%s%d", prefix, time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("%s%x", prefix, buf)
}

func appendExperimentCreated(runDir string, body ExperimentCreatedBody) error {
	body.ExperimentID = strings.TrimSpace(body.ExperimentID)
	body.CreatedAt = strings.TrimSpace(body.CreatedAt)
	if body.CreatedAt == "" {
		body.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	payload, err := json.Marshal(experimentCreatedAuthoringBody{
		ExperimentID:     body.ExperimentID,
		Session:          body.Session,
		Branch:           body.Branch,
		Worktree:         body.Worktree,
		Intent:           body.Intent,
		BaseRef:          body.BaseRef,
		BaseExperimentID: body.BaseExperimentID,
	})
	if err != nil {
		return fmt.Errorf("marshal experiment.created: %w", err)
	}
	return ApplyDurableMutation(runDir, DurableMutation{
		Surface: DurableSurfaceExperiments,
		Kind:    "experiment.created",
		Actor:   "goalx",
		At:      body.CreatedAt,
		Body:    payload,
	})
}

func appendExperimentIntegrated(runDir string, body ExperimentIntegratedBody) error {
	body.IntegrationID = strings.TrimSpace(body.IntegrationID)
	body.ResultExperimentID = strings.TrimSpace(body.ResultExperimentID)
	body.Method = strings.TrimSpace(body.Method)
	body.RecordedAt = strings.TrimSpace(body.RecordedAt)
	if body.RecordedAt == "" {
		body.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	payload, err := json.Marshal(experimentIntegratedAuthoringBody{
		IntegrationID:       body.IntegrationID,
		ResultExperimentID:  body.ResultExperimentID,
		SourceExperimentIDs: body.SourceExperimentIDs,
		Method:              body.Method,
		ResultBranch:        body.ResultBranch,
		ResultCommit:        body.ResultCommit,
	})
	if err != nil {
		return fmt.Errorf("marshal experiment.integrated: %w", err)
	}
	return ApplyDurableMutation(runDir, DurableMutation{
		Surface: DurableSurfaceExperiments,
		Kind:    "experiment.integrated",
		Actor:   "goalx",
		At:      body.RecordedAt,
		Body:    payload,
	})
}

func initializeRootExperimentLineage(runDir, runWorktree, runName, intent string) error {
	return initializeRootExperimentLineageWithBase(runDir, runWorktree, runName, intent, "", "")
}

func initializeRootExperimentLineageWithBase(runDir, runWorktree, runName, intent, baseRef, baseExperimentID string) error {
	rootExperimentID := newExperimentID()
	createdAt := time.Now().UTC().Format(time.RFC3339)
	rootBranch := fmt.Sprintf("goalx/%s/root", runName)
	if strings.TrimSpace(baseRef) == "" {
		baseRef = rootBranch
	}
	if err := appendExperimentCreated(runDir, ExperimentCreatedBody{
		ExperimentID:     rootExperimentID,
		Branch:           rootBranch,
		Worktree:         runWorktree,
		Intent:           strings.TrimSpace(intent),
		BaseRef:          strings.TrimSpace(baseRef),
		BaseExperimentID: strings.TrimSpace(baseExperimentID),
		CreatedAt:        createdAt,
	}); err != nil {
		return err
	}
	currentCommit, err := gitHeadRevision(runWorktree)
	if err != nil {
		return fmt.Errorf("resolve root worktree head: %w", err)
	}
	return SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:             1,
		CurrentExperimentID: rootExperimentID,
		CurrentBranch:       rootBranch,
		CurrentCommit:       currentCommit,
		UpdatedAt:           createdAt,
	})
}

func validateExperimentLogBody(kind string, body json.RawMessage) error {
	switch kind {
	case "experiment.created":
		var record ExperimentCreatedBody
		if err := decodeStrictJSON(body, &record); err != nil {
			return err
		}
		if strings.TrimSpace(record.ExperimentID) == "" {
			return fmt.Errorf("experiment.created requires experiment_id")
		}
		if strings.TrimSpace(record.CreatedAt) == "" {
			return fmt.Errorf("experiment.created requires created_at")
		}
		return nil
	case "experiment.integrated":
		var record ExperimentIntegratedBody
		if err := decodeStrictJSON(body, &record); err != nil {
			return err
		}
		if strings.TrimSpace(record.IntegrationID) == "" {
			return fmt.Errorf("experiment.integrated requires integration_id")
		}
		if strings.TrimSpace(record.ResultExperimentID) == "" {
			return fmt.Errorf("experiment.integrated requires result_experiment_id")
		}
		if len(record.SourceExperimentIDs) == 0 {
			return fmt.Errorf("experiment.integrated requires source_experiment_ids")
		}
		if _, ok := allowedIntegrationMethods[strings.TrimSpace(record.Method)]; !ok {
			return fmt.Errorf("experiment.integrated requires supported method")
		}
		if strings.TrimSpace(record.RecordedAt) == "" {
			return fmt.Errorf("experiment.integrated requires recorded_at")
		}
		return nil
	case "experiment.closed":
		var record ExperimentClosedBody
		if err := decodeStrictJSON(body, &record); err != nil {
			return err
		}
		if strings.TrimSpace(record.ExperimentID) == "" {
			return fmt.Errorf("experiment.closed requires experiment_id")
		}
		if _, ok := allowedExperimentDispositions[strings.TrimSpace(record.Disposition)]; !ok {
			return fmt.Errorf("experiment.closed requires supported disposition")
		}
		if strings.TrimSpace(record.Reason) == "" {
			return fmt.Errorf("experiment.closed requires reason")
		}
		if strings.TrimSpace(record.ClosedAt) == "" {
			return fmt.Errorf("experiment.closed requires closed_at")
		}
		return nil
	case "evolve.stopped":
		var record EvolveStoppedBody
		if err := decodeStrictJSON(body, &record); err != nil {
			return err
		}
		if _, ok := allowedEvolveStopReasonCodes[strings.TrimSpace(record.ReasonCode)]; !ok {
			return fmt.Errorf("evolve.stopped requires reason_code")
		}
		if strings.TrimSpace(record.Reason) == "" {
			return fmt.Errorf("evolve.stopped requires reason")
		}
		if strings.TrimSpace(record.StoppedAt) == "" {
			return fmt.Errorf("evolve.stopped requires stopped_at")
		}
		return nil
	default:
		return fmt.Errorf("unsupported experiment log kind %q", kind)
	}
}
