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

func TestAppendMemorySeedFromVerifyResult(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, _, _ := writeReadOnlyRunFixture(t, repo)
	evidencePath := filepath.Join(runDir, "evidence-gate.txt")
	if err := os.WriteFile(evidencePath, []byte("gate ok\n"), 0o644); err != nil {
		t.Fatalf("write acceptance evidence: %v", err)
	}
	exitCode := 0
	if err := writeAssuranceFixture(t, runDir, &AcceptanceState{
		Version: 2,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Command: "printf 'gate ok\\n'", State: acceptanceCheckStateActive},
		},
		LastResult: AcceptanceResult{
			CheckedAt:    "2026-03-27T08:00:00Z",
			ExitCode:     &exitCode,
			EvidencePath: evidencePath,
			CheckResults: []AcceptanceCheckResult{
				{ID: "chk-1", Command: "printf 'gate ok\\n'", ExitCode: &exitCode, EvidencePath: evidencePath},
			},
		},
	}); err != nil {
		t.Fatalf("SaveAcceptanceState: %v", err)
	}

	if err := AppendMemorySeedFromVerifyResult(runDir); err != nil {
		t.Fatalf("AppendMemorySeedFromVerifyResult: %v", err)
	}

	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		t.Fatalf("LoadMemorySeeds: %v", err)
	}
	if len(seeds) != 1 {
		t.Fatalf("memory seeds len = %d, want 1", len(seeds))
	}
	seed := seeds[0]
	if seed.Kind != "verify_result" {
		t.Fatalf("seed kind = %q, want verify_result", seed.Kind)
	}
	if seed.Run != runName {
		t.Fatalf("seed run = %q, want %q", seed.Run, runName)
	}
	if strings.TrimSpace(seed.CreatedAt) == "" {
		t.Fatalf("seed created_at = %q, want evidence log timestamp", seed.CreatedAt)
	}
	if !strings.Contains(seed.Message, "scenario=scenario-chk-1") {
		t.Fatalf("seed message = %q, want scenario detail", seed.Message)
	}
	for _, banned := range []string{"important", "best practice", "recommended"} {
		if strings.Contains(strings.ToLower(seed.Message), banned) {
			t.Fatalf("seed message should stay factual, found %q in %q", banned, seed.Message)
		}
	}
	if len(seed.Evidence) < 2 {
		t.Fatalf("seed evidence len = %d, want at least 2", len(seed.Evidence))
	}
}

func TestAppendMemorySeedFromVerifyResultUsesEvidenceLogWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	_, runDir, _, _ := writeReadOnlyRunFixture(t, repo)
	if err := AppendEvidenceLogEvent(EvidenceLogPath(runDir), "scenario.executed", "master", EvidenceEventBody{
		ScenarioID:   "scenario-cli-first-run",
		Scope:        "run-root",
		Revision:     "def456",
		HarnessKind:  "cli",
		OracleResult: map[string]any{"exit_code": 0},
		ArtifactRefs: []string{"reports/assurance/stdout.txt"},
	}); err != nil {
		t.Fatalf("AppendEvidenceLogEvent: %v", err)
	}

	if err := AppendMemorySeedFromVerifyResult(runDir); err != nil {
		t.Fatalf("AppendMemorySeedFromVerifyResult: %v", err)
	}

	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		t.Fatalf("LoadMemorySeeds: %v", err)
	}
	if len(seeds) != 1 {
		t.Fatalf("memory seeds len = %d, want 1", len(seeds))
	}
	if !strings.Contains(seeds[0].Message, "scenario=scenario-cli-first-run") {
		t.Fatalf("seed message = %q, want scenario detail", seeds[0].Message)
	}
}

func TestCollectRunMemorySeedsIncludesSavedArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	runName := "demo"
	runDir := goalx.RunDir(projectRoot, runName)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
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
		t.Fatalf("write run spec: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ReportsDir(runDir), "repo-summary.md"), []byte("repo report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	evidencePath := filepath.Join(runDir, "evidence-saved.txt")
	if err := os.WriteFile(evidencePath, []byte("gate ok\n"), 0o644); err != nil {
		t.Fatalf("write evidence artifact: %v", err)
	}
	if err := writeAssuranceFixture(t, runDir, &AcceptanceState{
		Version: 2,
		Checks: []AcceptanceCheck{
			{ID: "chk-1", Command: "printf 'gate ok\\n'", State: acceptanceCheckStateActive},
		},
		LastResult: AcceptanceResult{
			CheckedAt:    "2026-03-27T08:00:00Z",
			ExitCode:     intPtr(0),
			EvidencePath: evidencePath,
			CheckResults: []AcceptanceCheckResult{
				{ID: "chk-1", Command: "printf 'gate ok\\n'", ExitCode: intPtr(0), EvidencePath: evidencePath},
			},
		},
	}); err != nil {
		t.Fatalf("write assurance fixture: %v", err)
	}
	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	seeds, err := CollectRunMemorySeeds(runDir)
	if err != nil {
		t.Fatalf("CollectRunMemorySeeds: %v", err)
	}
	savedPrefix := SavedRunDir(projectRoot, runName)
	foundSavedArtifact := false
	for _, seed := range seeds {
		for _, evidence := range seed.Evidence {
			if strings.HasPrefix(evidence.Path, savedPrefix) {
				foundSavedArtifact = true
			}
		}
	}
	if !foundSavedArtifact {
		t.Fatalf("expected saved artifact seed evidence under %s, got %+v", savedPrefix, seeds)
	}
}

func TestCollectRunMemorySeedsIncludesConfiguredSavedArtifacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectRoot, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".goalx", "config.yaml"), []byte("run_root: ./custom-runs\nsaved_run_root: ./custom-saved\n"), 0o644); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	runName := "demo"
	layers, err := goalx.LoadConfigLayers(projectRoot)
	if err != nil {
		t.Fatalf("LoadConfigLayers: %v", err)
	}
	runDir := goalx.ResolveRunDir(projectRoot, runName, &layers.Config)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.MkdirAll(ReportsDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir reports dir: %v", err)
	}

	cfg := goalx.Config{
		Name:         runName,
		Mode:         goalx.ModeWorker,
		Objective:    "inspect",
		SavedRunRoot: "./custom-saved",
		Target:       goalx.TargetConfig{Files: []string{"notes.md"}},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	seedSaveRunProvenance(t, projectRoot, runDir, runName, cfg.Objective)
	seedSaveSessionIdentity(t, runDir, "session-1", goalx.ModeWorker, "codex", "", cfg.Target, goalx.LocalValidationConfig{})
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ReportsDir(runDir), "repo-summary.md"), []byte("repo report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := Save(projectRoot, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	seeds, err := CollectRunMemorySeeds(runDir)
	if err != nil {
		t.Fatalf("CollectRunMemorySeeds: %v", err)
	}
	savedPrefix := filepath.Join(projectRoot, "custom-saved", runName)
	foundSavedArtifact := false
	for _, seed := range seeds {
		for _, evidence := range seed.Evidence {
			if strings.HasPrefix(evidence.Path, savedPrefix) {
				foundSavedArtifact = true
			}
		}
	}
	if !foundSavedArtifact {
		t.Fatalf("expected saved artifact seed evidence under %s, got %+v", savedPrefix, seeds)
	}
}

func TestRuntimeHostRefreshesMemorySeedsWithoutCanonicalMutation(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	prevLookPath := lookPathFunc
	t.Cleanup(func() { lookPathFunc = prevLookPath })
	lookPathFunc = func(file string) (string, error) {
		switch file {
		case "gitnexus", "npx":
			return "", os.ErrNotExist
		default:
			return prevLookPath(file)
		}
	}

	prev := resourceReadFile
	t.Cleanup(func() { resourceReadFile = prev })
	resourceReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/proc/meminfo":
			return []byte("MemTotal: 32768 kB\nMemAvailable: 20971520 kB\nSwapTotal: 16384 kB\nSwapFree: 16384 kB\n"), nil
		case "/proc/pressure/memory":
			return []byte("some avg10=0.10 avg60=0 avg300=0 total=0\nfull avg10=0 avg60=0 avg300=0 total=0\n"), nil
		case "/sys/fs/cgroup/memory.current", "/sys/fs/cgroup/memory.high", "/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.swap.current", "/sys/fs/cgroup/memory.swap.max":
			return []byte("0\n"), nil
		case "/sys/fs/cgroup/memory.events":
			return []byte("low 0\nhigh 0\nmax 0\noom 0\noom_kill 0\n"), nil
		}
		if strings.HasSuffix(path, "/status") {
			return []byte("Name:\tgoalx\nVmRSS:\t1048576 kB\n"), nil
		}
		return nil, os.ErrNotExist
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	payloads := writeCanonicalMemorySentinels(t, home)

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("Please authenticate in browser to continue\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		t.Fatalf("LoadMemorySeeds: %v", err)
	}
	foundProviderDialog := false
	for _, seed := range seeds {
		if seed.Kind == "provider_dialog_visible" && strings.Contains(seed.Message, "target=session-1") {
			foundProviderDialog = true
		}
	}
	if !foundProviderDialog {
		t.Fatalf("memory seeds missing provider dialog incident: %+v", seeds)
	}
	for path, want := range payloads {
		assertFileUnchanged(t, path, want)
	}
}

func TestRuntimeHostRefreshesCompiledMemoryContext(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}

	writeCanonicalMemoryEntries(t, map[MemoryKind][]MemoryEntry{
		MemoryKindFact: {
			{
				ID:                "mem_fact_1",
				Kind:              MemoryKindFact,
				Statement:         "host is ops-3",
				Selectors:         map[string]string{"project_id": goalx.ProjectID(repo)},
				VerificationState: "validated",
				Confidence:        "grounded",
				CreatedAt:         "2026-03-27T00:00:00Z",
				UpdatedAt:         "2026-03-27T00:00:00Z",
			},
		},
	})

	masterCapture := filepath.Join(t.TempDir(), "master-pane.txt")
	sessionCapture := filepath.Join(t.TempDir(), "session-pane.txt")
	if err := os.WriteFile(masterCapture, []byte("master pane\n"), 0o644); err != nil {
		t.Fatalf("write master capture: %v", err)
	}
	if err := os.WriteFile(sessionCapture, []byte("session pane\n"), 0o644); err != nil {
		t.Fatalf("write session capture: %v", err)
	}
	t.Setenv("TMUX_MASTER_CAPTURE", masterCapture)
	t.Setenv("TMUX_SESSION1_CAPTURE", sessionCapture)
	installGuidanceFakeTmux(t, []string{"session-1"})

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
	}

	var query MemoryQuery
	queryData, err := os.ReadFile(MemoryQueryPath(runDir))
	if err != nil {
		t.Fatalf("ReadFile memory query: %v", err)
	}
	if err := json.Unmarshal(queryData, &query); err != nil {
		t.Fatalf("json.Unmarshal memory query: %v", err)
	}
	if query.ProjectID != goalx.ProjectID(repo) {
		t.Fatalf("memory query project_id = %q, want %q", query.ProjectID, goalx.ProjectID(repo))
	}

	var context MemoryContext
	contextData, err := os.ReadFile(MemoryContextPath(runDir))
	if err != nil {
		t.Fatalf("ReadFile memory context: %v", err)
	}
	if err := json.Unmarshal(contextData, &context); err != nil {
		t.Fatalf("json.Unmarshal memory context: %v", err)
	}
	if context.BuiltAt == "" {
		t.Fatal("memory context built_at empty")
	}
	if len(context.Facts) != 1 || context.Facts[0] != "host is ops-3" {
		t.Fatalf("memory context facts = %+v, want promoted canonical fact", context.Facts)
	}
}
