package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	goalx "github.com/vonbai/goalx"
)

// ProtocolData is passed to master.md.tmpl and program.md.tmpl.
type ProtocolData struct {
	RunName                string
	Objective              string
	Description            string
	Intent                 string
	CurrentTime            string
	RunStartedAt           string
	ExperimentsLogPath     string
	Mode                   goalx.Mode
	Engine                 string
	Engines                map[string]goalx.EngineConfig
	Sessions               []SessionData
	Master                 goalx.MasterConfig
	Roles                  goalx.RoleDefaultsConfig
	LocalValidationCommand string
	Budget                 goalx.BudgetConfig
	Target                 goalx.TargetConfig
	Context                goalx.ContextConfig
	Preferences            goalx.PreferencesConfig
	TmuxSession            string
	ProjectRoot            string
	RunWorktreePath        string
	SummaryPath            string
	ObjectiveContractPath  string
	ObligationModelPath    string
	ObligationLogPath      string
	AssurancePlanPath      string
	EvidenceLogPath        string
	SuccessModelPath       string
	ProofPlanPath          string
	WorkflowPlanPath       string
	DomainPackPath         string
	CompilerInputPath      string
	CompilerReportPath     string
	InterventionLogPath    string
	IntegrationStatePath   string
	CharterPath            string
	IdentityFencePath      string
	CompletionProofPath    string
	RunStatePath           string
	SessionsStatePath      string
	ReportsDir             string
	DimensionsPath         string
	DimensionsCatalog      map[string]string
	ProjectRegistryPath    string
	RunMetadataPath        string
	CoordinationPath       string
	MasterInboxPath        string
	MasterCursorPath       string
	ControlRunIdentityPath string
	ControlRunStatePath    string
	MasterLeasePath        string
	LivenessPath           string
	WorktreeSnapshotPath   string
	ControlRemindersPath   string
	ControlDeliveriesPath  string
	ActivityPath           string
	ContextIndexPath       string
	AffordancesPath        string
	SelectionSnapshotPath  string
	SelectionPolicy        goalx.EffectiveSelectionPolicy
	MasterJournalPath      string
	StatusPath             string // run-scoped master-written status record
	EngineCommand          string // resolved master engine command
	Composition            ProtocolComposition

	// Subagent-specific (used in program.md.tmpl)
	SessionName               string
	SessionIndex              int // 0-based index of this session in the Sessions slice
	CurrentDimensions         []goalx.ResolvedDimension
	JournalPath               string
	SessionIdentityPath       string
	SessionInboxPath          string
	SessionCursorPath         string
	WorktreePath              string
	SessionBaseBranchSelector string
	SessionBaseBranch         string
}

type ProtocolComposition struct {
	Enabled            bool
	Philosophy         []string
	BehaviorContract   []string
	RequiredRoles      []string
	RequiredGates      []string
	RequiredProofKinds []string
	SourceSlots        []ProtocolCompositionSlot
	OutputSources      []ProtocolCompositionOutput
	SelectedPriorRefs  []string
}

type ProtocolCompositionSlot struct {
	Slot string
	Refs []string
}

type ProtocolCompositionOutput struct {
	Output     string
	SourceSlot string
	Refs       []string
}

// SessionData is per-session info for the master protocol.
type SessionData struct {
	Name              string
	WindowName        string
	WorktreePath      string
	JournalPath       string
	SessionInboxPath  string
	SessionCursorPath string
	Engine            string
	Model             string
	Mode              goalx.Mode
	Hint              string
	Dimensions        []goalx.ResolvedDimension
	EngineCommand     string
	Prompt            string
}

// RenderMasterProtocol renders master.md.tmpl to the run directory.
func RenderMasterProtocol(data ProtocolData, runDir string) error {
	data = normalizeProtocolData(data, runDir)
	return renderTemplate("templates/master.md.tmpl", filepath.Join(runDir, "master.md"), data)
}

// RenderSubagentProtocol renders program.md.tmpl for a specific session.
func RenderSubagentProtocol(data ProtocolData, runDir string, sessionIdx int) error {
	data = normalizeProtocolData(data, runDir)
	outPath := filepath.Join(runDir, sessionName(sessionIdx)+".md")
	return renderTemplate("templates/program.md.tmpl", outPath, data)
}

func normalizeProtocolData(data ProtocolData, runDir string) ProtocolData {
	if data.ObligationLogPath == "" && data.ObligationModelPath != "" {
		data.ObligationLogPath = filepath.Join(filepath.Dir(data.ObligationModelPath), "obligation-log.jsonl")
	}
	if data.ObjectiveContractPath == "" && runDir != "" {
		data.ObjectiveContractPath = ObjectiveContractPath(runDir)
	}
	if data.SuccessModelPath == "" && runDir != "" {
		data.SuccessModelPath = SuccessModelPath(runDir)
	}
	if data.ProofPlanPath == "" && runDir != "" {
		data.ProofPlanPath = ProofPlanPath(runDir)
	}
	if data.WorkflowPlanPath == "" && runDir != "" {
		data.WorkflowPlanPath = WorkflowPlanPath(runDir)
	}
	if data.DomainPackPath == "" && runDir != "" {
		data.DomainPackPath = DomainPackPath(runDir)
	}
	if data.CompilerInputPath == "" && runDir != "" {
		data.CompilerInputPath = CompilerInputPath(runDir)
	}
	if data.CompilerReportPath == "" && runDir != "" {
		data.CompilerReportPath = CompilerReportPath(runDir)
	}
	if data.InterventionLogPath == "" && runDir != "" {
		data.InterventionLogPath = InterventionLogPath(runDir)
	}
	if data.IntegrationStatePath == "" && runDir != "" {
		data.IntegrationStatePath = IntegrationStatePath(runDir)
	}
	if data.RunWorktreePath == "" && runDir != "" {
		data.RunWorktreePath = RunWorktreePath(runDir)
	}
	if data.ExperimentsLogPath == "" && runDir != "" {
		data.ExperimentsLogPath = ExperimentsLogPath(runDir)
	}
	if data.CharterPath == "" && runDir != "" {
		data.CharterPath = RunCharterPath(runDir)
	}
	if data.IdentityFencePath == "" && runDir != "" {
		data.IdentityFencePath = IdentityFencePath(runDir)
	}
	if data.SessionIdentityPath == "" && runDir != "" && data.SessionName != "" {
		data.SessionIdentityPath = SessionIdentityPath(runDir, data.SessionName)
	}
	if data.ReportsDir == "" && runDir != "" {
		data.ReportsDir = ReportsDir(runDir)
	}
	if data.DimensionsPath == "" && runDir != "" {
		data.DimensionsPath = ControlDimensionsPath(runDir)
	}
	if data.ActivityPath == "" && runDir != "" {
		data.ActivityPath = ActivityPath(runDir)
	}
	if data.ContextIndexPath == "" && runDir != "" {
		data.ContextIndexPath = ContextIndexPath(runDir)
	}
	if data.AffordancesPath == "" && runDir != "" {
		data.AffordancesPath = AffordancesMarkdownPath(runDir)
	}
	if composition, err := buildProtocolComposition(runDir, data.Composition); err == nil {
		data.Composition = composition
	}
	return data
}

func buildProtocolComposition(runDir string, existing ProtocolComposition) (ProtocolComposition, error) {
	composition := normalizeProtocolComposition(existing)
	if composition.Enabled || strings.TrimSpace(runDir) == "" {
		return composition, nil
	}

	successModel, err := LoadSuccessModel(SuccessModelPath(runDir))
	if err != nil {
		return composition, err
	}
	proofPlan, err := LoadProofPlan(ProofPlanPath(runDir))
	if err != nil {
		return composition, err
	}
	workflowPlan, err := LoadWorkflowPlan(WorkflowPlanPath(runDir))
	if err != nil {
		return composition, err
	}
	compilerInput, err := LoadCompilerInput(CompilerInputPath(runDir))
	if err != nil {
		return composition, err
	}
	compilerReport, err := LoadCompilerReport(CompilerReportPath(runDir))
	if err != nil {
		return composition, err
	}
	if successModel == nil && proofPlan == nil && workflowPlan == nil && compilerInput == nil && compilerReport == nil {
		return composition, nil
	}

	composition.Enabled = true
	composition.Philosophy = compactStrings([]string{
		"durable_state_first",
		"dispatch_before_self_implementation",
		"success_model_before_local_optimization",
		"evidence_before_completion",
		"localized_override_not_reset",
		"thin_control_explicit_judgment",
	})
	composition.BehaviorContract = compactStrings([]string{
		"compact_decisive_output",
		"automatic_follow_through",
		"durable_state_first_recovery",
		"localized_override_semantics",
		"evidence_backed_completion",
		"workflow_gates_are_real",
	})
	if workflowPlan != nil {
		for _, role := range workflowPlan.RequiredRoles {
			if role.Required {
				composition.RequiredRoles = append(composition.RequiredRoles, role.ID)
			}
		}
		composition.RequiredGates = append(composition.RequiredGates, workflowPlan.Gates...)
	}
	if proofPlan != nil {
		seenProofKinds := make(map[string]struct{}, len(proofPlan.Items))
		for _, item := range proofPlan.Items {
			key := strings.TrimSpace(item.Kind)
			if key == "" {
				continue
			}
			if _, ok := seenProofKinds[key]; ok {
				continue
			}
			seenProofKinds[key] = struct{}{}
			composition.RequiredProofKinds = append(composition.RequiredProofKinds, key)
		}
	}
	if compilerInput != nil {
		for _, slot := range compilerInput.SourceSlots {
			composition.SourceSlots = append(composition.SourceSlots, ProtocolCompositionSlot{
				Slot: slot.Slot,
				Refs: append([]string(nil), slot.Refs...),
			})
		}
		composition.SelectedPriorRefs = append(composition.SelectedPriorRefs, compilerInput.SelectedPriorRefs...)
	}
	if compilerReport != nil {
		if len(compilerReport.SelectedPriorRefs) > 0 {
			composition.SelectedPriorRefs = append([]string(nil), compilerReport.SelectedPriorRefs...)
		}
		if len(composition.SourceSlots) == 0 {
			for _, slot := range compilerReport.AvailableSourceSlots {
				composition.SourceSlots = append(composition.SourceSlots, ProtocolCompositionSlot{
					Slot: slot.Slot,
					Refs: append([]string(nil), slot.Refs...),
				})
			}
		}
		for _, output := range compilerReport.OutputSources {
			composition.OutputSources = append(composition.OutputSources, ProtocolCompositionOutput{
				Output:     output.Output,
				SourceSlot: output.SourceSlot,
				Refs:       append([]string(nil), output.Refs...),
			})
		}
	}
	return normalizeProtocolComposition(composition), nil
}

func normalizeProtocolComposition(composition ProtocolComposition) ProtocolComposition {
	composition.Philosophy = compactStrings(composition.Philosophy)
	composition.BehaviorContract = compactStrings(composition.BehaviorContract)
	composition.RequiredRoles = compactStrings(composition.RequiredRoles)
	composition.RequiredGates = compactStrings(composition.RequiredGates)
	composition.RequiredProofKinds = compactStrings(composition.RequiredProofKinds)
	composition.SelectedPriorRefs = compactStrings(composition.SelectedPriorRefs)
	if composition.SourceSlots == nil {
		composition.SourceSlots = []ProtocolCompositionSlot{}
	}
	for i := range composition.SourceSlots {
		composition.SourceSlots[i].Slot = strings.TrimSpace(composition.SourceSlots[i].Slot)
		composition.SourceSlots[i].Refs = compactStrings(composition.SourceSlots[i].Refs)
	}
	if composition.OutputSources == nil {
		composition.OutputSources = []ProtocolCompositionOutput{}
	}
	for i := range composition.OutputSources {
		composition.OutputSources[i].Output = strings.TrimSpace(composition.OutputSources[i].Output)
		composition.OutputSources[i].SourceSlot = strings.TrimSpace(composition.OutputSources[i].SourceSlot)
		composition.OutputSources[i].Refs = compactStrings(composition.OutputSources[i].Refs)
	}
	composition.Enabled = composition.Enabled ||
		len(composition.Philosophy) > 0 ||
		len(composition.BehaviorContract) > 0 ||
		len(composition.RequiredRoles) > 0 ||
		len(composition.RequiredGates) > 0 ||
		len(composition.RequiredProofKinds) > 0 ||
		len(composition.SourceSlots) > 0 ||
		len(composition.OutputSources) > 0 ||
		len(composition.SelectedPriorRefs) > 0
	return composition
}

func existingProtocolPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return ""
	}
	return path
}

func sessionName(idx int) string {
	return fmt.Sprintf("program-%d", idx+1)
}

func renderTemplate(tmplPath, outPath string, data any) error {
	// Use embedded templates from the goalx package
	tmplContent, err := goalx.Templates.ReadFile(tmplPath)
	if err != nil {
		return fmt.Errorf("embedded template %s: %w", tmplPath, err)
	}

	t, err := template.New(filepath.Base(tmplPath)).Funcs(template.FuncMap{
		"join": strings.Join,
	}).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	return os.WriteFile(outPath, buf.Bytes(), 0644)
}
