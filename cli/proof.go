package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type CompletionProofItem struct {
	RequirementID     string   `json:"requirement_id"`
	Requirement       string   `json:"requirement,omitempty"`
	Kind              string   `json:"kind,omitempty"`
	Status            string   `json:"status,omitempty"`
	SatisfactionBasis string   `json:"satisfaction_basis,omitempty"`
	EvidencePaths     []string `json:"evidence_paths,omitempty"`
	EvidenceClass     string   `json:"evidence_class,omitempty"`
	CounterEvidence   []string `json:"counter_evidence,omitempty"`
	SemanticMatch     string   `json:"semantic_match,omitempty"`
	UserApproved      bool     `json:"user_approved,omitempty"`
}

func BuildCompletionProofItems(contract *GoalContractState) []CompletionProofItem {
	if contract == nil {
		return nil
	}
	items := make([]CompletionProofItem, 0, len(contract.Items))
	for _, item := range contract.Items {
		if !isRequiredGoalContractKind(strings.TrimSpace(item.Kind)) {
			continue
		}
		items = append(items, CompletionProofItem{
			RequirementID:     item.ID,
			Requirement:       item.Requirement,
			Kind:              item.Kind,
			Status:            item.Status,
			SatisfactionBasis: item.SatisfactionBasis,
			EvidencePaths:     append([]string(nil), item.Evidence...),
			EvidenceClass:     item.EvidenceClass,
			CounterEvidence:   append([]string(nil), item.CounterEvidence...),
			SemanticMatch:     item.SemanticMatch,
			UserApproved:      item.UserApproved,
		})
	}
	return items
}

func ValidateGoalContractStructuredProof(item GoalContractItem) error {
	if len(item.Evidence) == 0 {
		return fmt.Errorf("goal contract item %s is done but missing structured proof evidence", item.ID)
	}
	if strings.TrimSpace(item.EvidenceClass) == "" {
		return fmt.Errorf("goal contract item %s is done but missing structured proof evidence_class", item.ID)
	}
	if len(item.CounterEvidence) == 0 {
		return fmt.Errorf("goal contract item %s is done but missing structured proof counter_evidence", item.ID)
	}
	if strings.TrimSpace(item.SemanticMatch) == "" {
		return fmt.Errorf("goal contract item %s is done but missing structured proof semantic_match", item.ID)
	}
	return nil
}

func ValidateCompletionStateForVerification(projectRoot string, completion *CompletionState, contract *GoalContractState, acceptance *AcceptanceState) error {
	if completion == nil {
		return fmt.Errorf("completion proof manifest is missing")
	}
	if contract == nil {
		return nil
	}
	if contract.Version > 0 && completion.GoalContractVersion > 0 && completion.GoalContractVersion != contract.Version {
		return fmt.Errorf("completion proof targets contract version %d but current goal contract is version %d", completion.GoalContractVersion, contract.Version)
	}
	if contract.Version > 0 && completion.GoalContractVersion == 0 {
		return fmt.Errorf("completion proof is missing goal_contract_version")
	}
	if acceptance != nil && strings.TrimSpace(acceptance.Status) != "" && strings.TrimSpace(completion.AcceptanceStatus) != strings.TrimSpace(acceptance.Status) {
		return fmt.Errorf("completion proof acceptance_status=%q but acceptance state is %q", completion.AcceptanceStatus, acceptance.Status)
	}

	proofs := make(map[string]CompletionProofItem, len(completion.ProofItems))
	for _, item := range completion.ProofItems {
		proofs[item.RequirementID] = item
	}

	for _, item := range contract.Items {
		if !isRequiredGoalContractKind(strings.TrimSpace(item.Kind)) {
			continue
		}
		proof, ok := proofs[item.ID]
		if !ok {
			return fmt.Errorf("completion proof missing requirement_id %s", item.ID)
		}
		status := strings.TrimSpace(item.Status)
		switch status {
		case goalContractStatusDone:
			if err := ValidateGoalContractStructuredProof(item); err != nil {
				return err
			}
			if err := validateStructuredProofEvidence(projectRoot, item); err != nil {
				return err
			}
			if strings.TrimSpace(proof.SatisfactionBasis) != strings.TrimSpace(item.SatisfactionBasis) {
				return fmt.Errorf("completion proof requirement %s has satisfaction_basis=%q but goal contract says %q", item.ID, proof.SatisfactionBasis, item.SatisfactionBasis)
			}
			if strings.TrimSpace(proof.EvidenceClass) == "" || len(proof.EvidencePaths) == 0 || len(proof.CounterEvidence) == 0 || strings.TrimSpace(proof.SemanticMatch) == "" {
				return fmt.Errorf("completion proof requirement %s is missing structured proof fields", item.ID)
			}
			if strings.TrimSpace(proof.SemanticMatch) != "exact" {
				return fmt.Errorf("completion proof requirement %s must use semantic_match=exact for done items", item.ID)
			}
		case goalContractStatusWaived:
			if !item.UserApproved || !proof.UserApproved {
				return fmt.Errorf("completion proof requirement %s is waived without explicit user approval", item.ID)
			}
		}
	}
	return nil
}

func validateStructuredProofEvidence(projectRoot string, item GoalContractItem) error {
	if strings.TrimSpace(item.SemanticMatch) != "exact" {
		return fmt.Errorf("goal contract item %s is done but semantic_match=%q; done items require semantic_match=exact", item.ID, item.SemanticMatch)
	}
	for _, evidencePath := range item.Evidence {
		evidencePath = strings.TrimSpace(evidencePath)
		if evidencePath == "" {
			return fmt.Errorf("goal contract item %s has empty evidence path", item.ID)
		}
		resolved := evidencePath
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(projectRoot, resolved)
		}
		if _, err := os.Stat(resolved); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("goal contract item %s evidence path %s does not exist", item.ID, evidencePath)
			}
			return fmt.Errorf("goal contract item %s evidence path %s: %w", item.ID, evidencePath, err)
		}
	}
	return nil
}
