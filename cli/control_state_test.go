package cli

import (
	"os"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestEnsureControlStateMapsRunMetadataIntoRunIdentity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")

	cfg := &goalx.Config{
		Name:      "control-state",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	charter, err := NewRunCharter(runDir, cfg.Name, cfg.Objective, meta)
	if err != nil {
		t.Fatalf("NewRunCharter: %v", err)
	}
	if err := SaveRunCharter(RunCharterPath(runDir), charter); err != nil {
		t.Fatalf("SaveRunCharter: %v", err)
	}
	meta.PhaseKind = "develop"
	meta.CharterID = charter.CharterID
	charterHash, err := hashRunCharter(charter)
	if err != nil {
		t.Fatalf("hashRunCharter: %v", err)
	}
	meta.CharterHash = charterHash
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}

	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	identity, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("run identity missing")
	}
	if identity.RunID != meta.RunID {
		t.Fatalf("run identity run_id = %q, want %q", identity.RunID, meta.RunID)
	}
	if identity.RunName != cfg.Name {
		t.Fatalf("run identity run_name = %q, want %q", identity.RunName, cfg.Name)
	}
	if identity.ProjectRoot != repo {
		t.Fatalf("run identity project_root = %q, want %q", identity.ProjectRoot, repo)
	}
	if identity.Epoch != meta.Epoch {
		t.Fatalf("run identity epoch = %d, want %d", identity.Epoch, meta.Epoch)
	}
	if identity.CharterPath != RunCharterPath(runDir) {
		t.Fatalf("run identity charter_path = %q, want %q", identity.CharterPath, RunCharterPath(runDir))
	}
	if identity.CharterDigest == "" {
		t.Fatal("run identity charter digest empty")
	}
	if identity.CharterID != meta.CharterID {
		t.Fatalf("run identity charter id = %q, want %q", identity.CharterID, meta.CharterID)
	}
	if identity.CharterDigest != meta.CharterHash {
		t.Fatalf("run identity charter digest = %q, want %q", identity.CharterDigest, meta.CharterHash)
	}
	if identity.Mode != string(cfg.Mode) {
		t.Fatalf("run identity mode = %q, want %q", identity.Mode, cfg.Mode)
	}
	if identity.PhaseKind != meta.PhaseKind {
		t.Fatalf("run identity phase_kind = %q, want %q", identity.PhaseKind, meta.PhaseKind)
	}

	runState, err := LoadControlRunState(ControlRunStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if runState == nil {
		t.Fatal("run state missing")
	}
	if runState.GoalState != "open" {
		t.Fatalf("run state goal_state = %q, want open", runState.GoalState)
	}
	if runState.ContinuityState != "running" {
		t.Fatalf("run state continuity_state = %q, want running", runState.ContinuityState)
	}
	if _, err := os.Stat(ControlOperationsPath(runDir)); err != nil {
		t.Fatalf("stat control operations: %v", err)
	}
}

func TestEnsureControlStateDoesNotInventRunIDWithoutRunMetadata(t *testing.T) {
	runDir := t.TempDir()
	cfg := &goalx.Config{
		Name:      "control-state",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}

	identity, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir))
	if err != nil {
		t.Fatalf("LoadControlRunIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("run identity missing")
	}
	if identity.RunID != "" {
		t.Fatalf("run identity run_id = %q, want empty without canonical metadata", identity.RunID)
	}
	if identity.Epoch != 0 {
		t.Fatalf("run identity epoch = %d, want 0 without canonical metadata", identity.Epoch)
	}
	if identity.RunName != cfg.Name {
		t.Fatalf("run identity run_name = %q, want %q", identity.RunName, cfg.Name)
	}
	if identity.Mode != string(cfg.Mode) {
		t.Fatalf("run identity mode = %q, want %q", identity.Mode, cfg.Mode)
	}
}

func TestLoadControlStateLeavesFilesUntouched(t *testing.T) {
	runDir := t.TempDir()
	if err := os.MkdirAll(ControlLeasesDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir leases dir: %v", err)
	}
	if err := os.MkdirAll(ControlInboxDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir inbox dir: %v", err)
	}
	identityBefore := []byte("{\n  \"version\": 1,\n  \"run_name\": \"demo\"\n}\n")
	stateBefore := []byte("{\n  \"version\": 1,\n  \"lifecycle_state\": \"active\"\n}\n")
	remindersBefore := []byte("{\n  \"version\": 1,\n  \"items\": []\n}\n")
	deliveriesBefore := []byte("{\n  \"version\": 1,\n  \"items\": []\n}\n")
	leaseBefore := []byte("{\n  \"version\": 1,\n  \"holder\": \"master\"\n}\n")
	for path, data := range map[string][]byte{
		ControlRunIdentityPath(runDir):     identityBefore,
		ControlRunStatePath(runDir):        stateBefore,
		ControlRemindersPath(runDir):       remindersBefore,
		ControlDeliveriesPath(runDir):      deliveriesBefore,
		ControlLeasePath(runDir, "master"): leaseBefore,
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}

	if _, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir)); err != nil {
		t.Fatalf("LoadControlRunIdentity: %v", err)
	}
	if _, err := LoadControlRunState(ControlRunStatePath(runDir)); err != nil {
		t.Fatalf("LoadControlRunState: %v", err)
	}
	if _, err := LoadControlReminders(ControlRemindersPath(runDir)); err != nil {
		t.Fatalf("LoadControlReminders: %v", err)
	}
	if _, err := LoadControlDeliveries(ControlDeliveriesPath(runDir)); err != nil {
		t.Fatalf("LoadControlDeliveries: %v", err)
	}
	if _, err := LoadControlLease(ControlLeasePath(runDir, "master")); err != nil {
		t.Fatalf("LoadControlLease: %v", err)
	}

	for path, want := range map[string][]byte{
		ControlRunIdentityPath(runDir):     identityBefore,
		ControlRunStatePath(runDir):        stateBefore,
		ControlRemindersPath(runDir):       remindersBefore,
		ControlDeliveriesPath(runDir):      deliveriesBefore,
		ControlLeasePath(runDir, "master"): leaseBefore,
	} {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Fatalf("%s changed:\nwant %s\ngot  %s", path, string(want), string(got))
		}
	}
}

func TestSaveControlRunStateDoesNotPersistRecommendationField(t *testing.T) {
	runDir := t.TempDir()
	path := ControlRunStatePath(runDir)
	if err := SaveControlRunState(path, &ControlRunState{
		Version:         1,
		GoalState:       "open",
		ContinuityState: "running",
		Phase:           "working",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read control run state: %v", err)
	}
	text := string(data)
	if strings.Contains(text, `"recommendation"`) {
		t.Fatalf("control run state should not persist recommendation:\n%s", text)
	}
	if strings.Contains(text, `"lifecycle_state"`) {
		t.Fatalf("control run state should not persist legacy lifecycle_state:\n%s", text)
	}
}
