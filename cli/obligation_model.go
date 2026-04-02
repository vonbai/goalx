package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ObligationModel struct {
	Version               int              `json:"version"`
	ObjectiveContractHash string           `json:"objective_contract_hash"`
	Required              []ObligationItem `json:"required"`
	Optional              []ObligationItem `json:"optional,omitempty"`
	Guardrails            []ObligationItem `json:"guardrails,omitempty"`
	UpdatedAt             string           `json:"updated_at,omitempty"`
}

type ObligationItem struct {
	ID                string   `json:"id"`
	Text              string   `json:"text"`
	Source            string   `json:"source,omitempty"`
	Kind              string   `json:"kind"`
	State             string   `json:"state,omitempty"`
	CoversClauses     []string `json:"covers_clauses"`
	EvidencePaths     []string `json:"evidence_paths,omitempty"`
	Note              string   `json:"note,omitempty"`
	ApprovalRef       string   `json:"approval_ref,omitempty"`
	AssuranceRequired bool     `json:"assurance_required,omitempty"`
}

func ObligationModelPath(runDir string) string {
	return filepath.Join(runDir, "obligation-model.json")
}

func ObligationLogPath(runDir string) string {
	return filepath.Join(runDir, "obligation-log.jsonl")
}

func EnsureObligationLog(runDir string) error {
	path := ObligationLogPath(runDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, nil, 0o644)
}

func LoadObligationModel(path string) (*ObligationModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	model, err := parseObligationModel(data)
	if err != nil {
		return nil, fmt.Errorf("parse obligation model: %w", err)
	}
	return model, nil
}

func SaveObligationModel(path string, model *ObligationModel) error {
	if model == nil {
		return fmt.Errorf("obligation model is nil")
	}
	if err := validateObligationModelInput(model); err != nil {
		return err
	}
	normalizeObligationModel(model)
	if err := validateObligationModelIntegrity(filepath.Dir(path), model); err != nil {
		return err
	}
	model.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, model)
}

func parseObligationModel(data []byte) (*ObligationModel, error) {
	var model ObligationModel
	if err := decodeStrictJSON(data, &model); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	if err := validateObligationModelInput(&model); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	normalizeObligationModel(&model)
	return &model, nil
}

func validateObligationModelInput(model *ObligationModel) error {
	if model == nil {
		return fmt.Errorf("obligation model is nil")
	}
	if model.Version <= 0 {
		return fmt.Errorf("obligation model version must be positive")
	}
	if strings.TrimSpace(model.ObjectiveContractHash) == "" {
		return fmt.Errorf("obligation model objective_contract_hash is required")
	}
	seen := map[string]struct{}{}
	for _, set := range [][]ObligationItem{model.Required, model.Optional, model.Guardrails} {
		for _, item := range set {
			if strings.TrimSpace(item.ID) == "" {
				return fmt.Errorf("obligation item id is required")
			}
			if strings.TrimSpace(item.Text) == "" {
				return fmt.Errorf("obligation item %s text is required", item.ID)
			}
			if strings.TrimSpace(item.Kind) == "" {
				return fmt.Errorf("obligation item %s kind is required", item.ID)
			}
			switch firstNonEmpty(strings.TrimSpace(item.Source), goalItemSourceMaster) {
			case goalItemSourceUser, goalItemSourceMaster:
			default:
				return fmt.Errorf("invalid obligation item source %q", item.Source)
			}
			switch firstNonEmpty(normalizeGoalItemState(item.State), goalItemStateOpen) {
			case goalItemStateOpen, goalItemStateClaimed, goalItemStateWaived:
			default:
				return fmt.Errorf("invalid obligation item state %q", item.State)
			}
			if len(compactStrings(item.CoversClauses)) == 0 {
				return fmt.Errorf("obligation item %s covers_clauses is required", item.ID)
			}
			if normalizeGoalItemState(item.State) == goalItemStateWaived && strings.TrimSpace(item.ApprovalRef) == "" {
				return fmt.Errorf("obligation item %s is waived without explicit approval_ref", item.ID)
			}
			if _, ok := seen[item.ID]; ok {
				return fmt.Errorf("duplicate obligation item id %q", item.ID)
			}
			seen[item.ID] = struct{}{}
		}
	}
	return nil
}

func normalizeObligationModel(model *ObligationModel) {
	if model.Version <= 0 {
		model.Version = 1
	}
	model.ObjectiveContractHash = strings.TrimSpace(model.ObjectiveContractHash)
	model.UpdatedAt = strings.TrimSpace(model.UpdatedAt)
	model.Required = normalizeObligationItems(model.Required)
	model.Optional = normalizeObligationItems(model.Optional)
	model.Guardrails = normalizeObligationItems(model.Guardrails)
}

func normalizeObligationItems(items []ObligationItem) []ObligationItem {
	if items == nil {
		return []ObligationItem{}
	}
	for i := range items {
		items[i].ID = strings.TrimSpace(items[i].ID)
		items[i].Text = strings.TrimSpace(items[i].Text)
		items[i].Source = firstNonEmpty(strings.TrimSpace(items[i].Source), goalItemSourceMaster)
		items[i].Kind = strings.TrimSpace(items[i].Kind)
		items[i].State = firstNonEmpty(normalizeGoalItemState(items[i].State), goalItemStateOpen)
		items[i].CoversClauses = compactStrings(items[i].CoversClauses)
		items[i].EvidencePaths = compactStrings(items[i].EvidencePaths)
		items[i].Note = strings.TrimSpace(items[i].Note)
		items[i].ApprovalRef = strings.TrimSpace(items[i].ApprovalRef)
	}
	return items
}

func EnsureObligationModel(runDir string, goalState *GoalState, objectiveContract *ObjectiveContract, objectiveContractHash, objectiveText string) (*ObligationModel, error) {
	path := ObligationModelPath(runDir)
	model, err := LoadObligationModel(path)
	if err != nil {
		return nil, err
	}
	if model != nil {
		return model, nil
	}
	model = obligationModelFromGoalState(goalState, objectiveContract, objectiveContractHash, objectiveText)
	if err := SaveObligationModel(path, model); err != nil {
		return nil, err
	}
	return model, nil
}

func obligationModelFromGoalState(goalState *GoalState, objectiveContract *ObjectiveContract, objectiveContractHash, objectiveText string) *ObligationModel {
	model := &ObligationModel{
		Version:               1,
		ObjectiveContractHash: strings.TrimSpace(objectiveContractHash),
		Required:              []ObligationItem{},
		Optional:              []ObligationItem{},
		Guardrails:            []ObligationItem{},
	}
	if goalState == nil {
		goalState = &GoalState{}
	}
	normalizeGoalState(goalState)
	goalClauseIDs := []string{}
	for clauseID := range objectiveClausesBySurface(objectiveContract, objectiveRequiredSurfaceGoal) {
		goalClauseIDs = append(goalClauseIDs, clauseID)
	}
	for _, item := range goalState.Required {
		covers := append([]string(nil), item.Covers...)
		if len(compactStrings(covers)) == 0 && len(goalClauseIDs) > 0 {
			covers = append([]string(nil), goalClauseIDs...)
		} else if len(compactStrings(covers)) == 0 && strings.TrimSpace(item.ID) != "" {
			covers = []string{"legacy-goal:" + strings.TrimSpace(item.ID)}
		}
		converted := ObligationItem{
			ID:                item.ID,
			Text:              item.Text,
			Source:            item.Source,
			Kind:              firstNonEmpty(strings.TrimSpace(item.Role), "outcome"),
			State:             item.State,
			CoversClauses:     covers,
			EvidencePaths:     append([]string(nil), item.EvidencePaths...),
			Note:              item.Note,
			ApprovalRef:       item.ApprovalRef,
			AssuranceRequired: strings.TrimSpace(item.Role) == goalItemRoleProof,
		}
		if strings.TrimSpace(item.Role) == goalItemRoleGuardrail {
			model.Guardrails = append(model.Guardrails, converted)
			continue
		}
		model.Required = append(model.Required, converted)
	}
	for _, item := range goalState.Optional {
		covers := append([]string(nil), item.Covers...)
		if len(compactStrings(covers)) == 0 && len(goalClauseIDs) > 0 {
			covers = append([]string(nil), goalClauseIDs...)
		} else if len(compactStrings(covers)) == 0 && strings.TrimSpace(item.ID) != "" {
			covers = []string{"legacy-goal:" + strings.TrimSpace(item.ID)}
		}
		model.Optional = append(model.Optional, ObligationItem{
			ID:            item.ID,
			Text:          item.Text,
			Source:        item.Source,
			Kind:          firstNonEmpty(strings.TrimSpace(item.Role), "outcome"),
			State:         item.State,
			CoversClauses: covers,
			EvidencePaths: append([]string(nil), item.EvidencePaths...),
			Note:          item.Note,
			ApprovalRef:   item.ApprovalRef,
		})
	}
	if len(model.Required) == 0 {
		for _, clause := range objectiveClausesBySurface(objectiveContract, objectiveRequiredSurfaceGoal) {
			model.Required = append(model.Required, ObligationItem{
				ID:            clause.ID,
				Text:          clause.Text,
				Source:        goalItemSourceUser,
				Kind:          "outcome",
				State:         goalItemStateOpen,
				CoversClauses: []string{clause.ID},
			})
		}
	}
	normalizeObligationModel(model)
	return model
}
