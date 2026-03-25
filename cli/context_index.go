package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ContextIndex struct {
	Version             int              `json:"version"`
	CheckedAt           string           `json:"checked_at,omitempty"`
	ProjectRoot         string           `json:"project_root,omitempty"`
	RunDir              string           `json:"run_dir,omitempty"`
	RunName             string           `json:"run_name,omitempty"`
	RunWorktree         string           `json:"run_worktree,omitempty"`
	ReportsDir          string           `json:"reports_dir,omitempty"`
	CharterPath         string           `json:"charter_path,omitempty"`
	GoalPath            string           `json:"goal_path,omitempty"`
	AcceptanceStatePath string           `json:"acceptance_state_path,omitempty"`
	CompletionProofPath string           `json:"completion_proof_path,omitempty"`
	CoordinationPath    string           `json:"coordination_path,omitempty"`
	SummaryPath         string           `json:"summary_path,omitempty"`
	ControlDir          string           `json:"control_dir,omitempty"`
	ActivityPath        string           `json:"activity_path,omitempty"`
	AffordancesJSONPath string           `json:"affordances_json_path,omitempty"`
	AffordancesMarkdown string           `json:"affordances_markdown_path,omitempty"`
	ContextIndexPath    string           `json:"context_index_path,omitempty"`
	DimensionsPath      string           `json:"dimensions_path,omitempty"`
	Master              ContextMaster    `json:"master"`
	Sessions            []ContextSession `json:"sessions,omitempty"`
	ClaudeCodeAvailable bool             `json:"claude_code_available,omitempty"`
	CodexAvailable      bool             `json:"codex_available,omitempty"`
	GitAvailable        bool             `json:"git_available,omitempty"`
	TmuxAvailable       bool             `json:"tmux_available,omitempty"`
}

type ContextMaster struct {
	Engine string `json:"engine,omitempty"`
	Model  string `json:"model,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

type ContextSession struct {
	Name         string `json:"name,omitempty"`
	Mode         string `json:"mode,omitempty"`
	WindowName   string `json:"window_name,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
	JournalPath  string `json:"journal_path,omitempty"`
	InboxPath    string `json:"inbox_path,omitempty"`
	CursorPath   string `json:"cursor_path,omitempty"`
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
	index := &ContextIndex{
		Version:             1,
		CheckedAt:           time.Now().UTC().Format(time.RFC3339),
		ProjectRoot:         projectRoot,
		RunDir:              runDir,
		RunName:             runName,
		RunWorktree:         RunWorktreePath(runDir),
		ReportsDir:          ReportsDir(runDir),
		CharterPath:         RunCharterPath(runDir),
		GoalPath:            GoalPath(runDir),
		AcceptanceStatePath: AcceptanceStatePath(runDir),
		CompletionProofPath: CompletionStatePath(runDir),
		CoordinationPath:    CoordinationPath(runDir),
		SummaryPath:         filepath.Join(runDir, "summary.md"),
		ControlDir:          ControlDir(runDir),
		ActivityPath:        ActivityPath(runDir),
		AffordancesJSONPath: AffordancesJSONPath(runDir),
		AffordancesMarkdown: AffordancesMarkdownPath(runDir),
		ContextIndexPath:    ContextIndexPath(runDir),
		DimensionsPath:      ControlDimensionsPath(runDir),
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
		}
		if session.Mode == "" {
			if sessionsState, err := EnsureSessionsRuntimeState(runDir); err == nil {
				if current, ok := sessionsState.Sessions[name]; ok {
					session.Mode = current.Mode
				}
			}
		}
		index.Sessions = append(index.Sessions, session)
	}
	sort.Slice(index.Sessions, func(i, j int) bool { return index.Sessions[i].Name < index.Sessions[j].Name })
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
