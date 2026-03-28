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
	Version               int                `json:"version"`
	CheckedAt             string             `json:"checked_at,omitempty"`
	ProjectRoot           string             `json:"project_root,omitempty"`
	RunDir                string             `json:"run_dir,omitempty"`
	RunName               string             `json:"run_name,omitempty"`
	RunWorktree           string             `json:"run_worktree,omitempty"`
	RunIdentity           ContextRunIdentity `json:"run_identity"`
	ReportsDir            string             `json:"reports_dir,omitempty"`
	CharterPath           string             `json:"charter_path,omitempty"`
	GoalPath              string             `json:"goal_path,omitempty"`
	ExperimentsLogPath    string             `json:"experiments_log_path,omitempty"`
	IntegrationStatePath  string             `json:"integration_state_path,omitempty"`
	AcceptanceStatePath   string             `json:"acceptance_state_path,omitempty"`
	CompletionProofPath   string             `json:"completion_proof_path,omitempty"`
	CoordinationPath      string             `json:"coordination_path,omitempty"`
	SummaryPath           string             `json:"summary_path,omitempty"`
	ControlDir            string             `json:"control_dir,omitempty"`
	ActivityPath          string             `json:"activity_path,omitempty"`
	WorktreeSnapshotPath  string             `json:"worktree_snapshot_path,omitempty"`
	SelectionSnapshotPath string             `json:"selection_snapshot_path,omitempty"`
	TransportFactsPath    string             `json:"transport_facts_path,omitempty"`
	MemoryQueryPath       string             `json:"memory_query_path,omitempty"`
	MemoryContextPath     string             `json:"memory_context_path,omitempty"`
	AffordancesJSONPath   string             `json:"affordances_json_path,omitempty"`
	AffordancesMarkdown   string             `json:"affordances_markdown_path,omitempty"`
	ContextIndexPath      string             `json:"context_index_path,omitempty"`
	DimensionsPath        string             `json:"dimensions_path,omitempty"`
	Master                ContextMaster      `json:"master"`
	Selection             *ContextSelection  `json:"selection,omitempty"`
	Sessions              []ContextSession   `json:"sessions,omitempty"`
	ProviderFacts         []ProviderFact     `json:"provider_facts,omitempty"`
	ClaudeCodeAvailable   bool               `json:"claude_code_available,omitempty"`
	CodexAvailable        bool               `json:"codex_available,omitempty"`
	GitAvailable          bool               `json:"git_available,omitempty"`
	TmuxAvailable         bool               `json:"tmux_available,omitempty"`
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
	Engine string `json:"engine,omitempty"`
	Model  string `json:"model,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

type ContextSelection struct {
	ExplicitSelection  bool              `json:"explicit_selection,omitempty"`
	DisabledEngines    []string          `json:"disabled_engines,omitempty"`
	DisabledTargets    []string          `json:"disabled_targets,omitempty"`
	MasterCandidates   []string          `json:"master_candidates,omitempty"`
	ResearchCandidates []string          `json:"research_candidates,omitempty"`
	DevelopCandidates  []string          `json:"develop_candidates,omitempty"`
	MasterEffort       goalx.EffortLevel `json:"master_effort,omitempty"`
	ResearchEffort     goalx.EffortLevel `json:"research_effort,omitempty"`
	DevelopEffort      goalx.EffortLevel `json:"develop_effort,omitempty"`
}

type ContextSession struct {
	Name               string `json:"name,omitempty"`
	Mode               string `json:"mode,omitempty"`
	WindowName         string `json:"window_name,omitempty"`
	WorktreePath       string `json:"worktree_path,omitempty"`
	JournalPath        string `json:"journal_path,omitempty"`
	InboxPath          string `json:"inbox_path,omitempty"`
	CursorPath         string `json:"cursor_path,omitempty"`
	Branch             string `json:"branch,omitempty"`
	BaseBranchSelector string `json:"base_branch_selector,omitempty"`
	BaseBranch         string `json:"base_branch,omitempty"`
}

type ProviderFact struct {
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
	charter, err := RequireRunCharter(runDir)
	if err != nil {
		return nil, err
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	index := &ContextIndex{
		Version:              1,
		CheckedAt:            time.Now().UTC().Format(time.RFC3339),
		ProjectRoot:          projectRoot,
		RunDir:               runDir,
		RunName:              runName,
		RunWorktree:          RunWorktreePath(runDir),
		RunIdentity:          contextRunIdentity(charter, meta),
		ReportsDir:           ReportsDir(runDir),
		CharterPath:          RunCharterPath(runDir),
		GoalPath:             GoalPath(runDir),
		ExperimentsLogPath:   ExperimentsLogPath(runDir),
		IntegrationStatePath: IntegrationStatePath(runDir),
		AcceptanceStatePath:  AcceptanceStatePath(runDir),
		CompletionProofPath:  CompletionStatePath(runDir),
		CoordinationPath:     CoordinationPath(runDir),
		SummaryPath:          SummaryPath(runDir),
		ControlDir:           ControlDir(runDir),
		ActivityPath:         ActivityPath(runDir),
		WorktreeSnapshotPath: WorktreeSnapshotPath(runDir),
		TransportFactsPath:   TransportFactsPath(runDir),
		MemoryQueryPath:      MemoryQueryPath(runDir),
		MemoryContextPath:    MemoryContextPath(runDir),
		AffordancesJSONPath:  AffordancesJSONPath(runDir),
		AffordancesMarkdown:  AffordancesMarkdownPath(runDir),
		ContextIndexPath:     ContextIndexPath(runDir),
		DimensionsPath:       ControlDimensionsPath(runDir),
		Master: ContextMaster{
			Engine: cfg.Master.Engine,
			Model:  cfg.Master.Model,
			Mode:   string(cfg.Mode),
		},
		ClaudeCodeAvailable: toolAvailable("claude"),
		CodexAvailable:      toolAvailable("codex"),
		GitAvailable:        toolAvailable("git"),
		TmuxAvailable:       toolAvailable("tmux"),
	}
	selectionSnapshot, err := LoadSelectionSnapshot(SelectionSnapshotPath(runDir))
	if err != nil {
		return nil, err
	}
	if selectionSnapshot != nil {
		index.SelectionSnapshotPath = SelectionSnapshotPath(runDir)
		index.Selection = &ContextSelection{
			ExplicitSelection:  selectionSnapshot.ExplicitSelection,
			DisabledEngines:    append([]string(nil), selectionSnapshot.Policy.DisabledEngines...),
			DisabledTargets:    append([]string(nil), selectionSnapshot.Policy.DisabledTargets...),
			MasterCandidates:   append([]string(nil), selectionSnapshot.Policy.MasterCandidates...),
			ResearchCandidates: append([]string(nil), selectionSnapshot.Policy.ResearchCandidates...),
			DevelopCandidates:  append([]string(nil), selectionSnapshot.Policy.DevelopCandidates...),
			MasterEffort:       selectionSnapshot.Policy.MasterEffort,
			ResearchEffort:     selectionSnapshot.Policy.ResearchEffort,
			DevelopEffort:      selectionSnapshot.Policy.DevelopEffort,
		}
	}
	index.ProviderFacts = append(index.ProviderFacts, providerFactsForEngine("master", cfg.Master.Engine)...)
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
			session.BaseBranchSelector = identity.BaseBranchSelector
			session.BaseBranch = identity.BaseBranch
			index.ProviderFacts = append(index.ProviderFacts, providerFactsForEngine(name, identity.Engine)...)
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
		index.Sessions = append(index.Sessions, session)
	}
	sort.Slice(index.Sessions, func(i, j int) bool { return index.Sessions[i].Name < index.Sessions[j].Name })
	index.ProviderFacts = dedupeProviderFacts(index.ProviderFacts)
	return index, nil
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

func providerFactsForEngine(target, engine string) []ProviderFact {
	capability := providerCapabilityDescriptor(engine)
	switch strings.TrimSpace(engine) {
	case "claude-code":
		return []ProviderFact{
			{Target: target, Engine: engine, Fact: capability.runtimeFact()},
			{Target: target, Engine: engine, Fact: capability.nativeFact("Claude")},
			{Target: target, Engine: engine, Fact: capability.limitFact("Claude")},
			{Target: target, Engine: engine, Fact: "GoalX bootstraps a project-local PermissionRequest hook so unattended Claude MCP permission dialogs can be auto-allowed."},
			{Target: target, Engine: engine, Fact: "GoalX bootstraps a project-local Elicitation hook so unattended Claude MCP user-input or browser-auth requests are cancelled instead of hanging forever."},
			{Target: target, Engine: engine, Fact: "If a Claude permission or elicitation dialog still surfaces, GoalX writes an urgent master-inbox fact through a Notification hook so the run can recover."},
			{Target: target, Engine: engine, Fact: "Write/Edit requires prior read of the target file."},
			{Target: target, Engine: engine, Fact: "Direct large-file edits can fail when the provider read window is exceeded."},
		}
	case "codex":
		return []ProviderFact{
			{Target: target, Engine: engine, Fact: capability.runtimeFact()},
			{Target: target, Engine: engine, Fact: capability.nativeFact("Codex")},
			{Target: target, Engine: engine, Fact: "Configured MCP servers are usable without an extra GoalX approval layer in this environment."},
			{Target: target, Engine: engine, Fact: "Native subagents require explicit invocation."},
		}
	default:
		return nil
	}
}

func dedupeProviderFacts(facts []ProviderFact) []ProviderFact {
	if len(facts) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	out := make([]ProviderFact, 0, len(facts))
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

type providerCapabilityDescriptorState struct {
	native string
	limit  string
}

func providerCapabilityDescriptor(engine string) providerCapabilityDescriptorState {
	switch strings.TrimSpace(engine) {
	case "claude-code":
		return providerCapabilityDescriptorState{
			native: "skills,plugins,mcp",
			limit:  "claude_root_no_bypass",
		}
	case "codex":
		return providerCapabilityDescriptorState{
			native: "skills,mcp",
		}
	default:
		return providerCapabilityDescriptorState{}
	}
}

func (d providerCapabilityDescriptorState) summary() string {
	parts := []string{"provider_capability=tui"}
	if strings.TrimSpace(d.native) != "" {
		parts = append(parts, "provider_native="+d.native)
	}
	if strings.TrimSpace(d.limit) != "" {
		parts = append(parts, "provider_limit="+d.limit)
	}
	if len(parts) == 1 && strings.TrimSpace(d.native) == "" && strings.TrimSpace(d.limit) == "" {
		return ""
	}
	return strings.Join(parts, " ")
}

func (d providerCapabilityDescriptorState) runtimeFact() string {
	return "GoalX canonical provider runtime is tmux + interactive TUI."
}

func (d providerCapabilityDescriptorState) nativeFact(provider string) string {
	switch strings.TrimSpace(d.native) {
	case "skills,plugins,mcp":
		return "Interactive " + provider + " sessions can use installed skills, plugins, and MCP servers from the native TUI."
	case "skills,mcp":
		return "Interactive " + provider + " sessions can use installed skills and configured MCP servers from the native TUI."
	default:
		return ""
	}
}

func (d providerCapabilityDescriptorState) limitFact(provider string) string {
	switch strings.TrimSpace(d.limit) {
	case "claude_root_no_bypass":
		return provider + " root sessions cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions."
	default:
		return ""
	}
}
