package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
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

func TestServeHandlerGoalxActionRoutes(t *testing.T) {
	workspace := t.TempDir()

	type call struct {
		projectRoot string
		action      string
		args        []string
	}

	cases := []struct {
		name       string
		path       string
		body       string
		wantAction string
		wantArgs   []string
	}{
		{
			name:       "init",
			path:       "/projects/goalx/goalx/init",
			body:       `{"objective":"audit auth","mode":"research","parallel":2,"name":"auth-audit","context":["README.md","docs/arch.md"],"strategies":["depth","security"]}`,
			wantAction: "init",
			wantArgs:   []string{"audit auth", "--research", "--parallel", "2", "--name", "auth-audit", "--context", "README.md,docs/arch.md", "--strategy", "depth,security"},
		},
		{
			name:       "start",
			path:       "/projects/goalx/goalx/start",
			body:       `{"objective":"implement serve","mode":"develop","parallel":1}`,
			wantAction: "start",
			wantArgs:   []string{"implement serve", "--develop", "--parallel", "1"},
		},
		{
			name:       "auto",
			path:       "/projects/goalx/goalx/auto",
			body:       `{"objective":"research remote management","mode":"research","parallel":3}`,
			wantAction: "auto",
			wantArgs:   []string{"research remote management", "--research", "--parallel", "3"},
		},
		{
			name:       "observe",
			path:       "/projects/goalx/goalx/observe",
			body:       `{"run":"auth-audit"}`,
			wantAction: "observe",
			wantArgs:   []string{"--run", "auth-audit"},
		},
		{
			name:       "status",
			path:       "/projects/goalx/goalx/status",
			body:       `{"run":"auth-audit","session":"session-1"}`,
			wantAction: "status",
			wantArgs:   []string{"--run", "auth-audit", "session-1"},
		},
		{
			name:       "add",
			path:       "/projects/goalx/goalx/add",
			body:       `{"run":"auth-audit","direction":"investigate authz"}`,
			wantAction: "add",
			wantArgs:   []string{"investigate authz", "--run", "auth-audit"},
		},
		{
			name:       "stop",
			path:       "/projects/goalx/goalx/stop",
			body:       `{"run":"auth-audit"}`,
			wantAction: "stop",
			wantArgs:   []string{"--run", "auth-audit"},
		},
		{
			name:       "save",
			path:       "/projects/goalx/goalx/save",
			body:       `{"run":"auth-audit"}`,
			wantAction: "save",
			wantArgs:   []string{"--run", "auth-audit"},
		},
		{
			name:       "keep",
			path:       "/projects/goalx/goalx/keep",
			body:       `{"run":"auth-audit","session":"session-2"}`,
			wantAction: "keep",
			wantArgs:   []string{"--run", "auth-audit", "session-2"},
		},
		{
			name:       "drop",
			path:       "/projects/goalx/goalx/drop",
			body:       `{"run":"auth-audit"}`,
			wantAction: "drop",
			wantArgs:   []string{"--run", "auth-audit"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got call
			app := newServeApp(goalx.ServeConfig{
				Token:      "secret-token",
				Workspaces: map[string]string{"goalx": workspace},
			})
			app.runCLI = func(projectRoot, action string, args []string) (string, error) {
				got = call{projectRoot: projectRoot, action: action, args: append([]string(nil), args...)}
				return "ok", nil
			}

			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Authorization", "Bearer secret-token")
			rec := httptest.NewRecorder()
			app.routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if got.projectRoot != workspace || got.action != tc.wantAction || !reflect.DeepEqual(got.args, tc.wantArgs) {
				t.Fatalf("call = %+v, want action=%q args=%v", got, tc.wantAction, tc.wantArgs)
			}
		})
	}
}

func TestServeHandlerConfigEndpointReadsAndWritesGoalxYAML(t *testing.T) {
	workspace := t.TempDir()
	cfgDir := filepath.Join(workspace, ".goalx")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfg dir: %v", err)
	}
	cfgPath := filepath.Join(cfgDir, "goalx.yaml")
	if err := os.WriteFile(cfgPath, []byte("name: before\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})

	writeReq := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/config", bytes.NewBufferString(`{"content":"name: after\nmode: research\n"}`))
	writeReq.Header.Set("Authorization", "Bearer secret-token")
	writeRec := httptest.NewRecorder()
	app.routes().ServeHTTP(writeRec, writeReq)

	if writeRec.Code != http.StatusOK {
		t.Fatalf("write status = %d, want %d, body=%s", writeRec.Code, http.StatusOK, writeRec.Body.String())
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if string(data) != "name: after\nmode: research\n" {
		t.Fatalf("goalx.yaml = %q", string(data))
	}

	readReq := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/config", bytes.NewBufferString(`{}`))
	readReq.Header.Set("Authorization", "Bearer secret-token")
	readRec := httptest.NewRecorder()
	app.routes().ServeHTTP(readRec, readReq)

	if readRec.Code != http.StatusOK {
		t.Fatalf("read status = %d, want %d, body=%s", readRec.Code, http.StatusOK, readRec.Body.String())
	}

	var resp struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(readRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if resp.Content != "name: after\nmode: research\n" {
		t.Fatalf("content = %q", resp.Content)
	}
}

func TestServeHandlerConfigEndpointCanTargetRunSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir workspace .goalx: %v", err)
	}
	rootCfgPath := filepath.Join(workspace, ".goalx", "goalx.yaml")
	if err := os.WriteFile(rootCfgPath, []byte("name: root\nmode: research\n"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	writeRunSnapshot(t, workspace, "auth-audit", goalx.ModeResearch, "audit auth flow")
	runCfgPath := filepath.Join(goalx.RunDir(workspace, "auth-audit"), "goalx.yaml")

	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})

	req := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/config", bytes.NewBufferString(`{"run":"auth-audit","content":"name: auth-audit\nmode: develop\n"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	runData, err := os.ReadFile(runCfgPath)
	if err != nil {
		t.Fatalf("read run config: %v", err)
	}
	if string(runData) != "name: auth-audit\nmode: develop\n" {
		t.Fatalf("run goalx.yaml = %q", string(runData))
	}

	rootData, err := os.ReadFile(rootCfgPath)
	if err != nil {
		t.Fatalf("read root config: %v", err)
	}
	if string(rootData) != "name: root\nmode: research\n" {
		t.Fatalf("root goalx.yaml should stay unchanged, got %q", string(rootData))
	}
}

func TestServeHandlerTellWritesGuidanceAndNudgesSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()
	writeRunSnapshot(t, workspace, "auth-audit", goalx.ModeResearch, "audit auth flow")
	runDir := goalx.RunDir(workspace, "auth-audit")
	guidanceDir := filepath.Join(runDir, "guidance")
	for _, dir := range []string{
		filepath.Join(runDir, "journals"),
		guidanceDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session journal: %v", err)
	}

	var gotTarget, gotKeys string
	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})
	app.sendKeys = func(target, keys string) error {
		gotTarget, gotKeys = target, keys
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/tell", bytes.NewBufferString(`{"run":"auth-audit","session":"session-1","message":"focus on authz regressions"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	guidancePath := GuidancePath(runDir, "session-1")
	data, err := os.ReadFile(guidancePath)
	if err != nil {
		t.Fatalf("read guidance: %v", err)
	}
	if string(data) != "focus on authz regressions\n" {
		t.Fatalf("guidance = %q", string(data))
	}

	wantTarget := goalx.TmuxSessionName(workspace, "auth-audit") + ":" + sessionWindowName("auth-audit", 1)
	if gotTarget != wantTarget || gotKeys != "" {
		t.Fatalf("sendKeys target=%q keys=%q, want target=%q keys=\"\"", gotTarget, gotKeys, wantTarget)
	}
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
