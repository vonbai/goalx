package cli

import "strings"

func CanonicalBoundaryPath(runDir string) string {
	return ObligationModelPath(runDir)
}

func CanonicalBoundaryLogPath(runDir string) string {
	return ObligationLogPath(runDir)
}

func LoadCanonicalGoalState(runDir string) (*GoalState, error) {
	if model, err := LoadObligationModel(ObligationModelPath(runDir)); err != nil {
		return nil, err
	} else if model != nil && obligationModelHasContent(model) {
		return goalStateFromObligationModel(model), nil
	}
	return nil, nil
}

func obligationModelHasContent(model *ObligationModel) bool {
	if model == nil {
		return false
	}
	return len(model.Required) > 0 || len(model.Optional) > 0 || len(model.Guardrails) > 0
}

func hashOptionalCanonicalBoundary(runDir string) (string, error) {
	path := strings.TrimSpace(CanonicalBoundaryPath(runDir))
	if path == "" || !fileExists(path) {
		return "", nil
	}
	return hashFileContents(path)
}

func goalStateFromObligationModel(model *ObligationModel) *GoalState {
	if model == nil {
		return nil
	}
	normalizeObligationModel(model)
	state := &GoalState{
		Version:  1,
		Required: []GoalItem{},
		Optional: []GoalItem{},
	}
	for _, item := range model.Required {
		state.Required = append(state.Required, GoalItem{
			ID:            item.ID,
			Text:          item.Text,
			Source:        firstNonEmpty(item.Source, goalItemSourceMaster),
			Role:          normalizeGoalItemRole(item.Kind),
			Covers:        append([]string(nil), item.CoversClauses...),
			State:         firstNonEmpty(item.State, goalItemStateOpen),
			EvidencePaths: append([]string(nil), item.EvidencePaths...),
			Note:          item.Note,
			ApprovalRef:   item.ApprovalRef,
		})
	}
	for _, item := range model.Optional {
		state.Optional = append(state.Optional, GoalItem{
			ID:            item.ID,
			Text:          item.Text,
			Source:        firstNonEmpty(item.Source, goalItemSourceMaster),
			Role:          normalizeGoalItemRole(item.Kind),
			Covers:        append([]string(nil), item.CoversClauses...),
			State:         firstNonEmpty(item.State, goalItemStateOpen),
			EvidencePaths: append([]string(nil), item.EvidencePaths...),
			Note:          item.Note,
			ApprovalRef:   item.ApprovalRef,
		})
	}
	for _, item := range model.Guardrails {
		state.Optional = append(state.Optional, GoalItem{
			ID:            item.ID,
			Text:          item.Text,
			Source:        firstNonEmpty(item.Source, goalItemSourceMaster),
			Role:          goalItemRoleGuardrail,
			Covers:        append([]string(nil), item.CoversClauses...),
			State:         firstNonEmpty(item.State, goalItemStateOpen),
			EvidencePaths: append([]string(nil), item.EvidencePaths...),
			Note:          item.Note,
			ApprovalRef:   item.ApprovalRef,
		})
	}
	normalizeGoalState(state)
	return state
}
