package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type DurableLogEvent struct {
	Version int             `json:"version"`
	Kind    string          `json:"kind"`
	At      string          `json:"at"`
	Actor   string          `json:"actor"`
	Body    json.RawMessage `json:"body"`
}

var durableLogKinds = map[DurableSurfaceName]map[string]struct{}{
	DurableSurfaceGoalLog: {
		"decision":   {},
		"checkpoint": {},
		"blocker":    {},
		"handoff":    {},
		"closeout":   {},
		"note":       {},
		"update":     {},
	},
	DurableSurfaceExperiments: {
		"experiment.created":    {},
		"experiment.integrated": {},
		"experiment.closed":     {},
		"evolve.stopped":        {},
	},
	DurableSurfaceInterventionLog: {
		"user_redirect":  {},
		"user_tell":      {},
		"master_reframe": {},
	},
}

func LoadDurableLog(path string, surface DurableSurfaceName) ([]DurableLogEvent, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	events, err := parseDurableLogBuffer(data, surface)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return events, nil
}

func parseDurableLogBuffer(data []byte, surface DurableSurfaceName) ([]DurableLogEvent, error) {
	lines := splitNonEmptyLines(string(data))
	if len(lines) == 0 {
		return nil, nil
	}
	events := make([]DurableLogEvent, 0, len(lines))
	for i, line := range lines {
		event, err := parseDurableLogLine([]byte(line), surface)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		events = append(events, event)
	}
	return events, nil
}

func parseDurableLogLine(line []byte, surface DurableSurfaceName) (DurableLogEvent, error) {
	var event DurableLogEvent
	if err := decodeStrictJSON(line, &event); err != nil {
		return DurableLogEvent{}, err
	}
	if err := validateDurableLogEvent(event, surface); err != nil {
		return DurableLogEvent{}, err
	}
	return event, nil
}

func validateDurableLogEvent(event DurableLogEvent, surface DurableSurfaceName) error {
	if event.Version <= 0 {
		return fmt.Errorf("durable log event version must be positive")
	}
	kind := strings.TrimSpace(event.Kind)
	if kind == "" {
		return fmt.Errorf("durable log event kind is required")
	}
	allowedKinds, ok := durableLogKinds[surface]
	if !ok {
		return fmt.Errorf("durable log kinds are not defined for %s", surface)
	}
	if _, ok := allowedKinds[kind]; !ok {
		return fmt.Errorf("invalid durable log kind %q for %s", event.Kind, surface)
	}
	if strings.TrimSpace(event.Actor) == "" {
		return fmt.Errorf("durable log event actor is required")
	}
	if strings.TrimSpace(event.At) == "" {
		return fmt.Errorf("durable log event timestamp is required")
	}
	if _, err := time.Parse(time.RFC3339, event.At); err != nil {
		return fmt.Errorf("invalid durable log timestamp %q", event.At)
	}
	if len(bytes.TrimSpace(event.Body)) == 0 {
		return fmt.Errorf("durable log event body is required")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(event.Body, &body); err != nil {
		return fmt.Errorf("durable log event body must be a JSON object: %w", err)
	}
	if surface == DurableSurfaceExperiments {
		if err := validateExperimentLogBody(kind, event.Body); err != nil {
			return err
		}
	}
	if surface == DurableSurfaceInterventionLog {
		if _, err := parseInterventionEventBody(event.Body); err != nil {
			return err
		}
	}
	return nil
}
