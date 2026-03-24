package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type CoordinationState struct {
	Version       int                            `json:"version"`
	Objective     string                         `json:"objective,omitempty"`
	PlanSummary   []string                       `json:"plan_summary,omitempty"`
	Owners        map[string]string              `json:"owners,omitempty"`
	Sessions      map[string]CoordinationSession `json:"sessions,omitempty"`
	Decision      *CoordinationDecision          `json:"decision,omitempty"`
	Blocked       []string                       `json:"blocked,omitempty"`
	OpenQuestions []string                       `json:"open_questions,omitempty"`
	UpdatedAt     string                         `json:"updated_at,omitempty"`
}

type CoordinationSession struct {
	State              string                    `json:"state,omitempty"`
	ExecutionState     string                    `json:"execution_state,omitempty"`
	Scope              string                    `json:"scope,omitempty"`
	BlockedBy          string                    `json:"blocked_by,omitempty"`
	DispatchableSlices []goalx.DispatchableSlice `json:"dispatchable_slices,omitempty"`
	LastRound          int                       `json:"last_round,omitempty"`
	UpdatedAt          string                    `json:"updated_at,omitempty"`
}

type CoordinationDecision struct {
	RootCause        string `json:"root_cause,omitempty"`
	LocalPath        string `json:"local_path,omitempty"`
	CompatiblePath   string `json:"compatible_path,omitempty"`
	ArchitecturePath string `json:"architecture_path,omitempty"`
	ChosenPath       string `json:"chosen_path,omitempty"`
	ChosenPathReason string `json:"chosen_path_reason,omitempty"`
}

const (
	maxCoordinationScopeLen   = 240
	maxCoordinationReasonLen  = 240
	maxDecisionFieldLen       = 160
	maxDispatchableSliceCount = 3
	maxDispatchableTitleLen   = 96
	maxDispatchableWhyLen     = 160
)

func CoordinationPath(runDir string) string {
	return filepath.Join(runDir, "coordination.json")
}

func LoadCoordinationState(path string) (*CoordinationState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseCoordinationState(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return state, nil
}

func SaveCoordinationState(path string, state *CoordinationState) error {
	if state == nil {
		return fmt.Errorf("coordination state is nil")
	}
	normalizeCoordinationState(state)
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EnsureCoordinationState(runDir, objective string) (*CoordinationState, error) {
	path := CoordinationPath(runDir)
	state, err := LoadCoordinationState(path)
	if err != nil {
		return nil, err
	}
	if state == nil {
		state = &CoordinationState{
			Version:   1,
			Owners:    map[string]string{},
			Sessions:  map[string]CoordinationSession{},
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		if err := SaveCoordinationState(path, state); err != nil {
			return nil, err
		}
		return state, nil
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Owners == nil {
		state.Owners = map[string]string{}
	}
	if state.Sessions == nil {
		state.Sessions = map[string]CoordinationSession{}
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := SaveCoordinationState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}

type coordinationWire struct {
	Version       int                            `json:"version"`
	Objective     string                         `json:"objective,omitempty"`
	PlanSummary   []string                       `json:"plan_summary,omitempty"`
	Owners        map[string]string              `json:"owners,omitempty"`
	Sessions      map[string]CoordinationSession `json:"sessions,omitempty"`
	Decision      *CoordinationDecision          `json:"decision,omitempty"`
	Blocked       []string                       `json:"blocked,omitempty"`
	OpenQuestions []string                       `json:"open_questions,omitempty"`
	UpdatedAt     string                         `json:"updated_at,omitempty"`
}

type legacyCoordinationEntry struct {
	Owner       string   `json:"owner,omitempty"`
	Session     string   `json:"session,omitempty"`
	Scope       any      `json:"scope,omitempty"`
	Status      string   `json:"status,omitempty"`
	State       string   `json:"state,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	Note        string   `json:"note,omitempty"`
	Worktree    string   `json:"worktree_path,omitempty"`
	NextActions []string `json:"next_actions,omitempty"`
}

func parseCoordinationState(data []byte) (*CoordinationState, error) {
	wire := &coordinationWire{}
	if err := json.Unmarshal(data, wire); err == nil {
		state := &CoordinationState{
			Version:       maxInt(wire.Version, 1),
			Objective:     wire.Objective,
			PlanSummary:   wire.PlanSummary,
			Owners:        mapOrEmpty(wire.Owners),
			Sessions:      mapSessionsOrEmpty(wire.Sessions),
			Decision:      wire.Decision,
			Blocked:       append([]string(nil), wire.Blocked...),
			OpenQuestions: append([]string(nil), wire.OpenQuestions...),
			UpdatedAt:     wire.UpdatedAt,
		}
		normalizeCoordinationState(state)
		return state, nil
	}

	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	state := &CoordinationState{
		Version:   1,
		Owners:    map[string]string{},
		Sessions:  map[string]CoordinationSession{},
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	_ = json.Unmarshal(raw["version"], &state.Version)
	_ = json.Unmarshal(raw["objective"], &state.Objective)
	_ = json.Unmarshal(raw["updated_at"], &state.UpdatedAt)
	_ = json.Unmarshal(raw["plan_summary"], &state.PlanSummary)
	_ = json.Unmarshal(raw["open_questions"], &state.OpenQuestions)
	_ = json.Unmarshal(raw["decision"], &state.Decision)
	if state.Version <= 0 {
		state.Version = 1
	}
	normalizeLegacyCoordinationEntries(state, raw["active"], "active", "")
	normalizeLegacyCoordinationEntries(state, raw["blocked"], "blocked", "")
	normalizeLegacyCoordinationEntries(state, raw["parked"], "parked", "")
	normalizeLegacyCoordinationEntries(state, raw["waiting_external"], "active", "waiting_external")
	normalizeLegacyCoordinationEntries(state, raw["active_sessions"], "active", "")
	normalizeLegacyCoordinationEntries(state, raw["parked_sessions"], "parked", "")

	var recentSignals []string
	if err := json.Unmarshal(raw["recent_signals"], &recentSignals); err == nil {
		state.Blocked = append(state.Blocked, recentSignals...)
	}
	var nextActions []string
	if err := json.Unmarshal(raw["next_actions"], &nextActions); err == nil {
		state.PlanSummary = append(state.PlanSummary, nextActions...)
	}
	normalizeCoordinationState(state)
	return state, nil
}

func normalizeLegacyCoordinationEntries(state *CoordinationState, raw json.RawMessage, defaultState, executionState string) {
	if len(raw) == 0 {
		return
	}
	var entries []legacyCoordinationEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return
	}
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Session)
		if name == "" {
			name = strings.TrimSpace(entry.Owner)
		}
		if name == "" {
			continue
		}
		scope := legacyCoordinationScope(entry.Scope)
		current := state.Sessions[name]
		if current.State == "" {
			current.State = pickFirstNonEmpty(entry.State, entry.Status, defaultState)
		}
		if scope != "" && current.Scope == "" {
			current.Scope = scope
		}
		blockedBy := pickFirstNonEmpty(entry.Reason, entry.Note)
		if blockedBy != "" && current.BlockedBy == "" {
			current.BlockedBy = blockedBy
		}
		if executionState != "" && current.ExecutionState == "" {
			current.ExecutionState = executionState
		}
		if blockedBy != "" {
			state.Blocked = append(state.Blocked, blockedBy)
		}
		state.Sessions[name] = current
	}
}

func legacyCoordinationScope(scope any) string {
	switch v := scope.(type) {
	case string:
		return strings.TrimSpace(v)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, ", ")
	default:
		return ""
	}
}

func pickFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeCoordinationState(state *CoordinationState) {
	if state == nil {
		return
	}
	state.Objective = ""
	state.PlanSummary = dedupeNonEmptyStrings(state.PlanSummary, maxCoordinationReasonLen)
	state.Blocked = dedupeNonEmptyStrings(state.Blocked, maxCoordinationReasonLen)
	state.OpenQuestions = dedupeNonEmptyStrings(state.OpenQuestions, maxCoordinationReasonLen)
	if state.Sessions == nil {
		state.Sessions = map[string]CoordinationSession{}
	}
	for name, sess := range state.Sessions {
		sess.State = strings.TrimSpace(sess.State)
		sess.ExecutionState = strings.TrimSpace(sess.ExecutionState)
		sess.Scope = summarizeDigestText(sess.Scope, maxCoordinationScopeLen)
		sess.BlockedBy = summarizeDigestText(sess.BlockedBy, maxCoordinationReasonLen)
		sess.DispatchableSlices = sanitizeDispatchableSlices(sess.DispatchableSlices)
		state.Sessions[name] = sess
	}
	if state.Decision != nil {
		state.Decision.RootCause = summarizeDigestText(state.Decision.RootCause, maxDecisionFieldLen)
		state.Decision.LocalPath = summarizeDigestText(state.Decision.LocalPath, maxDecisionFieldLen)
		state.Decision.CompatiblePath = summarizeDigestText(state.Decision.CompatiblePath, maxDecisionFieldLen)
		state.Decision.ArchitecturePath = summarizeDigestText(state.Decision.ArchitecturePath, maxDecisionFieldLen)
		state.Decision.ChosenPath = summarizeDigestText(state.Decision.ChosenPath, maxDecisionFieldLen)
		state.Decision.ChosenPathReason = summarizeDigestText(state.Decision.ChosenPathReason, maxDecisionFieldLen)
	}
}

func sanitizeDispatchableSlices(src []goalx.DispatchableSlice) []goalx.DispatchableSlice {
	if len(src) == 0 {
		return nil
	}
	out := make([]goalx.DispatchableSlice, 0, minInt(len(src), maxDispatchableSliceCount))
	for _, slice := range src {
		title := summarizeDigestText(slice.Title, maxDispatchableTitleLen)
		if title == "" {
			continue
		}
		out = append(out, goalx.DispatchableSlice{
			Title:           title,
			Why:             summarizeDigestText(slice.Why, maxDispatchableWhyLen),
			Mode:            summarizeDigestText(slice.Mode, 32),
			SuggestedOwner:  summarizeDigestText(slice.SuggestedOwner, 48),
			SuggestedAction: summarizeDigestText(slice.SuggestedAction, 32),
			Evidence:        dedupeNonEmptyStrings(slice.Evidence, 160),
		})
		if len(out) >= maxDispatchableSliceCount {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func dedupeNonEmptyStrings(items []string, maxLen int) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		s := summarizeDigestText(item, maxLen)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func summarizeDigestText(s string, maxLen int) string {
	s = strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
	if s == "" || maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return strings.TrimSpace(s[:maxLen-3]) + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func mapOrEmpty(src map[string]string) map[string]string {
	if src == nil {
		return map[string]string{}
	}
	return src
}

func mapSessionsOrEmpty(src map[string]CoordinationSession) map[string]CoordinationSession {
	if src == nil {
		return map[string]CoordinationSession{}
	}
	return src
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
