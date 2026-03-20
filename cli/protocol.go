package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	goalx "github.com/vonbai/goalx"
)

// ProtocolData is passed to master.md.tmpl and program.md.tmpl.
type ProtocolData struct {
	Objective      string
	Description    string
	Mode           goalx.Mode
	Sessions       []SessionData
	Master         goalx.MasterConfig
	Harness        goalx.HarnessConfig
	Budget         goalx.BudgetConfig
	Target         goalx.TargetConfig
	Context        goalx.ContextConfig
	TmuxSession    string
	ProjectRoot       string
	SummaryPath       string
	AcceptancePath    string
	MasterJournalPath string
	StatusPath        string // .goalx/status.json for external progress reporting
	EngineCommand     string // resolved master engine command

	// Subagent-specific (used in program.md.tmpl)
	SessionName   string
	JournalPath   string
	GuidancePath  string
	WorktreePath  string
	DiversityHint string
}

// SessionData is per-session info for the master protocol.
type SessionData struct {
	Name          string
	WindowName    string
	WorktreePath  string
	JournalPath   string
	GuidancePath  string
	EngineCommand string
	Prompt        string
}

// RenderMasterProtocol renders master.md.tmpl to the run directory.
func RenderMasterProtocol(data ProtocolData, runDir string) error {
	return renderTemplate("templates/master.md.tmpl", filepath.Join(runDir, "master.md"), data)
}

// RenderSubagentProtocol renders program.md.tmpl for a specific session.
func RenderSubagentProtocol(data ProtocolData, runDir string, sessionIdx int) error {
	outPath := filepath.Join(runDir, sessionName(sessionIdx)+".md")
	return renderTemplate("templates/program.md.tmpl", outPath, data)
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

	t, err := template.New(filepath.Base(tmplPath)).Parse(string(tmplContent))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return err
	}

	return os.WriteFile(outPath, buf.Bytes(), 0644)
}
