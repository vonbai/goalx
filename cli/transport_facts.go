package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TransportFacts struct {
	Version   int                             `json:"version"`
	CheckedAt string                          `json:"checked_at,omitempty"`
	Targets   map[string]TransportTargetFacts `json:"targets,omitempty"`
}

type TransportTargetFacts struct {
	Target                string `json:"target,omitempty"`
	Window                string `json:"window,omitempty"`
	PaneID                string `json:"pane_id,omitempty"`
	Engine                string `json:"engine,omitempty"`
	PromptVisible         bool   `json:"prompt_visible,omitempty"`
	WorkingVisible        bool   `json:"working_visible,omitempty"`
	QueuedMessageVisible  bool   `json:"queued_message_visible,omitempty"`
	InputContainsWake     bool   `json:"input_contains_wake,omitempty"`
	TransportState        string `json:"transport_state,omitempty"`
	LastSampleAt          string `json:"last_sample_at,omitempty"`
	LastOutputAt          string `json:"last_output_at,omitempty"`
	LastSubmitAttemptAt   string `json:"last_submit_attempt_at,omitempty"`
	LastSubmitMode        string `json:"last_submit_mode,omitempty"`
	LastTransportAcceptAt string `json:"last_transport_accept_at,omitempty"`
	LastTransportError    string `json:"last_transport_error,omitempty"`
	ProviderDialogVisible bool   `json:"provider_dialog_visible,omitempty"`
	ProviderDialogKind    string `json:"provider_dialog_kind,omitempty"`
	ProviderDialogHint    string `json:"provider_dialog_hint,omitempty"`
}

func TransportFactsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "transport-facts.json")
}

func LoadTransportFacts(path string) (*TransportFacts, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state := &TransportFacts{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse transport facts: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveTransportFacts(runDir string, facts *TransportFacts) error {
	if facts == nil {
		return nil
	}
	if facts.Version == 0 {
		facts.Version = 1
	}
	if facts.CheckedAt == "" {
		facts.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(TransportFactsPath(runDir), facts)
}

func BuildTransportFacts(runDir, tmuxSession, masterEngine string) (*TransportFacts, error) {
	return buildTransportFacts(runDir, tmuxSession, masterEngine, nil)
}

func BuildTransportFactsWithPaneOutputTimes(runDir, tmuxSession, masterEngine string, paneOutputAt map[string]time.Time) (*TransportFacts, error) {
	return buildTransportFacts(runDir, tmuxSession, masterEngine, paneOutputAt)
}

func buildTransportFacts(runDir, tmuxSession, masterEngine string, paneOutputAt map[string]time.Time) (*TransportFacts, error) {
	facts := &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Targets:   map[string]TransportTargetFacts{},
	}
	panesByWindow, err := tmuxPanesByWindow(tmuxSession)
	if err != nil {
		return nil, err
	}
	if pane, ok := panesByWindow["master"]; ok {
		masterFacts := inspectTransportTarget(tmuxSession+":master", "master", "master", masterEngine)
		masterFacts.PaneID = pane.PaneID
		masterFacts.LastOutputAt = formatPaneOutputAt(paneOutputAt, pane.PaneID)
		applyLatestDeliveryFacts(runDir, "master", &masterFacts)
		facts.Targets["master"] = masterFacts
	}
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		name := SessionName(idx)
		identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, name))
		if err != nil {
			return nil, fmt.Errorf("load %s identity: %w", name, err)
		}
		if identity == nil {
			continue
		}
		window := name
		pane, ok := panesByWindow[window]
		if !ok {
			targetFacts := TransportTargetFacts{
				Target:       name,
				Window:       window,
				Engine:       identity.Engine,
				LastSampleAt: facts.CheckedAt,
			}
			applyLatestDeliveryFacts(runDir, name, &targetFacts)
			facts.Targets[name] = targetFacts
			continue
		}
		target := tmuxSession + ":" + window
		targetFacts := inspectTransportTarget(target, name, window, identity.Engine)
		targetFacts.PaneID = pane.PaneID
		targetFacts.LastOutputAt = formatPaneOutputAt(paneOutputAt, pane.PaneID)
		applyLatestDeliveryFacts(runDir, name, &targetFacts)
		facts.Targets[name] = targetFacts
	}
	if len(facts.Targets) == 0 {
		facts.Targets = nil
	}
	return facts, nil
}

func inspectTransportTarget(target, logicalTarget, window, engine string) TransportTargetFacts {
	facts := TransportTargetFacts{
		Target:       logicalTarget,
		Window:       window,
		Engine:       strings.TrimSpace(engine),
		LastSampleAt: time.Now().UTC().Format(time.RFC3339),
	}
	if captureAgentPane == nil {
		return facts
	}
	out, err := captureAgentPane(target)
	if err != nil {
		return facts
	}
	recent := tailRecentNonEmptyLines(out, 8)
	facts.PromptVisible = targetPromptVisible(recent)
	facts.WorkingVisible = targetWorkingVisible(facts.Engine, recent)
	facts.QueuedMessageVisible = targetQueuedMessageVisible(facts.Engine, recent)
	facts.InputContainsWake = targetWakeBuffered(recent)
	facts.ProviderDialogVisible, facts.ProviderDialogKind, facts.ProviderDialogHint = targetProviderDialogVisible(recent)
	facts.TransportState = inferTransportState(facts.Engine, facts)
	return facts
}

func inferTransportState(engine string, facts TransportTargetFacts) string {
	switch strings.TrimSpace(engine) {
	case "claude-code":
		if facts.QueuedMessageVisible || facts.WorkingVisible {
			return "sent"
		}
		if facts.InputContainsWake {
			return "buffered"
		}
	case "codex":
		if facts.QueuedMessageVisible {
			return "sent"
		}
		if facts.InputContainsWake {
			return "buffered"
		}
		if facts.WorkingVisible {
			return "sent"
		}
	default:
		if facts.InputContainsWake {
			return "buffered"
		}
		if facts.WorkingVisible {
			return "sent"
		}
	}
	return ""
}

func tailRecentNonEmptyLines(s string, limit int) []string {
	lines := splitNonEmptyLines(s)
	if len(lines) <= limit {
		return lines
	}
	return lines[len(lines)-limit:]
}

func targetPromptVisible(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "❯") || strings.HasPrefix(trimmed, "›") {
			return true
		}
	}
	return false
}

func targetWorkingVisible(engine string, lines []string) bool {
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "Working (") {
		return true
	}
	if strings.TrimSpace(engine) == "claude-code" {
		for _, marker := range []string{"Incubating", "Gesticulating", "Fermenting", "thinking with high effort"} {
			if strings.Contains(joined, marker) {
				return true
			}
		}
	}
	return false
}

func targetQueuedMessageVisible(engine string, lines []string) bool {
	joined := strings.Join(lines, "\n")
	switch strings.TrimSpace(engine) {
	case "claude-code":
		return strings.Contains(joined, "queued messages") || strings.Contains(joined, "Press up to edit queued messages")
	case "codex":
		return strings.Contains(joined, "Messages to be submitted after next tool call")
	default:
		return false
	}
}

func targetWakeBuffered(lines []string) bool {
	for _, line := range trailingPromptLines(lines, 3) {
		trimmed := strings.TrimSpace(line)
		if (strings.HasPrefix(trimmed, "❯") || strings.HasPrefix(trimmed, "›")) && strings.Contains(trimmed, transportWakeToken) {
			return true
		}
	}
	return false
}

func targetProviderDialogVisible(lines []string) (bool, string, string) {
	type dialogPattern struct {
		kind    string
		phrases []string
	}
	patterns := []dialogPattern{
		{
			kind: "permission_prompt",
			phrases: []string{
				"needs your permission",
				"permission needed",
				"approval required",
				"requires approval",
			},
		},
		{
			kind: "auth_prompt",
			phrases: []string{
				"please authenticate",
				"authentication required",
				"authenticate in browser",
				"continue in your browser",
				"open this url",
				"open the browser",
				"login required",
				"log in to continue",
				"sign in to continue",
				"authorize in your browser",
			},
		},
		{
			kind: "skill_ui",
			phrases: []string{
				"choose a skill",
				"select a skill",
				"skill chooser",
				"skill menu",
				"skill browser",
			},
		},
		{
			kind: "capacity_picker",
			phrases: []string{
				"choose a model",
				"select a model",
				"model picker",
				"capacity picker",
				"model capacity",
				"choose capacity",
				"pick a capacity",
			},
		},
		{
			kind: "input_prompt",
			phrases: []string{
				"requires user input",
				"waiting for user input",
				"enter your credentials",
				"provide your credentials",
				"complete authentication",
			},
		},
	}
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(trimmed)
		for _, pattern := range patterns {
			for _, phrase := range pattern.phrases {
				if strings.Contains(normalized, phrase) {
					return true, pattern.kind, compactHookText(trimmed)
				}
			}
		}
	}
	return false, "", ""
}

func trailingPromptLines(lines []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	var out []string
	for i := len(lines) - 1; i >= 0 && len(out) < limit; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		out = append(out, lines[i])
	}
	return out
}

func loadTransportTargetFacts(runDir, target string) TransportTargetFacts {
	facts, err := LoadTransportFacts(TransportFactsPath(runDir))
	if err != nil || facts == nil || facts.Targets == nil {
		return TransportTargetFacts{}
	}
	return facts.Targets[target]
}

func latestSessionTransportFacts(all *TransportFacts, sessionName string) TransportTargetFacts {
	if all == nil || all.Targets == nil {
		return TransportTargetFacts{}
	}
	return all.Targets[sessionName]
}

func tmuxPanesByWindow(session string) (map[string]tmuxPaneRef, error) {
	panes := map[string]tmuxPaneRef{}
	if !SessionExists(session) {
		return panes, nil
	}
	items, err := listTmuxSessionPanes(session)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.WindowName == "" {
			continue
		}
		panes[item.WindowName] = item
	}
	return panes, nil
}

func formatPaneOutputAt(paneOutputAt map[string]time.Time, paneID string) string {
	if len(paneOutputAt) == 0 || strings.TrimSpace(paneID) == "" {
		return ""
	}
	at, ok := paneOutputAt[paneID]
	if !ok || at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

func applyLatestDeliveryFacts(runDir, logicalTarget string, facts *TransportTargetFacts) {
	if facts == nil {
		return
	}
	delivery, ok := latestTargetDelivery(runDir, logicalTarget)
	if !ok {
		return
	}
	facts.LastSubmitAttemptAt = delivery.AttemptedAt
	facts.LastSubmitMode = delivery.SubmitMode
	facts.LastTransportError = delivery.LastError
	if transportState := strings.TrimSpace(delivery.TransportState); transportState != "" {
		if facts.TransportState == "" {
			facts.TransportState = transportState
		}
		if transportState == "sent" && delivery.AcceptedAt != "" {
			facts.LastTransportAcceptAt = delivery.AcceptedAt
		}
	}
	if facts.TransportState == "" && strings.TrimSpace(delivery.Status) != "" && delivery.Status != "pending" {
		facts.TransportState = delivery.Status
		if delivery.Status == "sent" && delivery.AcceptedAt != "" {
			facts.LastTransportAcceptAt = delivery.AcceptedAt
		}
	}
}
