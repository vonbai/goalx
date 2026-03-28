package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
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
  develop:
    engine: codex
    model: gpt-5.4
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeResolvedSavedRunFixture(t, projectRoot, "debate", launchOptions{
		Objective: "consensus fixes",
		Mode:      goalx.ModeResearch,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  develop:
    engine: claude-code
    model: opus
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}, nil); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "claude-code" || cfg.Master.Model != "opus" {
		t.Fatalf("master = %s/%s, want claude-code/opus", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Develop.Engine != "claude-code" || cfg.Roles.Develop.Model != "opus" {
		t.Fatalf("develop role = %s/%s, want claude-code/opus", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
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
  develop:
    engine: claude-code
    model: opus
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeResolvedSavedRunFixture(t, projectRoot, "debate", launchOptions{
		Objective: "consensus fixes",
		Mode:      goalx.ModeResearch,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})
	writeSelectionSnapshotFixture(t, SavedRunDir(projectRoot, "debate"), testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterCandidates:   []string{"codex/gpt-5.4"},
			ResearchCandidates: []string{"claude-code/opus"},
			DevelopCandidates:  []string{"codex/gpt-5.4-mini", "codex/gpt-5.4"},
		},
		Master:   goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
		Research: goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
		Develop:  goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini", Effort: goalx.EffortMedium},
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

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}, nil); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Master.Engine != "codex" || cfg.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %s/%s, want saved snapshot codex/gpt-5.4", cfg.Master.Engine, cfg.Master.Model)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "gpt-5.4-mini" {
		t.Fatalf("develop role = %s/%s, want saved snapshot codex/gpt-5.4-mini", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
	if cfg.Target.Files[0] != "config.go" {
		t.Fatalf("target.files = %#v, want current shared config target", cfg.Target.Files)
	}
	if cfg.LocalValidation.Command != "go test ./cli/..." {
		t.Fatalf("local_validation.command = %q, want current shared config command", cfg.LocalValidation.Command)
	}
}

func TestImplementIgnoresLegacyNextConfigSelectionOverrides(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: claude-code
  model: opus
roles:
  develop:
    engine: claude-code
    model: opus
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Parallel:  2,
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Develop: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	var nc nextConfigJSON
	if err := json.Unmarshal([]byte(`{
		"parallel": 4,
		"objective": "custom implement objective",
		"dimensions": ["depth", "adversarial", "evidence", "perfectionist"],
		"engine": "codex",
		"model": "fast",
		"preset": "codex",
		"mode": "develop",
		"route_role": "develop",
		"route_profile": "build_fast",
		"effort": "high"
	}`), &nc); err != nil {
		t.Fatalf("unmarshal next_config: %v", err)
	}
	if err := Implement(projectRoot, []string{"--from", "debate", "--budget", "20m", "--write-config"}, &nc); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Parallel != 4 {
		t.Fatalf("parallel = %d, want 4", cfg.Parallel)
	}
	if cfg.Roles.Develop.Engine != "claude-code" || cfg.Roles.Develop.Model != "opus" {
		t.Fatalf("develop role = %s/%s, want claude-code/opus", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
	if cfg.Objective != "custom implement objective" {
		t.Fatalf("objective = %q, want custom implement objective", cfg.Objective)
	}
	if cfg.Budget.MaxDuration != 20*60*1_000_000_000 {
		t.Fatalf("budget = %v, want 20m", cfg.Budget.MaxDuration)
	}
	if len(cfg.Sessions) != 4 {
		t.Fatalf("sessions = %#v, want 4 seeded sessions", cfg.Sessions)
	}
	for i, session := range cfg.Sessions {
		if got, want := session.Dimensions, []string{"depth", "adversarial", "evidence", "perfectionist"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] || got[3] != want[3] {
			t.Fatalf("session[%d].dimensions = %#v, want %#v", i, got, want)
		}
	}
}

func TestImplementAttachesNextConfigDimensionsToSessions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Parallel:  2,
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Develop: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
		},
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	nc := &nextConfigJSON{
		Parallel:   3,
		Dimensions: []string{"depth", "adversarial", "evidence"},
	}
	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}, nc); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if len(cfg.Sessions) != 3 {
		t.Fatalf("sessions = %#v, want 3 seeded sessions", cfg.Sessions)
	}
	for i := range cfg.Sessions {
		if got, want := cfg.Sessions[i].Dimensions, []string{"depth", "adversarial", "evidence"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
			t.Fatalf("sessions[%d].dimensions = %#v, want %#v", i, got, want)
		}
	}
}

func TestImplementIgnoresLegacyNextConfigPresetForResolvedSavedRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeProjectConfigFixture(t, projectRoot, `
master:
  engine: codex
  model: gpt-5.4
roles:
  develop:
    engine: codex
    model: gpt-5.4
target:
  files: [cli/]
local_validation:
  command: go test ./...
`)
	writeResolvedSavedRunFixture(t, projectRoot, "debate", launchOptions{
		Objective: "consensus fixes",
		Mode:      goalx.ModeResearch,
	}, map[string]string{
		"summary.md":          "# summary\n",
		"session-1-report.md": "# report\n",
	})

	var nc nextConfigJSON
	if err := json.Unmarshal([]byte(`{"preset":"claude-h"}`), &nc); err != nil {
		t.Fatalf("unmarshal next_config: %v", err)
	}

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}, &nc); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(projectRoot, ".goalx", "goalx.yaml"))
	if err != nil {
		t.Fatalf("load goalx.yaml: %v", err)
	}
	if cfg.Roles.Develop.Engine != "codex" || cfg.Roles.Develop.Model != "gpt-5.4" {
		t.Fatalf("develop role = %s/%s, want codex/gpt-5.4", cfg.Roles.Develop.Engine, cfg.Roles.Develop.Model)
	}
}

func TestImplementUsesSavedManifestReportArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	writeSavedRunFixture(t, projectRoot, "debate", goalx.Config{
		Name:      "debate",
		Mode:      goalx.ModeResearch,
		Objective: "consensus fixes",
		Parallel:  1,
		Master:    goalx.MasterConfig{Engine: "claude-code", Model: "opus"},
		Roles: goalx.RoleDefaultsConfig{
			Develop: goalx.SessionConfig{Engine: "claude-code", Model: "opus"},
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
				Mode: string(goalx.ModeResearch),
				Artifacts: []ArtifactMeta{
					{Kind: "report", Path: reportPath, RelPath: "custom-findings.txt", DurableName: "session-1-report.md"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("SaveArtifacts: %v", err)
	}

	if err := Implement(projectRoot, []string{"--from", "debate", "--write-config"}, nil); err != nil {
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

func TestImplementStartCreatesFreshCharterWithPreservedRootLineage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := initGitRepo(t)
	writeAndCommit(t, projectRoot, "base.txt", "base", "base commit")
	sourceMeta, sourceCharter := writeSavedPhaseSourceFixture(t, projectRoot, "debate", "debate")
	installPhaseStartFakeTmux(t)
	stubLaunchRunSidecar(t)

	if err := Implement(projectRoot, []string{"--from", "debate"}, nil); err != nil {
		t.Fatalf("Implement: %v", err)
	}

	assertPhaseRunLineage(t, projectRoot, derivePhaseRunName("debate", "implement", ""), "implement", "debate", sourceMeta, sourceCharter)
}
