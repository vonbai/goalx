package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExtractMemoryProposalsFromGroundedSeeds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	seeds := []MemorySeed{
		{
			Kind:      "observed_fact",
			Run:       "demo",
			Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
			Message:   "deploy_path=scripts/deploy/compose_up.sh provider=cloudflare host=ops-3 container=scheduler",
			Evidence:  []MemoryEvidence{{Kind: "report", Path: "/tmp/report.md"}},
			CreatedAt: "2026-03-27T10:00:00Z",
		},
	}
	if err := SaveMemorySeeds(MemorySeedsPath(runDir), seeds); err != nil {
		t.Fatalf("SaveMemorySeeds: %v", err)
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 4 {
		t.Fatalf("proposal len = %d, want 4", len(proposals))
	}
	statements := joinProposalStatements(proposals)
	for _, want := range []string{
		"deploy path is scripts/deploy/compose_up.sh",
		"provider is cloudflare",
		"host is ops-3",
		"container is scheduler",
	} {
		if !strings.Contains(statements, want) {
			t.Fatalf("missing proposal statement %q in %q", want, statements)
		}
	}
	for _, proposal := range proposals {
		if proposal.ValidFrom != "2026-03-27T10:00:00Z" {
			t.Fatalf("proposal valid_from = %q, want seed timestamp", proposal.ValidFrom)
		}
		if proposal.CreatedAt != "2026-03-27T10:00:00Z" || proposal.UpdatedAt != "2026-03-27T10:00:00Z" {
			t.Fatalf("proposal timestamps = created:%q updated:%q, want seed timestamp", proposal.CreatedAt, proposal.UpdatedAt)
		}
	}
}

func TestExtractMemoryProposalsRejectsSecretValues(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	seeds := []MemorySeed{
		{
			Kind:      "observed_fact",
			Run:       "demo",
			Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
			Message:   "secret_ref=sk-live-1234567890abcdefghijklmnop",
			CreatedAt: "2026-03-27T10:00:00Z",
		},
	}
	if err := SaveMemorySeeds(MemorySeedsPath(runDir), seeds); err != nil {
		t.Fatalf("SaveMemorySeeds: %v", err)
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 0 {
		t.Fatalf("proposals = %+v, want none for secret value", proposals)
	}
}

func TestExtractMemoryProposalsCarriesEvidencePaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	runDir := filepath.Join(t.TempDir(), "run")
	if err := os.MkdirAll(ControlDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir control dir: %v", err)
	}
	seeds := []MemorySeed{
		{
			Kind:      "observed_fact",
			Run:       "demo",
			Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
			Message:   "config_source=1password/team/prod/registry secret_ref=1password/team/prod/registry",
			Evidence: []MemoryEvidence{
				{Kind: "summary", Path: "/tmp/summary.md"},
				{Kind: "report", Path: "/tmp/report.md"},
			},
			CreatedAt: "2026-03-27T10:00:00Z",
		},
	}
	if err := SaveMemorySeeds(MemorySeedsPath(runDir), seeds); err != nil {
		t.Fatalf("SaveMemorySeeds: %v", err)
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("proposal len = %d, want 2", len(proposals))
	}
	for _, proposal := range proposals {
		if len(proposal.Evidence) != 2 {
			t.Fatalf("proposal evidence = %+v, want carried evidence", proposal)
		}
		if len(proposal.SourceRuns) != 1 || proposal.SourceRuns[0] != "demo" {
			t.Fatalf("proposal source_runs = %+v, want [demo]", proposal.SourceRuns)
		}
	}
}

func TestAppendMemoryProposalsUsesDailyShardAndDedupes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	now := time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)
	proposal := MemoryProposal{
		ID:         "prop_1",
		State:      "proposed",
		Kind:       MemoryKindFact,
		Statement:  "provider is cloudflare",
		Selectors:  map[string]string{"project_id": "demo"},
		ValidFrom:  now.Format(time.RFC3339),
		CreatedAt:  now.Format(time.RFC3339),
		UpdatedAt:  now.Format(time.RFC3339),
		SourceRuns: []string{"demo"},
	}
	if err := AppendMemoryProposals(now, []MemoryProposal{proposal, proposal}); err != nil {
		t.Fatalf("AppendMemoryProposals(first): %v", err)
	}
	if err := AppendMemoryProposals(now, []MemoryProposal{proposal}); err != nil {
		t.Fatalf("AppendMemoryProposals(second): %v", err)
	}

	data, err := os.ReadFile(MemoryProposalPath(now))
	if err != nil {
		t.Fatalf("ReadFile proposal shard: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("proposal shard lines = %d, want 1: %s", len(lines), string(data))
	}
}

func TestSaveAppendsExtractedMemoryProposals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	runName, runDir, _, _ := writeReadOnlyRunFixture(t, repo)
	if err := AppendMemorySeed(runDir, MemorySeed{
		Kind:      "observed_fact",
		Run:       runName,
		Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
		Message:   "deploy_path=scripts/deploy/compose_up.sh provider=cloudflare",
		Evidence:  []MemoryEvidence{{Kind: "summary", Path: SummaryPath(runDir)}},
		CreatedAt: "2026-03-27T12:00:00Z",
	}); err != nil {
		t.Fatalf("AppendMemorySeed: %v", err)
	}

	if err := Save(repo, []string{"--run", runName}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	proposals, err := loadMemoryProposals(MemoryProposalPath(time.Date(2026, time.March, 27, 12, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatalf("loadMemoryProposals: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("proposal len = %d, want 2", len(proposals))
	}
	statements := joinProposalStatements(proposals)
	for _, want := range []string{
		"deploy path is scripts/deploy/compose_up.sh",
		"provider is cloudflare",
	} {
		if !strings.Contains(statements, want) {
			t.Fatalf("missing proposal statement %q in %q", want, statements)
		}
	}
}

func TestSidecarAppendsExtractedMemoryProposals(t *testing.T) {
	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)

	if err := AppendMemorySeed(runDir, MemorySeed{
		Kind:      "observed_fact",
		Run:       cfg.Name,
		Selectors: map[string]string{"project_id": "demo", "service": "deploy"},
		Message:   "host=ops-3 container=scheduler",
		Evidence:  []MemoryEvidence{{Kind: "report", Path: filepath.Join(ReportsDir(runDir), "deploy.md")}},
		CreatedAt: "2026-03-27T13:00:00Z",
	}); err != nil {
		t.Fatalf("AppendMemorySeed: %v", err)
	}

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

	if err := runSidecarTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runSidecarTick: %v", err)
	}

	proposals, err := loadMemoryProposals(MemoryProposalPath(time.Now().UTC()))
	if err != nil {
		t.Fatalf("loadMemoryProposals: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("proposal len = %d, want 2", len(proposals))
	}
	statements := joinProposalStatements(proposals)
	for _, want := range []string{
		"host is ops-3",
		"container is scheduler",
	} {
		if !strings.Contains(statements, want) {
			t.Fatalf("missing proposal statement %q in %q", want, statements)
		}
	}
}

func joinProposalStatements(proposals []MemoryProposal) string {
	parts := make([]string, 0, len(proposals))
	for _, proposal := range proposals {
		parts = append(parts, proposal.Statement)
	}
	return strings.Join(parts, "\n")
}
