package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EvidenceLogEvent = DurableLogEvent

type EvidenceEventBody struct {
	ScenarioID   string         `json:"scenario_id"`
	Scope        string         `json:"scope,omitempty"`
	Revision     string         `json:"revision,omitempty"`
	HarnessKind  string         `json:"harness_kind"`
	OracleResult map[string]any `json:"oracle_result,omitempty"`
	ArtifactRefs []string       `json:"artifact_refs,omitempty"`
}

func EvidenceLogPath(runDir string) string {
	return filepath.Join(runDir, "evidence-log.jsonl")
}

func LoadEvidenceLog(path string) ([]EvidenceLogEvent, error) {
	events, err := LoadDurableLog(path, DurableSurfaceEvidenceLog)
	if err != nil {
		return nil, err
	}
	for _, event := range events {
		if _, err := parseEvidenceEventBody(event.Body); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func AppendEvidenceLogEvent(path, kind, actor string, body EvidenceEventBody) error {
	normalizeEvidenceEventBody(&body)
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	line, err := buildDurableEventLine(DurableSurfaceEvidenceLog, DurableMutation{
		Surface: DurableSurfaceEvidenceLog,
		Kind:    kind,
		Actor:   actor,
		Body:    data,
	})
	if err != nil {
		return err
	}
	return withExclusiveFileLock(path, func() error {
		existing, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		var buf bytes.Buffer
		if len(existing) > 0 {
			buf.Write(existing)
			if existing[len(existing)-1] != '\n' {
				buf.WriteByte('\n')
			}
		}
		buf.Write(line)
		buf.WriteByte('\n')
		return writeFileAtomic(path, buf.Bytes(), 0o644)
	})
}

func parseEvidenceEventBody(data []byte) (EvidenceEventBody, error) {
	var body EvidenceEventBody
	if err := decodeStrictJSON(data, &body); err != nil {
		return EvidenceEventBody{}, err
	}
	if strings.TrimSpace(body.ScenarioID) == "" {
		return EvidenceEventBody{}, fmt.Errorf("evidence log scenario_id is required")
	}
	if strings.TrimSpace(body.HarnessKind) == "" {
		return EvidenceEventBody{}, fmt.Errorf("evidence log harness_kind is required")
	}
	normalizeEvidenceEventBody(&body)
	return body, nil
}

func normalizeEvidenceEventBody(body *EvidenceEventBody) {
	if body == nil {
		return
	}
	body.ScenarioID = strings.TrimSpace(body.ScenarioID)
	body.Scope = strings.TrimSpace(body.Scope)
	body.Revision = strings.TrimSpace(body.Revision)
	body.HarnessKind = strings.TrimSpace(body.HarnessKind)
	body.ArtifactRefs = compactStrings(body.ArtifactRefs)
	if body.OracleResult == nil {
		body.OracleResult = map[string]any{}
	}
}
