package cli

import (
	"fmt"
	"strings"
)

type CanonicalAssuranceSummary struct {
	ScenarioCount int    `json:"scenario_count"`
	LastCheckedAt string `json:"last_checked_at,omitempty"`
	LastExitCode  *int   `json:"last_exit_code,omitempty"`
	EvidencePath  string `json:"evidence_path,omitempty"`
}

func CanonicalAssurancePlanPath(runDir string) string {
	return AssurancePlanPath(runDir)
}

func hashOptionalCanonicalAssurance(runDir string) (string, error) {
	path := strings.TrimSpace(CanonicalAssurancePlanPath(runDir))
	if path == "" || !fileExists(path) {
		return "", nil
	}
	return hashFileContents(path)
}

func LoadCanonicalAssuranceSummary(runDir string) (*CanonicalAssuranceSummary, error) {
	plan, err := LoadAssurancePlan(AssurancePlanPath(runDir))
	if err != nil {
		return nil, err
	}
	if plan != nil {
		summary := &CanonicalAssuranceSummary{
			ScenarioCount: len(plan.Scenarios),
			EvidencePath:  EvidenceLogPath(runDir),
		}
		events, err := LoadEvidenceLog(EvidenceLogPath(runDir))
		if err != nil {
			return nil, err
		}
		if len(events) > 0 {
			last := events[len(events)-1]
			summary.LastCheckedAt = last.At
			body, err := parseEvidenceEventBody(last.Body)
			if err == nil {
				if code, ok := oracleExitCode(body.OracleResult); ok {
					summary.LastExitCode = intPtr(code)
				}
			}
		}
		return summary, nil
	}
	return nil, nil
}

func oracleExitCode(result map[string]any) (int, bool) {
	if result == nil {
		return 0, false
	}
	switch v := result["exit_code"].(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(v), "%d", &parsed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
