package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
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

func TestAppendMemoryProposalsWaitsForMemoryStoreLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	lockFile, err := os.OpenFile(MemoryLockPath(), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatalf("OpenFile lock: %v", err)
	}
	defer lockFile.Close()
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("Flock lock: %v", err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	now := time.Date(2026, time.March, 27, 10, 30, 0, 0, time.UTC)
	done := make(chan error, 1)
	go func() {
		done <- AppendMemoryProposals(now, []MemoryProposal{
			{
				ID:         "prop_locked",
				State:      "proposed",
				Kind:       MemoryKindFact,
				Statement:  "provider is cloudflare",
				Selectors:  map[string]string{"project_id": "demo"},
				CreatedAt:  now.Format(time.RFC3339),
				UpdatedAt:  now.Format(time.RFC3339),
				SourceRuns: []string{"demo"},
			},
		})
	}()

	select {
	case err := <-done:
		t.Fatalf("AppendMemoryProposals completed before lock release: %v", err)
	case <-time.After(150 * time.Millisecond):
	}

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatalf("Flock unlock: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("AppendMemoryProposals(after unlock): %v", err)
	}

	proposals, err := loadMemoryProposals(MemoryProposalPath(now))
	if err != nil {
		t.Fatalf("loadMemoryProposals: %v", err)
	}
	if len(proposals) != 1 || proposals[0].ID != "prop_locked" {
		t.Fatalf("proposal shard = %+v, want locked proposal", proposals)
	}
}

func TestLLMExtractionBoundsProposalCount(t *testing.T) {
	repo, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\nscheduler first\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := os.MkdirAll(ReportsDir(runDir), 0o755); err != nil {
		t.Fatalf("mkdir reports: %v", err)
	}
	reportPath := filepath.Join(ReportsDir(runDir), "ops.md")
	if err := os.WriteFile(reportPath, []byte("run db checks inside scheduler first\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := AppendMemorySeed(runDir, MemorySeed{
		Kind:      "verify_result",
		Run:       "run-1",
		Selectors: map[string]string{"project_id": goalx.ProjectID(repo), "service": "postgres"},
		Message:   "acceptance command recorded exit_code=1",
		Evidence: []MemoryEvidence{
			{Kind: "summary", Path: SummaryPath(runDir)},
			{Kind: "report", Path: reportPath},
		},
		CreatedAt: "2026-03-27T18:00:00Z",
	}); err != nil {
		t.Fatalf("AppendMemorySeed: %v", err)
	}

	origRunner := runMemoryLLMExtract
	origExists := memoryLLMCommandExists
	defer func() {
		runMemoryLLMExtract = origRunner
		memoryLLMCommandExists = origExists
	}()
	memoryLLMCommandExists = func(name string) bool { return name == "codex" }
	runMemoryLLMExtract = func(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
		if req.Target.Engine != "codex" || req.Target.Model != "fast" {
			t.Fatalf("target = %+v, want codex/fast", req.Target)
		}
		if req.Selectors["project_id"] != goalx.ProjectID(repo) {
			t.Fatalf("selectors = %+v, want project selector", req.Selectors)
		}
		return memoryLLMExtractResponse{
			Proposals: []memoryLLMExtractItem{
				{Kind: MemoryKindProcedure, Statement: "run db checks inside scheduler first", EvidencePaths: []string{SummaryPath(runDir)}},
				{Kind: MemoryKindPitfall, Statement: "direct host db checks often fail", EvidencePaths: []string{reportPath}},
				{Kind: MemoryKindProcedure, Statement: "extra proposal should be bounded away", EvidencePaths: []string{reportPath}},
			},
		}, nil
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 2 {
		t.Fatalf("proposal len = %d, want bounded 2", len(proposals))
	}
	for _, proposal := range proposals {
		if proposal.Kind != MemoryKindProcedure && proposal.Kind != MemoryKindPitfall {
			t.Fatalf("unexpected proposal kind: %+v", proposal)
		}
		if len(proposal.Evidence) == 0 {
			t.Fatalf("proposal missing evidence: %+v", proposal)
		}
		if proposal.Selectors["project_id"] != goalx.ProjectID(repo) {
			t.Fatalf("proposal selectors = %+v, want project_id", proposal.Selectors)
		}
	}
}

func TestLLMExtractionRequiresEvidencePaths(t *testing.T) {
	repo, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := AppendMemorySeed(runDir, MemorySeed{
		Kind:      "verify_result",
		Run:       "run-1",
		Selectors: map[string]string{"project_id": goalx.ProjectID(repo), "service": "postgres"},
		Message:   "acceptance command recorded exit_code=1",
		Evidence:  []MemoryEvidence{{Kind: "summary", Path: SummaryPath(runDir)}},
		CreatedAt: "2026-03-27T18:10:00Z",
	}); err != nil {
		t.Fatalf("AppendMemorySeed: %v", err)
	}

	origRunner := runMemoryLLMExtract
	origExists := memoryLLMCommandExists
	defer func() {
		runMemoryLLMExtract = origRunner
		memoryLLMCommandExists = origExists
	}()
	memoryLLMCommandExists = func(name string) bool { return name == "codex" }
	runMemoryLLMExtract = func(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
		return memoryLLMExtractResponse{
			Proposals: []memoryLLMExtractItem{
				{Kind: MemoryKindProcedure, Statement: "missing evidence", EvidencePaths: nil},
				{Kind: MemoryKindPitfall, Statement: "unknown evidence", EvidencePaths: []string{"/tmp/unknown.md"}},
			},
		}, nil
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 0 {
		t.Fatalf("proposals = %+v, want none when llm evidence paths invalid", proposals)
	}
}

func TestLLMExtractionDoesNotPromoteDirectly(t *testing.T) {
	repo, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := AppendMemorySeed(runDir, MemorySeed{
		Kind:      "verify_result",
		Run:       "run-1",
		Selectors: map[string]string{"project_id": goalx.ProjectID(repo), "service": "postgres"},
		Message:   "acceptance command recorded exit_code=1",
		Evidence:  []MemoryEvidence{{Kind: "summary", Path: SummaryPath(runDir)}},
		CreatedAt: "2026-03-27T18:20:00Z",
	}); err != nil {
		t.Fatalf("AppendMemorySeed: %v", err)
	}

	origRunner := runMemoryLLMExtract
	origExists := memoryLLMCommandExists
	defer func() {
		runMemoryLLMExtract = origRunner
		memoryLLMCommandExists = origExists
	}()
	memoryLLMCommandExists = func(name string) bool { return name == "codex" }
	runMemoryLLMExtract = func(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
		return memoryLLMExtractResponse{
			Proposals: []memoryLLMExtractItem{
				{Kind: MemoryKindProcedure, Statement: "run db checks inside scheduler first", EvidencePaths: []string{SummaryPath(runDir)}},
			},
		}, nil
	}

	now := time.Date(2026, time.March, 27, 18, 20, 0, 0, time.UTC)
	if err := AppendExtractedMemoryProposals(runDir, now); err != nil {
		t.Fatalf("AppendExtractedMemoryProposals: %v", err)
	}

	proposals, err := loadMemoryProposals(MemoryProposalPath(now))
	if err != nil {
		t.Fatalf("loadMemoryProposals: %v", err)
	}
	if len(proposals) != 1 || proposals[0].Kind != MemoryKindProcedure {
		t.Fatalf("proposal shard = %+v, want one procedure proposal", proposals)
	}
	if entries := loadCanonicalEntriesByKind(t, MemoryKindProcedure); len(entries) != 0 {
		t.Fatalf("canonical procedures = %+v, want none before promotion", entries)
	}
}

func TestMemoryExtractBuildsSuccessDeltaProposalFromInterventionLog(t *testing.T) {
	repo, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\nui lane drifted to correctness-only checks\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := AppendInterventionEvent(runDir, "user_redirect", "user", InterventionEventBody{
		Run:             filepath.Base(runDir),
		Message:         "Do not stop at route cutover only; the page still needs real product polish.",
		AffectedTargets: []string{"session-1"},
		Before: InterventionBeforeState{
			GoalHash:         "sha256:goal",
			StatusHash:       "sha256:status",
			CoordinationHash: "sha256:coordination",
		},
	}); err != nil {
		t.Fatalf("AppendInterventionEvent: %v", err)
	}

	origRunner := runMemoryLLMExtract
	origExists := memoryLLMCommandExists
	defer func() {
		runMemoryLLMExtract = origRunner
		memoryLLMCommandExists = origExists
	}()
	memoryLLMCommandExists = func(name string) bool { return name == "codex" }
	runMemoryLLMExtract = func(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
		if _, ok := req.EvidenceMap[InterventionLogPath(runDir)]; !ok {
			t.Fatalf("intervention evidence missing from request: %+v", req.EvidenceMap)
		}
		if !strings.Contains(req.Bundle, "interventions:\n") {
			t.Fatalf("bundle missing interventions section:\n%s", req.Bundle)
		}
		return memoryLLMExtractResponse{
			Proposals: []memoryLLMExtractItem{
				{
					Kind:          MemoryKindSuccessPrior,
					Statement:     "frontend product goals require critique and polish proof before closeout",
					EvidencePaths: []string{InterventionLogPath(runDir), SummaryPath(runDir)},
				},
			},
		}, nil
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("proposal len = %d, want 1", len(proposals))
	}
	if proposals[0].Kind != MemoryKindSuccessPrior {
		t.Fatalf("proposal kind = %q, want success_prior", proposals[0].Kind)
	}
	if proposals[0].Selectors["project_id"] != goalx.ProjectID(repo) {
		t.Fatalf("proposal selectors = %+v, want project_id=%s", proposals[0].Selectors, goalx.ProjectID(repo))
	}
	if len(proposals[0].Evidence) != 2 {
		t.Fatalf("proposal evidence = %+v, want intervention log + summary", proposals[0].Evidence)
	}
}

func TestAppendExtractedMemoryProposalsStoresSuccessDeltaInProposalShard(t *testing.T) {
	repo, runDir, _, _ := writeGuidanceRunFixture(t)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := AppendInterventionEvent(runDir, "user_tell", "user", InterventionEventBody{
		Run:             filepath.Base(runDir),
		Message:         "Do not close on correctness alone.",
		AffectedTargets: []string{"master"},
	}); err != nil {
		t.Fatalf("AppendInterventionEvent: %v", err)
	}

	origRunner := runMemoryLLMExtract
	origExists := memoryLLMCommandExists
	defer func() {
		runMemoryLLMExtract = origRunner
		memoryLLMCommandExists = origExists
	}()
	memoryLLMCommandExists = func(name string) bool { return name == "codex" }
	runMemoryLLMExtract = func(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
		return memoryLLMExtractResponse{
			Proposals: []memoryLLMExtractItem{
				{
					Kind:          MemoryKindSuccessPrior,
					Statement:     "closeout should not rely on correctness-only evidence",
					EvidencePaths: []string{InterventionLogPath(runDir), SummaryPath(runDir)},
				},
			},
		}, nil
	}

	now := time.Date(2026, time.March, 31, 9, 0, 0, 0, time.UTC)
	if err := AppendExtractedMemoryProposals(runDir, now); err != nil {
		t.Fatalf("AppendExtractedMemoryProposals: %v", err)
	}

	proposals, err := loadMemoryProposals(MemoryProposalPath(now))
	if err != nil {
		t.Fatalf("loadMemoryProposals: %v", err)
	}
	if len(proposals) != 1 || proposals[0].Kind != MemoryKindSuccessPrior {
		t.Fatalf("proposal shard = %+v, want one success_prior proposal", proposals)
	}
	if proposals[0].Selectors["project_id"] != goalx.ProjectID(repo) {
		t.Fatalf("proposal selectors = %+v, want project_id=%s", proposals[0].Selectors, goalx.ProjectID(repo))
	}
}

func TestLLMExtractionAutoDetectsClaudeWhenCodexUnavailable(t *testing.T) {
	engines := map[string]goalx.EngineConfig{
		"claude-code": goalx.BuiltinEngines["claude-code"],
		"codex":       goalx.BuiltinEngines["codex"],
	}
	origExists := memoryLLMCommandExists
	defer func() { memoryLLMCommandExists = origExists }()
	memoryLLMCommandExists = func(name string) bool { return name == "claude" }

	target, ok := selectMemoryLLMExtractTarget(engines)
	if !ok {
		t.Fatal("selectMemoryLLMExtractTarget returned no target")
	}
	if target.Engine != "claude-code" || target.Model != "haiku" {
		t.Fatalf("target = %+v, want claude-code/haiku", target)
	}
}

func TestLLMExtractionSkipsWhenConfigDisablesIt(t *testing.T) {
	repo, runDir, _, _ := writeGuidanceRunFixture(t)
	writeProjectConfigFixture(t, repo, `
memory:
  llm_extract: off
`)
	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}
	if err := os.WriteFile(SummaryPath(runDir), []byte("# summary\n"), 0o644); err != nil {
		t.Fatalf("write summary: %v", err)
	}
	if err := AppendMemorySeed(runDir, MemorySeed{
		Kind:      "verify_result",
		Run:       "run-1",
		Selectors: map[string]string{"project_id": goalx.ProjectID(repo), "service": "postgres"},
		Message:   "acceptance command recorded exit_code=1",
		Evidence:  []MemoryEvidence{{Kind: "summary", Path: SummaryPath(runDir)}},
		CreatedAt: "2026-03-27T18:30:00Z",
	}); err != nil {
		t.Fatalf("AppendMemorySeed: %v", err)
	}

	origRunner := runMemoryLLMExtract
	origExists := memoryLLMCommandExists
	defer func() {
		runMemoryLLMExtract = origRunner
		memoryLLMCommandExists = origExists
	}()
	memoryLLMCommandExists = func(name string) bool { return name == "codex" }
	runMemoryLLMExtract = func(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
		t.Fatalf("runMemoryLLMExtract should not be called when memory.llm_extract=off")
		return memoryLLMExtractResponse{}, nil
	}

	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		t.Fatalf("ExtractMemoryProposals: %v", err)
	}
	if len(proposals) != 0 {
		t.Fatalf("proposals = %+v, want no llm proposals when disabled", proposals)
	}
}

func TestBuildCodexMemoryLLMExtractArgsOmitsLegacyApprovalFlag(t *testing.T) {
	req := memoryLLMExtractRequest{
		ProjectRoot: "/tmp/project",
		Target: memoryLLMExtractTarget{
			Engine:  "codex",
			ModelID: "gpt-5.4-mini",
			Effort:  goalx.EffortMinimal,
		},
		Schema: `{}`,
	}

	args := buildCodexMemoryLLMExtractArgs(req, "/tmp/schema.json", "/tmp/output.json")

	if slices.Contains(args, "-a") || slices.Contains(args, "--ask-for-approval") {
		t.Fatalf("codex args still include legacy approval flag: %v", args)
	}
	want := []string{
		"exec",
		"--cd", "/tmp/project",
		"--skip-git-repo-check",
		"--sandbox", "read-only",
		"--ephemeral",
		"--output-schema", "/tmp/schema.json",
		"-o", "/tmp/output.json",
		"-m", "gpt-5.4-mini",
		"-",
	}
	for _, item := range want {
		if !slices.Contains(args, item) {
			t.Fatalf("codex args missing %q: %v", item, args)
		}
	}
}

func TestRunCodexMemoryLLMExtractUsesCurrentExecCLIContract(t *testing.T) {
	fakeBin := t.TempDir()
	argsPath := filepath.Join(fakeBin, "args.txt")
	outputPayload := `{"proposals":[{"kind":"procedure","statement":"check scheduler first","evidence_paths":["/tmp/summary.md"]}]}`
	script := "#!/bin/sh\n" +
		"printf '%s\\n' \"$@\" > \"$GOALX_ARGS_PATH\"\n" +
		"out=''\n" +
		"while [ $# -gt 0 ]; do\n" +
		"  if [ \"$1\" = \"-o\" ]; then out=\"$2\"; shift 2; continue; fi\n" +
		"  shift\n" +
		"done\n" +
		"printf '%s' '" + outputPayload + "' > \"$out\"\n"
	codexPath := filepath.Join(fakeBin, "codex")
	if err := os.WriteFile(codexPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex: %v", err)
	}
	t.Setenv("GOALX_ARGS_PATH", argsPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	resp, err := runCodexMemoryLLMExtract(memoryLLMExtractRequest{
		ProjectRoot: t.TempDir(),
		Target: memoryLLMExtractTarget{
			Engine:  "codex",
			ModelID: "gpt-5.4-mini",
			Effort:  goalx.EffortMinimal,
		},
		Bundle:  "grounded bundle",
		Schema:  `{}`,
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("runCodexMemoryLLMExtract: %v", err)
	}
	if !reflect.DeepEqual(resp.Proposals, []memoryLLMExtractItem{{
		Kind:          MemoryKindProcedure,
		Statement:     "check scheduler first",
		EvidencePaths: []string{"/tmp/summary.md"},
	}}) {
		t.Fatalf("response = %+v", resp)
	}

	rawArgs, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	argsText := string(rawArgs)
	if strings.Contains(argsText, "\n-a\n") || strings.Contains(argsText, "\n--ask-for-approval\n") {
		t.Fatalf("codex exec args still include legacy approval flag:\n%s", argsText)
	}
	for _, want := range []string{"exec", "--sandbox", "read-only", "--ephemeral", "--output-schema", "-o", "-m", "gpt-5.4-mini"} {
		if !strings.Contains(argsText, want) {
			t.Fatalf("codex exec args missing %q:\n%s", want, argsText)
		}
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

	proposals, err := loadMemoryProposals(MemoryProposalPath(time.Now().UTC()))
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
	facts := loadCanonicalEntriesByKind(t, MemoryKindFact)
	if len(facts) != 2 {
		t.Fatalf("canonical fact len = %d, want 2", len(facts))
	}
}

func TestRuntimeHostAppendsExtractedMemoryProposals(t *testing.T) {
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

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick: %v", err)
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
	facts := loadCanonicalEntriesByKind(t, MemoryKindFact)
	if len(facts) != 2 {
		t.Fatalf("canonical fact len = %d, want 2", len(facts))
	}
}

func joinProposalStatements(proposals []MemoryProposal) string {
	parts := make([]string, 0, len(proposals))
	for _, proposal := range proposals {
		parts = append(parts, proposal.Statement)
	}
	return strings.Join(parts, "\n")
}
