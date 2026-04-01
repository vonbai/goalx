package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const contextUsage = "usage: goalx context [--run NAME] [--json]"

func Context(projectRoot string, args []string) error {
	if printUsageIfHelp(args, contextUsage) {
		return nil
	}
	runName, jsonOut, err := parseContextArgs(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if _, err := RefreshRunSuccessContextForRun(rc.ProjectRoot, rc.RunDir); err != nil {
		return err
	}
	if err := RefreshEvolveFacts(rc.RunDir); err != nil {
		return err
	}

	index, err := BuildContextIndex(projectRoot, rc.Name, rc.RunDir)
	if err != nil {
		return err
	}
	if jsonOut {
		data, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(renderContextIndex(index))
	return nil
}

func parseContextArgs(args []string) (runName string, jsonOut bool, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", false, err
	}
	positional := make([]string, 0, len(rest))
	for _, arg := range rest {
		switch arg {
		case "--json":
			jsonOut = true
		case "--help", "-h", "help":
			return "", false, fmt.Errorf(contextUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		return "", false, fmt.Errorf(contextUsage)
	}
	if len(positional) == 1 {
		if runName != "" {
			return "", false, fmt.Errorf(contextUsage)
		}
		runName = positional[0]
	}
	return runName, jsonOut, nil
}

func renderContextIndex(index *ContextIndex) string {
	if index == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# GoalX Context\n\n")
	writeContextLine := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		b.WriteString(fmt.Sprintf("- %s: `%s`\n", label, value))
	}
	writeContextLine("Project root", index.ProjectRoot)
	writeContextLine("Run dir", index.RunDir)
	writeContextLine("Run worktree", index.RunWorktree)
	writeContextLine("Control dir", index.ControlDir)
	writeContextLine("Reports dir", index.ReportsDir)
	writeContextLine("Charter", index.CharterPath)
	writeContextLine("Objective contract", index.ObjectiveContractPath)
	writeContextLine("Goal", index.GoalPath)
	writeContextLine("Status", index.StatusPath)
	writeContextLine("Experiment ledger", index.ExperimentsLogPath)
	writeContextLine("Integration state", index.IntegrationStatePath)
	writeContextLine("Evolve facts", index.EvolveFactsPath)
	writeContextLine("Acceptance", index.AcceptanceStatePath)
	writeContextLine("Closeout/evidence surface", index.CompletionProofPath)
	writeContextLine("Coordination", index.CoordinationPath)
	writeContextLine("Result", index.SummaryPath)
	writeContextLine("Activity", index.ActivityPath)
	writeContextLine("Worktree snapshot", index.WorktreeSnapshotPath)
	writeContextLine("Selection snapshot", index.SelectionSnapshotPath)
	writeContextLine("Memory query", index.MemoryQueryPath)
	writeContextLine("Memory context", index.MemoryContextPath)
	writeContextLine("Intake", index.IntakePath)
	writeContextLine("Compiler input", index.CompilerInputPath)
	writeContextLine("Compiler report", index.CompilerReportPath)
	writeContextLine("Context index", index.ContextIndexPath)
	writeContextLine("Affordances", index.AffordancesMarkdown)
	if identity := index.RunIdentity; identity.RunID != "" || identity.Objective != "" {
		b.WriteString("\n## Run Identity\n\n")
		writeContextLine("Objective", identity.Objective)
		writeContextLine("Run ID", identity.RunID)
		writeContextLine("Root run ID", identity.RootRunID)
		writeContextLine("Charter ID", identity.CharterID)
		writeContextLine("Intent", identity.Intent)
		writeContextLine("Mode", identity.Mode)
		writeContextLine("Phase kind", identity.PhaseKind)
		if identity.RoleContracts.Master != nil {
			writeContextLine("Master role", identity.RoleContracts.Master.Mandate)
		}
		if identity.RoleContracts.Worker != nil {
			writeContextLine("Worker role", identity.RoleContracts.Worker.Mandate)
		}
	}
	if len(index.TargetFiles) > 0 || len(index.ReadonlyPaths) > 0 {
		b.WriteString("\n## Run Boundary\n\n")
		if len(index.TargetFiles) > 0 {
			writeContextLine("Target files", strings.Join(index.TargetFiles, ", "))
		}
		if len(index.ReadonlyPaths) > 0 {
			writeContextLine("Readonly paths", strings.Join(index.ReadonlyPaths, ", "))
		}
	}
	if len(index.ContextFiles) > 0 || len(index.ContextRefs) > 0 {
		b.WriteString("\n## Declared Context\n\n")
		if len(index.ContextFiles) > 0 {
			b.WriteString("- Context files: " + formatContextValues(index.ContextFiles) + "\n")
		}
		if len(index.ContextRefs) > 0 {
			b.WriteString("- Context refs: " + formatContextValues(index.ContextRefs) + "\n")
		}
	}
	if index.GoalBoundary != nil {
		b.WriteString("\n## Goal Boundary\n\n")
		writeContextLine("Required items", fmt.Sprintf("%d", index.GoalBoundary.RequiredCount))
		writeContextLine("Optional items", fmt.Sprintf("%d", index.GoalBoundary.OptionalCount))
		writeContextLine("Required by source", formatContextCountMap(index.GoalBoundary.RequiredBySource))
		writeContextLine("Required by role", formatContextCountMap(index.GoalBoundary.RequiredByRole))
		writeContextLine("Required by state", formatContextCountMap(index.GoalBoundary.RequiredByState))
	}
	if index.RunStatus != nil {
		b.WriteString("\n## Run Status\n\n")
		writeContextLine("Phase", index.RunStatus.Phase)
		writeContextLine("Required remaining (status)", fmt.Sprintf("%d", index.RunStatus.RequiredRemaining))
		writeContextLine("Required remaining (goal)", fmt.Sprintf("%d", index.RunStatus.GoalRequiredRemaining))
		writeContextLine("Required remaining match", fmt.Sprintf("%t", index.RunStatus.RequiredRemainingMatch))
		writeContextLine("Status open required IDs recorded", fmt.Sprintf("%t", index.RunStatus.StatusOpenRequiredIDsRecorded))
		if index.RunStatus.StatusOpenRequiredIDsRecorded {
			writeContextLine("Status open required IDs", strings.Join(index.RunStatus.StatusOpenRequiredIDs, ", "))
			writeContextLine("Open required IDs match", fmt.Sprintf("%t", index.RunStatus.OpenRequiredIDsMatch))
		}
		writeContextLine("Goal remaining IDs", strings.Join(index.RunStatus.GoalRemainingRequiredIDs, ", "))
		writeContextLine("Last verified at", index.RunStatus.LastVerifiedAt)
	}
	if index.Budget != nil {
		b.WriteString("\n## Budget\n\n")
		writeContextLine("Summary", formatBudgetSummary(*index.Budget))
	}
	if index.Acceptance != nil {
		b.WriteString("\n## Acceptance\n\n")
		writeContextLine("Active checks", fmt.Sprintf("%d", index.Acceptance.ActiveCheckCount))
		writeContextLine("Last checked at", index.Acceptance.LastCheckedAt)
		if index.Acceptance.LastExitCode != nil {
			writeContextLine("Last exit code", fmt.Sprintf("%d", *index.Acceptance.LastExitCode))
		}
		writeContextLine("Evidence path", index.Acceptance.EvidencePath)
	}
	if index.QualityDebt != nil {
		b.WriteString("\n## Quality Debt\n\n")
		writeContextLine("Zero debt", fmt.Sprintf("%t", index.QualityDebt.Zero))
		writeContextLine("Success dimensions unowned", strings.Join(index.QualityDebt.SuccessDimensionUnowned, ", "))
		writeContextLine("Proof plan gaps", strings.Join(index.QualityDebt.ProofPlanGap, ", "))
		writeContextLine("Critic gate missing", fmt.Sprintf("%t", index.QualityDebt.CriticGateMissing))
		writeContextLine("Finisher gate missing", fmt.Sprintf("%t", index.QualityDebt.FinisherGateMissing))
		writeContextLine("Only correctness evidence present", fmt.Sprintf("%t", index.QualityDebt.OnlyCorrectnessEvidence))
		writeContextLine("Domain pack missing", fmt.Sprintf("%t", index.QualityDebt.DomainPackMissing))
	}
	if index.Closeout != nil {
		b.WriteString("\n## Closeout\n\n")
		writeContextLine("Summary exists", fmt.Sprintf("%t", index.Closeout.SummaryExists))
		writeContextLine("Completion proof exists", fmt.Sprintf("%t", index.Closeout.CompletionProofExists))
		writeContextLine("Ready to finalize", fmt.Sprintf("%t", index.Closeout.ReadyToFinalize))
	}
	if index.ObjectiveIntegrity != nil {
		b.WriteString("\n## Objective Contract\n\n")
		writeContextLine("State", index.ObjectiveIntegrity.ContractState)
		writeContextLine("Locked", fmt.Sprintf("%t", index.ObjectiveIntegrity.ContractLocked))
		writeContextLine("Clauses", fmt.Sprintf("%d", index.ObjectiveIntegrity.ClauseCount))
		writeContextLine("Goal clause coverage", fmt.Sprintf("%d/%d", index.ObjectiveIntegrity.GoalCoveredCount, index.ObjectiveIntegrity.GoalClauseCount))
		writeContextLine("Acceptance clause coverage", fmt.Sprintf("%d/%d", index.ObjectiveIntegrity.AcceptanceCoveredCount, index.ObjectiveIntegrity.AcceptanceClauseCount))
		writeContextLine("Integrity ready", fmt.Sprintf("%t", index.ObjectiveIntegrity.IntegrityReady))
		writeContextLine("Integrity OK", fmt.Sprintf("%t", index.ObjectiveIntegrity.IntegrityOK))
		if len(index.ObjectiveIntegrity.MissingGoalClauseIDs) > 0 {
			writeContextLine("Missing goal clauses", strings.Join(index.ObjectiveIntegrity.MissingGoalClauseIDs, ", "))
		}
		if len(index.ObjectiveIntegrity.MissingAcceptanceClauseIDs) > 0 {
			writeContextLine("Missing acceptance clauses", strings.Join(index.ObjectiveIntegrity.MissingAcceptanceClauseIDs, ", "))
		}
	}
	if index.EvolveFactsPath != "" || index.Evolve != nil {
		b.WriteString("\n## Evolve\n\n")
		writeContextLine("Evolve facts", index.EvolveFactsPath)
		if index.Evolve != nil {
			writeContextLine("Frontier state", index.Evolve.FrontierState)
			writeContextLine("Best experiment", index.Evolve.BestExperimentID)
			writeContextLine("Open candidate count", fmt.Sprintf("%d", index.Evolve.OpenCandidateCount))
			writeContextLine("Open candidate IDs", strings.Join(index.Evolve.OpenCandidateIDs, ", "))
			writeContextLine("Last stop reason", index.Evolve.LastStopReasonCode)
			writeContextLine("Last management event", index.Evolve.LastManagementEventAt)
		}
	}
	if len(index.ProviderRuntimeFacts) > 0 {
		b.WriteString("\n## Provider Runtime\n\n")
		for _, fact := range index.ProviderRuntimeFacts {
			target := fact.Target
			if strings.TrimSpace(target) == "" {
				target = "run"
			}
			label := target
			if fact.Engine != "" {
				label += " (" + fact.Engine + ")"
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", label, fact.Fact))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Master\n\n")
	writeContextLine("Engine", index.Master.Engine)
	writeContextLine("Model", index.Master.Model)
	writeContextLine("Mode", index.Master.Mode)
	writeContextLine("Requested effort", string(index.Master.RequestedEffort))
	writeContextLine("Effective effort", index.Master.EffectiveEffort)
	writeContextLine("Surface", index.Master.SurfaceKind)
	writeContextLine("Worktree kind", index.Master.WorktreeKind)
	writeContextLine("Mergeable output surface", fmt.Sprintf("%t", index.Master.MergeableOutputSurface))
	if index.Master.ProviderBootstrap != nil {
		writeContextLine("Permission mode", index.Master.ProviderBootstrap.PermissionMode)
		writeContextLine("Provider bootstrap verified", fmt.Sprintf("%t", index.Master.ProviderBootstrap.BootstrapVerified))
		writeContextLine("permission_request_hook_bootstrapped", fmt.Sprintf("%t", index.Master.ProviderBootstrap.PermissionRequestHookBootstrapped))
		writeContextLine("elicitation_hook_bootstrapped", fmt.Sprintf("%t", index.Master.ProviderBootstrap.ElicitationHookBootstrapped))
		writeContextLine("notification_hook_bootstrapped", fmt.Sprintf("%t", index.Master.ProviderBootstrap.NotificationHookBootstrapped))
	}
	if index.Selection != nil {
		b.WriteString("\n## Selection\n\n")
		if len(index.Selection.MasterCandidates) > 0 {
			writeContextLine("Master candidates", strings.Join(index.Selection.MasterCandidates, ", "))
		}
		if len(index.Selection.WorkerCandidates) > 0 {
			writeContextLine("Worker candidates", strings.Join(index.Selection.WorkerCandidates, ", "))
		}
		if len(index.Selection.DisabledEngines) > 0 {
			writeContextLine("Disabled engines", strings.Join(index.Selection.DisabledEngines, ", "))
		}
		if len(index.Selection.DisabledTargets) > 0 {
			writeContextLine("Disabled targets", strings.Join(index.Selection.DisabledTargets, ", "))
		}
	}
	if index.ProtocolComposition != nil {
		b.WriteString("\n## Protocol Composition\n\n")
		writeContextLine("Philosophy", strings.Join(index.ProtocolComposition.Philosophy, ", "))
		writeContextLine("Behavior contract", strings.Join(index.ProtocolComposition.BehaviorContract, ", "))
		writeContextLine("Required roles", strings.Join(index.ProtocolComposition.RequiredRoles, ", "))
		writeContextLine("Required gates", strings.Join(index.ProtocolComposition.RequiredGates, ", "))
		writeContextLine("Required proof kinds", strings.Join(index.ProtocolComposition.RequiredProofKinds, ", "))
		writeContextLine("Selected prior refs", strings.Join(index.ProtocolComposition.SelectedPriorRefs, ", "))
	}
	if index.ClaudeCodeAvailable || index.CodexAvailable || index.GitAvailable || index.TmuxAvailable {
		b.WriteString("## Capabilities\n\n")
		if index.ClaudeCodeAvailable {
			b.WriteString("- claude-code available\n")
		}
		if index.CodexAvailable {
			b.WriteString("- codex available\n")
		}
		if index.GitAvailable {
			b.WriteString("- git available\n")
		}
		if index.TmuxAvailable {
			b.WriteString("- tmux available\n")
		}
		b.WriteString("\n")
	}
	if len(index.Sessions) > 0 {
		b.WriteString("## Sessions\n\n")
		for _, sess := range index.Sessions {
			b.WriteString(fmt.Sprintf("- %s", sess.Name))
			if sess.Mode != "" {
				b.WriteString(fmt.Sprintf(" (%s)", sess.Mode))
			}
			b.WriteString("\n")
			writeIndentedContextLine(&b, "role", sess.RoleKind)
			writeIndentedContextLine(&b, "engine", sess.Engine)
			writeIndentedContextLine(&b, "model", sess.Model)
			writeIndentedContextLine(&b, "requested effort", string(sess.RequestedEffort))
			writeIndentedContextLine(&b, "effective effort", sess.EffectiveEffort)
			writeIndentedContextLine(&b, "surface", sess.SurfaceKind)
			writeIndentedContextLine(&b, "worktree kind", sess.WorktreeKind)
			writeIndentedContextLine(&b, "mergeable output", fmt.Sprintf("%t", sess.MergeableOutputSurface))
			if sess.ProviderBootstrap != nil {
				writeIndentedContextLine(&b, "permission mode", sess.ProviderBootstrap.PermissionMode)
				writeIndentedContextLine(&b, "provider bootstrap verified", fmt.Sprintf("%t", sess.ProviderBootstrap.BootstrapVerified))
				writeIndentedContextLine(&b, "permission_request_hook_bootstrapped", fmt.Sprintf("%t", sess.ProviderBootstrap.PermissionRequestHookBootstrapped))
				writeIndentedContextLine(&b, "elicitation_hook_bootstrapped", fmt.Sprintf("%t", sess.ProviderBootstrap.ElicitationHookBootstrapped))
				writeIndentedContextLine(&b, "notification_hook_bootstrapped", fmt.Sprintf("%t", sess.ProviderBootstrap.NotificationHookBootstrapped))
			}
			writeIndentedContextLine(&b, "window", sess.WindowName)
			writeIndentedContextLine(&b, "worktree", sess.WorktreePath)
			writeIndentedContextLine(&b, "branch", sess.Branch)
			writeIndentedContextLine(&b, "base selector", sess.BaseBranchSelector)
			writeIndentedContextLine(&b, "base branch", sess.BaseBranch)
			writeIndentedContextLine(&b, "target files", strings.Join(sess.TargetFiles, ", "))
			writeIndentedContextLine(&b, "readonly paths", strings.Join(sess.ReadonlyPaths, ", "))
			writeIndentedContextLine(&b, "journal", sess.JournalPath)
			writeIndentedContextLine(&b, "inbox", sess.InboxPath)
			writeIndentedContextLine(&b, "cursor", sess.CursorPath)
		}
	}
	return b.String()
}

func formatContextValues(values []string) string {
	wrapped := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		wrapped = append(wrapped, "`"+value+"`")
	}
	return strings.Join(wrapped, ", ")
}

func writeIndentedContextLine(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString(fmt.Sprintf("  - %s: `%s`\n", label, value))
}

func formatContextCountMap(counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, counts[key]))
	}
	return strings.Join(parts, ", ")
}
