package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestServeHandlerRequiresBearerToken(t *testing.T) {
	handler := newServeHandler(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"app": t.TempDir()},
	})

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestServeHandlerListsProjectsAndRuns(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspaceA := filepath.Join(t.TempDir(), "goalx-app")
	workspaceB := filepath.Join(t.TempDir(), "quantos-app")
	if err := os.MkdirAll(workspaceA, 0o755); err != nil {
		t.Fatalf("mkdir workspaceA: %v", err)
	}
	if err := os.MkdirAll(workspaceB, 0o755); err != nil {
		t.Fatalf("mkdir workspaceB: %v", err)
	}

	writeRunSnapshot(t, workspaceA, "auth-audit", goalx.ModeResearch, "audit auth flow")
	writeRunSnapshot(t, workspaceB, "serve-rollout", goalx.ModeDevelop, "implement serve API")

	app := newServeApp(goalx.ServeConfig{
		Token: "secret-token",
		Workspaces: map[string]string{
			"goalx":   workspaceA,
			"quantos": workspaceB,
		},
	})
	app.sessionExists = func(string) bool { return false }
	handler := app.routes()

	t.Run("projects", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/projects", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp struct {
			Projects []serveProject `json:"projects"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode /projects: %v", err)
		}
		if len(resp.Projects) != 2 {
			t.Fatalf("projects len = %d, want 2", len(resp.Projects))
		}
		if resp.Projects[0].Name != "goalx" || resp.Projects[0].Path != workspaceA {
			t.Fatalf("first project = %+v", resp.Projects[0])
		}
		if resp.Projects[1].Name != "quantos" || resp.Projects[1].Path != workspaceB {
			t.Fatalf("second project = %+v", resp.Projects[1])
		}
	})

	t.Run("project detail", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/projects/quantos", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp struct {
			Project serveProject `json:"project"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode /projects/:name: %v", err)
		}
		if resp.Project.Name != "quantos" || resp.Project.Path != workspaceB {
			t.Fatalf("project = %+v", resp.Project)
		}
	})

	t.Run("runs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/runs", nil)
		req.Header.Set("Authorization", "Bearer secret-token")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}

		var resp struct {
			Runs []serveRun `json:"runs"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode /runs: %v", err)
		}
		if len(resp.Runs) != 2 {
			t.Fatalf("runs len = %d, want 2", len(resp.Runs))
		}
		if resp.Runs[0].Workspace != "goalx" || resp.Runs[0].Name != "auth-audit" || resp.Runs[0].Status != "completed" {
			t.Fatalf("first run = %+v", resp.Runs[0])
		}
		if resp.Runs[1].Workspace != "quantos" || resp.Runs[1].Name != "serve-rollout" || resp.Runs[1].Mode != string(goalx.ModeDevelop) {
			t.Fatalf("second run = %+v", resp.Runs[1])
		}
	})
}

func writeRunSnapshot(t *testing.T, workspace, runName string, mode goalx.Mode, objective string) {
	t.Helper()

	runDir := goalx.RunDir(workspace, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      mode,
		Objective: objective,
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}
}
