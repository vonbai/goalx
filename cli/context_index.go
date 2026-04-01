package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type ContextIndex struct {
	Version               int                         `json:"version"`
	CheckedAt             string                      `json:"checked_at,omitempty"`
	ProjectRoot           string                      `json:"project_root,omitempty"`
	RunDir                string                      `json:"run_dir,omitempty"`
	RunName               string                      `json:"run_name,omitempty"`
	RunWorktree           string                      `json:"run_worktree,omitempty"`
	ContextFiles          []string                    `json:"context_files,omitempty"`
	ContextRefs           []string                    `json:"context_refs,omitempty"`
	TargetFiles           []string                    `json:"target_files,omitempty"`
	ReadonlyPaths         []string                    `json:"readonly_paths,omitempty"`
	RunIdentity           ContextRunIdentity          `json:"run_identity"`
	ReportsDir            string                      `json:"reports_dir,omitempty"`
	CharterPath           string                      `json:"charter_path,omitempty"`
	ObjectiveContractPath string                      `json:"objective_contract_path,omitempty"`
	GoalPath              string                      `json:"goal_path,omitempty"`
	StatusPath            string                      `json:"status_path,omitempty"`
	ExperimentsLogPath    string                      `json:"experiments_log_path,omitempty"`
	IntegrationStatePath  string                      `json:"integration_state_path,omitempty"`
	AcceptanceStatePath   string                      `json:"acceptance_state_path,omitempty"`
	CompletionProofPath   string                      `json:"completion_proof_path,omitempty"`
	CoordinationPath      string                      `json:"coordination_path,omitempty"`
	SummaryPath           string                      `json:"summary_path,omitempty"`
	ControlDir            string                      `json:"control_dir,omitempty"`
	ActivityPath          string                      `json:"activity_path,omitempty"`
	WorktreeSnapshotPath  string                      `json:"worktree_snapshot_path,omitempty"`
	SelectionSnapshotPath string                      `json:"selection_snapshot_path,omitempty"`
	TransportFactsPath    string                      `json:"transport_facts_path,omitempty"`
	MemoryQueryPath       string                      `json:"memory_query_path,omitempty"`
	MemoryContextPath     string                      `json:"memory_context_path,omitempty"`
	IntakePath            string                      `json:"intake_path,omitempty"`
	CompilerInputPath     string                      `json:"compiler_input_path,omitempty"`
	CompilerReportPath    string                      `json:"compiler_report_path,omitempty"`
	EvolveFactsPath       string                      `json:"evolve_facts_path,omitempty"`
	AffordancesJSONPath   string                      `json:"affordances_json_path,omitempty"`
	AffordancesMarkdown   string                      `json:"affordances_markdown_path,omitempty"`
	ContextIndexPath      string                      `json:"context_index_path,omitempty"`
	DimensionsPath        string                      `json:"dimensions_path,omitempty"`
	Budget                *ActivityBudget             `json:"budget,omitempty"`
	Master                ContextMaster               `json:"master"`
	Evolve                *ContextEvolve              `json:"evolve,omitempty"`
	ObjectiveIntegrity    *ContextObjectiveIntegrity  `json:"objective_integrity,omitempty"`
	GoalBoundary          *ContextGoalBoundary        `json:"goal_boundary,omitempty"`
	RunStatus             *ContextRunStatus           `json:"run_status,omitempty"`
	Acceptance            *ContextAcceptance          `json:"acceptance,omitempty"`
	QualityDebt           *ContextQualityDebt         `json:"quality_debt,omitempty"`
	Closeout              *ContextCloseout            `json:"closeout,omitempty"`
	Selection             *ContextSelection           `json:"selection,omitempty"`
	ProtocolComposition   *ContextProtocolComposition `json:"protocol_composition,omitempty"`
	Sessions              []ContextSession            `json:"sessions,omitempty"`
	ProviderRuntimeFacts  []ProviderRuntimeFact       `json:"provider_runtime_facts,omitempty"`
	ClaudeCodeAvailable   bool                        `json:"claude_code_available,omitempty"`
	CodexAvailable        bool                        `json:"codex_available,omitempty"`
	GitAvailable          bool                        `json:"git_available,omitempty"`
	TmuxAvailable         bool                        `json:"tmux_available,omitempty"`
}

type ContextRunIdentity struct {
	CharterID     string                  `json:"charter_id,omitempty"`
	RunID         string                  `json:"run_id,omitempty"`
	RootRunID     string                  `json:"root_run_id,omitempty"`
	Objective     string                  `json:"objective,omitempty"`
	Intent        string                  `json:"intent,omitempty"`
	Mode          string                  `json:"mode,omitempty"`
	PhaseKind     string                  `json:"phase_kind,omitempty"`
	RoleContracts RunCharterRoleContracts `json:"role_contracts,omitempty"`
}

type ContextMaster struct {
	Engine                 string                    `json:"engine,omitempty"`
	Model                  string                    `json:"model,omitempty"`
	Mode                   string                    `json:"mode,omitempty"`
	RequestedEffort        goalx.EffortLevel         `json:"requested_effort,omitempty"`
	EffectiveEffort        string                    `json:"effective_effort,omitempty"`
	SurfaceKind            string                    `json:"surface_kind,omitempty"`
	WorktreeKind           string                    `json:"worktree_kind,omitempty"`
	MergeableOutputSurface bool                      `json:"mergeable_output_surface"`
	ProviderBootstrap      *ContextProviderBootstrap `json:"provider_bootstrap,omitempty"`
}

type ContextGoalBoundary struct {
	RequiredCount    int            `json:"required_count,omitempty"`
	OptionalCount    int            `json:"optional_count,omitempty"`
	RequiredBySource map[string]int `json:"required_by_source,omitempty"`
	RequiredByRole   map[string]int `json:"required_by_role,omitempty"`
	RequiredByState  map[string]int `json:"required_by_state,omitempty"`
}

type ContextRunStatus struct {
	Phase                         string   `json:"phase,omitempty"`
	RequiredRemaining             int      `json:"required_remaining"`
	GoalRequiredRemaining         int      `json:"goal_required_remaining"`
	StatusOpenRequiredIDs         []string `json:"status_open_required_ids,omitempty"`
	GoalRemainingRequiredIDs      []string `json:"goal_remaining_required_ids,omitempty"`
	StatusOpenRequiredIDsRecorded bool     `json:"status_open_required_ids_recorded,omitempty"`
	RequiredRemainingMatch        bool     `json:"required_remaining_match"`
	OpenRequiredIDsMatch          bool     `json:"open_required_ids_match,omitempty"`
	LastVerifiedAt                string   `json:"last_verified_at,omitempty"`
}

type ContextAcceptance struct {
	ActiveCheckCount int    `json:"active_check_count"`
	LastCheckedAt    string `json:"last_checked_at,omitempty"`
	LastExitCode     *int   `json:"last_exit_code,omitempty"`
	EvidencePath     string `json:"evidence_path,omitempty"`
}

type ContextQualityDebt struct {
	SuccessDimensionUnowned []string `json:"success_dimension_unowned,omitempty"`
	ProofPlanGap            []string `json:"proof_plan_gap,omitempty"`
	CriticGateMissing       bool     `json:"critic_gate_missing,omitempty"`
	FinisherGateMissing     bool     `json:"finisher_gate_missing,omitempty"`
	OnlyCorrectnessEvidence bool     `json:"only_correctness_evidence_present,omitempty"`
	DomainPackMissing       bool     `json:"domain_pack_missing_for_nontrivial_run,omitempty"`
	Zero                    bool     `json:"zero,omitempty"`
}

type ContextCloseout struct {
	SummaryExists         bool `json:"summary_exists"`
	CompletionProofExists bool `json:"completion_proof_exists"`
	ReadyToFinalize       bool `json:"ready_to_finalize"`
}

type ContextEvolve struct {
	FrontierState         string   `json:"frontier_state,omitempty"`
	BestExperimentID      string   `json:"best_experiment_id,omitempty"`
	OpenCandidateCount    int      `json:"open_candidate_count,omitempty"`
	OpenCandidateIDs      []string `json:"open_candidate_ids,omitempty"`
	LastStopReasonCode    string   `json:"last_stop_reason_code,omitempty"`
	LastManagementEventAt string   `json:"last_management_event_at,omitempty"`
}

type ContextObjectiveIntegrity struct {
	ContractState              string   `json:"contract_state,omitempty"`
	ContractLocked             bool     `json:"contract_locked,omitempty"`
	ClauseCount                int      `json:"clause_count,omitempty"`
	GoalClauseCount            int      `json:"goal_clause_count,omitempty"`
	AcceptanceClauseCount      int      `json:"acceptance_clause_count,omitempty"`
	GoalCoveredCount           int      `json:"goal_covered_count,omitempty"`
	AcceptanceCoveredCount     int      `json:"acceptance_covered_count,omitempty"`
	MissingGoalClauseIDs       []string `json:"missing_goal_clause_ids,omitempty"`
	MissingAcceptanceClauseIDs []string `json:"missing_acceptance_clause_ids,omitempty"`
	IntegrityReady             bool     `json:"integrity_ready,omitempty"`
	IntegrityOK                bool     `json:"integrity_ok,omitempty"`
}

type ContextProviderBootstrap struct {
	PermissionMode                    string `json:"permission_mode,omitempty"`
	PermissionRequestHookBootstrapped bool   `json:"permission_request_hook_bootstrapped"`
	ElicitationHookBootstrapped       bool   `json:"elicitation_hook_bootstrapped"`
	NotificationHookBootstrapped      bool   `json:"notification_hook_bootstrapped"`
	BootstrapVerified                 bool   `json:"bootstrap_verified"`
}

type ContextSelection struct {
	ExplicitSelection bool              `json:"explicit_selection,omitempty"`
	DisabledEngines   []string          `json:"disabled_engines,omitempty"`
	DisabledTargets   []string          `json:"disabled_targets,omitempty"`
	MasterCandidates  []string          `json:"master_candidates,omitempty"`
	WorkerCandidates  []string          `json:"worker_candidates,omitempty"`
	MasterEffort      goalx.EffortLevel `json:"master_effort,omitempty"`
	WorkerEffort      goalx.EffortLevel `json:"worker_effort,omitempty"`
}

type ContextProtocolComposition struct {
	Philosophy         []string `json:"philosophy,omitempty"`
	BehaviorContract   []string `json:"behavior_contract,omitempty"`
	RequiredRoles      []string `json:"required_roles,omitempty"`
	RequiredGates      []string `json:"required_gates,omitempty"`
	RequiredProofKinds []string `json:"required_proof_kinds,omitempty"`
	SelectedPriorRefs  []string `json:"selected_prior_refs,omitempty"`
}

type ContextSession struct {
	Name                   string                    `json:"name,omitempty"`
	RoleKind               string                    `json:"role_kind,omitempty"`
	Mode                   string                    `json:"mode,omitempty"`
	Engine                 string                    `json:"engine,omitempty"`
	Model                  string                    `json:"model,omitempty"`
	RequestedEffort        goalx.EffortLevel         `json:"requested_effort,omitempty"`
	EffectiveEffort        string                    `json:"effective_effort,omitempty"`
	SurfaceKind            string                    `json:"surface_kind,omitempty"`
	WorktreeKind           string                    `json:"worktree_kind,omitempty"`
	MergeableOutputSurface bool                      `json:"mergeable_output_surface"`
	ProviderBootstrap      *ContextProviderBootstrap `json:"provider_bootstrap,omitempty"`
	WindowName             string                    `json:"window_name,omitempty"`
	WorktreePath           string                    `json:"worktree_path,omitempty"`
	JournalPath            string                    `json:"journal_path,omitempty"`
	InboxPath              string                    `json:"inbox_path,omitempty"`
	CursorPath             string                    `json:"cursor_path,omitempty"`
	Branch                 string                    `json:"branch,omitempty"`
	BaseBranchSelector     string                    `json:"base_branch_selector,omitempty"`
	BaseBranch             string                    `json:"base_branch,omitempty"`
	TargetFiles            []string                  `json:"target_files,omitempty"`
	ReadonlyPaths          []string                  `json:"readonly_paths,omitempty"`
}

type ProviderRuntimeFact struct {
	Target string `json:"target,omitempty"`
	Engine string `json:"engine,omitempty"`
	Fact   string `json:"fact,omitempty"`
}

func ContextIndexPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "context-index.json")
}

func LoadContextIndex(path string) (*ContextIndex, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	index := &ContextIndex{}
	if len(strings.TrimSpace(string(data))) == 0 {
		index.Version = 1
		return index, nil
	}
	if err := json.Unmarshal(data, index); err != nil {
		return nil, err
	}
	if index.Version == 0 {
		index.Version = 1
	}
	return index, nil
}

func SaveContextIndex(runDir string, index *ContextIndex) error {
	if index == nil {
		return nil
	}
	if index.Version == 0 {
		index.Version = 1
	}
	if index.CheckedAt == "" {
		index.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(ContextIndexPath(runDir), index)
}

func BuildContextIndex(projectRoot, runName, runDir string) (*ContextIndex, error) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return nil, err
	}
	engines, err := loadEngineCatalog(projectRoot)
	if err != nil {
		return nil, err
	}
	charter, err := RequireRunCharter(runDir)
	if err != nil {
		return nil, err
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	runtimeState, err := LoadRunRuntimeState(RunRuntimeStatePath(runDir))
	if err != nil {
		return nil, err
	}
	index := &ContextIndex{
		Version:               1,
		CheckedAt:             time.Now().UTC().Format(time.RFC3339),
		ProjectRoot:           projectRoot,
		RunDir:                runDir,
		RunName:               runName,
		RunWorktree:           RunWorktreePath(runDir),
		ContextFiles:          append([]string(nil), cfg.Context.Files...),
		ContextRefs:           append([]string(nil), cfg.Context.Refs...),
		TargetFiles:           append([]string(nil), cfg.Target.Files...),
		ReadonlyPaths:         append([]string(nil), cfg.Target.Readonly...),
		RunIdentity:           contextRunIdentity(charter, meta),
		ReportsDir:            ReportsDir(runDir),
		CharterPath:           RunCharterPath(runDir),
		ObjectiveContractPath: ObjectiveContractPath(runDir),
		GoalPath:              GoalPath(runDir),
		StatusPath:            RunStatusPath(runDir),
		ExperimentsLogPath:    ExperimentsLogPath(runDir),
		IntegrationStatePath:  IntegrationStatePath(runDir),
		AcceptanceStatePath:   AcceptanceStatePath(runDir),
		CompletionProofPath:   CompletionStatePath(runDir),
		CoordinationPath:      CoordinationPath(runDir),
		SummaryPath:           SummaryPath(runDir),
		ControlDir:            ControlDir(runDir),
		ActivityPath:          ActivityPath(runDir),
		WorktreeSnapshotPath:  WorktreeSnapshotPath(runDir),
		TransportFactsPath:    TransportFactsPath(runDir),
		MemoryQueryPath:       MemoryQueryPath(runDir),
		MemoryContextPath:     MemoryContextPath(runDir),
		IntakePath:            IntakePath(runDir),
		CompilerInputPath:     CompilerInputPath(runDir),
		CompilerReportPath:    CompilerReportPath(runDir),
		AffordancesJSONPath:   AffordancesJSONPath(runDir),
		AffordancesMarkdown:   AffordancesMarkdownPath(runDir),
		ContextIndexPath:      ContextIndexPath(runDir),
		DimensionsPath:        ControlDimensionsPath(runDir),
		ClaudeCodeAvailable:   toolAvailable("claude"),
		CodexAvailable:        toolAvailable("codex"),
		GitAvailable:          toolAvailable("git"),
		TmuxAvailable:         toolAvailable("tmux"),
	}
	if budget := buildActivityBudget(cfg, runtimeState, meta, index.CheckedAt); budget.MaxDurationSeconds > 0 {
		index.Budget = &budget
	}
	if goalState, err := LoadGoalState(GoalPath(runDir)); err != nil {
		return nil, err
	} else if goalState != nil {
		index.GoalBoundary = contextGoalBoundary(goalState)
	}
	if statusSummary, err := contextRunStatus(runDir); err != nil {
		return nil, err
	} else if statusSummary != nil {
		index.RunStatus = statusSummary
	}
	if acceptanceSummary, err := contextAcceptance(runDir); err != nil {
		return nil, err
	} else if acceptanceSummary != nil {
		index.Acceptance = acceptanceSummary
	}
	if qualityDebt, err := contextQualityDebt(runDir); err != nil {
		return nil, err
	} else if qualityDebt != nil {
		index.QualityDebt = qualityDebt
	}
	if closeoutSummary, err := contextCloseout(runDir); err != nil {
		return nil, err
	} else if closeoutSummary != nil {
		index.Closeout = closeoutSummary
	}
	if integrity, err := BuildObjectiveIntegritySummary(runDir); err != nil {
		return nil, err
	} else if integrity.ContractPresent {
		index.ObjectiveIntegrity = contextObjectiveIntegrity(integrity)
	}
	selectionSnapshot, err := LoadSelectionSnapshot(SelectionSnapshotPath(runDir))
	if err != nil {
		return nil, err
	}
	if selectionSnapshot != nil {
		index.SelectionSnapshotPath = SelectionSnapshotPath(runDir)
		index.Selection = &ContextSelection{
			ExplicitSelection: selectionSnapshot.ExplicitSelection,
			DisabledEngines:   append([]string(nil), selectionSnapshot.Policy.DisabledEngines...),
			DisabledTargets:   append([]string(nil), selectionSnapshot.Policy.DisabledTargets...),
			MasterCandidates:  append([]string(nil), selectionSnapshot.Policy.MasterCandidates...),
			WorkerCandidates:  append([]string(nil), selectionSnapshot.Policy.WorkerCandidates...),
			MasterEffort:      selectionSnapshot.Policy.MasterEffort,
			WorkerEffort:      selectionSnapshot.Policy.WorkerEffort,
		}
	}
	if composition, err := buildProtocolComposition(runDir, ProtocolComposition{}); err != nil {
		return nil, err
	} else if composition.Enabled {
		index.ProtocolComposition = &ContextProtocolComposition{
			Philosophy:         append([]string(nil), composition.Philosophy...),
			BehaviorContract:   append([]string(nil), composition.BehaviorContract...),
			RequiredRoles:      append([]string(nil), composition.RequiredRoles...),
			RequiredGates:      append([]string(nil), composition.RequiredGates...),
			RequiredProofKinds: append([]string(nil), composition.RequiredProofKinds...),
			SelectedPriorRefs:  append([]string(nil), composition.SelectedPriorRefs...),
		}
	}
	if strings.TrimSpace(index.RunIdentity.Intent) == runIntentEvolve {
		index.EvolveFactsPath = EvolveFactsPath(runDir)
		facts, err := BuildEvolveFacts(runDir)
		if err != nil {
			return nil, err
		}
		if facts != nil {
			index.Evolve = &ContextEvolve{
				FrontierState:         facts.FrontierState,
				BestExperimentID:      facts.BestExperimentID,
				OpenCandidateCount:    facts.OpenCandidateCount,
				OpenCandidateIDs:      append([]string(nil), facts.OpenCandidateIDs...),
				LastStopReasonCode:    facts.LastStopReasonCode,
				LastManagementEventAt: facts.LastManagementEventAt,
			}
		}
	}
	index.Master = contextMaster(cfg, selectionSnapshot, engines, runDir)
	index.ProviderRuntimeFacts = append(index.ProviderRuntimeFacts, providerRuntimeFactsForEngine("master", cfg.Master.Engine)...)
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		name := SessionName(idx)
		session := ContextSession{
			Name:         name,
			WindowName:   sessionWindowName(cfg.Name, idx),
			JournalPath:  JournalPath(runDir, name),
			InboxPath:    ControlInboxPath(runDir, name),
			CursorPath:   SessionCursorPath(runDir, name),
			WorktreePath: resolvedSessionContextWorktree(runDir, cfg.Name, name),
		}
		if identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, name)); err == nil && identity != nil {
			session.Mode = identity.Mode
			session.RoleKind = identity.RoleKind
			session.Engine = identity.Engine
			session.Model = identity.Model
			session.RequestedEffort = identity.RequestedEffort
			session.EffectiveEffort = identity.EffectiveEffort
			session.BaseBranchSelector = identity.BaseBranchSelector
			session.BaseBranch = identity.BaseBranch
			session.TargetFiles = append([]string(nil), identity.Target.Files...)
			session.ReadonlyPaths = append([]string(nil), identity.Target.Readonly...)
			index.ProviderRuntimeFacts = append(index.ProviderRuntimeFacts, providerRuntimeFactsForEngine(name, identity.Engine)...)
		}
		if sessionsState, err := EnsureSessionsRuntimeState(runDir); err == nil {
			if current, ok := sessionsState.Sessions[name]; ok {
				if session.Mode == "" {
					session.Mode = current.Mode
				}
				session.Branch = current.Branch
			}
		}
		if session.BaseBranchSelector == "" || session.BaseBranch == "" {
			if lineage, err := loadSessionWorktreeLineage(runDir, name); err == nil && lineage != nil {
				if session.BaseBranchSelector == "" {
					session.BaseBranchSelector = lineage.ParentSelector
				}
				if session.BaseBranch == "" {
					session.BaseBranch = lineage.ParentRef
				}
			}
		}
		session.SurfaceKind = "durable_session"
		session.WorktreeKind = contextWorktreeKind(runDir, session.WorktreePath)
		session.MergeableOutputSurface = session.WorktreeKind == "dedicated"
		session.ProviderBootstrap = contextProviderBootstrap(identityEngine(session), session.WorktreePath, contextPermissionMode(engines, identityEngine(session), identityModel(session), session.RequestedEffort))
		index.Sessions = append(index.Sessions, session)
	}
	sort.Slice(index.Sessions, func(i, j int) bool { return index.Sessions[i].Name < index.Sessions[j].Name })
	index.ProviderRuntimeFacts = dedupeProviderRuntimeFacts(index.ProviderRuntimeFacts)
	return index, nil
}

func contextGoalBoundary(state *GoalState) *ContextGoalBoundary {
	if state == nil {
		return nil
	}
	normalizeGoalState(state)
	summary := &ContextGoalBoundary{
		OptionalCount:    len(state.Optional),
		RequiredBySource: map[string]int{},
		RequiredByRole:   map[string]int{},
		RequiredByState:  map[string]int{},
	}
	for _, item := range state.Required {
		summary.RequiredCount++
		summary.RequiredBySource[item.Source]++
		summary.RequiredByRole[item.Role]++
		summary.RequiredByState[item.State]++
	}
	if len(summary.RequiredBySource) == 0 {
		summary.RequiredBySource = nil
	}
	if len(summary.RequiredByRole) == 0 {
		summary.RequiredByRole = nil
	}
	if len(summary.RequiredByState) == 0 {
		summary.RequiredByState = nil
	}
	return summary
}

func contextObjectiveIntegrity(summary ObjectiveIntegritySummary) *ContextObjectiveIntegrity {
	if !summary.ContractPresent {
		return nil
	}
	return &ContextObjectiveIntegrity{
		ContractState:              summary.ContractState,
		ContractLocked:             summary.ContractLocked,
		ClauseCount:                summary.ClauseCount,
		GoalClauseCount:            summary.GoalClauseCount,
		AcceptanceClauseCount:      summary.AcceptanceClauseCount,
		GoalCoveredCount:           summary.GoalCoveredCount,
		AcceptanceCoveredCount:     summary.AcceptanceCoveredCount,
		MissingGoalClauseIDs:       append([]string(nil), summary.MissingGoalClauseIDs...),
		MissingAcceptanceClauseIDs: append([]string(nil), summary.MissingAcceptanceClauseIDs...),
		IntegrityReady:             summary.ReadyForNoShrinkEnforcement(),
		IntegrityOK:                summary.IntegrityOK(),
	}
}

func contextRunStatus(runDir string) (*ContextRunStatus, error) {
	comparison, err := BuildRunStatusComparison(runDir)
	if err != nil || comparison == nil || comparison.StatusRequiredRemaining == nil {
		return nil, err
	}
	out := &ContextRunStatus{
		Phase:                         comparison.Phase,
		RequiredRemaining:             *comparison.StatusRequiredRemaining,
		StatusOpenRequiredIDsRecorded: comparison.StatusOpenRequiredIDsRecorded,
		RequiredRemainingMatch:        comparison.RequiredRemainingMatch,
		OpenRequiredIDsMatch:          comparison.OpenRequiredIDsMatch,
		LastVerifiedAt:                comparison.LastVerifiedAt,
	}
	if comparison.GoalRequiredRemaining != nil {
		out.GoalRequiredRemaining = *comparison.GoalRequiredRemaining
	}
	if comparison.StatusOpenRequiredIDsRecorded {
		out.StatusOpenRequiredIDs = append([]string(nil), comparison.StatusOpenRequiredIDs...)
	}
	if len(comparison.GoalRemainingRequiredIDs) > 0 {
		out.GoalRemainingRequiredIDs = append([]string(nil), comparison.GoalRemainingRequiredIDs...)
	}
	return out, nil
}

func contextAcceptance(runDir string) (*ContextAcceptance, error) {
	state, err := LoadAcceptanceState(AcceptanceStatePath(runDir))
	if err != nil || state == nil {
		return nil, err
	}
	activeChecks := 0
	for _, check := range state.Checks {
		if normalizeAcceptanceCheckState(check.State) == acceptanceCheckStateActive {
			activeChecks++
		}
	}
	return &ContextAcceptance{
		ActiveCheckCount: activeChecks,
		LastCheckedAt:    strings.TrimSpace(state.LastResult.CheckedAt),
		LastExitCode:     state.LastResult.ExitCode,
		EvidencePath:     strings.TrimSpace(state.LastResult.EvidencePath),
	}, nil
}

func contextQualityDebt(runDir string) (*ContextQualityDebt, error) {
	debt, err := BuildQualityDebt(runDir)
	if err != nil || debt == nil {
		return nil, err
	}
	return &ContextQualityDebt{
		SuccessDimensionUnowned: append([]string(nil), debt.SuccessDimensionUnowned...),
		ProofPlanGap:            append([]string(nil), debt.ProofPlanGap...),
		CriticGateMissing:       debt.CriticGateMissing,
		FinisherGateMissing:     debt.FinisherGateMissing,
		OnlyCorrectnessEvidence: debt.OnlyCorrectnessEvidence,
		DomainPackMissing:       debt.DomainPackMissing,
		Zero:                    debt.Zero(),
	}, nil
}

func contextCloseout(runDir string) (*ContextCloseout, error) {
	status, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		return nil, err
	}
	if status == nil && !fileExists(SummaryPath(runDir)) && !fileExists(CompletionStatePath(runDir)) {
		return nil, nil
	}
	facts, err := BuildRunCloseoutFacts(runDir)
	if err != nil {
		return nil, err
	}
	return &ContextCloseout{
		SummaryExists:         facts.SummaryExists,
		CompletionProofExists: facts.CompletionExists,
		ReadyToFinalize:       facts.ReadyToFinalize(),
	}, nil
}

func contextMaster(cfg *goalx.Config, selectionSnapshot *SelectionSnapshot, engines map[string]goalx.EngineConfig, runDir string) ContextMaster {
	requestedEffort := cfg.Master.Effort
	if selectionSnapshot != nil && selectionSnapshot.Master.Effort != "" {
		requestedEffort = selectionSnapshot.Master.Effort
	}
	effectiveEffort := ""
	if spec, ok := resolveContextLaunchSpec(engines, cfg.Master.Engine, cfg.Master.Model, requestedEffort); ok {
		effectiveEffort = spec.EffectiveEffort
	}
	return ContextMaster{
		Engine:                 cfg.Master.Engine,
		Model:                  cfg.Master.Model,
		Mode:                   string(cfg.Mode),
		RequestedEffort:        requestedEffort,
		EffectiveEffort:        effectiveEffort,
		SurfaceKind:            "root_master",
		WorktreeKind:           "run_root_shared",
		MergeableOutputSurface: false,
		ProviderBootstrap:      contextProviderBootstrap(cfg.Master.Engine, RunWorktreePath(runDir), contextPermissionMode(engines, cfg.Master.Engine, cfg.Master.Model, requestedEffort)),
	}
}

func resolveContextLaunchSpec(engines map[string]goalx.EngineConfig, engine, model string, effort goalx.EffortLevel) (goalx.LaunchSpec, bool) {
	if len(engines) == 0 || strings.TrimSpace(engine) == "" || strings.TrimSpace(model) == "" {
		return goalx.LaunchSpec{}, false
	}
	spec, err := goalx.ResolveLaunchSpec(engines, goalx.LaunchRequest{
		Engine: engine,
		Model:  model,
		Effort: effort,
	})
	if err != nil {
		return goalx.LaunchSpec{}, false
	}
	return spec, true
}

func contextPermissionMode(engines map[string]goalx.EngineConfig, engine, model string, effort goalx.EffortLevel) string {
	spec, ok := resolveContextLaunchSpec(engines, engine, model, effort)
	if !ok {
		return ""
	}
	fields := strings.Fields(spec.Command)
	for i, field := range fields {
		if strings.HasPrefix(field, "--permission-mode=") {
			return strings.TrimSpace(strings.TrimPrefix(field, "--permission-mode="))
		}
		if field == "--permission-mode" && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

func contextWorktreeKind(runDir, worktreePath string) string {
	if strings.TrimSpace(worktreePath) == "" {
		return ""
	}
	if filepath.Clean(worktreePath) == filepath.Clean(RunWorktreePath(runDir)) {
		return "run_root_shared"
	}
	return "dedicated"
}

func contextProviderBootstrap(engine, worktreePath, permissionMode string) *ContextProviderBootstrap {
	if strings.TrimSpace(engine) != "claude-code" || strings.TrimSpace(worktreePath) == "" {
		return nil
	}
	bootstrap := readClaudeProviderBootstrap(worktreePath)
	bootstrap.PermissionMode = permissionMode
	bootstrap.BootstrapVerified = bootstrap.PermissionRequestHookBootstrapped &&
		bootstrap.ElicitationHookBootstrapped &&
		bootstrap.NotificationHookBootstrapped
	return bootstrap
}

func readClaudeProviderBootstrap(worktreePath string) *ContextProviderBootstrap {
	settingsPath := filepath.Join(worktreePath, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return &ContextProviderBootstrap{}
	}
	doc := map[string]any{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return &ContextProviderBootstrap{}
	}
	hooks := coerceObject(doc["hooks"])
	return &ContextProviderBootstrap{
		PermissionRequestHookBootstrapped: claudeHookMatcherContainsCommand(coerceArray(hooks["PermissionRequest"]), claudeMCPPermissionHookMatcher, "claude-hook permission-request"),
		ElicitationHookBootstrapped:       claudeHookMatcherContainsCommand(coerceArray(hooks["Elicitation"]), claudeMCPElicitationHookMatcher, "claude-hook elicitation"),
		NotificationHookBootstrapped:      claudeHookMatcherContainsCommand(coerceArray(hooks["Notification"]), claudePermissionNotificationMatcher, "claude-hook notification") && claudeHookMatcherContainsCommand(coerceArray(hooks["Notification"]), claudeElicitationNotificationMatcher, "claude-hook notification"),
	}
}

func claudeHookMatcherContainsCommand(entries []any, matcher, commandPart string) bool {
	for _, raw := range entries {
		entry := coerceObject(raw)
		if strings.TrimSpace(toString(entry["matcher"])) != matcher {
			continue
		}
		for _, hookRaw := range coerceArray(entry["hooks"]) {
			hook := coerceObject(hookRaw)
			if strings.TrimSpace(toString(hook["type"])) != "command" {
				continue
			}
			if strings.Contains(strings.TrimSpace(toString(hook["command"])), commandPart) {
				return true
			}
		}
	}
	return false
}

func identityEngine(session ContextSession) string {
	return strings.TrimSpace(session.Engine)
}

func identityModel(session ContextSession) string {
	return strings.TrimSpace(session.Model)
}

func resolvedSessionContextWorktree(runDir, runName, sessionName string) string {
	sessionsState, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return RunWorktreePath(runDir)
	}
	worktreePath := resolvedSessionWorktreePath(runDir, runName, sessionName, sessionsState)
	if strings.TrimSpace(worktreePath) == "" {
		return RunWorktreePath(runDir)
	}
	return worktreePath
}

func toolAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func contextRunIdentity(charter *RunCharter, meta *RunMetadata) ContextRunIdentity {
	if charter == nil {
		return ContextRunIdentity{}
	}
	identity := ContextRunIdentity{
		CharterID:     charter.CharterID,
		RunID:         charter.RunID,
		RootRunID:     charter.RootRunID,
		Objective:     charter.Objective,
		Mode:          charter.Mode,
		PhaseKind:     charter.PhaseKind,
		RoleContracts: charter.RoleContracts,
	}
	if meta != nil && strings.TrimSpace(meta.Intent) != "" {
		identity.Intent = strings.TrimSpace(meta.Intent)
	}
	return identity
}

func providerRuntimeFactsForEngine(target, engine string) []ProviderRuntimeFact {
	runtimeFact := ProviderRuntimeFact{
		Target: target,
		Engine: engine,
		Fact:   "GoalX canonical provider runtime is tmux + interactive TUI.",
	}
	ownershipBoundaryFact := ProviderRuntimeFact{
		Target: target,
		Engine: engine,
		Fact:   "GoalX provider runtime does not change durable ownership boundaries.",
	}
	switch strings.TrimSpace(engine) {
	case "claude-code":
		return []ProviderRuntimeFact{
			runtimeFact,
			ownershipBoundaryFact,
			{Target: target, Engine: engine, Fact: "Claude root sessions cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions."},
			{Target: target, Engine: engine, Fact: "GoalX bootstraps a project-local PermissionRequest hook so unattended Claude permission dialogs can be auto-allowed."},
			{Target: target, Engine: engine, Fact: "GoalX bootstraps a project-local Elicitation hook so unattended Claude user-input or browser-auth requests are cancelled instead of hanging forever."},
			{Target: target, Engine: engine, Fact: "If a Claude permission or elicitation dialog still surfaces, GoalX writes an urgent master-inbox fact through a Notification hook so the run can recover."},
		}
	case "codex":
		return []ProviderRuntimeFact{
			runtimeFact,
			ownershipBoundaryFact,
		}
	default:
		return nil
	}
}

func dedupeProviderRuntimeFacts(facts []ProviderRuntimeFact) []ProviderRuntimeFact {
	if len(facts) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]ProviderRuntimeFact, 0, len(facts))
	for _, fact := range facts {
		key := fact.Target + "\x00" + fact.Engine + "\x00" + fact.Fact
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, fact)
	}
	return out
}
