package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestBuildContextIndexIncludesRunAnchors(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}

	if index.ProjectRoot != repo {
		t.Fatalf("project_root = %q, want %q", index.ProjectRoot, repo)
	}
	if index.RunDir != runDir {
		t.Fatalf("run_dir = %q, want %q", index.RunDir, runDir)
	}
	if index.RunWorktree != RunWorktreePath(runDir) {
		t.Fatalf("run_worktree = %q, want %q", index.RunWorktree, RunWorktreePath(runDir))
	}
	if index.CharterPath != RunCharterPath(runDir) {
		t.Fatalf("charter_path = %q, want %q", index.CharterPath, RunCharterPath(runDir))
	}
	if index.TransportFactsPath != TransportFactsPath(runDir) {
		t.Fatalf("transport_facts_path = %q, want %q", index.TransportFactsPath, TransportFactsPath(runDir))
	}
	if index.ExperimentsLogPath != ExperimentsLogPath(runDir) {
		t.Fatalf("experiments_log_path = %q, want %q", index.ExperimentsLogPath, ExperimentsLogPath(runDir))
	}
	if index.IntegrationStatePath != IntegrationStatePath(runDir) {
		t.Fatalf("integration_state_path = %q, want %q", index.IntegrationStatePath, IntegrationStatePath(runDir))
	}
	if index.Master.Engine != "codex" || index.Master.Model != "gpt-5.4" {
		t.Fatalf("master = %+v, want codex/gpt-5.4", index.Master)
	}
}

func TestBuildContextIndexIncludesImmutableRunIdentity(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	meta.Intent = runIntentEvolve
	if err := SaveRunMetadata(RunMetadataPath(runDir), meta); err != nil {
		t.Fatalf("SaveRunMetadata: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	charter, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	if index.RunIdentity.RunID != meta.RunID {
		t.Fatalf("run identity run_id = %q, want %q", index.RunIdentity.RunID, meta.RunID)
	}
	if index.RunIdentity.RootRunID != meta.RootRunID {
		t.Fatalf("run identity root_run_id = %q, want %q", index.RunIdentity.RootRunID, meta.RootRunID)
	}
	if index.RunIdentity.Objective != cfg.Objective {
		t.Fatalf("run identity objective = %q, want %q", index.RunIdentity.Objective, cfg.Objective)
	}
	if index.RunIdentity.Mode != string(cfg.Mode) {
		t.Fatalf("run identity mode = %q, want %q", index.RunIdentity.Mode, cfg.Mode)
	}
	if index.RunIdentity.Intent != runIntentEvolve {
		t.Fatalf("run identity intent = %q, want %q", index.RunIdentity.Intent, runIntentEvolve)
	}
	if index.RunIdentity.RoleContracts.Master == nil || index.RunIdentity.RoleContracts.Master.Kind != "master" {
		t.Fatalf("run identity master role contract = %+v, want master contract", index.RunIdentity.RoleContracts.Master)
	}
	if charter != nil && index.RunIdentity.CharterID != charter.CharterID {
		t.Fatalf("run identity charter_id = %q, want %q", index.RunIdentity.CharterID, charter.CharterID)
	}
	if !strings.Contains(renderContextIndex(index), "Intent: `evolve`") {
		t.Fatalf("rendered context missing intent:\n%s", renderContextIndex(index))
	}
}

func TestBuildContextIndexIncludesSelectionSnapshotFacts(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	writeSelectionSnapshotFixture(t, runDir, testSelectionSnapshot{
		Version:           1,
		ExplicitSelection: true,
		Policy: goalx.EffectiveSelectionPolicy{
			DisabledEngines:    []string{"aider"},
			DisabledTargets:    []string{"claude-code/sonnet"},
			MasterCandidates:   []string{"codex/gpt-5.4", "claude-code/opus"},
			ResearchCandidates: []string{"claude-code/opus"},
			DevelopCandidates:  []string{"codex/gpt-5.4-mini", "codex/gpt-5.4"},
			MasterEffort:       goalx.EffortHigh,
			ResearchEffort:     goalx.EffortHigh,
			DevelopEffort:      goalx.EffortMedium,
		},
		Master:   goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4", Effort: goalx.EffortHigh},
		Research: goalx.SessionConfig{Engine: "claude-code", Model: "opus", Effort: goalx.EffortHigh},
		Develop:  goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4-mini", Effort: goalx.EffortMedium},
	})

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if index.SelectionSnapshotPath != SelectionSnapshotPath(runDir) {
		t.Fatalf("selection_snapshot_path = %q, want %q", index.SelectionSnapshotPath, SelectionSnapshotPath(runDir))
	}
	if index.Selection == nil {
		t.Fatal("selection facts missing")
	}
	if len(index.Selection.MasterCandidates) != 2 || index.Selection.MasterCandidates[0] != "codex/gpt-5.4" {
		t.Fatalf("master_candidates = %#v, want codex first", index.Selection.MasterCandidates)
	}
	if len(index.Selection.DisabledTargets) != 1 || index.Selection.DisabledTargets[0] != "claude-code/sonnet" {
		t.Fatalf("disabled_targets = %#v, want claude-code/sonnet", index.Selection.DisabledTargets)
	}
	rendered := renderContextIndex(index)
	for _, want := range []string{
		"Selection snapshot",
		"Master candidates: `codex/gpt-5.4, claude-code/opus`",
		"Research candidates: `claude-code/opus`",
		"Develop candidates: `codex/gpt-5.4-mini, codex/gpt-5.4`",
		"Disabled targets: `claude-code/sonnet`",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered context missing %q:\n%s", want, rendered)
		}
	}
}

func TestContextIndexIncludesSessionRoster(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}

	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	session := index.Sessions[0]
	if session.Name != "session-1" {
		t.Fatalf("session name = %q, want session-1", session.Name)
	}
	if session.InboxPath != ControlInboxPath(runDir, "session-1") {
		t.Fatalf("session inbox = %q, want %q", session.InboxPath, ControlInboxPath(runDir, "session-1"))
	}
	if session.WorktreePath != WorktreePath(runDir, cfg.Name, 1) {
		t.Fatalf("session worktree = %q, want %q", session.WorktreePath, WorktreePath(runDir, cfg.Name, 1))
	}
}

func TestContextIndexIncludesSessionWorktreeLineage(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, "session-1"))
	if err != nil {
		t.Fatalf("LoadSessionIdentity: %v", err)
	}
	if identity == nil {
		t.Fatal("session-1 identity missing")
	}
	identity.BaseBranchSelector = "run-root"
	identity.BaseBranch = "goalx/" + cfg.Name + "/root"
	if err := os.Remove(SessionIdentityPath(runDir, "session-1")); err != nil {
		t.Fatalf("remove session identity for rewrite: %v", err)
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, "session-1"), identity); err != nil {
		t.Fatalf("SaveSessionIdentity rewrite: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	session := index.Sessions[0]
	if session.BaseBranchSelector != "run-root" {
		t.Fatalf("BaseBranchSelector = %q, want run-root", session.BaseBranchSelector)
	}
	if session.BaseBranch != "goalx/"+cfg.Name+"/root" {
		t.Fatalf("BaseBranch = %q, want %q", session.BaseBranch, "goalx/"+cfg.Name+"/root")
	}
}

func TestProviderFactsIncludeTUICapabilityFactsWithoutRoutingAdvice(t *testing.T) {
	claudeFacts := providerFactsForEngine("master", "claude-code")
	if len(claudeFacts) == 0 {
		t.Fatalf("claude provider facts missing")
	}
	claudeText := joinProviderFactText(claudeFacts)
	for _, want := range []string{
		"tmux + interactive TUI",
		"skills, plugins, and MCP servers",
		"cannot use --dangerously-skip-permissions or --permission-mode bypassPermissions",
	} {
		if !strings.Contains(claudeText, want) {
			t.Fatalf("claude provider facts missing %q:\n%s", want, claudeText)
		}
	}
	for _, unwanted := range []string{"route", "routing", "dispatch", "prefer"} {
		if strings.Contains(strings.ToLower(claudeText), unwanted) {
			t.Fatalf("claude provider facts should not encode %q:\n%s", unwanted, claudeText)
		}
	}

	codexFacts := providerFactsForEngine("master", "codex")
	if len(codexFacts) == 0 {
		t.Fatalf("codex provider facts missing")
	}
	codexText := joinProviderFactText(codexFacts)
	for _, want := range []string{
		"tmux + interactive TUI",
		"skills and configured MCP servers",
	} {
		if !strings.Contains(codexText, want) {
			t.Fatalf("codex provider facts missing %q:\n%s", want, codexText)
		}
	}
	for _, unwanted := range []string{"route", "routing", "dispatch", "prefer"} {
		if strings.Contains(strings.ToLower(codexText), unwanted) {
			t.Fatalf("codex provider facts should not encode %q:\n%s", unwanted, codexText)
		}
	}
}

func TestContextIndexUsesRunWorktreeForSharedSession(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	sessionName := "session-1"
	if err := EnsureSessionControl(runDir, sessionName); err != nil {
		t.Fatalf("EnsureSessionControl: %v", err)
	}
	identity := &SessionIdentity{
		Version:         1,
		SessionName:     sessionName,
		ExperimentID:    "exp_guidance_shared_session_1",
		RoleKind:        "develop",
		Mode:            string(goalx.ModeDevelop),
		Engine:          "codex",
		Model:           "gpt-5.4-mini",
		OriginCharterID: loadCharterIDForTests(t, runDir),
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, sessionName), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         sessionName,
		State:        "active",
		Mode:         string(goalx.ModeDevelop),
		WorktreePath: "",
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	if len(index.Sessions) != 1 {
		t.Fatalf("sessions len = %d, want 1", len(index.Sessions))
	}
	if index.Sessions[0].WorktreePath != RunWorktreePath(runDir) {
		t.Fatalf("shared session worktree = %q, want %q", index.Sessions[0].WorktreePath, RunWorktreePath(runDir))
	}
}

func joinProviderFactText(facts []ProviderFact) string {
	parts := make([]string, 0, len(facts))
	for _, fact := range facts {
		parts = append(parts, fact.Fact)
	}
	return strings.Join(parts, "\n")
}

func TestContextIndexExcludesRawEnvSnapshot(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)

	index, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err != nil {
		t.Fatalf("BuildContextIndex: %v", err)
	}
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	text := string(data)
	for _, unwanted := range []string{"raw_env_path", "captured_path"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("context index should not expose %q:\n%s", unwanted, text)
		}
	}
}

func TestBuildContextIndexFailsWithoutRunCharter(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	if err := os.Remove(RunCharterPath(runDir)); err != nil {
		t.Fatalf("remove run charter: %v", err)
	}

	_, err := BuildContextIndex(repo, cfg.Name, runDir)
	if err == nil || !strings.Contains(err.Error(), "run charter missing") {
		t.Fatalf("BuildContextIndex error = %v, want missing charter error", err)
	}
}
