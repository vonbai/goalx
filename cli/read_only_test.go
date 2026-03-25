package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestObserveLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	ensureSharedProofEvidence(t)
	logPath := installFakeTmux(t, "master")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})
	if !strings.Contains(out, "## Run: "+runName) {
		t.Fatalf("observe output missing run header:\n%s", out)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, RunStatusPath(runDir), statusBefore)
	assertTmuxTouched(t, logPath)
}

func TestObserveShowsDegradedRunWithoutTmux(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, LifecycleState: "active"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "master.jsonl"), []byte("{\"round\":1,\"desc\":\"master still coordinating\",\"status\":\"active\"}\n"), 0o644); err != nil {
		t.Fatalf("write master journal: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})
	for _, want := range []string{"transport degraded", "master still coordinating", "charter=ok", "epoch=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, RunStatusPath(runDir), statusBefore)
}

func TestObservePrefersCanonicalControlFactsOverStaleActivitySnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "fresh work"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", 1234); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := ExpireControlLease(runDir, "sidecar"); err != nil {
		t.Fatalf("ExpireControlLease sidecar: %v", err)
	}
	if err := SaveControlReminders(ControlRemindersPath(runDir), &ControlReminders{
		Version: 1,
		Items: []ControlReminder{
			{ReminderID: "rem-1", DedupeKey: "master-wake", Reason: "control-cycle", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlReminders: %v", err)
	}
	if err := SaveControlDeliveries(ControlDeliveriesPath(runDir), &ControlDeliveries{
		Version: 1,
		Items: []ControlDelivery{
			{DeliveryID: "del-1", DedupeKey: "master-wake", Status: "failed", Target: "gx-demo:master"},
		},
	}); err != nil {
		t.Fatalf("SaveControlDeliveries: %v", err)
	}
	if err := SaveActivitySnapshot(runDir, &ActivitySnapshot{
		Version:   1,
		CheckedAt: "2026-03-25T00:00:00Z",
		Run: ActivityRunInfo{
			ProjectID:   goalx.ProjectID(repo),
			RunName:     cfg.Name,
			RunID:       meta.RunID,
			Epoch:       meta.Epoch,
			TmuxSession: goalx.TmuxSessionName(repo, cfg.Name),
		},
		Queue: ActivityQueue{
			MasterUnread:     4,
			RemindersDue:     2,
			DeliveriesFailed: 1,
		},
		Actors: map[string]ActivityActor{
			"master":  {Lease: "healthy"},
			"sidecar": {Lease: "expired"},
		},
	}); err != nil {
		t.Fatalf("SaveActivitySnapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "master.jsonl"), []byte("{\"round\":1,\"desc\":\"master still coordinating\",\"status\":\"active\"}\n"), 0o644); err != nil {
		t.Fatalf("write master journal: %v", err)
	}

	out := captureStdout(t, func() {
		if err := Observe(repo, []string{"--run", cfg.Name}); err != nil {
			t.Fatalf("Observe: %v", err)
		}
	})
	for _, want := range []string{
		"unread_inbox=1",
		"master_lease=healthy",
		"sidecar_lease=expired",
		"reminders_due=1",
		"deliveries_failed=1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("observe output missing %q:\n%s", want, out)
		}
	}
}

func TestObserveHelpDoesNotResolveRun(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Observe(t.TempDir(), []string{"--help"}); err != nil {
			t.Fatalf("Observe --help: %v", err)
		}
	})
	if !strings.Contains(out, "usage: goalx observe [NAME]") {
		t.Fatalf("observe help output = %q", out)
	}
}

func TestStatusLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	_, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)

	out := captureStdout(t, func() {
		if err := Status(repo, []string{"--run", filepath.Base(runDir)}); err != nil {
			t.Fatalf("Status: %v", err)
		}
	})
	for _, want := range []string{"SESSION", "charter=ok", "epoch=1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("status output missing %q:\n%s", want, out)
		}
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, RunStatusPath(runDir), statusBefore)
}

func TestReportLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)

	out := captureStdout(t, func() {
		if err := Report(repo, []string{"--run", runName}); err != nil {
			t.Fatalf("Report: %v", err)
		}
	})
	if !strings.Contains(out, "=== Report: "+runName+" ===") {
		t.Fatalf("report output missing header:\n%s", out)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, RunStatusPath(runDir), statusBefore)
}

func TestSaveLeavesRunAndStatusStateUntouched(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, runStateBefore, statusBefore := writeReadOnlyRunFixture(t, repo)
	wtPath := WorktreePath(runDir, runName, 1)
	if err := os.MkdirAll(wtPath, 0o755); err != nil {
		t.Fatalf("mkdir worktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "report.md"), []byte("saved report\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}

	if err := Save(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, RunStatusPath(runDir), statusBefore)
}

func TestVerifyDoesNotRewriteRunStateOrStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName := "verify-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	cfg := goalx.Config{
		Name:       runName,
		Mode:       goalx.ModeDevelop,
		Objective:  "ship feature",
		Acceptance: goalx.AcceptanceConfig{Command: "printf 'gate ok\\n'"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"verify-run","mode":"develop","active":true,"phase":"working","recommendation":"keep going","updated_at":"2026-03-23T00:00:00Z"}`)
	if err := os.WriteFile(RunRuntimeStatePath(runDir), runStateBefore, 0o644); err != nil {
		t.Fatalf("write run state: %v", err)
	}
	statusBefore := []byte(`{"run":"verify-run","phase":"working","recommendation":"keep going"}`)
	if err := os.MkdirAll(filepath.Dir(RunStatusPath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir status dir: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), statusBefore, 0o644); err != nil {
		t.Fatalf("write run status record: %v", err)
	}
	if err := os.WriteFile(GoalPath(runDir), []byte(`{"version":1,"required":[{"id":"req-1","text":"ship feature","source":"user","state":"claimed","evidence_paths":["/tmp/e2e.txt"]}],"optional":[]}`), 0o644); err != nil {
		t.Fatalf("write goal state: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{Version: 1, Objective: "ship feature", BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)

	if err := Verify(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	assertFileUnchanged(t, RunRuntimeStatePath(runDir), runStateBefore)
	assertFileUnchanged(t, RunStatusPath(runDir), statusBefore)
}

func writeReadOnlyRunFixture(t *testing.T, repo string) (string, string, []byte, []byte) {
	t.Helper()

	runName := "readonly-run"
	runDir := goalx.RunDir(repo, runName)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatalf("mkdir run dir: %v", err)
	}
	if err := os.MkdirAll(StateDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goalx"), 0o755); err != nil {
		t.Fatalf("mkdir goalx dir: %v", err)
	}
	cfg := goalx.Config{
		Name:      runName,
		Mode:      goalx.ModeDevelop,
		Objective: "read only",
		Master:    goalx.MasterConfig{Engine: "codex"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(RunSpecPath(runDir), data, 0o644); err != nil {
		t.Fatalf("write run spec: %v", err)
	}
	runStateBefore := []byte(`{"version":1,"run":"readonly-run","mode":"develop","objective":"read only","active":true,"phase":"working","recommendation":"keep going","updated_at":"2026-03-23T00:00:00Z"}`)
	if err := os.WriteFile(RunRuntimeStatePath(runDir), runStateBefore, 0o644); err != nil {
		t.Fatalf("write run state: %v", err)
	}
	statusBefore := []byte(`{"run":"readonly-run","phase":"working","recommendation":"keep going","active":true}`)
	if err := os.MkdirAll(filepath.Dir(RunStatusPath(runDir)), 0o755); err != nil {
		t.Fatalf("mkdir status dir: %v", err)
	}
	if err := os.WriteFile(RunStatusPath(runDir), statusBefore, 0o644); err != nil {
		t.Fatalf("write run status record: %v", err)
	}
	if err := SaveRunMetadata(RunMetadataPath(runDir), &RunMetadata{
		Version:      1,
		Objective:    cfg.Objective,
		BaseRevision: strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD")),
	}); err != nil {
		t.Fatalf("write run metadata: %v", err)
	}
	seedRunCharterForTests(t, runDir, runName, repo)
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	identity, err := NewSessionIdentity(runDir, "session-1", sessionRoleKind(goalx.ModeDevelop), goalx.ModeDevelop, "codex", "", "", "", "", "", cfg.Target)
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	return runName, runDir, runStateBefore, statusBefore
}

func assertFileUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s changed:\nwant %s\ngot  %s", path, string(want), string(got))
	}
}

func assertTmuxTouched(t *testing.T, logPath string) {
	t.Helper()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Fatal("expected tmux log to be touched")
	}
}
