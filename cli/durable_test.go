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

func TestDurableCommandReplacesObligationModelSurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-obligation-model")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)

	objectivePath := filepath.Join(t.TempDir(), "objective-contract.body.json")
	objectivePayload := []byte(`{"objective_hash":"sha256:demo","state":"locked","clauses":[{"id":"ucl-1","text":"ship feature","kind":"delivery","source_excerpt":"ship feature","required_surfaces":["obligation"]}]}`)
	if err := os.WriteFile(objectivePath, objectivePayload, 0o644); err != nil {
		t.Fatalf("write objective payload: %v", err)
	}
	if err := Durable(repo, []string{"write", "objective-contract", "--run", cfg.Name, "--body-file", objectivePath}); err != nil {
		t.Fatalf("Durable objective-contract write: %v", err)
	}

	objectiveHash, err := hashFileContents(ObjectiveContractPath(runDir))
	if err != nil {
		t.Fatalf("hashFileContents(objective-contract): %v", err)
	}
	payloadPath := filepath.Join(t.TempDir(), "obligation-model.body.json")
	payload := []byte(`{"objective_contract_hash":"` + objectiveHash + `","required":[{"id":"req-1","text":"ship feature","kind":"outcome","covers_clauses":["ucl-1"],"assurance_required":true}],"optional":[],"guardrails":[]}`)
	if err := os.WriteFile(payloadPath, payload, 0o644); err != nil {
		t.Fatalf("write obligation payload: %v", err)
	}

	if err := Durable(repo, []string{"write", "obligation-model", "--run", cfg.Name, "--body-file", payloadPath}); err != nil {
		t.Fatalf("Durable obligation-model write: %v", err)
	}

	model, err := LoadObligationModel(ObligationModelPath(runDir))
	if err != nil {
		t.Fatalf("LoadObligationModel: %v", err)
	}
	if model == nil || len(model.Required) != 1 || model.Required[0].ID != "req-1" {
		t.Fatalf("unexpected obligation model: %#v", model)
	}
}

func TestDurableCommandReplacesAssurancePlanSurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-assurance-plan")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)

	objectivePath := filepath.Join(t.TempDir(), "objective-contract.body.json")
	objectivePayload := []byte(`{"objective_hash":"sha256:demo","state":"locked","clauses":[{"id":"ucl-1","text":"verify feature","kind":"verification","source_excerpt":"verify feature","required_surfaces":["assurance"]}]}`)
	if err := os.WriteFile(objectivePath, objectivePayload, 0o644); err != nil {
		t.Fatalf("write objective payload: %v", err)
	}
	if err := Durable(repo, []string{"write", "objective-contract", "--run", cfg.Name, "--body-file", objectivePath}); err != nil {
		t.Fatalf("Durable objective-contract write: %v", err)
	}

	payloadPath := filepath.Join(t.TempDir(), "assurance-plan.body.json")
	payload := []byte(`{"obligation_refs":["req-1"],"scenarios":[{"id":"scenario-1","covers_obligations":["ucl-1"],"harness":{"kind":"cli","command":"echo ok"},"oracle":{"kind":"compound","checks":[{"kind":"exit_code","equals":"0"}]},"evidence":[{"kind":"stdout"}],"gate_policy":{"verify_lane":"required","required_cognition_tier":"repo-native","closeout":"required","merge":"required"}}]}`)
	if err := os.WriteFile(payloadPath, payload, 0o644); err != nil {
		t.Fatalf("write assurance payload: %v", err)
	}

	if err := Durable(repo, []string{"write", "assurance-plan", "--run", cfg.Name, "--body-file", payloadPath}); err != nil {
		t.Fatalf("Durable assurance-plan write: %v", err)
	}

	plan, err := LoadAssurancePlan(AssurancePlanPath(runDir))
	if err != nil {
		t.Fatalf("LoadAssurancePlan: %v", err)
	}
	if plan == nil || len(plan.Scenarios) != 1 || plan.Scenarios[0].ID != "scenario-1" {
		t.Fatalf("unexpected assurance plan: %#v", plan)
	}
}

func TestDurableCommandReplacesAdvertisedStructuredSurfaces(t *testing.T) {
	cases := []struct {
		name    string
		surface string
		parse   func([]byte) error
	}{
		{
			name:    "cognition-state",
			surface: "cognition-state",
			parse: func(data []byte) error {
				_, err := parseCognitionState(data)
				return err
			},
		},
		{
			name:    "success-model",
			surface: "success-model",
			parse: func(data []byte) error {
				_, err := parseSuccessModel(data)
				return err
			},
		},
		{
			name:    "proof-plan",
			surface: "proof-plan",
			parse: func(data []byte) error {
				_, err := parseProofPlan(data)
				return err
			},
		},
		{
			name:    "workflow-plan",
			surface: "workflow-plan",
			parse: func(data []byte) error {
				_, err := parseWorkflowPlan(data)
				return err
			},
		},
		{
			name:    "domain-pack",
			surface: "domain-pack",
			parse: func(data []byte) error {
				_, err := parseDomainPack(data)
				return err
			},
		},
		{
			name:    "compiler-input",
			surface: "compiler-input",
			parse: func(data []byte) error {
				_, err := parseCompilerInput(data)
				return err
			},
		},
		{
			name:    "compiler-report",
			surface: "compiler-report",
			parse: func(data []byte) error {
				_, err := parseCompilerReport(data)
				return err
			},
		},
		{
			name:    "impact-state",
			surface: "impact-state",
			parse: func(data []byte) error {
				_, err := parseImpactState(data)
				return err
			},
		},
		{
			name:    "freshness-state",
			surface: "freshness-state",
			parse: func(data []byte) error {
				_, err := parseFreshnessState(data)
				return err
			},
		},
		{
			name:    "resource-state",
			surface: "resource-state",
			parse: func(data []byte) error {
				_, err := parseResourceState(data)
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			repo := initNamedGitRepo(t, "durable-"+tc.name)
			cfg := &goalx.Config{
				Name:      "demo",
				Objective: "ship it",
				Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
			}
			runDir := writeRunSpecFixture(t, repo, cfg)

			contract, err := LookupDurableContract(tc.surface)
			if err != nil {
				t.Fatalf("LookupDurableContract(%s): %v", tc.surface, err)
			}
			payloadPath := filepath.Join(t.TempDir(), tc.surface+".body.json")
			if err := os.WriteFile(payloadPath, []byte(contract.Example), 0o644); err != nil {
				t.Fatalf("write payload: %v", err)
			}

			if err := Durable(repo, []string{"write", tc.surface, "--run", cfg.Name, "--body-file", payloadPath}); err != nil {
				t.Fatalf("Durable %s write: %v", tc.surface, err)
			}

			spec, err := LookupDurableSurface(tc.surface)
			if err != nil {
				t.Fatalf("LookupDurableSurface(%s): %v", tc.surface, err)
			}
			data, err := os.ReadFile(spec.Path(runDir))
			if err != nil {
				t.Fatalf("read stored %s: %v", tc.surface, err)
			}
			if err := tc.parse(data); err != nil {
				t.Fatalf("parse stored %s: %v\n%s", tc.surface, err, string(data))
			}
		})
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

func TestDurableRejectsLegacyGoalSurface(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repo := initNamedGitRepo(t, "durable-goal-integrity")
	cfg := &goalx.Config{
		Name:      "demo",
		Objective: "ship it",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	_ = writeRunSpecFixture(t, repo, cfg)
	payloadPath := filepath.Join(t.TempDir(), "goal.body.json")
	if err := os.WriteFile(payloadPath, []byte(`{"required":[{"id":"req-1","text":"ship feature","source":"user","role":"outcome","state":"open"}],"optional":[]}`), 0o644); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	err := Durable(repo, []string{"write", "goal", "--run", cfg.Name, "--body-file", payloadPath})
	if err == nil || !strings.Contains(err.Error(), "obligation-model") {
		t.Fatalf("Durable write error = %v, want obligation-model migration hint", err)
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
	if err == nil || !strings.Contains(err.Error(), "obligation-log") {
		t.Fatalf("Durable write error = %v, want obligation-log migration hint", err)
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
	if err := writeBoundaryFixture(t, runDir, &GoalState{
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
