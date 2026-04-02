package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestImplementUsesSharedMasterConfigInsteadOfSavedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
roles:
  worker:
    engine: codex
    model: gpt-5.4
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeResolvedSavedRunFixture(t, projectRoot, "debate", launchOptions{
		Objective: "consensus fixes",
		Mode:      goalx.ModeWorker,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: claude-code
    model: opus
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Fatalf("master = %s/%s, want claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Worker.Engine != "claude-code" || cfg.Roles.Worker.Model != "opus" {
		t.Fatalf("develop role = %s/%s, want claude-code/opus", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
}

func TestImplementUsesSavedSelectionSnapshotWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: claude-code
    model: opus
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeResolvedSavedRunFixture(t, projectRoot, "debate", launchOptions{
		Objective: "consensus fixes",
		Mode:      goalx.ModeWorker,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeSelectionSnapshotFixture(t, SavedRunDir(projectRoot, "debate"), testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterCandidates: []string{"codex/gpt-5.4"},
			WorkerCandidates: []string{"codex/gpt-5.4-mini", "codex/gpt-5.4", "claude-code/opus"},
		},
		Master: goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
		Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini", Effort: goalx.EffortMedium},
	})
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: claude-code
  model: opus
target:
  files: [config.go]
local_validation:
  command: go test ./cli/...
`)

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want saved snapshot codex/gpt-5.4", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Worker.Engine != "codex" || cfg.Roles.Worker.Model != "gpt-5.4-mini" {
		t.Fatalf("develop role = %s/%s, want saved snapshot codex/gpt-5.4-mini", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
	if cfg.Target.Files[0] != "config.go" {
		t.Fatalf("target.files = %#v, want current shared config target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "go test ./cli/..." {
		t.Fatalf("local_validation.command = %q, want current shared config command", cfg.LocalValidation.Command)
	}
}

func TestImplementAppliesCanonicalPhaseOverridesFromCLI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  worker:
    engine: claude-code
    model: opus
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeWorker,
		Objective: "consensus fixes",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Implement(projectRoot, []string{
		"--from", "debate",
		"--objective", "custom implement objective",
		"--dimension", "depth,adversarial,evidence,perfectionist",
		"--budget", "20m",
		"--write-config",
	}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Roles.Worker.Engine != "claude-code" || cfg.Roles.Worker.Model != "opus" {
		t.Fatalf("develop role = %s/%s, want claude-code/opus", cfg.Roles.Worker.Engine, cfg.Roles.Worker.Model)
	}
	if cfg.Objective != "custom implement objective" {
		t.Fatalf("objective = %q, want custom implement objective", cfg.Objective)
	}
	if cfg.Budget.MaxDuration != 20*60*1_000_000_000 {
		t.Fatalf("budget = %v, want 20m", cfg.Budget.MaxDuration)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 canonical implement lanes", cfg.Sessions)
	}
	for i, session := range cfg.Sessions {
		if got, want := session.Dimensions, []string{"depth", "adversarial", "evidence", "perfectionist"}; len(got) != len(want) || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("session[%d].dimensions = %#v, want %#v", i, got, want)
		}
	}
}

func TestImplementAttachesCLIProvidedDimensionsToSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeWorker,
		Objective: "consensus fixes",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Implement(projectRoot, []string{
		"--from", "debate",
		"--dimension", "depth,adversarial,evidence",
		"--write-config",
	}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if len(cfg.Sessions) != 2 {
		t.Fatalf("sessions = %#v, want 2 canonical implement lanes", cfg.Sessions)
	}
	for i, session := range cfg.Sessions {
		if got, want := session.Dimensions, []string{"depth", "adversarial", "evidence"}; len(got) != len(want) || strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("session[%d].dimensions = %#v, want %#v", i, got, want)
		}
	}
}

func TestImplementUsesSavedManifestReportArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeWorker,
		Objective: "consensus fixes",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
	}, nil)
	runDir := SavedRunDir(projectRoot, "debate")
	reportPath := filepath.Join(runDir, "custom-findings.txt")
	if err := os.WriteFile(reportPath, []byte("report\n"), 0o644); err != nil {
		t.Fatalf("write custom report: %v", err)
	}
	if err := SaveArtifacts(filepath.Join(runDir, "artifacts.json"), &ArtifactsManifest{
		Run:     "debate",
		Version: 1,
		Sessions: []SessionArtifacts{
			{
				Name: "session-1",
				Mode: string(goalx.ModeWorker),
				Artifacts: []ArtifactMeta{
					{Kind: "report", Path: reportPath, RelPath: "custom-findings.txt", DurableName: "session-1-report.md"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveArtifacts: %v", err)
	}

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	found := false
	for _, path := range cfg.Context.Files {
		if path == reportPath {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("context.files = %#v, want %q from artifacts manifest", cfg.Context.Files, reportPath)
	}
}

func TestImplementUsesDistinctNameForLongSourceRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	sourceRun := "design-the-next-generation-backend-architecture-for-synapse"
	writeSavedRunFixture(t, projectRoot, sourceRun, goalx.Config{
		Name:      sourceRun,
		Mode:      goalx.ModeWorker,
		Objective: "consensus fixes",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
		Target:          goalx.TargetConfig{Files: []string{"cli/"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "go test ./..."},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	if err := Implement(projectRoot, []string{"--from", sourceRun, "--write-config"}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Name == sourceRun {
		t.Fatalf("implement cfg name = %q, want distinct phase name", cfg.Name)
	}
	if !strings.HasSuffix(cfg.Name, "-implement") {
		t.Fatalf("implement cfg name = %q, want -implement suffix", cfg.Name)
	}
}

func TestImplementRejectsSavedRunMissingCanonicalIntake(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runDir := SavedRunDir(projectRoot, "debate")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir saved run: %v", err)
	}
	cfg := goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeWorker,
		Objective: "consensus fixes",
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	if err := os.WriteFile(ObjectiveContractPath(runDir), []byte("{\n  \"version\": 1,\n  \"objective_hash\": \"sha256:demo\",\n  \"state\": \"locked\",\n  \"clauses\": []\n}\n"), 0o644); err != nil {
		t.Fatalf("write objective contract: %v", err)
	}
	if err := os.WriteFile(ObligationModelPath(runDir), []byte("{\n  \"version\": 1,\n  \"objective_contract_hash\": \"sha256:demo\",\n  \"required\": [],\n  \"optional\": [],\n  \"guardrails\": []\n}\n"), 0o644); err != nil {
		t.Fatalf("write obligation model: %v", err)
	}
	if err := os.WriteFile(AssurancePlanPath(runDir), []byte("{\n  \"version\": 1,\n  \"scenarios\": []\n}\n"), 0o644); err != nil {
		t.Fatalf("write assurance plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "summary.md"), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}

	err = Implement(projectRoot, []string{"--from", "debate", "--write-config"})
	if err == nil {
		t.Fatal("Implement unexpectedly succeeded without canonical intake")
	}
	if !strings.Contains(err.Error(), "intake") || !strings.Contains(err.Error(), "canonical") {
		t.Fatalf("Implement error = %v, want canonical intake rejection", err)
	}
}

func TestImplementStartCreatesFreshCharterWithPreservedRootLineage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")
	sourceMeta, sourceCharter := writeSavedPhaseSourceFixture(t, projectRoot, "debate", "debate")
	installPhaseStartFakeTmux(t)
	stubLaunchRunRuntimeHost(t)

	if err := Implement(projectRoot, []string{"--from", "debate"}); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	assertPhaseRunLineage(t, projectRoot, derivePhaseRunName("debate", "implement", ""), "implement", "debate", sourceMeta, sourceCharter)
}
