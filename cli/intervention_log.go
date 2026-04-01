package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type InterventionBeforeState struct {
	GoalHash         string `json:"goal_hash,omitempty"`
	StatusHash       string `json:"status_hash,omitempty"`
	CoordinationHash string `json:"coordination_hash,omitempty"`
	SuccessModelHash string `json:"success_model_hash,omitempty"`
}

type InterventionEventBody struct {
	Run                 string                  `json:"run,omitempty"`
	Message             string                  `json:"message,omitempty"`
	Urgent              bool                    `json:"urgent,omitempty"`
	AffectedTargets     []string                `json:"affected_targets,omitempty"`
	BudgetAction        string                  `json:"budget_action,omitempty"`
	BudgetBeforeSeconds int64                   `json:"budget_before_seconds,omitempty"`
	BudgetAfterSeconds  int64                   `json:"budget_after_seconds,omitempty"`
	Before              InterventionBeforeState `json:"before,omitempty"`
}

type InterventionLogEvent struct {
	Version int                   `json:"version"`
	Kind    string                `json:"kind"`
	At      string                `json:"at"`
	Actor   string                `json:"actor"`
	Body    InterventionEventBody `json:"body"`
}

func InterventionLogPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "intervention-log.jsonl")
}

func LoadInterventionLog(path string) ([]InterventionLogEvent, error) {
	events, err := LoadDurableLog(path, DurableSurfaceInterventionLog)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	out := make([]InterventionLogEvent, 0, len(events))
	for _, event := range events {
		body, err := parseInterventionEventBody(event.Body)
		if err != nil {
			return nil, fmt.Errorf("parse intervention body: %w", err)
		}
		out = append(out, InterventionLogEvent{
			Version: event.Version,
			Kind:    event.Kind,
			At:      event.At,
			Actor:   event.Actor,
			Body:    *body,
		})
	}
	return out, nil
}

func AppendInterventionEvent(runDir, kind, actor string, body InterventionEventBody) error {
	normalizeInterventionEventBody(&body)
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	line, err := buildDurableEventLine(DurableSurfaceInterventionLog, DurableMutation{
		Surface: DurableSurfaceInterventionLog,
		Kind:    kind,
		Actor:   actor,
		Body:    data,
	})
	if err != nil {
		return err
	}
	path := InterventionLogPath(runDir)
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

func RecordUserTellIntervention(runDir, runName, target, message string, urgent bool) error {
	target = strings.TrimSpace(target)
	if target == "" {
		target = "master"
	}
	kind := "user_tell"
	if target != "master" {
		kind = "user_redirect"
	}
	before, err := captureInterventionBeforeState(runDir)
	if err != nil {
		return err
	}
	return AppendInterventionEvent(runDir, kind, "user", InterventionEventBody{
		Run:             strings.TrimSpace(runName),
		Message:         strings.TrimSpace(message),
		Urgent:          urgent,
		AffectedTargets: []string{target},
		Before:          before,
	})
}

func captureInterventionBeforeState(runDir string) (InterventionBeforeState, error) {
	return InterventionBeforeState{
		GoalHash:         hashOptionalFileSHA256(GoalPath(runDir)),
		StatusHash:       hashOptionalFileSHA256(RunStatusPath(runDir)),
		CoordinationHash: hashOptionalFileSHA256(CoordinationPath(runDir)),
		SuccessModelHash: hashOptionalFileSHA256(SuccessModelPath(runDir)),
	}, nil
}

func hashOptionalFileSHA256(path string) string {
	if !fileExists(path) {
		return ""
	}
	hash, err := hashFileSHA256(path)
	if err != nil {
		return ""
	}
	return hash
}

func parseInterventionEventBody(data []byte) (*InterventionEventBody, error) {
	var body InterventionEventBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return nil, err
	}
	if err := validateInterventionEventBody(&body); err != nil {
		return nil, err
	}
	normalizeInterventionEventBody(&body)
	return &body, nil
}

func validateInterventionEventBody(body *InterventionEventBody) error {
	if body == nil {
		return fmt.Errorf("intervention body is nil")
	}
	if strings.TrimSpace(body.Run) == "" {
		return fmt.Errorf("intervention body run is required")
	}
	if strings.TrimSpace(body.Message) == "" {
		return fmt.Errorf("intervention body message is required")
	}
	if len(compactStrings(body.AffectedTargets)) == 0 {
		return fmt.Errorf("intervention body affected_targets is required")
	}
	return nil
}

func normalizeInterventionEventBody(body *InterventionEventBody) {
	if body == nil {
		return
	}
	body.Run = strings.TrimSpace(body.Run)
	body.Message = strings.TrimSpace(body.Message)
	body.AffectedTargets = compactStrings(body.AffectedTargets)
	body.BudgetAction = strings.TrimSpace(body.BudgetAction)
	body.Before.GoalHash = strings.TrimSpace(body.Before.GoalHash)
	body.Before.StatusHash = strings.TrimSpace(body.Before.StatusHash)
	body.Before.CoordinationHash = strings.TrimSpace(body.Before.CoordinationHash)
	body.Before.SuccessModelHash = strings.TrimSpace(body.Before.SuccessModelHash)
}
