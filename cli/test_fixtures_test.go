package cli

import (
	"os"
	"path/filepath"
	"testing"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func writeSavedRunFixture(t *testing.T, projectRoot, runName string, cfg goalx.Config, files map[string]string) {
	t.Helper()
	if files == nil {
		files = map[string]string{}
	}

	runDir := SavedRunDir(projectRoot, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run-spec.yaml: %v", err)
	}
	if err := SaveRunIntake(SavedRunIntakePath(runDir), &RunIntake{
		Version:      1,
		Objective:    cfg.Objective,
		Intent:       runIntentDeliver,
		Readonly:     len(cfg.Target.Readonly) > 0,
		ContextFiles: append([]string(nil), cfg.Context.Files...),
		ContextRefs:  append([]string(nil), cfg.Context.Refs...),
	}); err != nil {
		t.Fatalf("write intake.json: %v", err)
	}
	objectiveHash := hashObjectiveText(cfg.Objective)
	if _, ok := files["objective-contract.json"]; !ok {
		files["objective-contract.json"] = "{\n  \"version\": 1,\n  \"objective_hash\": \"" + objectiveHash + "\",\n  \"state\": \"locked\",\n  \"clauses\": []\n}\n"
	}
	if _, ok := files["obligation-model.json"]; !ok {
		files["obligation-model.json"] = "{\n  \"version\": 1,\n  \"objective_contract_hash\": \"" + objectiveHash + "\",\n  \"required\": [],\n  \"optional\": [],\n  \"guardrails\": []\n}\n"
	}
	if _, ok := files["assurance-plan.json"]; !ok {
		files["assurance-plan.json"] = "{\n  \"version\": 1,\n  \"scenarios\": []\n}\n"
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(runDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func writeResolvedSavedRunFixture(t *testing.T, projectRoot, runName string, opts launchOptions, files map[string]string) goalx.Config {
	t.Helper()

	opts.Name = runName
	resolved, err := resolveLaunchConfig(projectRoot, opts)
	if err != nil {
		t.Fatalf("resolveLaunchConfig: %v", err)
	}
	writeSavedRunFixture(t, projectRoot, runName, resolved.Config, files)
	return resolved.Config
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

func writeProjectConfigFixture(t *testing.T, projectRoot string, content string) {
	t.Helper()

	goalxDir := filepath.Join(projectRoot, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write project config.yaml: %v", err)
	}
}
