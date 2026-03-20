package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	goalx "github.com/vonbai/goalx"
)

type serveApp struct {
	cfg           goalx.ServeConfig
	sessionExists func(string) bool
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
	return &serveApp{
		cfg:           cfg,
		sessionExists: SessionExists,
	}
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
	if len(parts) == 2 && r.Method == http.MethodGet {
		project, ok := a.project(parts[1])
		if !ok {
			writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown project %q", parts[1]))
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"project": project,
		})
		return
	}

	writeJSONError(w, http.StatusNotFound, fmt.Errorf("unknown route"))
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
		return fmt.Errorf("serve.bind must not bind all interfaces")
	}
	if !isTailscaleIP(ip) {
		return fmt.Errorf("serve.bind must use a Tailscale IP, got %q", host)
	}
	return nil
}

func isTailscaleIP(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	if ip4[0] != 100 {
		return false
	}
	return ip4[1]&0xC0 == 0x40
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
