package cli

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestAutoPostsCompletionWebhookWhenConfigured(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}

	var payload autoCompletionPayload
	var authHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		authHeader = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := []byte(strings.TrimSpace(`
name: demo-run
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
serve:
  notification_url: `+server.URL+`
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), cfg, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
			KeepSession:    "session-1",
			NextObjective:  "",
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}

	if authHeader != "" {
		t.Fatalf("Authorization header = %q, want empty", authHeader)
	}
	if payload.Event != "goalx.auto.complete" {
		t.Fatalf("event = %q, want goalx.auto.complete", payload.Event)
	}
	if payload.Run != "demo-run" {
		t.Fatalf("run = %q, want demo-run", payload.Run)
	}
	if payload.Recommendation != "done" {
		t.Fatalf("recommendation = %q, want done", payload.Recommendation)
	}
	if !payload.AcceptanceMet {
		t.Fatal("acceptance_met = false, want true")
	}
	if payload.KeepSession != "session-1" {
		t.Fatalf("keep_session = %q, want session-1", payload.KeepSession)
	}
	if payload.CompletedAt == "" {
		t.Fatal("completed_at is empty")
	}
}

func TestAutoIgnoresCompletionWebhookFailure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	cfg := []byte(strings.TrimSpace(`
name: demo-run
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
serve:
  notification_url: ://bad-url
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), cfg, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it"}); err != nil {
		t.Fatalf("Auto should ignore webhook failure, got: %v", err)
	}
}

func TestAutoPostsCompletionWebhookOnlyOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}

	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	cfg := []byte(strings.TrimSpace(`
name: demo-run
objective: ship it
target:
  files: [README.md]
harness:
  command: go test ./...
serve:
  notification_url: `+server.URL+`
`) + "\n")
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), cfg, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if calls != 1 {
		t.Fatalf("webhook calls = %d, want 1", calls)
	}
}

func TestAutoReturnsErrorForMasterRerouteRecommendations(t *testing.T) {
	for _, rec := range []string{"debate", "implement", "more-research"} {
		t.Run(rec, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)

			projectRoot := t.TempDir()

			oldInit := autoInit
			oldStart := autoStart
			oldSave := autoSave
			oldDrop := autoDrop
			oldDebate := autoDebate
			oldImplement := autoImplement
			oldPollUntilComplete := autoPollUntilComplete
			autoInit = func(string, []string) error { return nil }
			autoStart = func(string, []string) error { return nil }
			autoSave = func(string, []string) error { return nil }
			autoDrop = func(string, []string) error { return nil }
			autoDebate = func(string, []string, *nextConfigJSON) error {
				t.Fatal("autoDebate should not be called")
				return nil
			}
			autoImplement = func(string, []string, *nextConfigJSON) error {
				t.Fatal("autoImplement should not be called")
				return nil
			}
			autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
				return &statusJSON{
					Phase:          "complete",
					Recommendation: rec,
				}, nil
			}
			defer func() {
				autoInit = oldInit
				autoStart = oldStart
				autoSave = oldSave
				autoDrop = oldDrop
				autoDebate = oldDebate
				autoImplement = oldImplement
				autoPollUntilComplete = oldPollUntilComplete
			}()

			err := Auto(projectRoot, []string{"ship it", "--research"})
			if err == nil || !strings.Contains(err.Error(), `auto expects the master to finish within one run; got recommendation "`+rec+`"`) {
				t.Fatalf("Auto error = %v, want single-run recommendation error", err)
			}
		})
	}
}

func legacyAutoSkipsInitAfterDebate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "research-a", goalx.Config{
		Name:      "research-a",
		Mode:      goalx.ModeResearch,
		Objective: "audit auth flow",
		Preset:    "codex",
		Parallel:  3,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(string, []string) error {
		initCalls++
		if initCalls > 1 {
			return errUnexpectedSecondInit
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{Phase: "complete", Recommendation: "debate"}, nil
		case 2:
			return &statusJSON{Phase: "complete", Recommendation: "done", AcceptanceMet: true}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", initCalls)
	}
}

func legacyAutoSkipsInitAfterImplement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Preset:    "codex",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(string, []string) error {
		initCalls++
		if initCalls > 1 {
			return errUnexpectedSecondInit
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{Phase: "complete", Recommendation: "implement"}, nil
		case 2:
			return &statusJSON{Phase: "complete", Recommendation: "done", AcceptanceMet: true}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", initCalls)
	}
}

func legacyAutoRoutesNextConfigIntoImplement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldImplement := autoImplement
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	var gotNC *nextConfigJSON
	autoImplement = func(_ string, _ []string, nc *nextConfigJSON) error {
		gotNC = nc
		return nil
	}
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "implement",
				NextConfig: &nextConfigJSON{
					Parallel:       3,
					Engine:         "codex",
					Model:          "fast",
					DiversityHints: []string{"P0", "P1", "verify"},
					BudgetSeconds:  600,
				},
			}, nil
		case 2:
			return &statusJSON{Phase: "complete", Recommendation: "done", AcceptanceMet: true}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoImplement = oldImplement
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if gotNC == nil {
		t.Fatal("implement next_config = nil, want forwarded payload")
	}
	if gotNC.Parallel != 3 || gotNC.Engine != "codex" || gotNC.Model != "fast" {
		t.Fatalf("implement next_config = %#v, want forwarded values", gotNC)
	}
}

func legacyAutoRoutesNextConfigIntoDebate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldDebate := autoDebate
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	var gotNC *nextConfigJSON
	autoDebate = func(_ string, _ []string, nc *nextConfigJSON) error {
		gotNC = nc
		return nil
	}
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "debate",
				NextConfig: &nextConfigJSON{
					Parallel:       11,
					Engine:         "codex",
					Model:          "fast",
					DiversityHints: []string{"for", "against"},
				},
			}, nil
		case 2:
			return &statusJSON{Phase: "complete", Recommendation: "done", AcceptanceMet: true}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoDebate = oldDebate
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if gotNC == nil {
		t.Fatal("debate next_config = nil, want forwarded payload")
	}
	if gotNC.Parallel != 10 {
		t.Fatalf("parallel = %d, want capped 10", gotNC.Parallel)
	}
}

func legacyAutoImplementContinuesWhenAcceptanceMetTrue(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Preset:    "codex",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(string, []string) error {
		initCalls++
		if initCalls > 1 {
			return errUnexpectedSecondInit
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{Phase: "complete", Recommendation: "implement", AcceptanceMet: true}, nil
		case 2:
			return &statusJSON{Phase: "complete", Recommendation: "done", AcceptanceMet: true}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", initCalls)
	}
	if pollCalls != 2 {
		t.Fatalf("poll calls = %d, want 2", pollCalls)
	}
}

func TestValidateNextConfigRejectsInvalidFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Parallel:       99,
		Engine:         "unknown-engine",
		BudgetSeconds:  -1,
		DiversityHints: []string{"a", "b"},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Parallel != 10 {
		t.Fatalf("parallel = %d, want 10", got.Parallel)
	}
	if got.Engine != "" {
		t.Fatalf("engine = %q, want empty", got.Engine)
	}
	if got.BudgetSeconds != 0 {
		t.Fatalf("budget_seconds = %d, want 0", got.BudgetSeconds)
	}
}

func TestValidateNextConfigNormalizesExtendedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Mode:          " research ",
		MaxIterations: 7,
		Context:       []string{" docs/plan.md ", " ", "README.md"},
		MasterEngine:  " codex ",
		MasterModel:   " fast ",
		Sessions: []sessionConfigJSON{
			{Hint: " alpha ", Engine: " codex ", Model: " fast "},
			{Hint: " beta ", Engine: " unknown ", Model: " fast "},
		},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Mode != "research" {
		t.Fatalf("mode = %q, want research", got.Mode)
	}
	if got.MaxIterations != 7 {
		t.Fatalf("max_iterations = %d, want 7", got.MaxIterations)
	}
	if len(got.Context) != 2 || got.Context[0] != "docs/plan.md" || got.Context[1] != "README.md" {
		t.Fatalf("context = %#v, want trimmed non-empty paths", got.Context)
	}
	if got.MasterEngine != "codex" || got.MasterModel != "fast" {
		t.Fatalf("master engine/model = %q/%q, want codex/fast", got.MasterEngine, got.MasterModel)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 entries", got.Sessions)
	}
	if got.Sessions[0].Hint != "alpha" || got.Sessions[0].Engine != "codex" || got.Sessions[0].Model != "fast" {
		t.Fatalf("sessions[0] = %#v, want trimmed codex/fast entry", got.Sessions[0])
	}
	if got.Sessions[1].Hint != "beta" || got.Sessions[1].Engine != "" || got.Sessions[1].Model != "" {
		t.Fatalf("sessions[1] = %#v, want invalid engine/model cleared", got.Sessions[1])
	}
}

func TestValidateNextConfigRejectsInvalidExtendedFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	got := validateNextConfig(projectRoot, &nextConfigJSON{
		Mode:          "invalid",
		MaxIterations: 42,
		MasterEngine:  "unknown",
		MasterModel:   "fast",
		Sessions: []sessionConfigJSON{
			{Hint: "x", Engine: "codex", Model: "gpt-5.2"},
			{Hint: "y", Model: "fast"},
		},
	})
	if got == nil {
		t.Fatal("validateNextConfig returned nil")
	}
	if got.Mode != "" {
		t.Fatalf("mode = %q, want empty", got.Mode)
	}
	if got.MaxIterations != 0 {
		t.Fatalf("max_iterations = %d, want 0", got.MaxIterations)
	}
	if got.MasterEngine != "" || got.MasterModel != "" {
		t.Fatalf("master engine/model = %q/%q, want empty", got.MasterEngine, got.MasterModel)
	}
	if len(got.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 entries", got.Sessions)
	}
	if got.Sessions[0].Model != "" {
		t.Fatalf("sessions[0].model = %q, want empty", got.Sessions[0].Model)
	}
	if got.Sessions[1].Model != "" {
		t.Fatalf("sessions[1].model = %q, want empty", got.Sessions[1].Model)
	}
}

func legacyAutoKeepsSessionOnlyWhenDone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Preset:    "codex",
		Parallel:  2,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(string, []string) error {
		initCalls++
		if initCalls > 1 {
			return errUnexpectedSecondInit
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	keepCalls := 0
	autoKeep = func(_ string, sessions []string) error {
		keepCalls++
		if len(sessions) != 1 || sessions[0] != "session-1" {
			t.Fatalf("keep sessions = %v, want [session-1]", sessions)
		}
		return nil
	}
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{Phase: "complete", Recommendation: "implement", KeepSession: "session-1"}, nil
		case 2:
			return &statusJSON{Phase: "complete", Recommendation: "done", AcceptanceMet: true, KeepSession: "session-1"}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if initCalls != 1 {
		t.Fatalf("init calls = %d, want 1", initCalls)
	}
	if keepCalls != 1 {
		t.Fatalf("keep calls = %d, want 1", keepCalls)
	}
}

func TestAutoDoneFailsWhenHarnessVerificationFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeRootConfigFixture(t, projectRoot, goalx.Config{
		Name:      "demo-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship it",
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "false"},
	})
	writeSavedRunFixture(t, projectRoot, "demo-run", goalx.Config{
		Name:      "demo-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship it",
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "false"},
	}, map[string]string{
		"summary.md": "# summary\n",
	})
	worktreePath := filepath.Join(projectRoot, ".goalx", "runs", "demo-run", "worktrees", "demo-run-1")
	if err := os.MkdirAll(worktreePath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	err := Auto(projectRoot, []string{"ship it", "--develop"})
	if err == nil {
		t.Fatal("Auto returned nil, want harness verification failure")
	}
	if !strings.Contains(err.Error(), "verify harness") {
		t.Fatalf("Auto error = %v, want verify harness failure", err)
	}
}

func TestAutoKillsTmuxSessionWhenPollFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	oldResolveRun := autoResolveRun
	oldKillSession := autoKillSession
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoResolveRun = func(projectRoot, runName string) (*RunContext, error) {
		return &RunContext{
			Name:        "demo",
			RunDir:      filepath.Join(projectRoot, ".goalx", "runs", "demo"),
			TmuxSession: "goalx-demo",
			Config: &goalx.Config{
				Master: goalx.MasterConfig{CheckInterval: 2 * time.Minute},
				Budget: goalx.BudgetConfig{MaxDuration: time.Hour},
			},
		}, nil
	}
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return nil, errors.New("heartbeat stalled")
	}

	killed := 0
	autoKillSession = func(session string) error {
		killed++
		if session != "goalx-demo" {
			t.Fatalf("kill session = %q, want goalx-demo", session)
		}
		return nil
	}

	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
		autoResolveRun = oldResolveRun
		autoKillSession = oldKillSession
	}()

	err := Auto(t.TempDir(), []string{"ship it", "--research"})
	if err == nil || !strings.Contains(err.Error(), "poll") {
		t.Fatalf("Auto error = %v, want poll failure", err)
	}
	if killed != 1 {
		t.Fatalf("kill count = %d, want 1", killed)
	}
}

func legacyAutoMoreResearchPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), []byte("name: demo\nobjective: ship it\ntarget:\n  files: [README.md]\nharness:\n  command: go test ./...\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(_ string, args []string) error {
		initCalls++
		if initCalls == 2 {
			if len(args) < 2 || args[0] != "investigate auth" || args[1] != "--research" {
				return errors.New("more-research args out of order")
			}
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "more-research",
				NextObjective:  "investigate auth",
			}, nil
		case 2:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "done",
				AcceptanceMet:  true,
			}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
}

func TestAutoDefaultsToResearchMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), []byte("name: demo\nobjective: ship it\ntarget:\n  files: [README.md]\nharness:\n  command: go test ./...\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(_ string, args []string) error {
		if len(args) < 2 || args[0] != "ship it" || args[1] != "--research" {
			return errors.New("missing default research mode")
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
}

func TestAutoReturnsErrorForUnknownRecommendation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), []byte("name: demo\nobjective: ship it\ntarget:\n  files: [README.md]\nharness:\n  command: go test ./...\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "mystery",
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	err := Auto(projectRoot, []string{"ship it", "--research"})
	if err == nil || !strings.Contains(err.Error(), `unknown recommendation "mystery"`) {
		t.Fatalf("Auto error = %v, want unknown recommendation", err)
	}
}

func legacyAutoMoreResearchPreservesOriginalFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), []byte("name: demo\nobjective: ship it\ntarget:\n  files: [README.md]\nharness:\n  command: go test ./...\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(_ string, args []string) error {
		initCalls++
		if initCalls == 2 {
			want := []string{"investigate auth", "--preset", "codex", "--parallel", "3", "--research"}
			if len(args) != len(want) {
				return errors.New("more-research flags were not preserved")
			}
			for i := range want {
				if args[i] != want[i] {
					return errors.New("more-research flags were not preserved")
				}
			}
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "more-research",
				NextObjective:  "investigate auth",
			}, nil
		case 2:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "done",
				AcceptanceMet:  true,
			}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--preset", "codex", "--parallel", "3"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
}

func legacyAutoMoreResearchUsesNextConfigOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "goalx.yaml"), []byte("name: demo\nobjective: ship it\ntarget:\n  files: [README.md]\nharness:\n  command: go test ./...\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	autoInit = func(_ string, args []string) error {
		initCalls++
		if initCalls == 2 {
			want := []string{"investigate auth", "--research", "--parallel", "10", "--preset", "codex"}
			if len(args) != len(want) {
				return errors.New("more-research next_config args were not applied")
			}
			for i := range want {
				if args[i] != want[i] {
					return errors.New("more-research next_config args were not applied")
				}
			}
		}
		return nil
	}
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "more-research",
				NextObjective:  "investigate auth",
				NextConfig: &nextConfigJSON{
					Parallel: 99,
					Preset:   "codex",
				},
			}, nil
		case 2:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "done",
				AcceptanceMet:  true,
			}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
}

func legacyAutoMoreResearchAppliesFullNextConfigToGeneratedConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	initCalls := 0
	startCalls := 0
	autoInit = func(projectRoot string, args []string) error {
		initCalls++
		if initCalls == 1 {
			return nil
		}
		return Init(projectRoot, args)
	}
	autoStart = func(projectRoot string, args []string) error {
		startCalls++
		if startCalls == 2 {
			cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
			if err != nil {
				t.Fatalf("load generated goalx.yaml: %v", err)
			}
			if cfg.Engine != "codex" || cfg.Model != "fast" {
				t.Fatalf("engine/model = %s/%s, want codex/fast", cfg.Engine, cfg.Model)
			}
			if cfg.Parallel != 3 {
				t.Fatalf("parallel = %d, want 3", cfg.Parallel)
			}
			if cfg.Objective != "overridden research objective" {
				t.Fatalf("objective = %q, want overridden research objective", cfg.Objective)
			}
			if cfg.Budget.MaxDuration != 15*60*time.Second {
				t.Fatalf("budget = %v, want 15m", cfg.Budget.MaxDuration)
			}
			if cfg.Harness.Command != "test -s report.md && echo overridden" {
				t.Fatalf("harness = %q, want overridden harness", cfg.Harness.Command)
			}
			wantHints := []string{
				goalx.BuiltinStrategies["depth"],
				goalx.BuiltinStrategies["adversarial"],
				"verification lane",
			}
			if len(cfg.DiversityHints) != len(wantHints) {
				t.Fatalf("diversity_hints = %#v, want %#v", cfg.DiversityHints, wantHints)
			}
			for i := range wantHints {
				if cfg.DiversityHints[i] != wantHints[i] {
					t.Fatalf("diversity_hints[%d] = %q, want %q", i, cfg.DiversityHints[i], wantHints[i])
				}
			}
		}
		return nil
	}
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	pollCalls := 0
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		pollCalls++
		switch pollCalls {
		case 1:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "more-research",
				NextObjective:  "investigate auth",
				NextConfig: &nextConfigJSON{
					Parallel:       3,
					Engine:         "codex",
					Model:          "fast",
					Strategies:     []string{"depth", "adversarial"},
					DiversityHints: []string{"verification lane"},
					BudgetSeconds:  900,
					Objective:      "overridden research objective",
					Harness:        "test -s report.md && echo overridden",
				},
			}, nil
		case 2:
			return &statusJSON{
				Phase:          "complete",
				Recommendation: "done",
				AcceptanceMet:  true,
			}, nil
		default:
			t.Fatalf("unexpected poll call %d", pollCalls)
			return nil, nil
		}
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
		t.Fatalf("Auto: %v", err)
	}
}

func TestPollUntilCompleteRequiresRecommendation(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "status.json")
	writeStatus := func(raw string) {
		t.Helper()
		if err := os.WriteFile(statusPath, []byte(raw), 0o644); err != nil {
			t.Fatalf("write status: %v", err)
		}
	}

	writeStatus(`{"phase":"complete","recommendation":"","heartbeat":1}`)
	go func() {
		time.Sleep(20 * time.Millisecond)
		writeStatus(`{"phase":"complete","recommendation":"done","heartbeat":2}`)
	}()

	got, err := pollUntilComplete(statusPath, 5*time.Millisecond, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("pollUntilComplete: %v", err)
	}
	if got.Recommendation != "done" {
		t.Fatalf("recommendation = %q, want done", got.Recommendation)
	}
}

func TestPollUntilCompleteDetectsStalledHeartbeat(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "status.json")
	writeStatus := func(raw string) {
		t.Helper()
		if err := os.WriteFile(statusPath, []byte(raw), 0o644); err != nil {
			t.Fatalf("write status: %v", err)
		}
	}

	writeStatus(`{"phase":"running","recommendation":"","heartbeat":0}`)
	go func() {
		time.Sleep(10 * time.Millisecond)
		writeStatus(`{"phase":"running","recommendation":"","heartbeat":1}`)
	}()

	_, err := pollUntilCompleteWithHeartbeat(statusPath, 2*time.Millisecond, 140*time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "heartbeat stalled") {
		t.Fatalf("pollUntilComplete error = %v, want heartbeat stalled", err)
	}
}

func TestPollUntilCompleteFailsFastWhenTmuxSessionDies(t *testing.T) {
	projectRoot := t.TempDir()
	goalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), []byte("name: demo\n"), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	statusPath := filepath.Join(goalxDir, "status.json")
	if err := os.WriteFile(statusPath, []byte(`{"phase":"running","recommendation":"","heartbeat":0}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}

	oldSessionExists := autoSessionExists
	autoSessionExists = func(session string) bool {
		wantSession := goalx.TmuxSessionName(projectRoot, "demo")
		if session != wantSession {
			t.Fatalf("session = %q, want %q", session, wantSession)
		}
		return false
	}
	defer func() {
		autoSessionExists = oldSessionExists
	}()

	_, err := pollUntilCompleteWithHeartbeat(statusPath, 2*time.Millisecond, 40*time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "tmux session") {
		t.Fatalf("pollUntilComplete error = %v, want tmux exit", err)
	}
}

func TestPollUntilCompleteGracePeriodBeforeSecondHeartbeat(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(statusPath, []byte(`{"phase":"running","recommendation":"","heartbeat":0}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}

	_, err := pollUntilCompleteWithHeartbeat(statusPath, 2*time.Millisecond, 30*time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timeout after") {
		t.Fatalf("pollUntilComplete error = %v, want timeout during startup grace", err)
	}
}

func TestHeartbeatStallPollLimitScalesWithCheckInterval(t *testing.T) {
	got := heartbeatStallPollLimit(2*time.Minute, 30*time.Second)
	if got != 32 {
		t.Fatalf("stall poll limit = %d, want 32", got)
	}
}

func TestPollUntilCompleteLogsProgressWhileWaiting(t *testing.T) {
	statusPath := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(statusPath, []byte(`{"phase":"running","recommendation":"","heartbeat":0}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}

	go func() {
		time.Sleep(24 * time.Millisecond)
		if err := os.WriteFile(statusPath, []byte(`{"phase":"complete","recommendation":"done","heartbeat":0}`), 0o644); err != nil {
			t.Errorf("write completion status: %v", err)
		}
	}()

	out := captureStdout(t, func() {
		got, err := pollUntilCompleteWithHeartbeat(statusPath, 2*time.Millisecond, 80*time.Millisecond, 10*time.Millisecond)
		if err != nil {
			t.Fatalf("pollUntilCompleteWithHeartbeat: %v", err)
		}
		if got.Recommendation != "done" {
			t.Fatalf("recommendation = %q, want done", got.Recommendation)
		}
	})

	if !strings.Contains(out, "polling progress -- elapsed:") || !strings.Contains(out, "phase: running") {
		t.Fatalf("poll output missing progress log:\n%s", out)
	}
}

func TestAutoPrintsResearchResultsSummary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := t.TempDir()
	writeRootConfigFixture(t, projectRoot, goalx.Config{
		Name:      "demo-run",
		Mode:      goalx.ModeResearch,
		Objective: "ship it",
		Parallel:  3,
		Target: goalx.TargetConfig{
			Files: []string{"report.md"},
		},
		Harness: goalx.HarnessConfig{Command: "go test ./..."},
	})

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error {
		writeSavedRunFixture(t, projectRoot, "demo-run", goalx.Config{
			Name:      "demo-run",
			Mode:      goalx.ModeResearch,
			Objective: "ship it",
			Parallel:  3,
		}, map[string]string{
			"summary.md": strings.TrimSpace(`
# Summary

## Key Findings
- finding 1
- finding 2
- finding 3
- finding 4
- finding 5
- finding 6

## Priority Fix List
- P0: config.go

## Recommendation
done
`) + "\n",
		})
		return nil
	}
	autoKeep = func(string, []string) error { return nil }
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
			Heartbeat:      8,
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	out := captureStdout(t, func() {
		if err := Auto(projectRoot, []string{"ship it", "--research"}); err != nil {
			t.Fatalf("Auto: %v", err)
		}
	})

	for _, want := range []string{
		"=== Results ===",
		"Summary: .goalx/runs/demo-run/summary.md",
		"- finding 1",
		"... (1 more lines)",
		"Sessions: 3",
		"Heartbeats: 8",
		"Recommendation: done",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("auto output missing %q:\n%s", want, out)
		}
	}
}

func TestAutoPrintsDevelopDiffAfterKeep(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	stubAutoVerifyHarness(t, func(string) error { return nil })

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "README.md", "base\n", "base commit")
	writeRootConfigFixture(t, projectRoot, goalx.Config{
		Name:      "demo-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship it",
		Target: goalx.TargetConfig{
			Files: []string{"README.md"},
		},
		Harness: goalx.HarnessConfig{Command: "go test ./..."},
	})

	oldInit := autoInit
	oldStart := autoStart
	oldSave := autoSave
	oldKeep := autoKeep
	oldDrop := autoDrop
	oldPollUntilComplete := autoPollUntilComplete
	autoInit = func(string, []string) error { return nil }
	autoStart = func(string, []string) error { return nil }
	autoSave = func(string, []string) error { return nil }
	autoKeep = func(string, []string) error {
		writeAndCommit(t, projectRoot, "README.md", "base\nupdated\n", "merged session-1")
		return nil
	}
	autoDrop = func(string, []string) error { return nil }
	autoPollUntilComplete = func(string, time.Duration, time.Duration) (*statusJSON, error) {
		return &statusJSON{
			Phase:          "complete",
			Recommendation: "done",
			AcceptanceMet:  true,
			KeepSession:    "session-1",
		}, nil
	}
	defer func() {
		autoInit = oldInit
		autoStart = oldStart
		autoSave = oldSave
		autoKeep = oldKeep
		autoDrop = oldDrop
		autoPollUntilComplete = oldPollUntilComplete
	}()

	out := captureStdout(t, func() {
		if err := Auto(projectRoot, []string{"ship it", "--develop"}); err != nil {
			t.Fatalf("Auto: %v", err)
		}
	})

	for _, want := range []string{
		"=== Results ===",
		"Merged session-1 into main",
		"README.md |",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("auto output missing %q:\n%s", want, out)
		}
	}
}

var errUnexpectedSecondInit = errors.New("unexpected second init")

func stubAutoVerifyHarness(t *testing.T, fn func(string) error) {
	t.Helper()

	oldVerifyHarness := autoVerifyHarness
	autoVerifyHarness = fn
	t.Cleanup(func() {
		autoVerifyHarness = oldVerifyHarness
	})
}

func writeSavedRunFixture(t *testing.T, projectRoot, runName string, cfg goalx.Config, files map[string]string) {
	t.Helper()

	runDir := filepath.Join(projectRoot, ".goalx", "runs", runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func writeRootConfigFixture(t *testing.T, projectRoot string, cfg goalx.Config) {
	t.Helper()

	goalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal root config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write root goalx.yaml: %v", err)
	}
}
