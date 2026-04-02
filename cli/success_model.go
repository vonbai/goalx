package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SuccessModel struct {
	Version               int                `json:"version"`
	CompiledAt            string             `json:"compiled_at,omitempty"`
	CompilerVersion       string             `json:"compiler_version,omitempty"`
	ObjectiveContractHash string             `json:"objective_contract_hash"`
	ObligationModelHash   string             `json:"obligation_model_hash"`
	Dimensions            []SuccessDimension `json:"dimensions"`
	AntiGoals             []SuccessAntiGoal  `json:"anti_goals,omitempty"`
	CloseoutRequirements  []string           `json:"closeout_requirements,omitempty"`
}

type SuccessDimension struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Text         string   `json:"text"`
	Required     bool     `json:"required,omitempty"`
	FailureModes []string `json:"failure_modes,omitempty"`
}

type SuccessAntiGoal struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func SuccessModelPath(runDir string) string {
	return filepath.Join(runDir, "success-model.json")
}

func LoadSuccessModel(path string) (*SuccessModel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	model, err := parseSuccessModel(data)
	if err != nil {
		return nil, fmt.Errorf("parse success model: %w", err)
	}
	return model, nil
}

func SaveSuccessModel(path string, model *SuccessModel) error {
	if model == nil {
		return fmt.Errorf("success model is nil")
	}
	if err := validateSuccessModelInput(model); err != nil {
		return err
	}
	normalizeSuccessModel(model)
	if model.CompiledAt == "" {
		model.CompiledAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseSuccessModel(data []byte) (*SuccessModel, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceSuccessModel, fmt.Errorf("success model is empty"))
	}
	type successModelCompat struct {
		Version               int                `json:"version"`
		CompiledAt            string             `json:"compiled_at,omitempty"`
		CompilerVersion       string             `json:"compiler_version,omitempty"`
		ObjectiveContractHash string             `json:"objective_contract_hash"`
		ObligationModelHash   string             `json:"obligation_model_hash"`
		Dimensions            []SuccessDimension `json:"dimensions"`
		AntiGoals             []SuccessAntiGoal  `json:"anti_goals,omitempty"`
		CloseoutRequirements  []string           `json:"closeout_requirements,omitempty"`
	}
	var payload successModelCompat
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceSuccessModel, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceSuccessModel, err)
	}
	model := SuccessModel{
		Version:               payload.Version,
		CompiledAt:            payload.CompiledAt,
		CompilerVersion:       payload.CompilerVersion,
		ObjectiveContractHash: payload.ObjectiveContractHash,
		ObligationModelHash:   strings.TrimSpace(payload.ObligationModelHash),
		Dimensions:            payload.Dimensions,
		AntiGoals:             payload.AntiGoals,
		CloseoutRequirements:  payload.CloseoutRequirements,
	}
	if err := validateSuccessModelInput(&model); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceSuccessModel, err)
	}
	normalizeSuccessModel(&model)
	return &model, nil
}

func validateSuccessModelInput(model *SuccessModel) error {
	if model == nil {
		return fmt.Errorf("success model is nil")
	}
	if model.Version <= 0 {
		return fmt.Errorf("success model version must be positive")
	}
	if strings.TrimSpace(model.ObjectiveContractHash) == "" {
		return fmt.Errorf("success model objective_contract_hash is required")
	}
	if strings.TrimSpace(model.ObligationModelHash) == "" {
		return fmt.Errorf("success model obligation_model_hash is required")
	}
	if len(model.Dimensions) == 0 {
		return fmt.Errorf("success model dimensions are required")
	}
	seenDimensions := make(map[string]struct{}, len(model.Dimensions))
	for _, dimension := range model.Dimensions {
		if strings.TrimSpace(dimension.ID) == "" {
			return fmt.Errorf("success model dimension id is required")
		}
		if strings.TrimSpace(dimension.Kind) == "" {
			return fmt.Errorf("success model dimension %s kind is required", dimension.ID)
		}
		if strings.TrimSpace(dimension.Text) == "" {
			return fmt.Errorf("success model dimension %s text is required", dimension.ID)
		}
		if _, ok := seenDimensions[dimension.ID]; ok {
			return fmt.Errorf("duplicate success model dimension id %q", dimension.ID)
		}
		seenDimensions[dimension.ID] = struct{}{}
	}
	seenAntiGoals := make(map[string]struct{}, len(model.AntiGoals))
	for _, antiGoal := range model.AntiGoals {
		if strings.TrimSpace(antiGoal.ID) == "" {
			return fmt.Errorf("success model anti_goal id is required")
		}
		if strings.TrimSpace(antiGoal.Text) == "" {
			return fmt.Errorf("success model anti_goal %s text is required", antiGoal.ID)
		}
		if _, ok := seenAntiGoals[antiGoal.ID]; ok {
			return fmt.Errorf("duplicate success model anti_goal id %q", antiGoal.ID)
		}
		seenAntiGoals[antiGoal.ID] = struct{}{}
	}
	return nil
}

func normalizeSuccessModel(model *SuccessModel) {
	if model.Version <= 0 {
		model.Version = 1
	}
	model.CompiledAt = strings.TrimSpace(model.CompiledAt)
	model.CompilerVersion = strings.TrimSpace(model.CompilerVersion)
	model.ObjectiveContractHash = strings.TrimSpace(model.ObjectiveContractHash)
	model.ObligationModelHash = strings.TrimSpace(model.ObligationModelHash)
	if model.Dimensions == nil {
		model.Dimensions = []SuccessDimension{}
	}
	for i := range model.Dimensions {
		model.Dimensions[i].ID = strings.TrimSpace(model.Dimensions[i].ID)
		model.Dimensions[i].Kind = strings.TrimSpace(model.Dimensions[i].Kind)
		model.Dimensions[i].Text = strings.TrimSpace(model.Dimensions[i].Text)
		model.Dimensions[i].FailureModes = compactStrings(model.Dimensions[i].FailureModes)
	}
	if model.AntiGoals == nil {
		model.AntiGoals = []SuccessAntiGoal{}
	}
	for i := range model.AntiGoals {
		model.AntiGoals[i].ID = strings.TrimSpace(model.AntiGoals[i].ID)
		model.AntiGoals[i].Text = strings.TrimSpace(model.AntiGoals[i].Text)
	}
	model.CloseoutRequirements = compactStrings(model.CloseoutRequirements)
}
