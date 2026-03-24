package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	completionVerdictSatisfied   = "satisfied"
	completionVerdictWaived      = "waived"
	completionVerdictUnsatisfied = "unsatisfied"

	completionBasisPreexisting = "preexisting"
	completionBasisRunChange   = "run_change"
	completionBasisMixed       = "mixed"
	completionBasisWaived      = "waived"
)

type CompletionProofItem struct {
	GoalItemID    string   `json:"goal_item_id"`
	Verdict       string   `json:"verdict,omitempty"`
	Basis         string   `json:"basis,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	Note          string   `json:"note,omitempty"`
	UserApproved  bool     `json:"user_approved,omitempty"`
}

func BuildCompletionProofItems(goal *GoalState, codeChanged bool) []CompletionProofItem {
	if goal == nil {
		return nil
	}
	items := make([]CompletionProofItem, 0, len(goal.Required))
	for _, item := range goal.Required {
		proof := CompletionProofItem{
			GoalItemID:    item.ID,
			EvidencePaths: append([]string(nil), item.EvidencePaths...),
			Note:          item.Note,
			UserApproved:  item.UserApproved,
		}
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			proof.Verdict = completionVerdictSatisfied
			if codeChanged {
				proof.Basis = completionBasisRunChange
			} else {
				proof.Basis = completionBasisPreexisting
			}
		case goalItemStateWaived:
			proof.Verdict = completionVerdictWaived
			proof.Basis = completionBasisWaived
		default:
			proof.Verdict = completionVerdictUnsatisfied
		}
		items = append(items, proof)
	}
	return items
}

func ValidateCompletionStateForVerification(projectRoot string, completion *CompletionState, goal *GoalState, acceptance *AcceptanceState) error {
	if completion == nil {
		return fmt.Errorf("completion proof manifest is missing")
	}
	if goal == nil {
		return fmt.Errorf("goal state is missing")
	}

	summary := SummarizeGoalState(goal)
	if completion.GoalVersion != summary.Version {
		return fmt.Errorf("completion proof goal_version=%d but goal.json version is %d", completion.GoalVersion, summary.Version)
	}
	if acceptance != nil && acceptanceStatus(acceptance) != "" && completion.AcceptanceStatus != acceptanceStatus(acceptance) {
		return fmt.Errorf("completion proof acceptance_status=%q but acceptance state is %q", completion.AcceptanceStatus, acceptanceStatus(acceptance))
	}
	if completion.RequiredTotal != summary.RequiredTotal {
		return fmt.Errorf("completion proof required_total=%d but goal.json requires %d items", completion.RequiredTotal, summary.RequiredTotal)
	}
	if completion.OptionalOpen != summary.OptionalOpen {
		return fmt.Errorf("completion proof optional_open=%d but goal.json has %d open optional items", completion.OptionalOpen, summary.OptionalOpen)
	}

	proofs := make(map[string]CompletionProofItem, len(completion.Items))
	for _, item := range completion.Items {
		proofs[item.GoalItemID] = item
	}

	requiredSatisfied := 0
	for _, item := range goal.Required {
		proof, ok := proofs[item.ID]
		if !ok {
			return fmt.Errorf("completion proof missing goal_item_id %s", item.ID)
		}
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			if proof.Verdict != completionVerdictSatisfied {
				return fmt.Errorf("completion proof item %s verdict=%q, want satisfied", item.ID, proof.Verdict)
			}
			if completion.CodeChanged {
				if proof.Basis != completionBasisRunChange && proof.Basis != completionBasisMixed {
					return fmt.Errorf("completion proof item %s basis=%q, want run_change or mixed", item.ID, proof.Basis)
				}
			} else if proof.Basis != completionBasisPreexisting {
				return fmt.Errorf("completion proof item %s basis=%q, want preexisting", item.ID, proof.Basis)
			}
			if len(trimmedGoalEvidencePaths(proof.EvidencePaths)) == 0 {
				return fmt.Errorf("completion proof item %s is satisfied but has no evidence_paths", item.ID)
			}
			if err := validateGoalEvidencePaths(projectRoot, item.ID, proof.EvidencePaths); err != nil {
				return err
			}
			requiredSatisfied++
		case goalItemStateWaived:
			if proof.Verdict != completionVerdictWaived {
				return fmt.Errorf("completion proof item %s verdict=%q, want waived", item.ID, proof.Verdict)
			}
			if !item.UserApproved || !proof.UserApproved {
				return fmt.Errorf("completion proof item %s is waived without explicit user approval", item.ID)
			}
			requiredSatisfied++
		default:
			if proof.Verdict != completionVerdictUnsatisfied {
				return fmt.Errorf("completion proof item %s verdict=%q, want unsatisfied", item.ID, proof.Verdict)
			}
		}
	}

	requiredRemaining := summary.RequiredTotal - requiredSatisfied
	if completion.RequiredSatisfied != requiredSatisfied {
		return fmt.Errorf("completion proof required_satisfied=%d, want %d", completion.RequiredSatisfied, requiredSatisfied)
	}
	if completion.RequiredRemaining != requiredRemaining {
		return fmt.Errorf("completion proof required_remaining=%d, want %d", completion.RequiredRemaining, requiredRemaining)
	}

	wantSatisfied := acceptanceStatus(acceptance) == acceptanceStatusPassed && summary.RequiredTotal > 0 && requiredRemaining == 0
	if completion.GoalSatisfied != wantSatisfied {
		return fmt.Errorf("completion proof goal_satisfied=%t, want %t", completion.GoalSatisfied, wantSatisfied)
	}

	return nil
}

func validateGoalEvidencePaths(projectRoot, itemID string, evidencePaths []string) error {
	for _, evidencePath := range evidencePaths {
		evidencePath = strings.TrimSpace(evidencePath)
		if evidencePath == "" {
			return fmt.Errorf("goal item %s has empty evidence path", itemID)
		}
		resolved := evidencePath
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(projectRoot, resolved)
		}
		if _, err := os.Stat(resolved); err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("goal item %s evidence path %s does not exist", itemID, evidencePath)
			}
			return fmt.Errorf("goal item %s evidence path %s: %w", itemID, evidencePath, err)
		}
	}
	return nil
}
