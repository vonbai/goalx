package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestNotifyAutoCompletionPostsConfiguredWebhook(t *testing.T) {
	projectRoot := t.TempDir()
	goalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}

	var gotMethod string
	var gotPayload map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	cfg := "name: auth-audit\nobjective: audit auth flow\nserve:\n  notification_url: " + srv.URL + "\n"
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}

	status := &statusJSON{
		Phase:          "complete",
		Recommendation: "done",
		AcceptanceMet:  true,
		KeepSession:    "session-1",
	}
	if err := notifyAutoCompletion(projectRoot, status); err != nil {
		t.Fatalf("notifyAutoCompletion: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotPayload["event"] != "auto_complete" {
		t.Fatalf("event = %v", gotPayload["event"])
	}
	if gotPayload["run"] != "auth-audit" {
		t.Fatalf("run = %v", gotPayload["run"])
	}
	if gotPayload["objective"] != "audit auth flow" {
		t.Fatalf("objective = %v", gotPayload["objective"])
	}
	if gotPayload["recommendation"] != "done" {
		t.Fatalf("recommendation = %v", gotPayload["recommendation"])
	}
}

func TestNotifyAutoCompletionSkipsWhenNotificationURLMissing(t *testing.T) {
	projectRoot := t.TempDir()
	if err := notifyAutoCompletion(projectRoot, &statusJSON{Phase: "complete"}); err != nil {
		t.Fatalf("notifyAutoCompletion without URL: %v", err)
	}
}
