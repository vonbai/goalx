package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

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
	runDirA := goalx.RunDir(workspaceA, "auth-audit")
	if err := EnsureControlState(runDirA); err != nil {
		t.Fatalf("EnsureControlState auth-audit: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDirA), &ControlRunState{
		Version:        1,
		LifecycleState: "completed",
	}); err != nil {
		t.Fatalf("SaveControlRunState auth-audit: %v", err)
	}
	runDirB := goalx.RunDir(workspaceB, "serve-rollout")
	if err := EnsureControlState(runDirB); err != nil {
		t.Fatalf("EnsureControlState serve-rollout: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(goalx.RunDir(workspaceB, "serve-rollout")), &ControlRunState{
		Version:        1,
		LifecycleState: "active",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := RenewControlLease(goalx.RunDir(workspaceB, "serve-rollout"), "sidecar", "run_demo", 1, time.Minute, "process", 4242); err != nil {
		t.Fatalf("RenewControlLease: %v", err)
	}
	metaA, err := LoadRunMetadata(RunMetadataPath(runDirA))
	if err != nil {
		t.Fatalf("LoadRunMetadata auth-audit: %v", err)
	}
	metaB, err := LoadRunMetadata(RunMetadataPath(runDirB))
	if err != nil {
		t.Fatalf("LoadRunMetadata serve-rollout: %v", err)
	}

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
		if resp.Runs[0].Workspace != "goalx" || resp.Runs[0].Name != "auth-audit" || resp.Runs[0].Status != "completed" || resp.Runs[0].RunID != metaA.RunID || resp.Runs[0].Epoch != metaA.Epoch || resp.Runs[0].Charter != "ok" {
			t.Fatalf("first run = %+v", resp.Runs[0])
		}
		if resp.Runs[1].Workspace != "quantos" || resp.Runs[1].Name != "serve-rollout" || resp.Runs[1].Mode != string(goalx.ModeDevelop) || resp.Runs[1].Status != "active" || resp.Runs[1].RunID != metaB.RunID || resp.Runs[1].Epoch != metaB.Epoch || resp.Runs[1].Charter != "ok" {
			t.Fatalf("second run = %+v", resp.Runs[1])
		}
	})
}

func TestServeHandlerGoalxActionRoutes(t *testing.T) {
	workspace := t.TempDir()

	type call struct {
		projectRoot string
		action      string
		req         serveActionRequest
	}

	cases := []struct {
		name       string
		path       string
		body       string
		wantAction string
		wantReq    serveActionRequest
	}{
		{
			name:       "init",
			path:       "/projects/goalx/goalx/init",
			body:       `{"objective":"audit auth","mode":"research","parallel":2,"name":"auth-audit","context":["README.md","docs/arch.md"],"dimensions":["depth","security"]}`,
			wantAction: "init",
			wantReq: serveActionRequest{
				Objective: "audit auth",
				Mode:      "research",
				Parallel:  2,
				Name:      "auth-audit",
				Context:   []string{"README.md", "docs/arch.md"},
				Dimensions: []string{
					"depth",
					"security",
				},
			},
		},
		{
			name:       "start",
			path:       "/projects/goalx/goalx/start",
			body:       `{"objective":"implement serve","mode":"develop","parallel":1}`,
			wantAction: "start",
			wantReq: serveActionRequest{
				Objective: "implement serve",
				Mode:      "develop",
				Parallel:  1,
			},
		},
		{
			name:       "start from manual draft",
			path:       "/projects/goalx/goalx/start",
			body:       `{"config_scope":"draft"}`,
			wantAction: "start",
			wantReq:    serveActionRequest{ConfigScope: "draft"},
		},
		{
			name:       "auto",
			path:       "/projects/goalx/goalx/auto",
			body:       `{"objective":"research remote management","mode":"research","parallel":3}`,
			wantAction: "auto",
			wantReq: serveActionRequest{
				Objective: "research remote management",
				Mode:      "research",
				Parallel:  3,
			},
		},
		{
			name:       "research",
			path:       "/projects/goalx/goalx/research",
			body:       `{"objective":"triage auth bugs","parallel":2,"preset":"hybrid","master":"codex/best","research_role":"claude-code/opus"}`,
			wantAction: "research",
			wantReq: serveActionRequest{
				Objective:    "triage auth bugs",
				Parallel:     2,
				Preset:       "hybrid",
				Master:       "codex/best",
				ResearchRole: "claude-code/opus",
			},
		},
		{
			name:       "implement",
			path:       "/projects/goalx/goalx/implement",
			body:       `{"from":"auth-audit","objective":"implement fixes","parallel":2,"develop_role":"codex/fast","write_config":true}`,
			wantAction: "implement",
			wantReq: serveActionRequest{
				From:        "auth-audit",
				Objective:   "implement fixes",
				Parallel:    2,
				DevelopRole: "codex/fast",
				WriteConfig: true,
			},
		},
		{
			name:       "observe",
			path:       "/projects/goalx/goalx/observe",
			body:       `{"run":"auth-audit"}`,
			wantAction: "observe",
			wantReq:    serveActionRequest{Run: "auth-audit"},
		},
		{
			name:       "context",
			path:       "/projects/goalx/goalx/context",
			body:       `{"run":"auth-audit"}`,
			wantAction: "context",
			wantReq:    serveActionRequest{Run: "auth-audit"},
		},
		{
			name:       "afford",
			path:       "/projects/goalx/goalx/afford",
			body:       `{"run":"auth-audit","session":"session-1"}`,
			wantAction: "afford",
			wantReq:    serveActionRequest{Run: "auth-audit", Session: "session-1"},
		},
		{
			name:       "status",
			path:       "/projects/goalx/goalx/status",
			body:       `{"run":"auth-audit","session":"session-1"}`,
			wantAction: "status",
			wantReq:    serveActionRequest{Run: "auth-audit", Session: "session-1"},
		},
		{
			name:       "add",
			path:       "/projects/goalx/goalx/add",
			body:       `{"run":"auth-audit","direction":"investigate authz"}`,
			wantAction: "add",
			wantReq:    serveActionRequest{Run: "auth-audit", Direction: "investigate authz"},
		},
		{
			name:       "stop",
			path:       "/projects/goalx/goalx/stop",
			body:       `{"run":"auth-audit"}`,
			wantAction: "stop",
			wantReq:    serveActionRequest{Run: "auth-audit"},
		},
		{
			name:       "save",
			path:       "/projects/goalx/goalx/save",
			body:       `{"run":"auth-audit"}`,
			wantAction: "save",
			wantReq:    serveActionRequest{Run: "auth-audit"},
		},
		{
			name:       "keep",
			path:       "/projects/goalx/goalx/keep",
			body:       `{"run":"auth-audit","session":"session-2"}`,
			wantAction: "keep",
			wantReq:    serveActionRequest{Run: "auth-audit", Session: "session-2"},
		},
		{
			name:       "drop",
			path:       "/projects/goalx/goalx/drop",
			body:       `{"run":"auth-audit"}`,
			wantAction: "drop",
			wantReq:    serveActionRequest{Run: "auth-audit"},
		},
		{
			name:       "park",
			path:       "/projects/goalx/goalx/park",
			body:       `{"run":"auth-audit","session":"session-2"}`,
			wantAction: "park",
			wantReq:    serveActionRequest{Run: "auth-audit", Session: "session-2"},
		},
		{
			name:       "resume",
			path:       "/projects/goalx/goalx/resume",
			body:       `{"run":"auth-audit","session":"session-2"}`,
			wantAction: "resume",
			wantReq:    serveActionRequest{Run: "auth-audit", Session: "session-2"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got call
			app := newServeApp(goalx.ServeConfig{
				Token:      "secret-token",
				Workspaces: map[string]string{"goalx": workspace},
			})
			app.runAction = func(projectRoot, action string, req serveActionRequest) (string, error) {
				got = call{projectRoot: projectRoot, action: action, req: req}
				return "ok", nil
			}

			req := httptest.NewRequest(http.MethodPost, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Authorization", "Bearer secret-token")
			rec := httptest.NewRecorder()
			app.routes().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
			}
			if got.projectRoot != workspace || got.action != tc.wantAction || !reflect.DeepEqual(got.req, tc.wantReq) {
				t.Fatalf("call = %+v, want action=%q req=%+v", got, tc.wantAction, tc.wantReq)
			}
		})
	}
}

func TestServeHandlerConfigEndpointDistinguishesSharedAndDraft(t *testing.T) {
	workspace := t.TempDir()
	cfgDir := filepath.Join(workspace, ".goalx")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir cfg dir: %v", err)
	}
	sharedPath := filepath.Join(cfgDir, "config.yaml")
	if err := os.WriteFile(sharedPath, []byte("parallel: 2\n"), 0o644); err != nil {
		t.Fatalf("write shared config: %v", err)
	}
	draftPath := filepath.Join(cfgDir, "goalx.yaml")
	if err := os.WriteFile(draftPath, []byte("name: before\n"), 0o644); err != nil {
		t.Fatalf("write draft config: %v", err)
	}

	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})

	writeReq := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/config", bytes.NewBufferString(`{"content":"parallel: 4\n"}`))
	writeReq.Header.Set("Authorization", "Bearer secret-token")
	writeRec := httptest.NewRecorder()
	app.routes().ServeHTTP(writeRec, writeReq)

	if writeRec.Code != http.StatusOK {
		t.Fatalf("write status = %d, want %d, body=%s", writeRec.Code, http.StatusOK, writeRec.Body.String())
	}

	data, err := os.ReadFile(sharedPath)
	if err != nil {
		t.Fatalf("read shared config: %v", err)
	}
	if string(data) != "parallel: 4\n" {
		t.Fatalf("config.yaml = %q", string(data))
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
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(readRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode config response: %v", err)
	}
	if resp.Content != "parallel: 4\n" {
		t.Fatalf("content = %q", resp.Content)
	}
	if resp.Path != sharedPath {
		t.Fatalf("path = %q, want %q", resp.Path, sharedPath)
	}

	draftReq := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/config", bytes.NewBufferString(`{"config_scope":"draft"}`))
	draftReq.Header.Set("Authorization", "Bearer secret-token")
	draftRec := httptest.NewRecorder()
	app.routes().ServeHTTP(draftRec, draftReq)

	if draftRec.Code != http.StatusOK {
		t.Fatalf("draft read status = %d, want %d, body=%s", draftRec.Code, http.StatusOK, draftRec.Body.String())
	}
	if err := json.Unmarshal(draftRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode draft response: %v", err)
	}
	if resp.Content != "name: before\n" {
		t.Fatalf("draft content = %q", resp.Content)
	}
	if resp.Path != draftPath {
		t.Fatalf("draft path = %q, want %q", resp.Path, draftPath)
	}
}

func TestServeHandlerConfigEndpointCanReadRunSpec(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir workspace .goalx: %v", err)
	}
	rootCfgPath := filepath.Join(workspace, ".goalx", "config.yaml")
	if err := os.WriteFile(rootCfgPath, []byte("parallel: 2\n"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}

	writeRunSnapshot(t, workspace, "auth-audit", goalx.ModeResearch, "audit auth flow")
	runCfgPath := RunSpecPath(goalx.RunDir(workspace, "auth-audit"))

	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})

	req := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/config", bytes.NewBufferString(`{"run":"auth-audit"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp struct {
		Content string `json:"content"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	runData, err := os.ReadFile(runCfgPath)
	if err != nil {
		t.Fatalf("read run spec: %v", err)
	}
	if resp.Content != string(runData) {
		t.Fatalf("run spec content = %q, want %q", resp.Content, string(runData))
	}
	if resp.Path != runCfgPath {
		t.Fatalf("path = %q, want %q", resp.Path, runCfgPath)
	}

	rootData, err := os.ReadFile(rootCfgPath)
	if err != nil {
		t.Fatalf("read root config: %v", err)
	}
	if string(rootData) != "parallel: 2\n" {
		t.Fatalf("root config.yaml should stay unchanged, got %q", string(rootData))
	}
}

func TestServeHandlerTellWritesSessionInboxAndNudgesSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()
	writeRunSnapshot(t, workspace, "auth-audit", goalx.ModeResearch, "audit auth flow")
	runDir := goalx.RunDir(workspace, "auth-audit")
	for _, dir := range []string{
		filepath.Join(runDir, "journals"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session journal: %v", err)
	}
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeResearch, "", "", goalx.TargetConfig{}, goalx.HarnessConfig{})

	var gotTarget, gotEngine string
	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})
	app.sendNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/tell", bytes.NewBufferString(`{"run":"auth-audit","session":"session-1","message":"focus on authz regressions"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	inboxPath := ControlInboxPath(runDir, "session-1")
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		t.Fatalf("read session inbox: %v", err)
	}
	if !strings.Contains(string(data), `"body":"focus on authz regressions"`) {
		t.Fatalf("session inbox = %q", string(data))
	}

	wantTarget := goalx.TmuxSessionName(workspace, "auth-audit") + ":" + sessionWindowName("auth-audit", 1)
	if gotTarget != wantTarget || gotEngine != "" {
		t.Fatalf("sendNudge target=%q engine=%q, want target=%q engine=\"\"", gotTarget, gotEngine, wantTarget)
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "sent" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
	}
}

func TestServeHandlerTellWritesMasterInboxAndUsesControlNudge(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	workspace := t.TempDir()
	writeRunSnapshot(t, workspace, "auth-audit", goalx.ModeDevelop, "implement auth flow")
	runDir := goalx.RunDir(workspace, "auth-audit")
	if err := EnsureMasterControl(runDir); err != nil {
		t.Fatalf("EnsureMasterControl: %v", err)
	}

	var gotTarget, gotEngine string
	app := newServeApp(goalx.ServeConfig{
		Token:      "secret-token",
		Workspaces: map[string]string{"goalx": workspace},
	})
	app.sendNudge = func(target, engine string) error {
		gotTarget, gotEngine = target, engine
		return nil
	}

	req := httptest.NewRequest(http.MethodPost, "/projects/goalx/goalx/tell", bytes.NewBufferString(`{"run":"auth-audit","message":"focus on the final acceptance gap"}`))
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	app.routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	inbox, err := os.ReadFile(MasterInboxPath(runDir))
	if err != nil {
		t.Fatalf("read master inbox: %v", err)
	}
	text := string(inbox)
	for _, want := range []string{`"type":"tell"`, `"source":"user"`, `"body":"focus on the final acceptance gap"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("master inbox missing %q:\n%s", want, text)
		}
	}

	wantTarget := goalx.TmuxSessionName(workspace, "auth-audit") + ":master"
	if gotTarget != wantTarget || gotEngine != "" {
		t.Fatalf("sendNudge target=%q engine=%q, want target=%q with default engine", gotTarget, gotEngine, wantTarget)
	}
	deliveries, err := LoadControlDeliveries(ControlDeliveriesPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if len(deliveries.Items) != 1 || deliveries.Items[0].Status != "sent" {
		t.Fatalf("unexpected deliveries: %+v", deliveries.Items)
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
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run-spec.yaml: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:         1,
		Objective:       objective,
		ProjectRoot:     workspace,
		ProtocolVersion: currentProtocolVersion,
		RunID:           newRunID(),
		RootRunID:       "",
		Epoch:           1,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, workspace)
}
