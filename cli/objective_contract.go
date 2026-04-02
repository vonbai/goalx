package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	objectiveContractStateDraft  = "draft"
	objectiveContractStateLocked = "locked"

	objectiveClauseKindDelivery      = "delivery"
	objectiveClauseKindQualityBar    = "quality_bar"
	objectiveClauseKindVerification  = "verification"
	objectiveClauseKindGuardrail     = "guardrail"
	objectiveClauseKindOperatingRule = "operating_rule"

	objectiveRequiredSurfaceGoal       ObjectiveRequiredSurface = "obligation"
	objectiveRequiredSurfaceAcceptance ObjectiveRequiredSurface = "assurance"
)

type ObjectiveRequiredSurface string

type ObjectiveContract struct {
	Version       int               `json:"version"`
	ObjectiveHash string            `json:"objective_hash"`
	State         string            `json:"state"`
	CreatedAt     string            `json:"created_at,omitempty"`
	LockedAt      string            `json:"locked_at,omitempty"`
	Clauses       []ObjectiveClause `json:"clauses"`
}

type ObjectiveClause struct {
	ID               string                     `json:"id"`
	Text             string                     `json:"text"`
	Kind             string                     `json:"kind"`
	SourceExcerpt    string                     `json:"source_excerpt"`
	RequiredSurfaces []ObjectiveRequiredSurface `json:"required_surfaces"`
}

func ObjectiveContractPath(runDir string) string {
	return filepath.Join(runDir, "objective-contract.json")
}

func NewObjectiveContract(objective string) *ObjectiveContract {
	return &ObjectiveContract{
		Version:       1,
		ObjectiveHash: hashObjectiveText(objective),
		State:         objectiveContractStateDraft,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		Clauses:       []ObjectiveClause{},
	}
}

func EnsureObjectiveContract(runDir, objective string) (*ObjectiveContract, error) {
	path := ObjectiveContractPath(runDir)
	contract, err := LoadObjectiveContract(path)
	if err != nil {
		return nil, err
	}
	if contract != nil {
		return contract, nil
	}
	contract = NewObjectiveContract(objective)
	if err := SaveObjectiveContract(path, contract); err != nil {
		return nil, err
	}
	return contract, nil
}

func LoadObjectiveContract(path string) (*ObjectiveContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	contract, err := parseObjectiveContract(data)
	if err != nil {
		return nil, fmt.Errorf("parse objective contract: %w", err)
	}
	return contract, nil
}

func RequireObjectiveContract(runDir string) (*ObjectiveContract, error) {
	contract, err := LoadObjectiveContract(ObjectiveContractPath(runDir))
	if err != nil {
		return nil, err
	}
	if contract == nil {
		return nil, fmt.Errorf("objective contract missing at %s", ObjectiveContractPath(runDir))
	}
	return contract, nil
}

func SaveObjectiveContract(path string, contract *ObjectiveContract) error {
	if contract == nil {
		return fmt.Errorf("objective contract is nil")
	}
	if existing, err := LoadObjectiveContract(path); err != nil {
		return err
	} else if existing != nil && existing.State == objectiveContractStateLocked {
		return fmt.Errorf("objective contract is locked at %s", path)
	}
	if err := validateObjectiveContractInput(contract); err != nil {
		return err
	}
	normalizeObjectiveContract(contract)
	now := time.Now().UTC().Format(time.RFC3339)
	if contract.CreatedAt == "" {
		contract.CreatedAt = now
	}
	if contract.State == objectiveContractStateLocked && contract.LockedAt == "" {
		contract.LockedAt = now
	}
	data, err := json.MarshalIndent(contract, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, data, 0o644)
}

func parseObjectiveContract(data []byte) (*ObjectiveContract, error) {
	var contract ObjectiveContract
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceObjectiveContract, fmt.Errorf("objective contract is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObjectiveContract, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObjectiveContract, err)
	}
	if err := validateObjectiveContractInput(&contract); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObjectiveContract, err)
	}
	normalizeObjectiveContract(&contract)
	return &contract, nil
}

func validateObjectiveContractInput(contract *ObjectiveContract) error {
	if contract == nil {
		return fmt.Errorf("objective contract is nil")
	}
	if contract.Version <= 0 {
		return fmt.Errorf("objective contract version must be positive")
	}
	switch strings.TrimSpace(contract.State) {
	case objectiveContractStateDraft, objectiveContractStateLocked:
	default:
		return fmt.Errorf("objective contract state %q is invalid", contract.State)
	}
	if strings.TrimSpace(contract.ObjectiveHash) == "" {
		return fmt.Errorf("objective contract objective_hash is required")
	}
	if contract.Clauses == nil {
		contract.Clauses = []ObjectiveClause{}
	}
	seen := make(map[string]struct{}, len(contract.Clauses))
	for _, clause := range contract.Clauses {
		if err := validateObjectiveClauseInput(clause); err != nil {
			return err
		}
		if _, ok := seen[clause.ID]; ok {
			return fmt.Errorf("duplicate objective clause id %q", clause.ID)
		}
		seen[clause.ID] = struct{}{}
	}
	return nil
}

func validateObjectiveClauseInput(clause ObjectiveClause) error {
	if strings.TrimSpace(clause.ID) == "" {
		return fmt.Errorf("objective clause id is required")
	}
	if strings.TrimSpace(clause.Text) == "" {
		return fmt.Errorf("objective clause %s is missing text", clause.ID)
	}
	switch strings.TrimSpace(clause.Kind) {
	case objectiveClauseKindDelivery, objectiveClauseKindQualityBar, objectiveClauseKindVerification, objectiveClauseKindGuardrail, objectiveClauseKindOperatingRule:
	default:
		return fmt.Errorf("objective clause %s has invalid kind %q", clause.ID, clause.Kind)
	}
	if strings.TrimSpace(clause.SourceExcerpt) == "" {
		return fmt.Errorf("objective clause %s is missing source_excerpt", clause.ID)
	}
	if len(clause.RequiredSurfaces) == 0 {
		return fmt.Errorf("objective clause %s is missing required_surfaces", clause.ID)
	}
	seen := make(map[ObjectiveRequiredSurface]struct{}, len(clause.RequiredSurfaces))
	for _, surface := range clause.RequiredSurfaces {
		switch surface {
		case objectiveRequiredSurfaceGoal, objectiveRequiredSurfaceAcceptance, "goal", "acceptance":
		default:
			return fmt.Errorf("objective clause %s has invalid required surface %q", clause.ID, surface)
		}
		if _, ok := seen[surface]; ok {
			return fmt.Errorf("objective clause %s has duplicate required surface %q", clause.ID, surface)
		}
		seen[surface] = struct{}{}
	}
	return nil
}

func normalizeObjectiveContract(contract *ObjectiveContract) {
	if contract.Version <= 0 {
		contract.Version = 1
	}
	contract.ObjectiveHash = strings.TrimSpace(contract.ObjectiveHash)
	contract.State = strings.TrimSpace(contract.State)
	if contract.Clauses == nil {
		contract.Clauses = []ObjectiveClause{}
	}
	for i := range contract.Clauses {
		normalizeObjectiveClause(&contract.Clauses[i])
	}
}

func normalizeObjectiveClause(clause *ObjectiveClause) {
	if clause == nil {
		return
	}
	clause.ID = strings.TrimSpace(clause.ID)
	clause.Text = strings.TrimSpace(clause.Text)
	clause.Kind = strings.TrimSpace(clause.Kind)
	clause.SourceExcerpt = strings.TrimSpace(clause.SourceExcerpt)
	if clause.RequiredSurfaces == nil {
		clause.RequiredSurfaces = []ObjectiveRequiredSurface{}
	}
	out := make([]ObjectiveRequiredSurface, 0, len(clause.RequiredSurfaces))
	for _, surface := range clause.RequiredSurfaces {
		trimmed := ObjectiveRequiredSurface(strings.TrimSpace(string(surface)))
		switch trimmed {
		case "goal":
			trimmed = objectiveRequiredSurfaceGoal
		case "acceptance":
			trimmed = objectiveRequiredSurfaceAcceptance
		}
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	clause.RequiredSurfaces = out
}

func hashObjectiveText(objective string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(objective)))
	return "sha256:" + hex.EncodeToString(sum[:])
}
