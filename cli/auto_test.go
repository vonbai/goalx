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
  notification_url: ` + server.URL + `
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
  notification_url: ` + server.URL + `
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

func TestAutoSkipsInitAfterDebate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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

func TestAutoSkipsInitAfterImplement(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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

func TestAutoMoreResearchPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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

func TestAutoMoreResearchPreservesOriginalFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
	if err := os.WriteFile(statusPath, []byte(`{"phase":"running","recommendation":"","heartbeat":1}`), 0o644); err != nil {
		t.Fatalf("write status: %v", err)
	}

	_, err := pollUntilComplete(statusPath, 5*time.Millisecond, 200*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "heartbeat stalled") {
		t.Fatalf("pollUntilComplete error = %v, want heartbeat stalled", err)
	}
}

func TestAutoPrintsResearchResultsSummary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

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
