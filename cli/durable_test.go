package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	payloadPath := filepath.Join(t.TempDir(), "status.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"phase":"working","required_remaining":2,"active_sessions":["session-1"]}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	if err := Durable(repo, []string{"write", "status", "--run", cfg.Name, "--body-file", payloadPath}); err != nil {
		t.Fatalf("Durable write: %v", err)
	}

	record, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunStatusRecord: %v", err)
	}
	if record == nil || record.RequiredRemaining == nil || *record.RequiredRemaining != 2 {
		t.Fatalf("unexpected status record: %#v", record)
	}
	if record.Version != 1 {
		t.Fatalf("record.Version = %d, want 1", record.Version)
	}
	if strings.TrimSpace(record.UpdatedAt) == "" {
		t.Fatal("record.UpdatedAt is empty")
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
	payloadPath := filepath.Join(t.TempDir(), "experiments.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"experiment_id":"exp-1"}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	if err := Durable(repo, []string{"write", "experiments", "--run", cfg.Name, "--kind", "experiment.created", "--actor", "master", "--body-file", payloadPath}); err != nil {
		t.Fatalf("Durable write: %v", err)
	}

	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) != 1 || events[0].Kind != "experiment.created" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if strings.TrimSpace(events[0].At) == "" {
		t.Fatal("event At is empty")
	}
	var body ExperimentCreatedBody
	if err := json.Unmarshal(events[0].Body, &body); err != nil {
		t.Fatalf("json.Unmarshal(event body): %v", err)
	}
	if body.CreatedAt != events[0].At {
		t.Fatalf("body.CreatedAt = %q, want %q", body.CreatedAt, events[0].At)
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
	writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "summary.md")
	if err := os.WriteFile(payloadPath, []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"write", "summary", "--run", cfg.Name, "--body-file", payloadPath})
	if err == nil || !strings.Contains(err.Error(), "not machine-consumed") {
		t.Fatalf("Durable write error = %v, want machine-consumed failure", err)
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
	payloadPath := filepath.Join(t.TempDir(), "status.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"phase":"working","required_remaining":1,"run":"demo"}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"write", "status", "--run", cfg.Name, "--body-file", payloadPath})
	if err == nil {
		t.Fatal("Durable write should fail")
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
		"usage: goalx durable write <surface> --run NAME --body-file /abs/path.json [--kind KIND] [--actor ACTOR]",
		"inspect the authoring contract first with `goalx schema <surface>`",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("durable help missing %q:\n%s", want, out)
		}
	}
}

func TestDurableReplaceGoalRespectsLockedObjectiveContractIntegrity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-goal-integrity")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), &ObjectiveContract{
		Version:       1,
		ObjectiveHash: "sha256:demo",
		State:         objectiveContractStateLocked,
		Clauses: []ObjectiveClause{
			{
				ID:               "ucl-1",
				Text:             "ship feature",
				Kind:             objectiveClauseKindDelivery,
				SourceExcerpt:    "ship feature",
				RequiredSurfaces: []ObjectiveRequiredSurface{objectiveRequiredSurfaceGoal},
			},
		},
	}); err != nil {
		t.Fatalf("SaveObjectiveContract: %v", err)
	}
	payloadPath := filepath.Join(t.TempDir(), "goal.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"required":[{"id":"req-1","text":"ship feature","source":"user","role":"outcome","state":"open"}],"optional":[]}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"write", "goal", "--run", cfg.Name, "--body-file", payloadPath})
	if err == nil {
		t.Fatal("Durable write should reject goal payload that bypasses locked contract coverage")
	}
	if !strings.Contains(err.Error(), "missing covers") {
		t.Fatalf("Durable replace error = %v, want missing covers", err)
	}
}

func TestDurableReplaceObjectiveContractRejectsLockedRewrite(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-objective-contract")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "objective-contract.body.json")
	payload := []byte(`{"objective_hash":"sha256:demo","state":"locked","clauses":[{"id":"ucl-1","text":"ship feature","kind":"delivery","source_excerpt":"ship feature","required_surfaces":["goal"]}]}`)
	if err := os.WriteFile(payloadPath, payload, 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	if err := Durable(repo, []string{"write", "objective-contract", "--run", cfg.Name, "--body-file", payloadPath}); err != nil {
		t.Fatalf("first Durable write: %v", err)
	}
	err := Durable(repo, []string{"write", "objective-contract", "--run", cfg.Name, "--body-file", payloadPath})
	if err == nil {
		t.Fatal("second Durable write should reject locked contract rewrite")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Fatalf("Durable write error = %v, want locked contract failure", err)
	}
}

func TestDurableCommandRejectsEventLogWriteWithoutKind(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-missing-kind")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "goal-log.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"decision":"boundary"}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"write", "goal-log", "--run", cfg.Name, "--actor", "master", "--body-file", payloadPath})
	if err == nil || !strings.Contains(err.Error(), `requires --kind`) {
		t.Fatalf("Durable write error = %v, want missing kind failure", err)
	}
}

func TestDurableCommandRejectsKindFlagForStructuredSurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-kind-on-structured")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "status.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"phase":"working","required_remaining":0}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"write", "status", "--run", cfg.Name, "--kind", "decision", "--body-file", payloadPath})
	if err == nil || !strings.Contains(err.Error(), `does not accept --kind`) {
		t.Fatalf("Durable write error = %v, want structured flag failure", err)
	}
}

func TestApplyDurableMutationConcurrentEventWritesPreserveAllEvents(t *testing.T) {
	runDir := t.TempDir()

	const writers = 24
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, writers)
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			body := json.RawMessage([]byte(`{"experiment_id":"exp-` + fmt.Sprintf("%02d", i) + `"}`))
			if err := ApplyDurableMutation(runDir, DurableMutation{
				Surface: DurableSurfaceExperiments,
				Kind:    "experiment.created",
				Actor:   "master",
				Body:    body,
			}); err != nil {
				errCh <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("ApplyDurableMutation concurrent append: %v", err)
		}
	}

	events, err := LoadDurableLog(ExperimentsLogPath(runDir), DurableSurfaceExperiments)
	if err != nil {
		t.Fatalf("LoadDurableLog: %v", err)
	}
	if len(events) != writers {
		t.Fatalf("events len = %d, want %d", len(events), writers)
	}
}

func TestApplyDurableMutationConcurrentStructuredWritesRemainValid(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-structured-concurrency")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	if err := SaveGoalState(GoalPath(runDir), &GoalState{
		Version: 1,
		Required: []GoalItem{
			{ID: "req-1", Text: "ship it", Source: goalItemSourceUser, Role: goalItemRoleOutcome, State: goalItemStateOpen},
		},
	}); err != nil {
		t.Fatalf("SaveGoalState: %v", err)
	}

	const writers = 12
	start := make(chan struct{})
	var wg sync.WaitGroup
	errCh := make(chan error, writers)
	for i := 0; i < writers; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := ApplyDurableMutation(runDir, DurableMutation{
				Surface: DurableSurfaceStatus,
				Body:    json.RawMessage([]byte(fmt.Sprintf(`{"phase":"working","required_remaining":1,"open_required_ids":["req-1"],"active_sessions":["session-%d"]}`, i))),
			}); err != nil {
				errCh <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("ApplyDurableMutation concurrent structured write: %v", err)
		}
	}

	record, err := LoadRunStatusRecord(RunStatusPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunStatusRecord: %v", err)
	}
	if record == nil || record.Version != 1 {
		t.Fatalf("unexpected record: %#v", record)
	}
}

func TestApplyDurableMutationRejectsEventMetadataOnStructuredSurface(t *testing.T) {
	runDir := t.TempDir()

	err := ApplyDurableMutation(runDir, DurableMutation{
		Surface: DurableSurfaceStatus,
		Kind:    "decision",
		Actor:   "master",
		At:      "2026-03-30T00:00:00Z",
		Body:    json.RawMessage([]byte(`{"phase":"working","required_remaining":0}`)),
	})
	if err == nil || !strings.Contains(err.Error(), `does not accept kind`) {
		t.Fatalf("ApplyDurableMutation error = %v, want structured metadata failure", err)
	}
}
