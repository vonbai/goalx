package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

type serveApp struct {
	cfg           goalx.ServeConfig
	sessionExists func(string) bool
	runCLI        func(projectRoot, action string, args []string) (string, error)
	sendNudge     func(target, engine string) error
}

type serveProject struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	ProjectID string `json:"project_id"`
}

type serveRun struct {
	Workspace string `json:"workspace"`
	ProjectID string `json:"project_id"`
	Name      string `json:"name"`
	Mode      string `json:"mode"`
	Objective string `json:"objective,omitempty"`
	Status    string `json:"status"`
}

type serveActionRequest struct {
	Objective  string   `json:"objective"`
	Mode       string   `json:"mode"`
	Parallel   int      `json:"parallel"`
	Name       string   `json:"name"`
	Context    []string `json:"context"`
	Strategies []string `json:"strategies"`
	Run        string   `json:"run"`
	Session    string   `json:"session"`
	Direction  string   `json:"direction"`
	Message    string   `json:"message"`
	Content    string   `json:"content"`
}

var serveOutputMu sync.Mutex

func Serve(projectRoot string, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("usage: goalx serve")
	}

	cfg, _, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := validateServeConfig(cfg.Serve); err != nil {
		return err
	}

	return http.ListenAndServe(cfg.Serve.Bind, newServeHandler(cfg.Serve))
}

func newServeHandler(cfg goalx.ServeConfig) http.Handler {
	return newServeApp(cfg).routes()
}

func newServeApp(cfg goalx.ServeConfig) *serveApp {
	app := &serveApp{
		cfg:           cfg,
		sessionExists: SessionExists,
		sendNudge:     SendAgentNudge,
	}
	app.runCLI = app.runCLIAction
	return app
}

func (a *serveApp) routes() http.Handler {
	return http.HandlerFunc(a.serveHTTP)
}

func (a *serveApp) serveHTTP(w http.ResponseWriter, r *http.Request) {
	if err := a.authorize(r); err != nil {
		writeJSONError(w, http.StatusUnauthorized, err)
		return
	}

	path := strings.Trim(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodGet && path == "projects":
		a.handleProjects(w)
	case r.Method == http.MethodGet && path == "runs":
		a.handleRuns(w)
	case r.Method == http.MethodGet && path == "workspaces":
		a.handleListWorkspaces(w)
	case r.Method == http.MethodPost && path == "workspaces":
		a.handleAddWorkspace(w, r)
	case strings.HasPrefix(path, "workspaces/") && r.Method == http.MethodDelete:
		name := strings.TrimPrefix(path, "workspaces/")
		a.handleRemoveWorkspace(w, name)
	case strings.HasPrefix(path, "projects/"):
		a.handleProjectRoutes(w, r, strings.Split(path, "/"))
	default:
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown route"))
	}
}

func (a *serveApp) handleProjects(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{
		"projects": a.projects(),
	})
}

func (a *serveApp) handleProjectRoutes(w http.ResponseWriter, r *http.Request, parts []string) {
	project, ok := a.project(parts[1])
	if !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown project %q", parts[1]))
		return
	}

	if len(parts) == 2 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{
			"project": project,
		})
		return
	}

	if len(parts) == 4 && parts[2] == "goalx" && r.Method == http.MethodPost {
		a.handleGoalxAction(w, r, project, parts[3])
		return
	}

	writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown route"))
}

func (a *serveApp) handleGoalxAction(w http.ResponseWriter, r *http.Request, project serveProject, action string) {
	req, err := decodeServeActionRequest(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	switch action {
	case "config":
		a.handleConfigAction(w, project.Path, req)
		return
	case "tell":
		a.handleTellAction(w, project.Path, req)
		return
	}

	args, err := buildServeCLIArgs(action, req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	output, err := a.runCLI(project.Path, action, args)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("%s: %w", action, err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"action": action,
		"output": output,
	})
}

func (a *serveApp) handleConfigAction(w http.ResponseWriter, projectRoot string, req serveActionRequest) {
	cfgPath := filepath.Join(projectRoot, ".goalx", "goalx.yaml")
	if req.Run != "" {
		cfgPath = filepath.Join(goalx.RunDir(projectRoot, req.Run), "goalx.yaml")
	}
	content := req.Content
	if content != "" {
		if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
	} else {
		data, err := os.ReadFile(cfgPath)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, err)
			return
		}
		content = string(data)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"action":  "config",
		"path":    cfgPath,
		"content": content,
	})
}

func (a *serveApp) handleRuns(w http.ResponseWriter) {
	runs, err := a.runs()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"runs": runs,
	})
}

func (a *serveApp) authorize(r *http.Request) error {
	if a.cfg.Token == "" {
		return nil
	}
	auth := r.Header.Get("Authorization")
	if auth != "Bearer "+a.cfg.Token {
		return fmt.Errorf("missing or invalid bearer token")
	}
	return nil
}

func (a *serveApp) projects() []serveProject {
	projects := make([]serveProject, 0, len(a.cfg.Workspaces))
	for name, path := range a.cfg.Workspaces {
		projects = append(projects, serveProject{
			Name:      name,
			Path:      path,
			ProjectID: goalx.ProjectID(path),
		})
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})
	return projects
}

func (a *serveApp) project(name string) (serveProject, bool) {
	path, ok := a.cfg.Workspaces[name]
	if !ok {
		return serveProject{}, false
	}
	return serveProject{
		Name:      name,
		Path:      path,
		ProjectID: goalx.ProjectID(path),
	}, true
}

func (a *serveApp) runs() ([]serveRun, error) {
	home, _ := os.UserHomeDir()
	var runs []serveRun

	for _, project := range a.projects() {
		runsDir := filepath.Join(home, ".goalx", "runs", project.ProjectID)
		entries, err := os.ReadDir(runsDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read runs for %s: %w", project.Name, err)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(runsDir, entry.Name(), "goalx.yaml"))
			if err != nil || cfg.Name == "" {
				continue
			}

			status := "completed"
			if a.sessionExists(goalx.TmuxSessionName(project.Path, cfg.Name)) {
				status = "active"
			}

			runs = append(runs, serveRun{
				Workspace: project.Name,
				ProjectID: project.ProjectID,
				Name:      cfg.Name,
				Mode:      string(cfg.Mode),
				Objective: cfg.Objective,
				Status:    status,
			})
		}
	}

	sort.Slice(runs, func(i, j int) bool {
		if runs[i].Workspace == runs[j].Workspace {
			return runs[i].Name < runs[j].Name
		}
		return runs[i].Workspace < runs[j].Workspace
	})

	return runs, nil
}

func (a *serveApp) handleTellAction(w http.ResponseWriter, projectRoot string, req serveActionRequest) {
	if req.Run == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("run is required"))
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("message is required"))
		return
	}

	rc, err := ResolveRun(projectRoot, req.Run)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	session := req.Session
	if session == "" {
		session = "master"
	}

	if session == "master" {
		if _, err := AppendMasterInboxMessage(rc.RunDir, "tell", "user", message); err != nil {
			writeJSONError(w, http.StatusInternalServerError, err)
			return
		}
		target := rc.TmuxSession + ":master"
		if err := a.sendNudge(target, rc.Config.Master.Engine); err != nil {
			writeJSONError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":     true,
			"action": "tell",
			"target": target,
		})
		return
	}

	idx, err := parseSessionIndex(session)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	ok, err := hasSessionIndex(rc.RunDir, idx)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("session %q out of range for run %q", session, rc.Name))
		return
	}

	guidancePath := GuidancePath(rc.RunDir, session)
	if err := os.MkdirAll(filepath.Dir(guidancePath), 0o755); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if err := os.WriteFile(guidancePath, []byte(message+"\n"), 0o644); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err)
		return
	}

	windowName, err := resolveWindowName(rc.Name, session)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}
	target := rc.TmuxSession + ":" + windowName
	effective := goalx.EffectiveSessionConfig(rc.Config, idx-1)
	if err := a.sendNudge(target, effective.Engine); err != nil {
		writeJSONError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":            true,
		"action":        "tell",
		"target":        target,
		"guidance_path": guidancePath,
	})
}

func validateServeConfig(cfg goalx.ServeConfig) error {
	if cfg.Bind == "" {
		return fmt.Errorf("serve.bind is required")
	}
	if err := validateServeBind(cfg.Bind); err != nil {
		return err
	}
	if cfg.Token == "" {
		return fmt.Errorf("serve.token is required")
	}
	if len(cfg.Workspaces) == 0 {
		return fmt.Errorf("serve.workspaces is required")
	}
	return nil
}

func validateServeBind(bind string) error {
	host, _, err := net.SplitHostPort(bind)
	if err != nil {
		return fmt.Errorf("invalid serve.bind %q: %w", bind, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("serve.bind host must be an IP address, got %q", host)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("serve.bind must not bind 0.0.0.0 — use a specific IP")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

func decodeServeActionRequest(r *http.Request) (serveActionRequest, error) {
	var req serveActionRequest
	if r.Body == nil {
		return req, nil
	}
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil && err != io.EOF {
		return req, fmt.Errorf("decode request: %w", err)
	}
	return req, nil
}

func buildServeCLIArgs(action string, req serveActionRequest) ([]string, error) {
	switch action {
	case "init":
		if strings.TrimSpace(req.Objective) == "" {
			return nil, fmt.Errorf("objective is required")
		}
		return buildServeStartInitArgs(req), nil
	case "start":
		if strings.TrimSpace(req.Objective) == "" {
			return nil, nil
		}
		return buildServeStartInitArgs(req), nil
	case "auto":
		if strings.TrimSpace(req.Objective) == "" {
			return nil, fmt.Errorf("objective is required")
		}
		return buildServeStartInitArgs(req), nil
	case "observe", "stop", "save", "drop":
		return buildServeRunArgs(req.Run), nil
	case "status":
		return buildServeStatusArgs(req.Run, req.Session), nil
	case "add":
		if strings.TrimSpace(req.Direction) == "" {
			return nil, fmt.Errorf("direction is required")
		}
		args := []string{req.Direction}
		if req.Run != "" {
			args = append(args, "--run", req.Run)
		}
		return args, nil
	case "keep":
		if strings.TrimSpace(req.Session) == "" {
			return nil, fmt.Errorf("session is required")
		}
		args := buildServeRunArgs(req.Run)
		args = append(args, req.Session)
		return args, nil
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}

func buildServeStartInitArgs(req serveActionRequest) []string {
	args := []string{req.Objective}
	switch req.Mode {
	case string(goalx.ModeResearch):
		args = append(args, "--research")
	case string(goalx.ModeDevelop):
		args = append(args, "--develop")
	}
	if req.Parallel > 0 {
		args = append(args, "--parallel", strconv.Itoa(req.Parallel))
	}
	if req.Name != "" {
		args = append(args, "--name", req.Name)
	}
	if len(req.Context) > 0 {
		args = append(args, "--context", strings.Join(req.Context, ","))
	}
	if len(req.Strategies) > 0 {
		args = append(args, "--strategy", strings.Join(req.Strategies, ","))
	}
	return args
}

func buildServeRunArgs(run string) []string {
	if run == "" {
		return nil
	}
	return []string{"--run", run}
}

func buildServeStatusArgs(run, session string) []string {
	args := buildServeRunArgs(run)
	if session != "" {
		args = append(args, session)
	}
	return args
}

func (a *serveApp) runCLIAction(projectRoot, action string, args []string) (string, error) {
	return captureServeOutput(func() error {
		switch action {
		case "init":
			return Init(projectRoot, args)
		case "start":
			return Start(projectRoot, args)
		case "auto":
			return Auto(projectRoot, args)
		case "observe":
			return Observe(projectRoot, args)
		case "status":
			return Status(projectRoot, args)
		case "add":
			return Add(projectRoot, args)
		case "stop":
			return Stop(projectRoot, args)
		case "save":
			return Save(projectRoot, args)
		case "keep":
			return Keep(projectRoot, args)
		case "drop":
			return Drop(projectRoot, args)
		default:
			return fmt.Errorf("unsupported action %q", action)
		}
	})
}

func captureServeOutput(fn func() error) (string, error) {
	serveOutputMu.Lock()
	defer serveOutputMu.Unlock()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return "", err
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return "", err
	}

	os.Stdout = stdoutW
	os.Stderr = stderrW

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdoutBuf, stdoutR)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderrBuf, stderrR)
	}()

	runErr := fn()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	wg.Wait()
	_ = stdoutR.Close()
	_ = stderrR.Close()

	output := strings.TrimSpace(stdoutBuf.String())
	stderrText := strings.TrimSpace(stderrBuf.String())
	if stderrText != "" {
		if output != "" {
			output += "\n" + stderrText
		} else {
			output = stderrText
		}
	}

	return output, runErr
}

// Workspace management API

func (a *serveApp) handleListWorkspaces(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{
		"workspaces": a.cfg.Workspaces,
	})
}

func (a *serveApp) handleAddWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}
	if req.Name == "" || req.Path == "" {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("name and path are required"))
		return
	}

	// Verify path exists
	info, err := os.Stat(req.Path)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("path %q does not exist", req.Path))
		return
	}
	if !info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, fmt.Errorf("path %q is not a directory", req.Path))
		return
	}

	// Auto git-init if not a git repo
	if _, err := os.Stat(filepath.Join(req.Path, ".git")); os.IsNotExist(err) {
		if initErr := exec.Command("git", "-C", req.Path, "init").Run(); initErr != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("git init failed: %w", initErr))
			return
		}
		exec.Command("git", "-C", req.Path, "add", "-A").Run()
		exec.Command("git", "-C", req.Path, "commit", "-m", "init: project scaffold").Run()
	}

	if a.cfg.Workspaces == nil {
		a.cfg.Workspaces = make(map[string]string)
	}
	a.cfg.Workspaces[req.Name] = req.Path

	// Persist to config
	if err := a.saveWorkspaces(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("save config: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"added":      req.Name,
		"path":       req.Path,
		"git_inited": true,
	})
}

func (a *serveApp) handleRemoveWorkspace(w http.ResponseWriter, name string) {
	if _, ok := a.cfg.Workspaces[name]; !ok {
		writeJSONError(w, http.StatusNotFound, fmt.Errorf("workspace %q not found", name))
		return
	}
	delete(a.cfg.Workspaces, name)

	if err := a.saveWorkspaces(); err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Errorf("save config: %w", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"removed": name})
}

func (a *serveApp) saveWorkspaces() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(home, ".goalx", "config.yaml")

	// Load existing config, update workspaces only
	var raw map[string]any
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		yaml.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = make(map[string]any)
	}

	serve, _ := raw["serve"].(map[string]any)
	if serve == nil {
		serve = make(map[string]any)
	}
	serve["workspaces"] = a.cfg.Workspaces
	raw["serve"] = serve

	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(cfgPath, out, 0o644)
}
