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
	Mode                   goalx.Mode
	Engine                 string
	Engines                map[string]goalx.EngineConfig
	Sessions               []SessionData
	Master                 goalx.MasterConfig
	Harness                goalx.HarnessConfig
	Budget                 goalx.BudgetConfig
	Target                 goalx.TargetConfig
	Context                goalx.ContextConfig
	Preferences            goalx.PreferencesConfig
	TmuxSession            string
	ProjectRoot            string
	SummaryPath            string
	GoalPath               string
	GoalLogPath            string
	CharterPath            string
	IdentityFencePath      string
	AcceptanceNotesPath    string
	AcceptanceStatePath    string
	CompletionProofPath    string
	RunStatePath           string
	SessionsStatePath      string
	ReportsDir             string
	DimensionsPath         string
	Routing                goalx.RoutingTableConfig
	DimensionsCatalog      map[string]string
	ProjectRegistryPath    string
	RunMetadataPath        string
	CoordinationPath       string
	MasterInboxPath        string
	MasterCursorPath       string
	ControlRunIdentityPath string
	ControlRunStatePath    string
	MasterLeasePath        string
	SidecarLeasePath       string
	LivenessPath           string
	WorktreeSnapshotPath   string
	ControlRemindersPath   string
	ControlDeliveriesPath  string
	MasterJournalPath      string
	StatusPath             string // user-scoped status cache for external progress reporting
	EngineCommand          string // resolved master engine command

	// Subagent-specific (used in program.md.tmpl)
	SessionName         string
	SessionIndex        int // 0-based index of this session in the Sessions slice
	JournalPath         string
	SessionIdentityPath string
	SessionInboxPath    string
	SessionCursorPath   string
	WorktreePath        string
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
