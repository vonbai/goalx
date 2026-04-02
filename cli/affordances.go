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

type AffordancesDocument struct {
	Version    int              `json:"version"`
	CheckedAt  string           `json:"checked_at,omitempty"`
	RunName    string           `json:"run_name,omitempty"`
	Target     string           `json:"target,omitempty"`
	RunDir     string           `json:"run_dir,omitempty"`
	ControlDir string           `json:"control_dir,omitempty"`
	Items      []AffordanceItem `json:"items"`
}

type AffordanceItem struct {
	ID      string   `json:"id,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Command string   `json:"command,omitempty"`
	Facts   []string `json:"facts,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

func AffordancesJSONPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "affordances.json")
}

func AffordancesMarkdownPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "affordances.md")
}

func LoadAffordances(path string) (*AffordancesDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	doc := &AffordancesDocument{}
	if len(strings.TrimSpace(string(data))) == 0 {
		doc.Version = 1
		return doc, nil
	}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, err
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	return doc, nil
}

func SaveAffordances(runDir string, doc *AffordancesDocument) error {
	if doc == nil {
		return nil
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	if doc.CheckedAt == "" {
		doc.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := writeJSONFile(AffordancesJSONPath(runDir), doc); err != nil {
		return err
	}
	return writeFileAtomic(AffordancesMarkdownPath(runDir), []byte(RenderAffordancesMarkdown(doc)), 0o644)
}

func BuildAffordances(projectRoot, runName, runDir, target string) (*AffordancesDocument, error) {
	index, err := BuildContextIndex(projectRoot, runName, runDir)
	if err != nil {
		return nil, err
	}
	normalizedTarget := normalizedAffordanceTarget(target)
	compilerDoctrinePaths := []string{SuccessModelPath(runDir), ProofPlanPath(runDir), WorkflowPlanPath(runDir), DomainPackPath(runDir), CompilerInputPath(runDir), CompilerReportPath(runDir)}
	if fileExists(IntakePath(runDir)) {
		compilerDoctrinePaths = append(compilerDoctrinePaths, IntakePath(runDir))
	}
	doc := &AffordancesDocument{
		Version:    1,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		RunName:    runName,
		Target:     normalizedTarget,
		RunDir:     runDir,
		ControlDir: ControlDir(runDir),
	}
	doc.Items = []AffordanceItem{
		{
			ID:      "status",
			Kind:    "observe",
			Summary: "Read the current run progress and control summary.",
			Command: fmt.Sprintf("goalx status --run %s", runName),
			Paths:   []string{ActivityPath(runDir), TransportFactsPath(runDir)},
		},
		{
			ID:      "observe",
			Kind:    "observe",
			Summary: "Read the live transport capture plus current run facts.",
			Command: fmt.Sprintf("goalx observe --run %s", runName),
			Paths:   []string{ActivityPath(runDir), TransportFactsPath(runDir)},
		},
		{
			ID:      "budget",
			Kind:    "control",
			Summary: "Read the current run budget boundary or adjust it through the canonical budget control surface.",
			Command: fmt.Sprintf("goalx budget --run %s", runName),
			Paths:   []string{RunSpecPath(runDir), ActivityPath(runDir)},
		},
		{
			ID:      "context",
			Kind:    "context",
			Summary: "Read the structural context index for this run.",
			Command: fmt.Sprintf("goalx context --run %s", runName),
			Paths:   []string{ContextIndexPath(runDir), CompilerInputPath(runDir), CompilerReportPath(runDir)},
		},
		{
			ID:      "compiler-doctrine",
			Kind:    "fact",
			Summary: "Read the compiler-composed doctrine currently shaping protocol behavior.",
			Facts:   buildCompilerDoctrineFacts(index),
			Paths:   compilerDoctrinePaths,
		},
		{
			ID:      "afford",
			Kind:    "context",
			Summary: "Read the GoalX command and path affordances for this run.",
			Command: buildAffordanceCommand(runName, normalizedTarget),
			Paths:   []string{AffordancesJSONPath(runDir), AffordancesMarkdownPath(runDir)},
		},
		{
			ID:      "verify",
			Kind:    "control",
			Summary: "Run the active assurance scenarios and refresh recorded evidence before review or closeout.",
			Command: fmt.Sprintf("goalx verify --run %s", runName),
			Paths:   []string{AssurancePlanPath(runDir), EvidenceLogPath(runDir), RunStatusPath(runDir)},
		},
		{
			ID:      "closeout",
			Kind:    "fact",
			Summary: "Status, assurance, and closeout surfaces for final review and run completion.",
			Facts:   buildCloseoutAffordanceFacts(index),
			Paths:   []string{AssurancePlanPath(runDir), EvidenceLogPath(runDir), RunStatusPath(runDir), SummaryPath(runDir), CompletionStatePath(runDir)},
		},
		{
			ID:      "tell",
			Kind:    "control",
			Summary: "Dispatch or redirect durable session work through the control plane.",
			Command: buildTellCommand(runName, normalizedTarget),
			Paths:   []string{ControlInboxDir(runDir)},
		},
		{
			ID:      "durable-write-state",
			Kind:    "control",
			Summary: "Write a machine-consumed structured durable surface through the authoring plane. Inspect the contract with `goalx schema <surface>` first.",
			Command: fmt.Sprintf("goalx durable write status --run %s --body-file /abs/path.json", runName),
			Paths:   []string{ObligationModelPath(runDir), AssurancePlanPath(runDir), CoordinationPath(runDir), RunStatusPath(runDir)},
		},
		{
			ID:      "durable-write-event",
			Kind:    "control",
			Summary: "Write a machine-consumed durable event through the authoring plane. Inspect the contract with `goalx schema <surface>` first.",
			Command: fmt.Sprintf("goalx durable write obligation-log --run %s --kind decision --actor master --body-file /abs/path.json", runName),
			Paths:   []string{ObligationLogPath(runDir), ExperimentsLogPath(runDir)},
		},
		{
			ID:      "schema",
			Kind:    "context",
			Summary: "Read the canonical contract for a durable surface before writing it.",
			Command: "goalx schema status",
		},
		{
			ID:      "attach",
			Kind:    "control",
			Summary: "Attach to a tmux window for inspection or emergency manual intervention.",
			Command: buildAttachCommand(runName, normalizedTarget),
		},
		{
			ID:      "add-worker",
			Kind:    "control",
			Summary: "Launch a worker using the current selection policy.",
			Command: fmt.Sprintf(`goalx add --run %s --effort high --worktree "sub-goal"`, runName),
		},
		{
			ID:      "add-override",
			Kind:    "control",
			Summary: "Launch an explicit engine/model override worker that bypasses the current selection policy.",
			Command: fmt.Sprintf(`goalx add --run %s --engine ENGINE --model MODEL --effort LEVEL --worktree "sub-goal"`, runName),
		},
		{
			ID:      "replace",
			Kind:    "control",
			Summary: "Replace a stale or unsuitable durable worker.",
			Command: fmt.Sprintf("goalx replace --run %s session-N --effort high", runName),
		},
		{
			ID:      "keep-session",
			Kind:    "control",
			Summary: "Merge a reviewed worker session branch into the run worktree only. goalx keep only merges committed session branch history relative to that session's recorded parent/base ref; dirty uncommitted work must be committed first. If you need partial adoption or conflict resolution, inspect the session worktree and integrate manually. This does not merge into the source root yet.",
			Command: fmt.Sprintf("goalx keep --run %s session-N", runName),
		},
		{
			ID:      "integrate",
			Kind:    "control",
			Summary: "Record the lineage of a manual run-root integration after master has already merged, cherry-picked, or partially adopted work in the run-root worktree. goalx integrate requires a clean run-root worktree and does not perform the merge itself.",
			Command: fmt.Sprintf("goalx integrate --run %s --method partial_adopt --from session-1,session-2", runName),
		},
		{
			ID:      "keep-run",
			Kind:    "control",
			Summary: "Merge the run worktree into the source root when source HEAD still descends from the run base revision; skips if already integrated. This is distinct from goalx keep session-N, which only merges a committed session branch into the run worktree.",
			Command: fmt.Sprintf("goalx keep --run %s", runName),
		},
	}
	if index != nil && strings.TrimSpace(index.RunIdentity.Intent) == runIntentEvolve {
		doc.Items = append(doc.Items,
			AffordanceItem{
				ID:      "diff-experiments",
				Kind:    "control",
				Summary: "Compare two session branches or experiment paths before keeping or partially adopting one.",
				Command: fmt.Sprintf("goalx diff --run %s session-1 session-2", runName),
			},
			AffordanceItem{
				ID:      "fork-experiment",
				Kind:    "control",
				Summary: "Fork a follow-on dedicated worktree from an existing session branch to continue or compete on a concrete direction.",
				Command: fmt.Sprintf(`goalx add --run %s --worktree --base-branch session-N "follow-on direction"`, runName),
			},
			AffordanceItem{
				ID:      "record-experiment-closed",
				Kind:    "control",
				Summary: "Append an `experiment.closed` event after master explicitly rejects, abandons, or supersedes a path.",
				Command: fmt.Sprintf("goalx durable write experiments --run %s --kind experiment.closed --actor master --body-file /abs/path.experiment-closed.json", runName),
				Paths:   []string{ExperimentsLogPath(runDir)},
			},
			AffordanceItem{
				ID:      "record-evolve-stop",
				Kind:    "control",
				Summary: "Append an `evolve.stopped` event when master intentionally closes the current frontier.",
				Command: fmt.Sprintf("goalx durable write experiments --run %s --kind evolve.stopped --actor master --body-file /abs/path.evolve-stopped.json", runName),
				Paths:   []string{ExperimentsLogPath(runDir)},
			},
		)
	}
	if index != nil {
		if items := buildCognitionAffordances(index, normalizedTarget); len(items) > 0 {
			doc.Items = append(doc.Items, items...)
		}
		if facts, err := experimentAffordanceFacts(runDir); err != nil {
			return nil, err
		} else if len(facts) > 0 {
			doc.Items = append(doc.Items, AffordanceItem{
				ID:      "experiments",
				Kind:    "fact",
				Summary: "Canonical experiment lineage surfaces for the current integrated result and recorded experiment history.",
				Facts:   facts,
				Paths:   []string{index.ExperimentsLogPath, index.IntegrationStatePath},
			})
		}
		if item := buildSelectionFactsAffordance(index); item != nil {
			doc.Items = append(doc.Items, *item)
		}
		if item := buildResourceStateAffordance(index); item != nil {
			doc.Items = append(doc.Items, *item)
		}
		if item := buildEvolveFactsAffordance(index); item != nil {
			doc.Items = append(doc.Items, *item)
		}
		if item := buildWorktreeBoundaryAffordance(index, normalizedTarget); item != nil {
			doc.Items = append(doc.Items, *item)
		}
		if facts := providerRuntimeFactsForTarget(index.ProviderRuntimeFacts, normalizedTarget); len(facts) > 0 {
			doc.Items = append(doc.Items, AffordanceItem{
				ID:      "provider-runtime",
				Kind:    "fact",
				Summary: providerRuntimeFactsSummary(normalizedTarget, facts),
				Facts:   renderProviderRuntimeFactLines(normalizedTarget, facts),
				Paths:   []string{ContextIndexPath(runDir)},
			})
		}
		doc.Items = append(doc.Items, AffordanceItem{
			ID:      "paths",
			Kind:    "path",
			Summary: "Absolute run paths for durable state and reports.",
			Command: "",
			Paths:   dedupeStrings([]string{index.RunDir, index.ControlDir, index.CharterPath, index.ObligationModelPath, index.ObligationLogPath, index.AssurancePlanPath, index.EvidenceLogPath, index.StatusPath, index.ResourceStatePath, index.SummaryPath, index.CompletionProofPath, index.ExperimentsLogPath, index.IntegrationStatePath, index.EvolveFactsPath}),
		})
	}
	return doc, nil
}

func buildCloseoutAffordanceFacts(index *ContextIndex) []string {
	if index == nil {
		return nil
	}
	facts := make([]string, 0, 8)
	if index.RunStatus != nil {
		facts = append(facts,
			fmt.Sprintf("status.phase=`%s`.", blankAsUnknown(index.RunStatus.Phase)),
			fmt.Sprintf("status.required_remaining=`%d`.", index.RunStatus.RequiredRemaining),
			fmt.Sprintf("obligation.required_remaining=`%d`.", index.RunStatus.GoalRequiredRemaining),
			fmt.Sprintf("required_remaining_match=`%t`.", index.RunStatus.RequiredRemainingMatch),
			fmt.Sprintf("status_open_required_ids_recorded=`%t`.", index.RunStatus.StatusOpenRequiredIDsRecorded),
		)
		if index.RunStatus.StatusOpenRequiredIDsRecorded {
			facts = append(facts,
				fmt.Sprintf("status.remaining_ids=`%s`.", strings.Join(index.RunStatus.StatusOpenRequiredIDs, ",")),
				fmt.Sprintf("open_required_ids_match=`%t`.", index.RunStatus.OpenRequiredIDsMatch),
			)
		}
		if len(index.RunStatus.GoalRemainingRequiredIDs) > 0 {
			facts = append(facts, fmt.Sprintf("obligation.remaining_ids=`%s`.", strings.Join(index.RunStatus.GoalRemainingRequiredIDs, ",")))
		}
	}
	if index.Acceptance != nil {
		facts = append(facts, fmt.Sprintf("assurance.active_scenarios=`%d`.", index.Acceptance.ActiveCheckCount))
		if index.Acceptance.LastExitCode != nil {
			facts = append(facts, fmt.Sprintf("assurance.last_exit_code=`%d`.", *index.Acceptance.LastExitCode))
		}
		if index.Acceptance.LastCheckedAt != "" {
			facts = append(facts, fmt.Sprintf("assurance.last_checked_at=`%s`.", index.Acceptance.LastCheckedAt))
		}
	}
	if index.Closeout != nil {
		facts = append(facts,
			fmt.Sprintf("summary_exists=`%t`.", index.Closeout.SummaryExists),
			fmt.Sprintf("completion_proof_exists=`%t`.", index.Closeout.CompletionProofExists),
			fmt.Sprintf("ready_to_finalize=`%t`.", index.Closeout.ReadyToFinalize),
		)
	}
	if len(facts) == 0 {
		return nil
	}
	return facts
}

func buildCognitionAffordances(index *ContextIndex, target string) []AffordanceItem {
	if index == nil || len(index.CognitionScopes) == 0 {
		return nil
	}
	items := []AffordanceItem{}
	item := AffordanceItem{
		ID:      "cognition",
		Kind:    "fact",
		Summary: "Available code cognition providers and current graph freshness facts for this run scope.",
		Paths:   []string{index.ContextIndexPath},
	}
	for _, scope := range index.CognitionScopes {
		for _, provider := range scope.Providers {
			fact := fmt.Sprintf("scope=%s provider=%s invocation=%s available=%t", scope.Scope, provider.Name, provider.InvocationKind, provider.Available)
			if provider.Version != "" {
				fact += " version=" + provider.Version
			}
			if provider.IndexState != "" {
				fact += " index_state=" + provider.IndexState
			}
			if len(provider.ReadTransportsSupported) > 0 {
				fact += " read_transports=" + strings.Join(provider.ReadTransportsSupported, ",")
			}
			item.Facts = append(item.Facts, fact)
		}
	}
	items = append(items, item)

	scope := cognitionScopeForTarget(index.CognitionScopes, target)
	if scope == nil {
		return items
	}
	for _, provider := range scope.Providers {
		if provider.Name != "gitnexus" || !provider.Available {
			continue
		}
		if commands := buildGitNexusAffordanceItems(*scope, provider); len(commands) > 0 {
			items = append(items, commands...)
		}
		if mcp := buildGitNexusMCPAffordanceItem(provider); mcp != nil {
			items = append(items, *mcp)
		}
	}
	return items
}

func cognitionScopeForTarget(scopes []CognitionScopeState, target string) *CognitionScopeState {
	scopeName := "run-root"
	allowRunRootFallback := false
	switch normalized := normalizedAffordanceTarget(target); {
	case normalized == "":
		scopeName = "run-root"
	case normalized == "master":
		scopeName = "run-root"
	case strings.HasPrefix(normalized, "session-"):
		scopeName = normalized
		allowRunRootFallback = true
	}
	for i := range scopes {
		if scopes[i].Scope == scopeName {
			return &scopes[i]
		}
	}
	if allowRunRootFallback {
		for i := range scopes {
			if scopes[i].Scope == "run-root" {
				return &scopes[i]
			}
		}
	}
	return nil
}

func buildGitNexusAffordanceItems(scope CognitionScopeState, provider CognitionProviderState) []AffordanceItem {
	worktreePath := strings.TrimSpace(scope.WorktreePath)
	commandPrefix := strings.TrimSpace(provider.Command)
	if worktreePath == "" || commandPrefix == "" {
		return nil
	}
	cwdPrefix := "cd " + shellQuote(worktreePath) + " && "
	items := []AffordanceItem{
		{
			ID:      "cognition-status",
			Kind:    "context",
			Summary: "Read GitNexus index freshness for the selected cognition scope.",
			Command: cwdPrefix + commandPrefix + " status",
			Paths:   []string{worktreePath},
		},
	}
	if provider.IndexState != "fresh" {
		return items
	}
	items = append(items,
		AffordanceItem{
			ID:      "cognition-query",
			Kind:    "context",
			Summary: "Use GitNexus process-grouped search for architecture or feature lookup in the selected scope.",
			Command: cwdPrefix + commandPrefix + ` query "concept"`,
			Paths:   []string{worktreePath},
		},
		AffordanceItem{
			ID:      "cognition-context",
			Kind:    "context",
			Summary: "Use GitNexus symbol context for callers, callees, and process participation in the selected scope.",
			Command: cwdPrefix + commandPrefix + " context symbolName",
			Paths:   []string{worktreePath},
		},
		AffordanceItem{
			ID:      "cognition-impact",
			Kind:    "context",
			Summary: "Use GitNexus blast-radius analysis before editing a symbol in the selected scope.",
			Command: cwdPrefix + commandPrefix + " impact symbolName --direction upstream",
			Paths:   []string{worktreePath},
		},
	)
	return items
}

func buildGitNexusMCPAffordanceItem(provider CognitionProviderState) *AffordanceItem {
	if len(provider.ReadTransportsSupported) == 0 || !slices.Contains(provider.ReadTransportsSupported, "mcp") {
		return nil
	}
	item := &AffordanceItem{
		ID:      "cognition-mcp",
		Kind:    "fact",
		Summary: "GitNexus MCP read capabilities available to runtimes that already support this provider.",
		Facts:   []string{},
	}
	if provider.MCPServerCommand != "" {
		item.Facts = append(item.Facts, "mcp_server_command="+provider.MCPServerCommand)
	}
	for _, tool := range provider.MCPToolsSupported {
		item.Facts = append(item.Facts, "mcp_tool="+tool)
	}
	for _, resource := range provider.MCPResourcesSupported {
		item.Facts = append(item.Facts, "mcp_resource="+resource)
	}
	if len(item.Facts) == 0 {
		return nil
	}
	return item
}

func buildCompilerDoctrineFacts(index *ContextIndex) []string {
	if index == nil || index.ProtocolComposition == nil {
		return nil
	}
	facts := []string{}
	if len(index.ProtocolComposition.Philosophy) > 0 {
		facts = append(facts, "philosophy="+strings.Join(index.ProtocolComposition.Philosophy, ","))
	}
	if len(index.ProtocolComposition.BehaviorContract) > 0 {
		facts = append(facts, "contract="+strings.Join(index.ProtocolComposition.BehaviorContract, ","))
	}
	if len(index.ProtocolComposition.RequiredRoles) > 0 {
		facts = append(facts, "roles="+strings.Join(index.ProtocolComposition.RequiredRoles, ","))
	}
	if len(index.ProtocolComposition.RequiredGates) > 0 {
		facts = append(facts, "gates="+strings.Join(index.ProtocolComposition.RequiredGates, ","))
	}
	if len(index.ProtocolComposition.RequiredProofKinds) > 0 {
		facts = append(facts, "proof_kinds="+strings.Join(index.ProtocolComposition.RequiredProofKinds, ","))
	}
	if len(index.ProtocolComposition.SelectedPriorRefs) > 0 {
		facts = append(facts, "selected_priors="+strings.Join(index.ProtocolComposition.SelectedPriorRefs, ","))
	}
	return facts
}

func buildEvolveFactsAffordance(index *ContextIndex) *AffordanceItem {
	if index == nil || strings.TrimSpace(index.EvolveFactsPath) == "" {
		return nil
	}
	item := &AffordanceItem{
		ID:      "evolve-facts",
		Kind:    "fact",
		Summary: "Derived evolve management facts for the current frontier.",
		Paths:   []string{index.EvolveFactsPath},
	}
	if index.Evolve == nil {
		return item
	}
	if index.Evolve.FrontierState != "" {
		item.Facts = append(item.Facts, fmt.Sprintf("Frontier state: `%s`.", index.Evolve.FrontierState))
	}
	if index.Evolve.BestExperimentID != "" {
		item.Facts = append(item.Facts, fmt.Sprintf("Best experiment: `%s`.", index.Evolve.BestExperimentID))
	}
	item.Facts = append(item.Facts, fmt.Sprintf("Open candidate count: `%d`.", index.Evolve.OpenCandidateCount))
	if len(index.Evolve.OpenCandidateIDs) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Open candidate IDs: `%s`.", strings.Join(index.Evolve.OpenCandidateIDs, ", ")))
	}
	if index.Evolve.LastStopReasonCode != "" {
		item.Facts = append(item.Facts, fmt.Sprintf("Last stop reason: `%s`.", index.Evolve.LastStopReasonCode))
	}
	if index.Evolve.LastManagementEventAt != "" {
		item.Facts = append(item.Facts, fmt.Sprintf("Last management event: `%s`.", index.Evolve.LastManagementEventAt))
	}
	return item
}

func buildSelectionFactsAffordance(index *ContextIndex) *AffordanceItem {
	if index == nil || index.Selection == nil {
		return nil
	}
	item := &AffordanceItem{
		ID:      "selection-facts",
		Kind:    "fact",
		Summary: "Selection candidate pools and disabled targets recorded for this run.",
	}
	if len(index.Selection.MasterCandidates) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Master candidates: `%s`.", strings.Join(index.Selection.MasterCandidates, ", ")))
	}
	if len(index.Selection.WorkerCandidates) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Worker candidates: `%s`.", strings.Join(index.Selection.WorkerCandidates, ", ")))
	}
	if len(index.Selection.DisabledEngines) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Disabled engines: `%s`.", strings.Join(index.Selection.DisabledEngines, ", ")))
	}
	if len(index.Selection.DisabledTargets) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Disabled targets: `%s`.", strings.Join(index.Selection.DisabledTargets, ", ")))
	}
	if index.SelectionSnapshotPath != "" {
		item.Paths = []string{index.SelectionSnapshotPath}
	}
	return item
}

func buildResourceStateAffordance(index *ContextIndex) *AffordanceItem {
	if index == nil || strings.TrimSpace(index.ResourceStatePath) == "" {
		return nil
	}
	item := &AffordanceItem{
		ID:      "resource-state",
		Kind:    "path",
		Summary: "Framework-owned resource safety facts for this run. Use this only when launch admission, OOM events, or runtime pressure needs explicit diagnosis.",
		Paths:   []string{index.ResourceStatePath},
	}
	return item
}

func buildWorktreeBoundaryAffordance(index *ContextIndex, target string) *AffordanceItem {
	if index == nil {
		return nil
	}
	item := &AffordanceItem{
		ID:   "worktree-boundary",
		Kind: "fact",
	}
	if strings.TrimSpace(target) == "" || target == "master" {
		item.Summary = "Understand source-root, run-root, and session lineage boundaries before inspecting, keeping, or manually integrating work."
		if index.ProjectRoot != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Source root: `%s`.", index.ProjectRoot))
			item.Paths = append(item.Paths, index.ProjectRoot)
		}
		if index.RunWorktree != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Run-root integration boundary: `%s`.", index.RunWorktree))
			item.Paths = append(item.Paths, index.RunWorktree)
		}
		if len(index.TargetFiles) > 0 {
			item.Facts = append(item.Facts, fmt.Sprintf("Declared target files/paths: `%s`.", strings.Join(index.TargetFiles, "`, `")))
		}
		if len(index.ReadonlyPaths) > 0 {
			item.Facts = append(item.Facts, fmt.Sprintf("Declared readonly paths: `%s`.", strings.Join(index.ReadonlyPaths, "`, `")))
			item.Facts = append(item.Facts, "Treat the readonly boundary as a run-level execution contract when dispatching, reusing, or taking over work.")
		}
		item.Facts = append(item.Facts, "`goalx keep session-N` merges committed session branch history into the run-root worktree only.")
		item.Facts = append(item.Facts, "`goalx integrate` records the current run-root result after master manually merged, cherry-picked, or partially adopted work there.")
		item.Facts = append(item.Facts, "`goalx keep --run NAME` is the separate source-root merge step.")
		for _, session := range index.Sessions {
			fact := fmt.Sprintf("%s", session.Name)
			if session.WorktreePath != "" {
				fact += fmt.Sprintf(": worktree `%s`", session.WorktreePath)
				item.Paths = append(item.Paths, session.WorktreePath)
			} else {
				fact += ": shared run-root worktree"
			}
			if session.BaseBranchSelector != "" {
				fact += fmt.Sprintf(", base selector `%s`", session.BaseBranchSelector)
			}
			if session.BaseBranch != "" {
				fact += fmt.Sprintf(", base ref `%s`", session.BaseBranch)
			}
			item.Facts = append(item.Facts, fact+".")
		}
		item.Paths = dedupeStrings(item.Paths)
		return item
	}
	for _, session := range index.Sessions {
		if session.Name != target {
			continue
		}
		item.Summary = "Understand this session's default edit boundary and recorded lineage before changing files."
		if session.WorktreePath != "" {
			item.Facts = append(item.Facts, "Default edit boundary is this session's dedicated worktree.")
			item.Paths = append(item.Paths, session.WorktreePath)
		} else {
			item.Facts = append(item.Facts, "This session shares the run-root worktree; assume overlap with other no-worktree sessions.")
		}
		if index.RunWorktree != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Run-root integration boundary: `%s`.", index.RunWorktree))
			item.Paths = append(item.Paths, index.RunWorktree)
		}
		if index.ProjectRoot != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Source root: `%s`.", index.ProjectRoot))
			item.Paths = append(item.Paths, index.ProjectRoot)
		}
		if session.WorktreePath != "" {
			item.Facts = append(item.Facts, "Do not edit the source root or run-root worktree from a dedicated session unless master explicitly redirects you to inspect or integrate there.")
		}
		if session.BaseBranchSelector != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Recorded parent/base selector: `%s`.", session.BaseBranchSelector))
		}
		if session.BaseBranch != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Recorded parent/base ref: `%s`.", session.BaseBranch))
		}
		if len(session.ReadonlyPaths) > 0 {
			item.Facts = append(item.Facts, fmt.Sprintf("Declared readonly paths: `%s`.", strings.Join(session.ReadonlyPaths, "`, `")))
			item.Facts = append(item.Facts, "GoalX records this as an execution contract.")
			item.Facts = append(item.Facts, "If the assignment conflicts with it, stop and redirect instead of crossing the readonly boundary.")
		}
		if session.WorktreePath != "" {
			item.Facts = append(item.Facts, "If you discover accidental edits outside your assigned worktree, stop, record the boundary violation, and migrate or revert those edits before continuing.")
		}
		item.Paths = dedupeStrings(item.Paths)
		return item
	}
	return nil
}

func RenderAffordancesMarkdown(doc *AffordancesDocument) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# GoalX Affordances\n\n")
	b.WriteString(fmt.Sprintf("- Run: `%s`\n", doc.RunName))
	if doc.Target != "" {
		b.WriteString(fmt.Sprintf("- Target: `%s`\n", doc.Target))
	}
	if doc.RunDir != "" {
		b.WriteString(fmt.Sprintf("- Run dir: `%s`\n", doc.RunDir))
	}
	if doc.ControlDir != "" {
		b.WriteString(fmt.Sprintf("- Control dir: `%s`\n", doc.ControlDir))
	}
	b.WriteString("\n")
	for _, item := range doc.Items {
		b.WriteString(fmt.Sprintf("## %s\n\n", item.ID))
		if item.Summary != "" {
			b.WriteString(item.Summary + "\n\n")
		}
		if item.Command != "" {
			b.WriteString("```bash\n" + item.Command + "\n```\n\n")
		}
		facts := markdownFactsForAffordance(item)
		for _, fact := range facts {
			b.WriteString("- " + fact + "\n")
		}
		if len(facts) > 0 {
			b.WriteString("\n")
		}
		for _, path := range item.Paths {
			b.WriteString("- `" + path + "`\n")
		}
		if len(item.Paths) > 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func markdownFactsForAffordance(item AffordanceItem) []string {
	if len(item.Facts) == 0 {
		return nil
	}
	if item.Kind != "fact" {
		return item.Facts
	}
	limit := 4
	switch item.ID {
	case "worktree-boundary", "resource-state":
		return item.Facts
	}
	if len(item.Facts) <= limit {
		return item.Facts
	}
	facts := append([]string(nil), item.Facts[:limit]...)
	facts = append(facts, fmt.Sprintf("(%d more facts in affordances.json)", len(item.Facts)-limit))
	return facts
}

func RefreshRunGuidance(projectRoot, runName, runDir string) (bool, error) {
	if err := RefreshSessionRuntimeProjection(runDir, runName); err != nil {
		return false, err
	}
	if err := RefreshWorktreeSnapshot(runDir); err != nil {
		return false, err
	}
	if err := RefreshCognitionStateForRun(runDir, runName); err != nil {
		return false, err
	}
	if err := RefreshImpactState(runDir, "run-root"); err != nil {
		return false, err
	}
	if err := RefreshFreshnessState(runDir); err != nil {
		return false, err
	}
	successContextChanged, err := RefreshRunSuccessContextForRun(projectRoot, runDir)
	if err != nil {
		return false, err
	}
	if err := RefreshEvolveFacts(runDir); err != nil {
		return false, err
	}
	activity, err := BuildActivitySnapshot(projectRoot, runName, runDir)
	if err != nil {
		return false, err
	}
	if err := SaveActivitySnapshot(runDir, activity); err != nil {
		return false, err
	}
	index, err := BuildContextIndex(projectRoot, runName, runDir)
	if err != nil {
		return false, err
	}
	if err := SaveContextIndex(runDir, index); err != nil {
		return false, err
	}
	affordances, err := BuildAffordances(projectRoot, runName, runDir, "")
	if err != nil {
		return false, err
	}
	return successContextChanged, SaveAffordances(runDir, affordances)
}

func buildAffordanceCommand(runName, target string) string {
	args := []string{"goalx afford", "--run", runName}
	if normalized := normalizedAffordanceTarget(target); normalized != "" {
		args = append(args, normalized)
	}
	return strings.Join(args, " ")
}

func normalizedAffordanceTarget(target string) string {
	if strings.TrimSpace(target) == "" {
		return ""
	}
	return strings.TrimSpace(target)
}

func buildTellCommand(runName, target string) string {
	args := []string{"goalx", "tell", "--run", runName}
	if normalized := normalizedAffordanceTarget(target); normalized != "" {
		args = append(args, normalized)
	} else {
		args = append(args, "session-N")
	}
	args = append(args, "\"message\"")
	return strings.Join(args, " ")
}

func buildAttachCommand(runName, target string) string {
	args := []string{"goalx", "attach", "--run", runName}
	if normalized := normalizedAffordanceTarget(target); normalized != "" {
		args = append(args, normalized)
	} else {
		args = append(args, "session-N")
	}
	return strings.Join(args, " ")
}

func providerRuntimeFactsForTarget(facts []ProviderRuntimeFact, target string) []ProviderRuntimeFact {
	if len(facts) == 0 {
		return nil
	}
	if strings.TrimSpace(target) == "" {
		return facts
	}
	filtered := make([]ProviderRuntimeFact, 0, len(facts))
	for _, fact := range facts {
		if fact.Target == target {
			filtered = append(filtered, fact)
		}
	}
	return filtered
}

func providerRuntimeFactsSummary(target string, facts []ProviderRuntimeFact) string {
	if len(facts) == 0 {
		return ""
	}
	if strings.TrimSpace(target) == "" {
		return "Provider runtime and bootstrap facts for this run."
	}
	engine := facts[0].Engine
	if strings.TrimSpace(engine) == "" {
		return fmt.Sprintf("Provider runtime and bootstrap facts for `%s`.", target)
	}
	return fmt.Sprintf("Provider runtime and bootstrap facts for `%s` (`%s`).", target, engine)
}

func renderProviderRuntimeFactLines(target string, facts []ProviderRuntimeFact) []string {
	lines := make([]string, 0, len(facts))
	for _, fact := range facts {
		line := fact.Fact
		if strings.TrimSpace(target) == "" {
			prefix := fact.Target
			if strings.TrimSpace(prefix) == "" {
				prefix = "run"
			}
			if fact.Engine != "" {
				prefix += " (" + fact.Engine + ")"
			}
			line = prefix + ": " + line
		}
		lines = append(lines, line)
	}
	return lines
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
