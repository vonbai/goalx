package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	ControlOperationKindRunBootstrap          = "run_bootstrap"
	ControlOperationKindBoundaryEstablishment = "boundary_establishment"
	ControlOperationKindSessionDispatch       = "session_dispatch"

	ControlOperationStatePreparing     = "preparing"
	ControlOperationStateAwaitingAgent = "awaiting_agent"
	ControlOperationStateHandshaking   = "handshaking"
	ControlOperationStateReconciling   = "reconciling"
	ControlOperationStateCommitted     = "committed"
	ControlOperationStateFailed        = "failed"
)

type ControlOperationsState struct {
	Version   int                               `json:"version"`
	Targets   map[string]ControlOperationTarget `json:"targets,omitempty"`
	UpdatedAt string                            `json:"updated_at,omitempty"`
}

type ControlOperationTarget struct {
	Kind              string   `json:"kind,omitempty"`
	State             string   `json:"state,omitempty"`
	Summary           string   `json:"summary,omitempty"`
	PendingConditions []string `json:"pending_conditions,omitempty"`
	LastError         string   `json:"last_error,omitempty"`
	StartedAt         string   `json:"started_at,omitempty"`
	UpdatedAt         string   `json:"updated_at,omitempty"`
	CommittedAt       string   `json:"committed_at,omitempty"`
	FailedAt          string   `json:"failed_at,omitempty"`
}

func RunBootstrapOperationKey() string {
	return "run.bootstrap"
}

func BoundaryEstablishmentOperationKey() string {
	return "run.boundary"
}

func SessionDispatchOperationKey(sessionName string) string {
	return strings.TrimSpace(sessionName)
}

func ControlOperationsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "operations.json")
}

func EnsureControlOperationsState(runDir string) (*ControlOperationsState, error) {
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		return nil, err
	}
	var ensured *ControlOperationsState
	if err := mutateStructuredFile(
		ControlOperationsPath(runDir),
		0o644,
		func(data []byte) (*ControlOperationsState, error) {
			return parseControlOperationsState(data)
		},
		func() *ControlOperationsState {
			return &ControlOperationsState{
				Version:   1,
				Targets:   map[string]ControlOperationTarget{},
				UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			}
		},
		func(state *ControlOperationsState) error {
			normalizeControlOperationsState(state)
			ensured = cloneControlOperationsState(state)
			return nil
		},
		func(state *ControlOperationsState) ([]byte, error) {
			normalizeControlOperationsState(state)
			if state.UpdatedAt == "" {
				state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			}
			return json.MarshalIndent(state, "", "  ")
		},
	); err != nil {
		return nil, err
	}
	return ensured, nil
}

func LoadControlOperationsState(path string) (*ControlOperationsState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return parseControlOperationsState(data)
}

func SaveControlOperationsState(path string, state *ControlOperationsState) error {
	if state == nil {
		return fmt.Errorf("control operations state is nil")
	}
	normalizeControlOperationsState(state)
	if err := validateControlOperationsState(state); err != nil {
		return err
	}
	if state.UpdatedAt == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(path, state)
}

func parseControlOperationsState(data []byte) (*ControlOperationsState, error) {
	state := &ControlOperationsState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		state.Targets = map[string]ControlOperationTarget{}
		return state, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(state); err != nil {
		return nil, fmt.Errorf("parse control operations state: %w", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, fmt.Errorf("parse control operations state: %w", err)
	}
	normalizeControlOperationsState(state)
	if err := validateControlOperationsState(state); err != nil {
		return nil, err
	}
	return state, nil
}

func validateControlOperationsState(state *ControlOperationsState) error {
	if state == nil {
		return fmt.Errorf("control operations state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("control operations state version must be positive")
	}
	for target, op := range state.Targets {
		if strings.TrimSpace(target) == "" {
			return fmt.Errorf("control operations target name is required")
		}
		if err := validateControlOperationTarget(op); err != nil {
			return fmt.Errorf("target %s: %w", target, err)
		}
	}
	return nil
}

func validateControlOperationTarget(op ControlOperationTarget) error {
	switch op.Kind {
	case ControlOperationKindRunBootstrap, ControlOperationKindBoundaryEstablishment, ControlOperationKindSessionDispatch:
	default:
		return fmt.Errorf("invalid kind %q", op.Kind)
	}
	switch op.State {
	case ControlOperationStatePreparing, ControlOperationStateAwaitingAgent, ControlOperationStateHandshaking, ControlOperationStateReconciling, ControlOperationStateCommitted, ControlOperationStateFailed:
	default:
		return fmt.Errorf("invalid state %q", op.State)
	}
	for _, condition := range op.PendingConditions {
		if strings.TrimSpace(condition) == "" {
			return fmt.Errorf("pending condition must be non-empty")
		}
	}
	return nil
}

func normalizeControlOperationsState(state *ControlOperationsState) {
	if state == nil {
		return
	}
	if state.Version <= 0 {
		state.Version = 1
	}
	if state.Targets == nil {
		state.Targets = map[string]ControlOperationTarget{}
	}
	normalized := make(map[string]ControlOperationTarget, len(state.Targets))
	for target, op := range state.Targets {
		trimmedTarget := strings.TrimSpace(target)
		if trimmedTarget == "" {
			continue
		}
		normalizeControlOperationTarget(&op)
		normalized[trimmedTarget] = op
	}
	state.Targets = normalized
	state.UpdatedAt = strings.TrimSpace(state.UpdatedAt)
}

func normalizeControlOperationTarget(op *ControlOperationTarget) {
	if op == nil {
		return
	}
	op.Kind = strings.TrimSpace(op.Kind)
	op.State = strings.TrimSpace(op.State)
	op.Summary = strings.TrimSpace(op.Summary)
	op.LastError = strings.TrimSpace(op.LastError)
	op.StartedAt = strings.TrimSpace(op.StartedAt)
	op.UpdatedAt = strings.TrimSpace(op.UpdatedAt)
	op.CommittedAt = strings.TrimSpace(op.CommittedAt)
	op.FailedAt = strings.TrimSpace(op.FailedAt)
	if op.PendingConditions == nil {
		op.PendingConditions = []string{}
		return
	}
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(op.PendingConditions))
	for _, condition := range op.PendingConditions {
		trimmed := strings.TrimSpace(condition)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	op.PendingConditions = normalized
}

func cloneControlOperationsState(state *ControlOperationsState) *ControlOperationsState {
	if state == nil {
		return nil
	}
	cloned := &ControlOperationsState{
		Version:   state.Version,
		Targets:   map[string]ControlOperationTarget{},
		UpdatedAt: state.UpdatedAt,
	}
	for target, op := range state.Targets {
		opClone := op
		opClone.PendingConditions = append([]string(nil), op.PendingConditions...)
		cloned.Targets[target] = opClone
	}
	return cloned
}

func refreshControlOperationFacts(runDir string) error {
	if err := refreshBoundaryEstablishmentOperation(runDir); err != nil {
		return err
	}
	return reconcileSessionDispatchOperations(runDir, 15*time.Second)
}

func reconcileSessionDispatchOperations(runDir string, staleAfter time.Duration) error {
	state, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil || state == nil || len(state.Targets) == 0 {
		return err
	}
	sessionState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	changed := false
	for target, op := range state.Targets {
		if op.Kind != ControlOperationKindSessionDispatch {
			continue
		}
		switch op.State {
		case ControlOperationStateCommitted, ControlOperationStateFailed, ControlOperationStateReconciling:
			continue
		}
		if sessionState != nil {
			if sess, ok := sessionState.Sessions[target]; ok && strings.TrimSpace(sess.State) != "" {
				continue
			}
		}
		if staleAfter > 0 && strings.TrimSpace(op.UpdatedAt) != "" {
			updatedAt, parseErr := time.Parse(time.RFC3339, op.UpdatedAt)
			if parseErr == nil && now.Sub(updatedAt) < staleAfter {
				continue
			}
		}
		if !hasProvisionalSessionSurfaces(runDir, target) {
			continue
		}
		op.State = ControlOperationStateReconciling
		op.Summary = "provisional session surfaces exist without committed runtime state"
		op.PendingConditions = []string{"runtime_state_published"}
		op.UpdatedAt = ""
		state.Targets[target] = op
		changed = true
	}
	if !changed {
		return nil
	}
	return SaveControlOperationsState(ControlOperationsPath(runDir), state)
}

func hasProvisionalSessionSurfaces(runDir, sessionName string) bool {
	paths := []string{
		SessionIdentityPath(runDir, sessionName),
		JournalPath(runDir, sessionName),
		ControlInboxPath(runDir, sessionName),
		SessionCursorPath(runDir, sessionName),
		PanePIDPath(runDir, sessionName),
	}
	if idx, err := sessionIndexFromName(sessionName); err == nil {
		paths = append(paths, filepath.Join(runDir, fmt.Sprintf("program-%d.md", idx)))
	}
	for _, path := range paths {
		if fileExists(path) {
			return true
		}
	}
	return false
}

func operationSessionNames(operations map[string]ControlOperationTarget) []string {
	names := make([]string, 0, len(operations))
	for target, op := range operations {
		if op.Kind != ControlOperationKindSessionDispatch {
			continue
		}
		if _, err := sessionIndexFromName(target); err != nil {
			continue
		}
		names = append(names, target)
	}
	sortStrings(names)
	return names
}

func sessionIndexFromName(name string) (int, error) {
	name = strings.TrimSpace(name)
	if !strings.HasPrefix(name, "session-") {
		return 0, fmt.Errorf("not a session target %q", name)
	}
	value := strings.TrimPrefix(name, "session-")
	idx, err := strconv.Atoi(value)
	if err != nil || idx <= 0 {
		return 0, fmt.Errorf("invalid session target %q", name)
	}
	return idx, nil
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	for i := 0; i < len(values)-1; i++ {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
