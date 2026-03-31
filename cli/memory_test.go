package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMemoryRootUsesUserScopedGoalxDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := MemoryRootDir()
	want := filepath.Join(home, ".goalx", "memory")
	if got != want {
		t.Fatalf("MemoryRootDir() = %q, want %q", got, want)
	}
}

func TestEnsureMemoryStoreCreatesCanonicalLayout(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := EnsureMemoryStore(); err != nil {
		t.Fatalf("EnsureMemoryStore: %v", err)
	}

	for _, path := range memoryStoreLayoutPaths(home) {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestMemoryProposalPathUsesUTCDateShard(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	got := MemoryProposalPath(time.Date(2026, time.March, 27, 0, 30, 0, 0, time.FixedZone("UTC+14", 14*3600)))
	want := filepath.Join(home, ".goalx", "memory", "proposals", "2026-03-26.jsonl")
	if got != want {
		t.Fatalf("MemoryProposalPath() = %q, want %q", got, want)
	}
}

func TestNormalizeMemoryEntryRejectsUnknownKind(t *testing.T) {
	_, err := NormalizeMemoryEntry(&MemoryEntry{
		ID:        "mem-1",
		Kind:      MemoryKind("unknown"),
		Statement: "keep this stable",
	})
	if err == nil {
		t.Fatal("NormalizeMemoryEntry accepted unknown kind")
	}
}

func TestNormalizeMemoryEntryRejectsEmptyStatement(t *testing.T) {
	_, err := NormalizeMemoryEntry(&MemoryEntry{
		ID:   "mem-2",
		Kind: MemoryKindFact,
	})
	if err == nil {
		t.Fatal("NormalizeMemoryEntry accepted empty statement")
	}
}

func TestNormalizeMemoryEntryRejectsSecretRefWithoutSelectors(t *testing.T) {
	_, err := NormalizeMemoryEntry(&MemoryEntry{
		ID:        "mem-3",
		Kind:      MemoryKindSecretRef,
		Statement: "registry token lives in 1Password",
	})
	if err == nil {
		t.Fatal("NormalizeMemoryEntry accepted secret_ref without selectors")
	}
}

func TestNormalizeMemoryEntryRejectsSuccessPriorWithoutSelectors(t *testing.T) {
	_, err := NormalizeMemoryEntry(&MemoryEntry{
		ID:        "mem-3b",
		Kind:      MemoryKindSuccessPrior,
		Statement: "frontend product goals need critique and finisher passes",
	})
	if err == nil {
		t.Fatal("NormalizeMemoryEntry accepted success_prior without selectors")
	}
}

func TestEnsureMemoryControlCreatesRunLocalFiles(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	_, runDir, _, _ := writeReadOnlyRunFixture(t, repo)

	if err := EnsureMemoryControl(runDir); err != nil {
		t.Fatalf("EnsureMemoryControl: %v", err)
	}

	for _, path := range []string{
		MemorySeedsPath(runDir),
		MemoryQueryPath(runDir),
		MemoryContextPath(runDir),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
	}
}

func TestEnsureMemoryStoreRejectsInvalidExistingIndexJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(MemoryIndexesDir(), 0o755); err != nil {
		t.Fatalf("mkdir indexes dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(MemoryIndexesDir(), "selectors.json"), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write selectors index: %v", err)
	}

	if err := EnsureMemoryStore(); err == nil {
		t.Fatal("EnsureMemoryStore accepted invalid existing selectors.json")
	}
}

func TestEnsureMemoryControlRejectsInvalidExistingJSON(t *testing.T) {
	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	_, runDir, _, _ := writeReadOnlyRunFixture(t, repo)

	if err := os.WriteFile(MemoryQueryPath(runDir), []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write memory query: %v", err)
	}

	if err := EnsureMemoryControl(runDir); err == nil {
		t.Fatal("EnsureMemoryControl accepted invalid existing memory-query.json")
	}
}

func TestMemoryEntryRejectsNil(t *testing.T) {
	if _, err := NormalizeMemoryEntry(nil); err == nil {
		t.Fatal("NormalizeMemoryEntry accepted nil entry")
	}
}

func TestMemoryEntryKeepsValidSelectors(t *testing.T) {
	entry := &MemoryEntry{
		ID:        "mem-4",
		Kind:      MemoryKindFact,
		Statement: "scheduler container hosts db checks",
		Selectors: map[string]string{
			"project_id":  "demo",
			"environment": "prod",
			"service":     "postgres",
			"infra_group": "shared",
			"host":        "ops-3",
			"provider":    "cloudflare",
			"tool":        "ssh",
			"intent":      "deploy",
		},
	}
	if _, err := NormalizeMemoryEntry(entry); err != nil {
		t.Fatalf("NormalizeMemoryEntry(valid) = %v", err)
	}
}

func TestNormalizeMemoryEntrySerializesFullSchemaContract(t *testing.T) {
	entry := &MemoryEntry{
		ID:                "mem-5",
		Kind:              MemoryKindProcedure,
		Statement:         "verify postgres from scheduler container first",
		Selectors:         map[string]string{"project_id": "demo", "environment": "staging", "service": "postgres"},
		VerificationState: "validated",
		Confidence:        "grounded",
		Evidence: []MemoryEvidence{
			{Kind: "summary", Path: "/tmp/run/summary.md"},
		},
		SourceRuns:              []string{"run-1"},
		RetrievedCount:          2,
		UsedCount:               1,
		SuccessAssociationCount: 1,
		ContradictedCount:       1,
		ValidFrom:               "2026-03-27T00:00:00Z",
		ValidTo:                 "2026-03-28T00:00:00Z",
		SupersededBy:            "mem-6",
		CreatedAt:               "2026-03-27T00:00:00Z",
		UpdatedAt:               "2026-03-27T00:00:00Z",
	}

	normalized, err := NormalizeMemoryEntry(entry)
	if err != nil {
		t.Fatalf("NormalizeMemoryEntry(valid schema) = %v", err)
	}

	if normalized.VerificationState != entry.VerificationState ||
		normalized.Confidence != entry.Confidence ||
		normalized.RetrievedCount != entry.RetrievedCount ||
		normalized.UsedCount != entry.UsedCount ||
		normalized.SuccessAssociationCount != entry.SuccessAssociationCount ||
		normalized.ContradictedCount != entry.ContradictedCount ||
		normalized.ValidFrom != entry.ValidFrom ||
		normalized.ValidTo != entry.ValidTo ||
		normalized.SupersededBy != entry.SupersededBy ||
		normalized.CreatedAt != entry.CreatedAt ||
		normalized.UpdatedAt != entry.UpdatedAt {
		t.Fatalf("NormalizeMemoryEntry changed canonical fields: got %+v", normalized)
	}

	data, err := json.Marshal(normalized)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	wantKeys := []string{
		"id",
		"kind",
		"statement",
		"selectors",
		"verification_state",
		"confidence",
		"evidence",
		"source_runs",
		"retrieved_count",
		"used_count",
		"success_association_count",
		"contradicted_count",
		"valid_from",
		"valid_to",
		"superseded_by",
		"created_at",
		"updated_at",
	}
	if len(payload) != len(wantKeys) {
		t.Fatalf("normalized entry json keys = %d, want %d: %s", len(payload), len(wantKeys), string(data))
	}
	for _, key := range wantKeys {
		if _, ok := payload[key]; !ok {
			t.Fatalf("normalized entry json missing key %q: %s", key, string(data))
		}
	}
}

func memoryStoreLayoutPaths(home string) []string {
	root := filepath.Join(home, ".goalx", "memory")
	return []string{
		filepath.Join(root),
		filepath.Join(root, "entries", "facts.jsonl"),
		filepath.Join(root, "entries", "procedures.jsonl"),
		filepath.Join(root, "entries", "pitfalls.jsonl"),
		filepath.Join(root, "entries", "secret_refs.jsonl"),
		filepath.Join(root, "entries", "success_priors.jsonl"),
		filepath.Join(root, "proposals"),
		filepath.Join(root, "indexes", "selectors.json"),
		filepath.Join(root, "indexes", "tokens.json"),
		filepath.Join(root, "indexes", "trust.json"),
		filepath.Join(root, "indexes", "stats.json"),
	}
}

func canonicalMemorySentinelPaths(home string) []string {
	root := filepath.Join(home, ".goalx", "memory")
	return []string{
		filepath.Join(root, "entries", "facts.jsonl"),
		filepath.Join(root, "entries", "procedures.jsonl"),
		filepath.Join(root, "entries", "pitfalls.jsonl"),
		filepath.Join(root, "entries", "secret_refs.jsonl"),
		filepath.Join(root, "entries", "success_priors.jsonl"),
	}
}

func writeCanonicalMemorySentinels(t *testing.T, home string) map[string][]byte {
	t.Helper()

	payloads := map[string][]byte{}
	entries := map[MemoryKind]MemoryEntry{
		MemoryKindFact: {
			ID:                "mem_sentinel_fact",
			Kind:              MemoryKindFact,
			Statement:         "host is sentinel",
			Selectors:         map[string]string{"project_id": "sentinel"},
			VerificationState: "validated",
			Confidence:        "grounded",
			CreatedAt:         "2026-03-27T00:00:00Z",
			UpdatedAt:         "2026-03-27T00:00:00Z",
		},
		MemoryKindProcedure: {
			ID:                "mem_sentinel_procedure",
			Kind:              MemoryKindProcedure,
			Statement:         "run sentinel checks first",
			Selectors:         map[string]string{"project_id": "sentinel"},
			VerificationState: "validated",
			Confidence:        "grounded",
			CreatedAt:         "2026-03-27T00:00:00Z",
			UpdatedAt:         "2026-03-27T00:00:00Z",
		},
		MemoryKindPitfall: {
			ID:                "mem_sentinel_pitfall",
			Kind:              MemoryKindPitfall,
			Statement:         "sentinel pitfall",
			Selectors:         map[string]string{"project_id": "sentinel"},
			VerificationState: "repeated",
			Confidence:        "grounded",
			CreatedAt:         "2026-03-27T00:00:00Z",
			UpdatedAt:         "2026-03-27T00:00:00Z",
		},
		MemoryKindSecretRef: {
			ID:                "mem_sentinel_secret",
			Kind:              MemoryKindSecretRef,
			Statement:         "secret reference is sentinel/vault/item",
			Selectors:         map[string]string{"project_id": "sentinel", "service": "deploy"},
			VerificationState: "validated",
			Confidence:        "grounded",
			CreatedAt:         "2026-03-27T00:00:00Z",
			UpdatedAt:         "2026-03-27T00:00:00Z",
		},
		MemoryKindSuccessPrior: {
			ID:                "mem_sentinel_success_prior",
			Kind:              MemoryKindSuccessPrior,
			Statement:         "frontend product goals require critique before closeout",
			Selectors:         map[string]string{"project_id": "sentinel", "intent": "worker"},
			VerificationState: "repeated",
			Confidence:        "grounded",
			CreatedAt:         "2026-03-27T00:00:00Z",
			UpdatedAt:         "2026-03-27T00:00:00Z",
		},
	}
	for _, kind := range []MemoryKind{MemoryKindFact, MemoryKindProcedure, MemoryKindPitfall, MemoryKindSecretRef, MemoryKindSuccessPrior} {
		path := MemoryEntryPath(kind)
		if !strings.HasPrefix(path, filepath.Join(home, ".goalx", "memory")) {
			t.Fatalf("unexpected sentinel path %s for home %s", path, home)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		data, err := json.Marshal(entries[kind])
		if err != nil {
			t.Fatalf("marshal %s: %v", kind, err)
		}
		data = append(data, '\n')
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
		payloads[path] = data
	}
	return payloads
}
