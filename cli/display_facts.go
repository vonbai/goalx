package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

func refreshDisplayFacts(rc *RunContext) error {
	if rc == nil {
		return nil
	}
	if err := repairCompletedRunFinalization(rc); err != nil {
		return err
	}
	if err := reconcileRunContinuityForRun(rc.ProjectRoot, rc.Name, rc.RunDir); err != nil {
		return err
	}
	_ = reconcileControlDeliveries(rc.RunDir)
	if _, err := RefreshRunSuccessContextForRun(rc.ProjectRoot, rc.RunDir); err != nil {
		return err
	}
	if err := RefreshWorktreeSnapshot(rc.RunDir); err != nil {
		return err
	}
	if err := refreshControlOperationFacts(rc.RunDir); err != nil {
		return err
	}
	masterEngine := ""
	if rc.Config != nil {
		masterEngine = rc.Config.Master.Engine
	}
	if SessionExistsInRun(rc.RunDir, rc.TmuxSession) {
		facts, err := BuildTransportFacts(rc.RunDir, rc.TmuxSession, masterEngine)
		if err != nil {
			return err
		}
		if facts != nil {
			if err := SaveTransportFacts(rc.RunDir, facts); err != nil {
				return err
			}
		}
	} else {
		if err := SaveTransportFacts(rc.RunDir, &TransportFacts{
			Version:   1,
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			return err
		}
	}
	snapshot, err := BuildActivitySnapshot(rc.ProjectRoot, rc.Name, rc.RunDir)
	if err != nil {
		return err
	}
	if snapshot != nil {
		if err := SaveActivitySnapshot(rc.RunDir, snapshot); err != nil {
			return err
		}
	}
	if err := RefreshEvolveFacts(rc.RunDir); err != nil {
		return err
	}
	return nil
}

func printRunAdvisories(rc *RunContext) error {
	advisories, err := collectRunAdvisories(rc)
	if err != nil {
		return err
	}
	if len(advisories) == 0 {
		return nil
	}
	fmt.Println("### advisories")
	for _, advisory := range advisories {
		fmt.Printf("- %s\n", advisory)
	}
	fmt.Println()
	return nil
}

func collectRunAdvisories(rc *RunContext) ([]string, error) {
	if rc == nil {
		return nil, nil
	}
	status, err := LoadRunStatusRecord(RunStatusPath(rc.RunDir))
	closeoutFacts, closeoutErr := BuildRunCloseoutFacts(rc.RunDir)
	if closeoutErr != nil {
		closeoutFacts = RunCloseoutFacts{}
	}
	summaryExists := closeoutFacts.SummaryExists
	completionExists := closeoutFacts.CompletionExists
	advisories := make([]string, 0, 2)
	if err == nil && status != nil && status.RequiredRemaining != nil && *status.RequiredRemaining == 0 && (!summaryExists || !completionExists) {
		advisories = append(advisories, fmt.Sprintf("Closeout artifacts missing: required_remaining=0 summary_exists=%t completion_proof_exists=%t", summaryExists, completionExists))
	} else if err != nil {
		return nil, err
	}
	statusComparison, err := BuildRunStatusComparison(rc.RunDir)
	if err != nil {
		return nil, err
	}
	if statusComparison != nil && statusComparison.StatusRequiredRemaining != nil && statusComparison.GoalRequiredRemaining != nil && !statusComparison.RequiredRemainingMatch {
		advisories = append(advisories, fmt.Sprintf("Status drift: status_required_remaining=%d goal_required_remaining=%d goal_remaining_ids=%s", *statusComparison.StatusRequiredRemaining, *statusComparison.GoalRequiredRemaining, strings.Join(statusComparison.GoalRemainingRequiredIDs, ",")))
	}
	if statusComparison != nil && statusComparison.StatusOpenRequiredIDsRecorded && !statusComparison.OpenRequiredIDsMatch {
		advisories = append(advisories, fmt.Sprintf("Status drift: status_open_required_ids=%s goal_remaining_ids=%s", strings.Join(statusComparison.StatusOpenRequiredIDs, ","), strings.Join(statusComparison.GoalRemainingRequiredIDs, ",")))
	}
	if statusComparison != nil && statusComparison.StatusActiveSessionsRecorded && !statusComparison.ActiveSessionsMatch {
		advisories = append(advisories, fmt.Sprintf("Status drift: status_active_sessions=%s runtime_active_sessions=%s", strings.Join(statusComparison.StatusActiveSessions, ","), strings.Join(statusComparison.RuntimeActiveSessions, ",")))
	}
	if objective := formatObjectiveIntegritySummary(rc.RunDir); objective != "" {
		if closeoutFacts.ObjectiveContractPresent && (!closeoutFacts.ObjectiveContractLocked || !closeoutFacts.ObjectiveIntegrityOK) {
			advisories = append(advisories, "Objective integrity pending: "+objective)
		}
	}
	activity, err := LoadActivitySnapshot(ActivityPath(rc.RunDir))
	if err != nil {
		return nil, err
	}
	if activity != nil {
		if activity.Budget.MaxDurationSeconds > 0 && activity.Budget.Exhausted && activity.Lifecycle.RunActive {
			advisories = append(advisories, "Budget exhausted: "+formatBudgetSummary(activity.Budget))
		}
		coverage := activity.Coverage
		if parts := requiredFrontierFactParts(coverage); coverage.RequiredPresent && len(parts) > 0 {
			reusable := append([]string{}, coverage.IdleReusableSessions...)
			reusable = append(reusable, coverage.ParkedReusableSessions...)
			if len(reusable) > 0 {
				parts = append(parts, "reusable_sessions="+strings.Join(reusable, ","))
			}
			advisories = append(advisories, "Required frontier facts: "+strings.Join(parts, " "))
		}
		if targetAttention := formatTargetAttentionAdvisory(activity.Attention); targetAttention != "" {
			advisories = append(advisories, targetAttention)
		}
		if frontier := formatRequiredFrontierAdvisory(rc.RunDir, coverage, activity.Attention); frontier != "" {
			advisories = append(advisories, frontier)
		}
		if operations := formatOperationAdvisory(activity.Operations); operations != "" {
			advisories = append(advisories, operations)
		}
	}
	evolveFacts, err := LoadCurrentEvolveFacts(rc.RunDir)
	if err != nil {
		return nil, err
	}
	if advisory := formatEvolveManagementAdvisory(evolveFacts, status); advisory != "" {
		advisories = append(advisories, advisory)
	}
	controlGapFacts, err := BuildControlGapFacts(rc.RunDir)
	if err != nil {
		return nil, err
	}
	advisories = append(advisories, formatControlGapAdvisories(controlGapFacts)...)
	qualityDebt, err := BuildQualityDebt(rc.RunDir)
	if err != nil {
		return nil, err
	}
	if advisory := formatQualityDebtAdvisory(qualityDebt); advisory != "" {
		advisories = append(advisories, advisory)
	}
	return advisories, nil
}

func formatQualityDebtAdvisory(debt *QualityDebt) string {
	if debt == nil || debt.Zero() {
		return ""
	}
	parts := make([]string, 0, 6)
	if len(debt.SuccessDimensionUnowned) > 0 {
		parts = append(parts, "success_dimension_unowned="+strings.Join(debt.SuccessDimensionUnowned, ","))
	}
	if len(debt.ProofPlanGap) > 0 {
		parts = append(parts, "proof_plan_gap="+strings.Join(debt.ProofPlanGap, ","))
	}
	if debt.CriticGateMissing {
		parts = append(parts, "critic_gate_missing")
	}
	if debt.FinisherGateMissing {
		parts = append(parts, "finisher_gate_missing")
	}
	if debt.OnlyCorrectnessEvidence {
		parts = append(parts, "only_correctness_evidence_present")
	}
	if debt.DomainPackMissing {
		parts = append(parts, "domain_pack_missing_for_nontrivial_run")
	}
	if len(parts) == 0 {
		return ""
	}
	return "Quality debt: " + strings.Join(parts, " ")
}

func formatControlGapAdvisories(facts *ControlGapFacts) []string {
	if facts == nil {
		return nil
	}
	advisories := make([]string, 0, 3)
	if facts.StatusDrift {
		parts := []string{"Control gap: status_drift"}
		if facts.StatusUpdatedAt != "" {
			parts = append(parts, "status_updated_at="+facts.StatusUpdatedAt)
		}
		advisories = append(advisories, strings.Join(parts, " "))
	}
	if facts.CoordinationStale {
		parts := []string{"Control gap: coordination_stale"}
		if facts.CoordinationUpdatedAt != "" {
			parts = append(parts, "coordination_updated_at="+facts.CoordinationUpdatedAt)
		}
		if facts.LatestControlChangeAt != "" {
			parts = append(parts, "latest_control_change_at="+facts.LatestControlChangeAt)
		}
		advisories = append(advisories, strings.Join(parts, " "))
	}
	if facts.SerializedRequiredFrontier {
		parts := []string{
			"Control gap: serialized_required_frontier",
			fmt.Sprintf("open_required_count=%d", facts.OpenRequiredCount),
			"active_required_owners=" + strings.Join(facts.ActiveRequiredOwners, ","),
		}
		if len(facts.ReusableSessions) > 0 {
			parts = append(parts, "reusable_sessions="+strings.Join(facts.ReusableSessions, ","))
		}
		advisories = append(advisories, strings.Join(parts, " "))
	}
	return advisories
}

func formatEvolveManagementAdvisory(facts *EvolveFacts, status *RunStatusRecord) string {
	if facts == nil || strings.TrimSpace(facts.ManagementGap) == "" {
		return ""
	}
	phase := ""
	if status != nil {
		phase = strings.TrimSpace(status.Phase)
	}
	parts := make([]string, 0, 6)
	switch facts.ManagementGap {
	case EvolveManagementGapMissingStopOrDispatch:
		parts = append(parts,
			"frontier_state="+blankAsUnknown(facts.FrontierState),
			fmt.Sprintf("open_candidate_count=%d", facts.OpenCandidateCount),
			fmt.Sprintf("active_sessions=%d", facts.ActiveSessionCount),
		)
		if facts.LastManagementEventAt != "" {
			parts = append(parts, "last_management_event_at="+facts.LastManagementEventAt)
		}
	case EvolveManagementGapReviewWithoutManagedStop:
		parts = append(parts,
			"frontier_state="+blankAsUnknown(facts.FrontierState),
			fmt.Sprintf("open_candidate_count=%d", facts.OpenCandidateCount),
			"phase="+blankAsUnknown(phase),
			fmt.Sprintf("active_sessions=%d", facts.ActiveSessionCount),
		)
	default:
		return ""
	}
	return facts.ManagementGap + ": " + strings.Join(parts, " ")
}

func formatCoverageSummary(coverage RequiredCoverage) string {
	if len(coverage.OpenRequiredIDs) == 0 &&
		len(coverage.MappedRequiredIDs) == 0 &&
		len(coverage.UnmappedRequiredIDs) == 0 &&
		len(coverage.SessionOwnerMissingIDs) == 0 &&
		len(coverage.MasterOwnedRequiredIDs) == 0 &&
		len(coverage.MasterOrphanedRequiredIDs) == 0 &&
		len(coverage.ProbingRequiredIDs) == 0 &&
		len(coverage.WaitingRequiredIDs) == 0 &&
		len(coverage.BlockedRequiredIDs) == 0 &&
		len(coverage.PrematureBlockedRequiredIDs) == 0 {
		return ""
	}
	parts := make([]string, 0, 14)
	if !coverage.RequiredPresent {
		parts = append(parts, "coverage=unknown")
	} else {
		parts = append(parts, "coverage=explicit")
	}
	appendCoverageSummaryPart(&parts, "open_required", coverage.OpenRequiredIDs)
	appendCoverageSummaryPart(&parts, "mapped_required", coverage.MappedRequiredIDs)
	appendCoverageSummaryPart(&parts, "unmapped_required", coverage.UnmappedRequiredIDs)
	appendCoverageSummaryPart(&parts, "session_owner_missing", coverage.SessionOwnerMissingIDs)
	appendCoverageSummaryPart(&parts, "master_owned", coverage.MasterOwnedRequiredIDs)
	appendCoverageSummaryPart(&parts, "master_orphaned", coverage.MasterOrphanedRequiredIDs)
	appendCoverageSummaryPart(&parts, "probing_required", coverage.ProbingRequiredIDs)
	appendCoverageSummaryPart(&parts, "waiting_required", coverage.WaitingRequiredIDs)
	appendCoverageSummaryPart(&parts, "blocked_required", coverage.BlockedRequiredIDs)
	appendCoverageSummaryPart(&parts, "premature_blocked", coverage.PrematureBlockedRequiredIDs)
	appendCoverageSummaryPart(&parts, "idle_reusable", coverage.IdleReusableSessions)
	appendCoverageSummaryPart(&parts, "parked_reusable", coverage.ParkedReusableSessions)
	return strings.Join(parts, " ")
}

func formatOperationSummary(operations map[string]ControlOperationTarget) string {
	if len(operations) == 0 {
		return ""
	}
	keys := make([]string, 0, len(operations))
	for key := range operations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+blankAsUnknown(operations[key].State))
	}
	return strings.Join(parts, " ")
}

func formatOperationAdvisory(operations map[string]ControlOperationTarget) string {
	if len(operations) == 0 {
		return ""
	}
	keys := make([]string, 0, len(operations))
	for key, op := range operations {
		if op.State == ControlOperationStateCommitted {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		op := operations[key]
		part := key + "=" + blankAsUnknown(op.State)
		if op.Summary != "" {
			part += " summary=" + op.Summary
		}
		parts = append(parts, part)
	}
	return "Operations: " + strings.Join(parts, " ")
}

func formatOperationDetailLine(key string, op ControlOperationTarget) string {
	parts := []string{key, "state=" + blankAsUnknown(op.State)}
	if op.Kind != "" {
		parts = append(parts, "kind="+op.Kind)
	}
	if op.Summary != "" {
		parts = append(parts, "summary="+op.Summary)
	}
	if len(op.PendingConditions) > 0 {
		parts = append(parts, "pending="+strings.Join(op.PendingConditions, ","))
	}
	if op.LastError != "" {
		parts = append(parts, "error="+op.LastError)
	}
	return strings.Join(parts, " ")
}

func formatBudgetSummary(budget ActivityBudget) string {
	if budget.MaxDurationSeconds <= 0 {
		return ""
	}
	parts := []string{
		"max_duration=" + formatBudgetDurationSeconds(budget.MaxDurationSeconds),
	}
	if budget.StartedAt != "" {
		parts = append(parts, "started_at="+budget.StartedAt)
	}
	if budget.DeadlineAt != "" {
		parts = append(parts, "deadline_at="+budget.DeadlineAt)
	}
	if budget.ElapsedSeconds > 0 {
		parts = append(parts, "elapsed="+formatBudgetDurationSeconds(budget.ElapsedSeconds))
	}
	switch {
	case budget.RemainingSeconds > 0:
		parts = append(parts, "remaining="+formatBudgetDurationSeconds(budget.RemainingSeconds))
	case budget.RemainingSeconds < 0:
		parts = append(parts, "overrun="+formatBudgetDurationSeconds(-budget.RemainingSeconds))
	}
	parts = append(parts, fmt.Sprintf("exhausted=%t", budget.Exhausted))
	return strings.Join(parts, " ")
}

func formatBudgetDurationSeconds(seconds int64) string {
	return (time.Duration(seconds) * time.Second).String()
}

func formatMemorySummary(runDir string) string {
	queryPresent := fileExists(MemoryQueryPath(runDir))
	contextPresent := fileExists(MemoryContextPath(runDir))
	if !queryPresent && !contextPresent {
		return ""
	}
	parts := []string{
		fmt.Sprintf("query_present=%t", queryPresent),
		fmt.Sprintf("context_present=%t", contextPresent),
	}
	if context, err := LoadMemoryContextFile(MemoryContextPath(runDir)); err == nil && context != nil && strings.TrimSpace(context.BuiltAt) != "" {
		parts = append(parts, "built_at="+context.BuiltAt)
	}
	return strings.Join(parts, " ")
}

func appendCoverageSummaryPart(parts *[]string, label string, values []string) {
	if len(values) == 0 {
		return
	}
	*parts = append(*parts, label+"="+strings.Join(values, ","))
}

func requiredFrontierFactParts(coverage RequiredCoverage) []string {
	parts := make([]string, 0, 8)
	appendCoverageSummaryPart(&parts, "unmapped_required", coverage.UnmappedRequiredIDs)
	appendCoverageSummaryPart(&parts, "session_owner_missing", coverage.SessionOwnerMissingIDs)
	appendCoverageSummaryPart(&parts, "master_orphaned", coverage.MasterOrphanedRequiredIDs)
	appendCoverageSummaryPart(&parts, "probing_required", coverage.ProbingRequiredIDs)
	appendCoverageSummaryPart(&parts, "waiting_required", coverage.WaitingRequiredIDs)
	appendCoverageSummaryPart(&parts, "blocked_required", coverage.BlockedRequiredIDs)
	appendCoverageSummaryPart(&parts, "premature_blocked", coverage.PrematureBlockedRequiredIDs)
	return parts
}

func formatRequiredFrontierAdvisory(runDir string, coverage RequiredCoverage, attention map[string]TargetAttentionFacts) string {
	if len(coverage.MappedRequiredIDs) == 0 {
		return ""
	}
	coord, err := LoadCoordinationState(CoordinationPath(runDir))
	if err != nil || coord == nil || len(coord.Required) == 0 {
		return ""
	}
	parts := make([]string, 0, len(coverage.MappedRequiredIDs))
	for _, id := range sortedRequiredFrontierDetailIDs(coverage) {
		required, ok := coord.Required[id]
		if !ok {
			continue
		}
		owner := strings.TrimSpace(required.Owner)
		if owner == "" {
			continue
		}
		attentionFacts, hasAttention := attention[owner]
		attentionState := strings.TrimSpace(attentionFacts.AttentionState)
		if !requiredFrontierDetailNeeded(id, required, coverage, hasAttention && targetAttentionNeedsAction(attentionFacts)) {
			continue
		}
		detail := id + " owner=" + owner + " execution_state=" + required.ExecutionState
		if blockedBy := strings.TrimSpace(required.BlockedBy); blockedBy != "" {
			detail += " blocked_by=" + blockedBy
		}
		if containsString(coverage.SessionOwnerMissingIDs, id) {
			detail += " owner_missing=true"
		}
		if containsString(coverage.MasterOrphanedRequiredIDs, id) {
			detail += " master_orphaned=true"
		}
		if containsString(coverage.PrematureBlockedRequiredIDs, id) {
			detail += " premature_blocked=true"
		}
		if hasAttention && targetAttentionNeedsAction(attentionFacts) && attentionState != "" {
			detail += " owner_attention=" + attentionState
		}
		parts = append(parts, detail)
	}
	if len(parts) == 0 {
		return ""
	}
	return "Required frontier: " + strings.Join(parts, " | ")
}

func sortedRequiredFrontierDetailIDs(coverage RequiredCoverage) []string {
	idSet := map[string]struct{}{}
	for _, ids := range [][]string{
		coverage.MappedRequiredIDs,
		coverage.SessionOwnerMissingIDs,
		coverage.MasterOrphanedRequiredIDs,
		coverage.ProbingRequiredIDs,
		coverage.WaitingRequiredIDs,
		coverage.BlockedRequiredIDs,
		coverage.PrematureBlockedRequiredIDs,
	} {
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			idSet[id] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func requiredFrontierDetailNeeded(id string, required CoordinationRequiredItem, coverage RequiredCoverage, ownerAttention bool) bool {
	switch required.ExecutionState {
	case coordinationRequiredExecutionStateProbing, coordinationRequiredExecutionStateWaiting, coordinationRequiredExecutionStateBlocked:
		return true
	}
	return containsString(coverage.SessionOwnerMissingIDs, id) ||
		containsString(coverage.MasterOrphanedRequiredIDs, id) ||
		containsString(coverage.PrematureBlockedRequiredIDs, id) ||
		ownerAttention
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func formatTargetAttentionAdvisory(attention map[string]TargetAttentionFacts) string {
	if len(attention) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attention))
	for key, facts := range attention {
		if targetAttentionNeedsAction(facts) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		facts := attention[key]
		part := key + ":" + facts.AttentionState
		if facts.Unread > 0 {
			part += fmt.Sprintf(" unread=%d", facts.Unread)
		}
		if facts.CursorLag > 0 {
			part += fmt.Sprintf(" cursor_lag=%d", facts.CursorLag)
		}
		if facts.JournalStaleMinutes > 0 {
			part += fmt.Sprintf(" journal_stale=%dm", facts.JournalStaleMinutes)
		}
		if facts.OutputStaleMinutes > 0 {
			part += fmt.Sprintf(" output_stale=%dm", facts.OutputStaleMinutes)
		}
		if facts.WorktreeStaleMinutes > 0 {
			part += fmt.Sprintf(" worktree_stale=%dm", facts.WorktreeStaleMinutes)
		}
		parts = append(parts, part)
	}
	return "Target attention: " + strings.Join(parts, " | ")
}

func experimentsLogFacts(path string) (int, string, error) {
	events, err := LoadDurableLog(path, DurableSurfaceExperiments)
	if err != nil {
		return 0, "", err
	}
	if len(events) == 0 {
		return 0, "", nil
	}
	return len(events), events[len(events)-1].At, nil
}

func fileExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
