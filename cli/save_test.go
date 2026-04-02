package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestSaveUsesConfiguredResearchTargetFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})

	want := "saved custom report"
	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedPath := filepath.Join(SavedRunDir(projectRoot, runName), "session-1-report.md")
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != want+"\n" {
		t.Fatalf("saved report = %q, want %q", string(got), want+"\n")
	}
	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "run-charter.json")); err != nil {
		t.Fatalf("saved run charter missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "sessions", "session-1", "identity.json")); err != nil {
		t.Fatalf("saved session identity missing: %v", err)
	}
}

func TestSaveWritesArtifactsManifestForResearchSession(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Sessions: []goalx.SessionConfig{
			{
				Hint:            "investigate",
				Mode:            goalx.ModeWorker,
				Target:          &goalx.TargetConfig{Files: []string{"missing.md"}, Readonly: []string{"."}},
				LocalValidation: &goalx.LocalValidationConfig{Command: "test -s missing.md"},
			},
		},
		Target:          goalx.TargetConfig{Files: []string{"src/"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "go test ./..."},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", *cfg.Sessions[0].Target, *cfg.Sessions[0].LocalValidation)

	want := "saved manifest report"
	reportPath := filepath.Join(wtPath, "analysis.txt")
	if err := os.WriteFile(reportPath, []byte(want+"\n"), 0o644); err != nil {
		t.Fatalf("write analysis.txt: %v", err)
	}
	if _, err := EnsureArtifactsManifest(runDir); err != nil {
		t.Fatalf("EnsureArtifactsManifest: %v", err)
	}
	if err := RegisterSessionArtifact(runDir, "session-1", ArtifactMeta{
		Kind:        "report",
		Path:        reportPath,
		RelPath:     "analysis.txt",
		DurableName: "session-1-report.md",
	}); err != nil {
		t.Fatalf("RegisterSessionArtifact: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedPath := filepath.Join(SavedRunDir(projectRoot, runName), "session-1-report.md")
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != want+"\n" {
		t.Fatalf("saved report = %q, want %q", string(got), want+"\n")
	}

	manifest, err := LoadArtifacts(filepath.Join(SavedRunDir(projectRoot, runName), "artifacts.json"))
	if err != nil {
		t.Fatalf("LoadArtifacts: %v", err)
	}
	if len(manifest.Sessions) != 1 || len(manifest.Sessions[0].Artifacts) != 1 {
		t.Fatalf("saved manifest = %#v, want one research artifact", manifest)
	}
}

func TestSaveCopiesGoalBoundaryArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	if err := SaveObjectiveContract(ObjectiveContractPath(runDir), &ObjectiveContract{
		Version:       1,
		State:         objectiveContractStateLocked,
		ObjectiveHash: "sha256:demo",
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
	goal := `{"version":1,"objective_contract_hash":"sha256:demo","required":[{"id":"req-1","text":"ship feature","kind":"outcome","covers_clauses":["ucl-1"]}],"optional":[],"guardrails":[]}`
	if err := os.WriteFile(filepath.Join(runDir, "obligation-model.json"), []byte(goal), 0o644); err != nil {
		t.Fatalf("write obligation model: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "obligation-log.jsonl"), []byte("{\"type\":\"path_selected\"}\n"), 0o644); err != nil {
		t.Fatalf("write obligation log: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(SavedRunDir(projectRoot, runName), "obligation-model.json"))
	if err != nil {
		t.Fatalf("read saved goal state: %v", err)
	}
	if string(got) != goal {
		t.Fatalf("saved obligation model = %q, want %q", string(got), goal)
	}
	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "obligation-log.jsonl")); err != nil {
		t.Fatalf("saved obligation log missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "run-charter.json")); err != nil {
		t.Fatalf("saved run charter missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "objective-contract.json")); err != nil {
		t.Fatalf("saved objective contract missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "obligation-model.json")); err != nil {
		t.Fatalf("saved obligation model missing: %v", err)
	}
}

func TestSaveCopiesAcceptanceCheckEvidence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)

	evidencePath := filepath.Join(runDir, "evidence-chk-live.txt")
	if err := os.WriteFile(evidencePath, []byte("check ok\n"), 0o644); err != nil {
		t.Fatalf("write check evidence: %v", err)
	}
	if err := writeAssuranceFixture(t, runDir, &AcceptanceState{
		Version: 2,
		Checks: []AcceptanceCheck{
			{ID: "chk-live", Label: "live gate", Command: "printf ok", State: acceptanceCheckStateActive},
		},
		LastResult: AcceptanceResult{
			CheckResults: []AcceptanceCheckResult{
				{ID: "chk-live", Command: "printf ok", EvidencePath: evidencePath},
			},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), filepath.Base(evidencePath))); err != nil {
		t.Fatalf("saved acceptance check evidence missing: %v", err)
	}
}

func TestSaveCopiesEvidenceLogWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target:    goalx.TargetConfig{Files: []string{"notes.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	if err := AppendEvidenceLogEvent(EvidenceLogPath(runDir), "scenario.executed", "master", EvidenceEventBody{
		ScenarioID:   "scenario-cli-first-run",
		HarnessKind:  "cli",
		OracleResult: map[string]any{"exit_code": 0},
	}); err != nil {
		t.Fatalf("AppendEvidenceLogEvent: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "evidence-log.jsonl")); err != nil {
		t.Fatalf("saved evidence-log missing: %v", err)
	}
}

func TestSaveCopiesDevelopIntegrationAndExperimentState(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)

	experiments := `{"version":1,"kind":"experiment.created","at":"2026-03-28T10:00:00Z","actor":"goalx","body":{"experiment_id":"exp-2","created_at":"2026-03-28T10:00:00Z"}}`
	if err := os.WriteFile(ExperimentsLogPath(runDir), []byte(experiments), 0o644); err != nil {
		t.Fatalf("write experiments.jsonl: %v", err)
	}
	if err := SaveIntegrationState(IntegrationStatePath(runDir), &IntegrationState{
		Version:                 1,
		CurrentExperimentID:     "exp-2",
		CurrentBranch:           "goalx/demo/2",
		CurrentCommit:           "abc123",
		LastIntegrationID:       "int-1",
		LastMethod:              "keep",
		LastSourceExperimentIDs: []string{"exp-2"},
		UpdatedAt:               "2026-03-28T10:00:00Z",
	}); err != nil {
		t.Fatalf("SaveIntegrationState: %v", err)
	}
	coordination := `{"version":1,"owners":{"req-1":"session-2"}}`
	if err := os.WriteFile(CoordinationPath(runDir), []byte(coordination), 0o644); err != nil {
		t.Fatalf("write coordination.json: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if got, err := os.ReadFile(filepath.Join(SavedRunDir(projectRoot, runName), "integration.json")); err != nil {
		t.Fatalf("read saved integration.json: %v", err)
	} else if !strings.Contains(string(got), `"current_experiment_id": "exp-2"`) {
		t.Fatalf("saved integration.json = %q", string(got))
	}
	if got, err := os.ReadFile(filepath.Join(SavedRunDir(projectRoot, runName), "experiments.jsonl")); err != nil {
		t.Fatalf("read saved experiments.jsonl: %v", err)
	} else if string(got) != experiments {
		t.Fatalf("saved experiments.jsonl = %q, want %q", string(got), experiments)
	}
	if got, err := os.ReadFile(filepath.Join(SavedRunDir(projectRoot, runName), "coordination.json")); err != nil {
		t.Fatalf("read saved coordination.json: %v", err)
	} else if string(got) != coordination {
		t.Fatalf("saved coordination.json = %q, want %q", string(got), coordination)
	}
}

func TestSaveCopiesSelectionSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	want := testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterCandidates: []string{"codex/gpt-5.4"},
			WorkerCandidates: []string{"codex/gpt-5.4-mini"},
		},
		Master: goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
		Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini", Effort: goalx.EffortMedium},
	}
	writeSelectionSnapshotFixture(t, runDir, want)

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err = os.ReadFile(testSelectionSnapshotPath(SavedRunDir(projectRoot, runName)))
	if err != nil {
		t.Fatalf("read saved selection snapshot: %v", err)
	}
	var got testSelectionSnapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal saved selection snapshot: %v", err)
	}
	if got.Master.Engine != want.Master.Engine || got.Master.Model != want.Master.Model {
		t.Fatalf("saved snapshot master = %s/%s, want %s/%s", got.Master.Engine, got.Master.Model, want.Master.Engine, want.Master.Model)
	}
	if len(got.Policy.WorkerCandidates) != 1 || got.Policy.WorkerCandidates[0] != "codex/gpt-5.4-mini" {
		t.Fatalf("saved snapshot worker_candidates = %#v", got.Policy.WorkerCandidates)
	}
}

func TestSaveCopiesCanonicalIntakeArtifact(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	if err := SaveRunIntake(IntakePath(runDir), &RunIntake{
		Version:   1,
		Objective: cfg.Objective,
		Intent:    runIntentDeliver,
	}); err != nil {
		t.Fatalf("SaveRunIntake: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "intake.json")); err != nil {
		t.Fatalf("saved intake missing: %v", err)
	}
}

func TestSaveDoesNotMutateRunStateFromRunStatusRecord(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Target:    goalx.TargetConfig{Files: []string{"README.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)

	runState := &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(goalx.ModeWorker),
		Phase:     "researching",
		StartedAt: "2026-03-23T00:00:00Z",
		UpdatedAt: "2026-03-23T00:00:00Z",
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), runState); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	statusPath := RunStatusPath(runDir)
	if err := os.MkdirAll(filepath.Dir(statusPath), 0o755); err != nil {
		t.Fatalf("mkdir status dir: %v", err)
	}
	if err := os.WriteFile(statusPath, []byte(`{"version":1,"phase":"complete","required_remaining":0,"updated_at":"2026-03-28T10:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write run status record: %v", err)
	}

	before, err := os.ReadFile(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("read run state before: %v", err)
	}
	beforeInfo, err := os.Stat(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("stat run state before: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	after, err := os.ReadFile(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("read run state after: %v", err)
	}
	afterInfo, err := os.Stat(RunRuntimeStatePath(runDir))
	if err != nil {
		t.Fatalf("stat run state after: %v", err)
	}

	if string(after) != string(before) {
		t.Fatalf("run state mutated during save:\nwant: %s\ngot:  %s", string(before), string(after))
	}
	if !afterInfo.ModTime().Equal(beforeInfo.ModTime()) {
		t.Fatalf("run state modtime changed during save: before=%s after=%s", beforeInfo.ModTime(), afterInfo.ModTime())
	}
	savedStatus, err := os.ReadFile(filepath.Join(SavedRunDir(projectRoot, runName), "status.json"))
	if err != nil {
		t.Fatalf("read saved status record: %v", err)
	}
	if string(savedStatus) != `{"version":1,"phase":"complete","required_remaining":0,"updated_at":"2026-03-28T10:00:00Z"}` {
		t.Fatalf("saved status record mismatch:\nwant: %s\ngot:  %s", `{"version":1,"phase":"complete","required_remaining":0,"updated_at":"2026-03-28T10:00:00Z"}`, string(savedStatus))
	}
}

func TestSaveDoesNotGuessReportWhenManifestExistsWithoutDeclaredReport(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})
	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte("guessed report\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	if err := SaveArtifacts(ArtifactsPath(runDir), &ArtifactsManifest{
		Run:     runName,
		Version: 1,
		Sessions: []SessionArtifacts{
			{Name: "session-1", Mode: string(goalx.ModeWorker)},
		},
	}); err != nil {
		t.Fatalf("SaveArtifacts: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedPath := filepath.Join(SavedRunDir(projectRoot, runName), "session-1-report.md")
	if _, err := os.Stat(savedPath); err == nil {
		t.Fatalf("save guessed a report even though manifest declared no report: %s", savedPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat saved report: %v", err)
	}
}

func TestSaveDoesNotCreateActiveRunArtifactsManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})
	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte("custom report\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}

	if _, err := os.Stat(ArtifactsPath(runDir)); !os.IsNotExist(err) {
		t.Fatalf("expected no active-run artifacts manifest before save, got err=%v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(ArtifactsPath(runDir)); err == nil {
		t.Fatalf("save created active-run artifacts manifest at %s", ArtifactsPath(runDir))
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat active-run artifacts manifest: %v", err)
	}

	if _, err := os.Stat(filepath.Join(SavedRunDir(projectRoot, runName), "artifacts.json")); err != nil {
		t.Fatalf("expected saved artifacts manifest: %v", err)
	}
}

func TestSavePrefersRunReportsDirOverWorktreeFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	reportsDir := filepath.Join(runDir, "reports")
	for _, dir := range []string{wtPath, reportsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})

	if err := os.WriteFile(filepath.Join(wtPath, "notes.md"), []byte("legacy worktree report\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(reportsDir, "session-1-report.md"), []byte("run reports dir report\n"), 0o644); err != nil {
		t.Fatalf("write reports dir report: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedPath := filepath.Join(SavedRunDir(projectRoot, runName), "session-1-report.md")
	got, err := os.ReadFile(savedPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != "run reports dir report\n" {
		t.Fatalf("saved report = %q, want %q", string(got), "run reports dir report\n")
	}
}

func TestSaveCopiesRunScopedReportsDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	if err := os.MkdirAll(ReportsDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir reports dir: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target:    goalx.TargetConfig{Files: []string{"notes.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	if err := os.WriteFile(filepath.Join(ReportsDir(runDir), "repo-summary.md"), []byte("repo report\n"), 0o644); err != nil {
		t.Fatalf("write run-scoped report: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(SavedRunDir(projectRoot, runName), "reports", "repo-summary.md"))
	if err != nil {
		t.Fatalf("read saved run-scoped report: %v", err)
	}
	if string(got) != "repo report\n" {
		t.Fatalf("saved run-scoped report = %q, want %q", string(got), "repo report\n")
	}
}

func TestSaveCopiesSessionReportEvidenceManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}

	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeWorker,
		Objective: "inspect",
		Target: goalx.TargetConfig{
			Files: []string{"notes.md"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run snapshot: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})

	reportPath := filepath.Join(wtPath, "notes.md")
	if err := os.WriteFile(reportPath, []byte("saved report\n"), 0o644); err != nil {
		t.Fatalf("write notes.md: %v", err)
	}
	manifest := ReportEvidenceManifest{
		Version:           1,
		ReportPath:        reportPath,
		Covers:            []string{"ucl-1"},
		RepoEvidencePaths: []string{filepath.Join(wtPath, "source.txt")},
		ExternalRefs:      []string{"https://example.com/ref"},
	}
	if err := os.WriteFile(filepath.Join(wtPath, "source.txt"), []byte("evidence\n"), 0o644); err != nil {
		t.Fatalf("write source evidence: %v", err)
	}
	manifestData, err := json.MarshalIndent(&manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(ReportEvidenceManifestPath(reportPath), manifestData, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	savedReportPath := filepath.Join(SavedRunDir(projectRoot, runName), "session-1-report.md")
	got, err := os.ReadFile(savedReportPath)
	if err != nil {
		t.Fatalf("read saved report: %v", err)
	}
	if string(got) != "saved report\n" {
		t.Fatalf("saved report = %q, want %q", string(got), "saved report\n")
	}

	savedManifestPath := ReportEvidenceManifestPath(savedReportPath)
	savedManifest, err := LoadReportEvidenceManifest(savedReportPath)
	if err != nil {
		t.Fatalf("LoadReportEvidenceManifest(saved): %v", err)
	}
	if savedManifest.ReportPath != savedReportPath {
		t.Fatalf("saved manifest report_path = %q, want %q", savedManifest.ReportPath, savedReportPath)
	}
	if len(savedManifest.ExternalRefs) != 1 || savedManifest.ExternalRefs[0] != "https://example.com/ref" {
		t.Fatalf("saved manifest external refs = %#v, want one ref", savedManifest.ExternalRefs)
	}
	if _, err := os.Stat(savedManifestPath); err != nil {
		t.Fatalf("saved manifest missing: %v", err)
	}
}

func seedSaveRunProvenance(t *testing.T, projectRoot, runDir, runName, objective string) {
	t.Helper()

	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:         1,
		Objective:       objective,
		ProjectRoot:     projectRoot,
		ProtocolVersion: currentProtocolVersion,
		RunID:           newRunID(),
		Epoch:           1,
	}); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, projectRoot)
}

func seedSaveSessionIdentity(t *testing.T, runDir, sessionName string, mode goalx.Mode, engine, model string, target goalx.TargetConfig, localValidation goalx.LocalValidationConfig) {
	t.Helper()

	identity, err := NewSessionIdentity(runDir, sessionName, sessionRoleKind(mode), mode, engine, model, "", "", "", target)
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	identity.LocalValidationCommand = localValidation.Command
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, sessionName), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
}
