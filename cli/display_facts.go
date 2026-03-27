package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type displayStatusRecord struct {
	Phase             string   `json:"phase,omitempty"`
	RequiredRemaining *int     `json:"required_remaining,omitempty"`
	ActiveSessions    []string `json:"active_sessions,omitempty"`
}

func refreshDisplayFacts(rc *RunContext) {
	if rc == nil {
		return
	}
	_ = reconcileControlDeliveries(rc.RunDir)
	if snapshot, err := BuildActivitySnapshot(rc.ProjectRoot, rc.Name, rc.RunDir); err == nil && snapshot != nil {
		_ = SaveActivitySnapshot(rc.RunDir, snapshot)
	}
	masterEngine := ""
	if rc.Config != nil {
		masterEngine = rc.Config.Master.Engine
	}
	if SessionExists(rc.TmuxSession) {
		if facts, err := BuildTransportFacts(rc.RunDir, rc.TmuxSession, masterEngine); err == nil && facts != nil {
			_ = SaveTransportFacts(rc.RunDir, facts)
		}
		return
	}
	_ = SaveTransportFacts(rc.RunDir, &TransportFacts{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	})
}

func printRunAdvisories(rc *RunContext) {
	advisories := collectRunAdvisories(rc)
	if len(advisories) == 0 {
		return
	}
	fmt.Println("### advisories")
	for _, advisory := range advisories {
		fmt.Printf("- %s\n", advisory)
	}
	fmt.Println()
}

func collectRunAdvisories(rc *RunContext) []string {
	if rc == nil {
		return nil
	}
	status, err := loadDisplayStatusRecord(RunStatusPath(rc.RunDir))
	if err != nil || status == nil {
		return nil
	}
	summaryExists := fileExists(SummaryPath(rc.RunDir))
	completionExists := fileExists(CompletionStatePath(rc.RunDir))
	advisories := make([]string, 0, 2)
	if status.RequiredRemaining != nil && *status.RequiredRemaining == 0 && (!summaryExists || !completionExists) {
		advisories = append(advisories, fmt.Sprintf("Closeout artifacts missing: required_remaining=0 summary_exists=%t completion_proof_exists=%t", summaryExists, completionExists))
	}
	if activity, err := LoadActivitySnapshot(ActivityPath(rc.RunDir)); err == nil && activity != nil {
		coverage := activity.Coverage
		if coverage.OwnersPresent && (len(coverage.UnmappedOpenIDs) > 0 || len(coverage.OwnerSessionMissingIDs) > 0) {
			parts := make([]string, 0, 3)
			if len(coverage.UnmappedOpenIDs) > 0 {
				parts = append(parts, "unmapped_open="+strings.Join(coverage.UnmappedOpenIDs, ","))
			}
			if len(coverage.OwnerSessionMissingIDs) > 0 {
				parts = append(parts, "owner_session_missing="+strings.Join(coverage.OwnerSessionMissingIDs, ","))
			}
			reusable := append([]string{}, coverage.IdleReusableSessions...)
			reusable = append(reusable, coverage.ParkedReusableSessions...)
			if len(reusable) > 0 {
				parts = append(parts, "reusable_sessions="+strings.Join(reusable, ","))
			}
			advisories = append(advisories, "Coverage facts: "+strings.Join(parts, " "))
		}
	}
	meta, err := LoadRunMetadata(RunMetadataPath(rc.RunDir))
	if err != nil || meta == nil || strings.TrimSpace(meta.Intent) != runIntentEvolve {
		return advisories
	}
	if strings.TrimSpace(status.Phase) != "review" || (summaryExists && completionExists) {
		return advisories
	}
	evolutionEntries, lastTrialAt := evolutionLogFacts(EvolutionLogPath(rc.RunDir))
	parts := []string{
		"phase=review",
		fmt.Sprintf("active_sessions=%d", len(status.ActiveSessions)),
		fmt.Sprintf("evolution_entries=%d", evolutionEntries),
		fmt.Sprintf("summary_exists=%t", summaryExists),
		fmt.Sprintf("completion_proof_exists=%t", completionExists),
	}
	if lastTrialAt != "" {
		parts = append(parts, "last_trial_record_at="+lastTrialAt)
	}
	advisories = append(advisories, "Potential evolve stall: "+strings.Join(parts, " "))
	return advisories
}

func formatCoverageSummary(coverage RequiredCoverage) string {
	if len(coverage.OpenRequiredIDs) == 0 && len(coverage.OwnedOpenIDs) == 0 && len(coverage.UnmappedOpenIDs) == 0 && len(coverage.OwnerSessionMissingIDs) == 0 {
		return ""
	}
	parts := make([]string, 0, 7)
	if !coverage.OwnersPresent {
		parts = append(parts, "coverage=unknown")
	} else {
		parts = append(parts, "coverage=explicit")
	}
	appendCoverageSummaryPart(&parts, "open_required", coverage.OpenRequiredIDs)
	appendCoverageSummaryPart(&parts, "owned_open", coverage.OwnedOpenIDs)
	appendCoverageSummaryPart(&parts, "unmapped_open", coverage.UnmappedOpenIDs)
	appendCoverageSummaryPart(&parts, "owner_session_missing", coverage.OwnerSessionMissingIDs)
	appendCoverageSummaryPart(&parts, "idle_reusable", coverage.IdleReusableSessions)
	appendCoverageSummaryPart(&parts, "parked_reusable", coverage.ParkedReusableSessions)
	return strings.Join(parts, " ")
}

func appendCoverageSummaryPart(parts *[]string, label string, values []string) {
	if len(values) == 0 {
		return
	}
	*parts = append(*parts, label+"="+strings.Join(values, ","))
}

func loadDisplayStatusRecord(path string) (*displayStatusRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var record displayStatusRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	return &record, nil
}

func evolutionLogFacts(path string) (int, string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, ""
	}
	count := len(splitNonEmptyLines(string(data)))
	if count == 0 {
		return 0, ""
	}
	info, err := os.Stat(path)
	if err != nil {
		return count, ""
	}
	return count, info.ModTime().UTC().Format(time.RFC3339)
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
