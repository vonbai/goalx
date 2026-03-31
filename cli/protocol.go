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
	SuccessModelPath       string
	ProofPlanPath          string
	WorkflowPlanPath       string
	DomainPackPath         string
	CompilerInputPath      string
	CompilerReportPath     string
	GoalPath               string
	GoalLogPath            string
	InterventionLogPath    string
	IntegrationStatePath   string
	CharterPath            string
	IdentityFencePath      string
	AcceptanceNotesPath    string
	AcceptanceStatePath    string
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
	if data.GoalLogPath == "" && data.GoalPath != "" {
		data.GoalLogPath = filepath.Join(filepath.Dir(data.GoalPath), "goal-log.jsonl")
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
	return data
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
