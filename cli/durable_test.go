package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestDurableCommandReplacesStructuredSurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-replace")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(payloadPath, []byte(`{"version":1,"phase":"working","required_remaining":2,"active_sessions":["session-1"],"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	if err := Durable(repo, []string{"replace", "status", "--run", cfg.Name, "--file", payloadPath}); err != nil {
		t.Fatalf("Durable replace: %v", err)
	}

	record, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunStatusRecord: %v", err)
	}
	if record == nil || record.RequiredRemaining == nil || *record.RequiredRemaining != 2 {
		t.Fatalf("unexpected status record: %#v", record)
	}
}

func TestDurableCommandAppendsEventLog(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-append")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "experiments.jsonl")
	if err := os.WriteFile(payloadPath, []byte(`{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"master","body":{"experiment_id":"exp-1","created_at":"2026-03-28T10:00:00Z"}}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	if err := Durable(repo, []string{"append", "experiments", "--run", cfg.Name, "--file", payloadPath}); err != nil {
		t.Fatalf("Durable append: %v", err)
	}

	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) != 1 || events[0].Kind != "experiment.created" {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestDurableCommandRejectsWrongSurfaceMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-bad-mode")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	_ = writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(payloadPath, []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"replace", "summary", "--run", cfg.Name, "--file", payloadPath})
	if err == nil || !strings.Contains(err.Error(), "not a structured state surface") {
		t.Fatalf("Durable replace error = %v, want structured state failure", err)
	}
}

func TestDurableCommandRejectsUnknownStatusFieldWithSchemaHint(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-bad-status")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	_ = writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "status.json")
	if err := os.WriteFile(payloadPath, []byte(`{"version":1,"phase":"working","required_remaining":1,"run":"demo"}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"replace", "status", "--run", cfg.Name, "--file", payloadPath})
	if err == nil {
		t.Fatal("Durable replace should fail")
	}
	for _, want := range []string{`unknown field`, `goalx schema status`} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Durable replace error = %v, want %q", err, want)
		}
	}
}

func TestDurableHelpPointsToSchemaAuthority(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Durable(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Durable --help: %v", err)
		}
	})

	for _, want := range []string{
		"usage: goalx durable <replace|append> <surface> --run NAME --file /abs/path",
		"inspect the canonical contract first with `goalx schema <surface>`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("durable help missing %q:\n%s", want, out)
		}
	}
}
