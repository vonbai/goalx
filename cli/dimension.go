package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const dimensionUsage = "usage: goalx dimension [--run NAME] <session-N|all> (--set NAMES | --add NAME | --remove NAME)"

type DimensionsState struct {
	Version   int                 `json:"version"`
	Sessions  map[string][]string `json:"sessions,omitempty"`
	UpdatedAt string              `json:"updated_at,omitempty"`
}

type dimensionMutation struct {
	target string
	action string
	values []string
}

func ControlDimensionsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "dimensions.json")
}

func Dimension(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if printUsageIfHelp(rest, dimensionUsage) {
		return nil
	}

	mutation, err := parseDimensionMutation(rest)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	targets, err := resolveDimensionTargets(rc.RunDir, mutation.target, mutation.action)
	if err != nil {
		return err
	}

	state, err := EnsureDimensionsState(rc.RunDir)
	if err != nil {
		return err
	}
	for _, sessionName := range targets {
		next := applyDimensionMutation(state.Sessions[sessionName], mutation.action, mutation.values)
		if len(next) == 0 {
			delete(state.Sessions, sessionName)
			continue
		}
		state.Sessions[sessionName] = next
	}
	if err := SaveDimensionsState(ControlDimensionsPath(rc.RunDir), state); err != nil {
		return err
	}

	for _, sessionName := range targets {
		fmt.Printf("%s: %s\n", sessionName, formatDimensions(state.Sessions[sessionName]))
	}
	return nil
}

func parseDimensionMutation(args []string) (dimensionMutation, error) {
	if len(args) != 3 {
		return dimensionMutation{}, fmt.Errorf(dimensionUsage)
	}

	mutation := dimensionMutation{
		target: strings.TrimSpace(args[0]),
		action: args[1],
	}
	if mutation.target == "" {
		return dimensionMutation{}, fmt.Errorf(dimensionUsage)
	}

	allowEmpty := mutation.action == "--set"
	values, err := parseDimensionNames(args[2], allowEmpty)
	if err != nil {
		return dimensionMutation{}, err
	}
	switch mutation.action {
	case "--set":
		mutation.values = values
	case "--add", "--remove":
		if mutation.target == "all" || len(values) != 1 {
			return dimensionMutation{}, fmt.Errorf(dimensionUsage)
		}
		mutation.values = values
	default:
		return dimensionMutation{}, fmt.Errorf(dimensionUsage)
	}
	return mutation, nil
}

func resolveDimensionTargets(runDir, target, action string) ([]string, error) {
	if target == "all" {
		if action != "--set" {
			return nil, fmt.Errorf(dimensionUsage)
		}
		indexes, err := existingSessionIndexes(runDir)
		if err != nil {
			return nil, err
		}
		if len(indexes) == 0 {
			return nil, fmt.Errorf("run has no sessions")
		}
		targets := make([]string, 0, len(indexes))
		for _, idx := range indexes {
			targets = append(targets, SessionName(idx))
		}
		return targets, nil
	}

	idx, err := parseSessionIndex(target)
	if err != nil {
		return nil, err
	}
	ok, err := hasSessionIndex(runDir, idx)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("session %q out of range", target)
	}
	return []string{target}, nil
}

func parseDimensionNames(raw string, allowEmpty bool) ([]string, error) {
	parts := strings.Split(raw, ",")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	if len(names) == 0 && !allowEmpty {
		return nil, fmt.Errorf(dimensionUsage)
	}
	return names, nil
}

func applyDimensionMutation(current []string, action string, values []string) []string {
	switch action {
	case "--set":
		return append([]string(nil), values...)
	case "--add":
		next := append([]string(nil), current...)
		for _, value := range values {
			if !slices.Contains(next, value) {
				next = append(next, value)
			}
		}
		return next
	case "--remove":
		next := make([]string, 0, len(current))
		for _, value := range current {
			if !slices.Contains(values, value) {
				next = append(next, value)
			}
		}
		return next
	default:
		return append([]string(nil), current...)
	}
}

func formatDimensions(values []string) string {
	if len(values) == 0 {
		return "(none)"
	}
	return strings.Join(values, ",")
}

func LoadDimensionsState(path string) (*DimensionsState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	state := &DimensionsState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		state.Sessions = map[string][]string{}
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse dimensions state: %w", err)
	}
	normalizeDimensionsState(state)
	return state, nil
}

func SaveDimensionsState(path string, state *DimensionsState) error {
	if state == nil {
		return fmt.Errorf("dimensions state is nil")
	}
	normalizeDimensionsState(state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func EnsureDimensionsState(runDir string) (*DimensionsState, error) {
	path := ControlDimensionsPath(runDir)
	state, err := LoadDimensionsState(path)
	if err != nil {
		return nil, err
	}
	if state != nil {
		return state, nil
	}
	state = &DimensionsState{
		Version:  1,
		Sessions: map[string][]string{},
	}
	if err := SaveDimensionsState(path, state); err != nil {
		return nil, err
	}
	return state, nil
}

func normalizeDimensionsState(state *DimensionsState) {
	if state.Version == 0 {
		state.Version = 1
	}
	if state.Sessions == nil {
		state.Sessions = map[string][]string{}
		return
	}
	for sessionName, values := range state.Sessions {
		normalized := make([]string, 0, len(values))
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" || slices.Contains(normalized, value) {
				continue
			}
			normalized = append(normalized, value)
		}
		if len(normalized) == 0 {
			delete(state.Sessions, sessionName)
			continue
		}
		state.Sessions[sessionName] = normalized
	}
}
