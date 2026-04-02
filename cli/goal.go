package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	goalItemStateOpen    = "open"
	goalItemStateClaimed = "claimed"
	goalItemStateWaived  = "waived"

	goalItemSourceUser   = "user"
	goalItemSourceMaster = "master"

	goalItemRoleOutcome   = "outcome"
	goalItemRoleEnabler   = "enabler"
	goalItemRoleProof     = "proof"
	goalItemRoleGuardrail = "guardrail"
)

type GoalState struct {
	Version   int        `json:"version"`
	UpdatedAt string     `json:"updated_at,omitempty"`
	Required  []GoalItem `json:"required,omitempty"`
	Optional  []GoalItem `json:"optional,omitempty"`
}

type GoalItem struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	Source        string   `json:"source"`
	Role          string   `json:"role"`
	Covers        []string `json:"covers,omitempty"`
	State         string   `json:"state,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	Note          string   `json:"note,omitempty"`
	ApprovalRef   string   `json:"approval_ref,omitempty"`
}

type GoalSummary struct {
	Version           int
	RequiredTotal     int
	RequiredSatisfied int
	RequiredRemaining int
	OptionalOpen      int
}

func NewGoalState() *GoalState {
	state := &GoalState{
		Version:   1,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Required:  []GoalItem{},
		Optional:  []GoalItem{},
	}
	normalizeGoalState(state)
	return state
}

func SummarizeGoalState(state *GoalState) GoalSummary {
	var summary GoalSummary
	if state == nil {
		return summary
	}
	normalizeGoalState(state)
	summary.Version = state.Version
	for _, item := range state.Required {
		summary.RequiredTotal++
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			summary.RequiredSatisfied++
		case goalItemStateWaived:
			if strings.TrimSpace(item.ApprovalRef) != "" {
				summary.RequiredSatisfied++
			} else {
				summary.RequiredRemaining++
			}
		default:
			summary.RequiredRemaining++
		}
	}
	for _, item := range state.Optional {
		if normalizeGoalItemState(item.State) == goalItemStateOpen {
			summary.OptionalOpen++
		}
	}
	return summary
}

func goalRemainingRequiredIDs(state *GoalState) []string {
	if state == nil {
		return nil
	}
	normalizeGoalState(state)
	ids := make([]string, 0, len(state.Required))
	for _, item := range state.Required {
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			continue
		case goalItemStateWaived:
			if strings.TrimSpace(item.ApprovalRef) != "" {
				continue
			}
		}
		if strings.TrimSpace(item.ID) != "" {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

func ValidateGoalStateForVerification(state *GoalState) (GoalSummary, error) {
	summary := SummarizeGoalState(state)
	if state == nil {
		return summary, fmt.Errorf("goal state is missing")
	}
	if summary.RequiredTotal == 0 {
		return summary, fmt.Errorf("goal state has no required outcomes")
	}

	for _, item := range state.Required {
		if strings.TrimSpace(item.ID) == "" {
			return summary, fmt.Errorf("goal state has required item with empty id")
		}
		if strings.TrimSpace(item.Text) == "" {
			return summary, fmt.Errorf("goal item %s is missing text", item.ID)
		}
		switch normalizeGoalItemState(item.State) {
		case goalItemStateClaimed:
			if len(trimmedGoalEvidencePaths(item.EvidencePaths)) == 0 {
				return summary, fmt.Errorf("goal item %s is claimed but has no evidence_paths", item.ID)
			}
		case goalItemStateWaived:
			if strings.TrimSpace(item.ApprovalRef) == "" {
				return summary, fmt.Errorf("goal item %s is waived without explicit approval_ref", item.ID)
			}
		default:
			return summary, fmt.Errorf("goal item %s remains open", item.ID)
		}
	}

	return summary, nil
}

func normalizeGoalState(state *GoalState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Required == nil {
		state.Required = []GoalItem{}
	}
	if state.Optional == nil {
		state.Optional = []GoalItem{}
	}
	for i := range state.Required {
		normalizeGoalItem(&state.Required[i])
	}
	for i := range state.Optional {
		normalizeGoalItem(&state.Optional[i])
	}
}

func normalizeGoalItem(item *GoalItem) {
	if item == nil {
		return
	}
	item.Source = normalizeGoalItemSource(item.Source)
	item.Role = normalizeGoalItemRole(item.Role)
	item.Covers = trimmedGoalCovers(item.Covers)
	item.State = normalizeGoalItemState(item.State)
	item.EvidencePaths = trimmedGoalEvidencePaths(item.EvidencePaths)
	item.ApprovalRef = strings.TrimSpace(item.ApprovalRef)
}

func validateGoalStateInput(state *GoalState) error {
	if state == nil {
		return fmt.Errorf("goal state is nil")
	}
	for _, item := range state.Required {
		if err := validateGoalItemInput(item); err != nil {
			return err
		}
	}
	for _, item := range state.Optional {
		if err := validateGoalItemInput(item); err != nil {
			return err
		}
	}
	for _, item := range append(append([]GoalItem(nil), state.Required...), state.Optional...) {
		if normalizeGoalItemState(item.State) == goalItemStateWaived && strings.TrimSpace(item.ApprovalRef) == "" {
			return fmt.Errorf("goal item %s is waived without explicit approval_ref", item.ID)
		}
	}
	return nil
}

func validateGoalItemInput(item GoalItem) error {
	switch source := strings.TrimSpace(item.Source); source {
	case "":
		return fmt.Errorf("goal item source is required")
	case goalItemSourceUser, goalItemSourceMaster:
	default:
		return fmt.Errorf("invalid goal item source %q", item.Source)
	}
	switch role := strings.TrimSpace(item.Role); role {
	case "":
		return fmt.Errorf("goal item role is required")
	case goalItemRoleOutcome, goalItemRoleEnabler, goalItemRoleProof, goalItemRoleGuardrail:
	default:
		return fmt.Errorf("invalid goal item role %q", item.Role)
	}
	if state := strings.TrimSpace(item.State); state != "" {
		switch state {
		case goalItemStateOpen, goalItemStateClaimed, goalItemStateWaived:
		default:
			return fmt.Errorf("invalid goal item state %q", item.State)
		}
	}
	return nil
}

func parseGoalState(data []byte) (*GoalState, error) {
	var state GoalState
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, fmt.Errorf("goal state is empty"))
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	if err := validateGoalStateInput(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceObligationModel, err)
	}
	normalizeGoalState(&state)
	return &state, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return fmt.Errorf("unexpected trailing JSON content")
}

func normalizeGoalItemSource(source string) string {
	switch strings.TrimSpace(source) {
	case goalItemSourceUser:
		return goalItemSourceUser
	case goalItemSourceMaster:
		return goalItemSourceMaster
	default:
		return strings.TrimSpace(source)
	}
}

func normalizeGoalItemRole(role string) string {
	switch strings.TrimSpace(role) {
	case goalItemRoleOutcome:
		return goalItemRoleOutcome
	case goalItemRoleEnabler:
		return goalItemRoleEnabler
	case goalItemRoleProof:
		return goalItemRoleProof
	case goalItemRoleGuardrail:
		return goalItemRoleGuardrail
	default:
		return strings.TrimSpace(role)
	}
}

func normalizeGoalItemState(state string) string {
	switch strings.TrimSpace(state) {
	case goalItemStateClaimed:
		return goalItemStateClaimed
	case goalItemStateWaived:
		return goalItemStateWaived
	default:
		return goalItemStateOpen
	}
}

func trimmedGoalEvidencePaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if trimmed := strings.TrimSpace(path); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func trimmedGoalCovers(covers []string) []string {
	if len(covers) == 0 {
		return nil
	}
	out := make([]string, 0, len(covers))
	for _, cover := range covers {
		if trimmed := strings.TrimSpace(cover); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
