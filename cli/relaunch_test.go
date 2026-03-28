package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestRelaunchMasterIgnoresLegacyHandoffFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	legacyPath := filepath.Join(ControlDir(runDir), "handoffs", "master.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("mkdir legacy handoff dir: %v", err)
	}
	if err := os.WriteFile(legacyPath, []byte("{"), 0o644); err != nil {
		t.Fatalf("write malformed legacy handoff: %v", err)
	}

	err = relaunchMaster(repo, runDir, goalx.TmuxSessionName(repo, runName), cfg)
	if err != nil {
		t.Fatalf("relaunchMaster: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	for _, want := range []string{
		"kill-window -t " + goalx.TmuxSessionName(repo, runName) + ":master",
		"new-window -t " + goalx.TmuxSessionName(repo, runName) + " -n master -c " + RunWorktreePath(runDir),
		filepath.Join(runDir, "master.md"),
	} {
		if !strings.Contains(logText, want) {
			t.Fatalf("tmux log missing %q:\n%s", want, logText)
		}
	}
}

func TestRelaunchMasterRerendersProtocolWithFreshFacts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, "session-2"), nil, 0o644); err != nil {
		t.Fatalf("seed session-2 journal: %v", err)
	}
	identity, err := NewSessionIdentity(runDir, "session-2", "master-derived-develop", goalx.ModeDevelop, "codex", "gpt-5.4", "", "", "", cfg.Target)
	if err != nil {
		t.Fatalf("NewSessionIdentity: %v", err)
	}
	identity.LocalValidationCommand = goalx.ResolveLocalValidationCommand(cfg)
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-2"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:  "session-2",
		State: "active",
		Mode:  string(goalx.ModeDevelop),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	if err := relaunchMaster(repo, runDir, goalx.TmuxSessionName(repo, runName), cfg); err != nil {
		t.Fatalf("relaunchMaster: %v", err)
	}

	out, err := os.ReadFile(filepath.Join(runDir, "master.md"))
	if err != nil {
		t.Fatalf("read master.md: %v", err)
	}
	text := string(out)
	for _, want := range []string{
		"Current time (UTC):",
		"Run started at (UTC):",
		"Intent: evolve",
		"This run was launched with explicit `evolve` intent.",
		"experiments.jsonl",
		"`goalx afford --run lifecycle-run master`",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("master.md missing %q:\n%s", want, text)
		}
	}
}

func TestRelaunchMasterUsesSelectionSnapshotWhenPresent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")

	logPath := installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	writeSelectionSnapshotFixture(t, runDir, testSelectionSnapshot{
		Version: 1,
		Policy: goalx.EffectiveSelectionPolicy{
			MasterCandidates: []string{"claude-code/opus", "codex/gpt-5.4"},
		},
		Master:   goalx.MasterConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
		Research: goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
		Develop:  goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortMedium},
	})

	if err := relaunchMaster(repo, runDir, goalx.TmuxSessionName(repo, runName), cfg); err != nil {
		t.Fatalf("relaunchMaster: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := string(logData)
	if !strings.Contains(logText, "claude --model claude-opus-4-6 --permission-mode auto") {
		t.Fatalf("tmux log missing snapshot-selected claude launch:\n%s", logText)
	}
}
