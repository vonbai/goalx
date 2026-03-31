package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MemoryKind string

const (
	MemoryKindFact         MemoryKind = "fact"
	MemoryKindProcedure    MemoryKind = "procedure"
	MemoryKindPitfall      MemoryKind = "pitfall"
	MemoryKindSecretRef    MemoryKind = "secret_ref"
	MemoryKindSuccessPrior MemoryKind = "success_prior"
)

type MemoryEvidence struct {
	Kind string `json:"kind,omitempty"`
	Path string `json:"path,omitempty"`
}

type MemoryEntry struct {
	ID                      string            `json:"id,omitempty"`
	Kind                    MemoryKind        `json:"kind,omitempty"`
	Statement               string            `json:"statement,omitempty"`
	Selectors               map[string]string `json:"selectors,omitempty"`
	VerificationState       string            `json:"verification_state,omitempty"`
	Confidence              string            `json:"confidence,omitempty"`
	Evidence                []MemoryEvidence  `json:"evidence,omitempty"`
	SourceRuns              []string          `json:"source_runs,omitempty"`
	RetrievedCount          int               `json:"retrieved_count,omitempty"`
	UsedCount               int               `json:"used_count,omitempty"`
	SuccessAssociationCount int               `json:"success_association_count,omitempty"`
	ContradictedCount       int               `json:"contradicted_count,omitempty"`
	ValidFrom               string            `json:"valid_from,omitempty"`
	ValidTo                 string            `json:"valid_to,omitempty"`
	SupersededBy            string            `json:"superseded_by,omitempty"`
	CreatedAt               string            `json:"created_at,omitempty"`
	UpdatedAt               string            `json:"updated_at,omitempty"`
}

type MemoryProposal struct {
	ID         string            `json:"id,omitempty"`
	State      string            `json:"state,omitempty"`
	Kind       MemoryKind        `json:"kind,omitempty"`
	Statement  string            `json:"statement,omitempty"`
	Selectors  map[string]string `json:"selectors,omitempty"`
	Evidence   []MemoryEvidence  `json:"evidence,omitempty"`
	SourceRuns []string          `json:"source_runs,omitempty"`
	ValidFrom  string            `json:"valid_from,omitempty"`
	ValidTo    string            `json:"valid_to,omitempty"`
	CreatedAt  string            `json:"created_at,omitempty"`
	UpdatedAt  string            `json:"updated_at,omitempty"`
}

type MemoryQuery struct {
	ProjectID   string `json:"project_id,omitempty"`
	Environment string `json:"environment,omitempty"`
	InfraGroup  string `json:"infra_group,omitempty"`
	Host        string `json:"host,omitempty"`
	Service     string `json:"service,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Tool        string `json:"tool,omitempty"`
	Intent      string `json:"intent,omitempty"`
}

type MemoryContext struct {
	Facts         []string `json:"facts,omitempty"`
	Procedures    []string `json:"procedures,omitempty"`
	Pitfalls      []string `json:"pitfalls,omitempty"`
	SecretRefs    []string `json:"secret_refs,omitempty"`
	SuccessPriors []string `json:"success_priors,omitempty"`
	BuiltAt       string   `json:"built_at,omitempty"`
}

func EnsureMemoryStore() error {
	if err := ensureEmptyFile(MemoryEntryPath(MemoryKindFact)); err != nil {
		return err
	}
	if err := ensureEmptyFile(MemoryEntryPath(MemoryKindProcedure)); err != nil {
		return err
	}
	if err := ensureEmptyFile(MemoryEntryPath(MemoryKindPitfall)); err != nil {
		return err
	}
	if err := ensureEmptyFile(MemoryEntryPath(MemoryKindSecretRef)); err != nil {
		return err
	}
	if err := ensureEmptyFile(MemoryEntryPath(MemoryKindSuccessPrior)); err != nil {
		return err
	}
	if err := ensureDir(MemoryProposalsDir()); err != nil {
		return err
	}
	if err := ensureJSONFile(filepath.Join(MemoryIndexesDir(), "selectors.json"), map[string]any{}); err != nil {
		return err
	}
	if err := ensureJSONFile(filepath.Join(MemoryIndexesDir(), "tokens.json"), map[string]any{}); err != nil {
		return err
	}
	if err := ensureJSONFile(filepath.Join(MemoryIndexesDir(), "trust.json"), map[string]any{}); err != nil {
		return err
	}
	if err := ensureJSONFile(filepath.Join(MemoryIndexesDir(), "stats.json"), map[string]any{}); err != nil {
		return err
	}
	if err := ensureDir(MemoryProjectsDir()); err != nil {
		return err
	}
	if err := ensureJSONFile(MemoryGCPath(), map[string]any{}); err != nil {
		return err
	}
	return nil
}

func EnsureMemoryControl(runDir string) error {
	if err := ensureEmptyFile(MemorySeedsPath(runDir)); err != nil {
		return err
	}
	if err := ensureJSONFile(MemoryQueryPath(runDir), &MemoryQuery{}); err != nil {
		return err
	}
	if err := ensureJSONFile(MemoryContextPath(runDir), &MemoryContext{}); err != nil {
		return err
	}
	return nil
}

func NormalizeMemoryEntry(entry *MemoryEntry) (*MemoryEntry, error) {
	if entry == nil {
		return nil, fmt.Errorf("memory entry is nil")
	}
	normalized := *entry
	normalized.ID = strings.TrimSpace(normalized.ID)
	if normalized.ID == "" {
		return nil, fmt.Errorf("memory entry id is required")
	}
	normalized.Statement = strings.TrimSpace(normalized.Statement)
	if normalized.Statement == "" {
		return nil, fmt.Errorf("memory entry statement is required")
	}
	switch normalized.Kind {
	case MemoryKindFact, MemoryKindProcedure, MemoryKindPitfall, MemoryKindSecretRef, MemoryKindSuccessPrior:
	default:
		return nil, fmt.Errorf("unknown memory kind %q", normalized.Kind)
	}
	normalized.Selectors = normalizeMemorySelectors(normalized.Selectors)
	if (normalized.Kind == MemoryKindSecretRef || normalized.Kind == MemoryKindSuccessPrior) && len(normalized.Selectors) == 0 {
		return nil, fmt.Errorf("%s memory entries require selectors", normalized.Kind)
	}
	return &normalized, nil
}

func NormalizeMemoryProposal(proposal *MemoryProposal) (*MemoryProposal, error) {
	if proposal == nil {
		return nil, fmt.Errorf("memory proposal is nil")
	}
	normalized := *proposal
	normalized.ID = strings.TrimSpace(normalized.ID)
	if normalized.ID == "" {
		return nil, fmt.Errorf("memory proposal id is required")
	}
	normalized.State = strings.TrimSpace(normalized.State)
	if normalized.State == "" {
		normalized.State = "proposed"
	}
	normalized.Statement = strings.TrimSpace(normalized.Statement)
	if normalized.Statement == "" {
		return nil, fmt.Errorf("memory proposal statement is required")
	}
	switch normalized.Kind {
	case MemoryKindFact, MemoryKindProcedure, MemoryKindPitfall, MemoryKindSecretRef, MemoryKindSuccessPrior:
	default:
		return nil, fmt.Errorf("unknown memory proposal kind %q", normalized.Kind)
	}
	normalized.Selectors = normalizeMemorySelectors(normalized.Selectors)
	if (normalized.Kind == MemoryKindSecretRef || normalized.Kind == MemoryKindSuccessPrior) && len(normalized.Selectors) == 0 {
		return nil, fmt.Errorf("%s memory proposals require selectors", normalized.Kind)
	}
	normalized.Evidence = normalizeMemoryEvidence(normalized.Evidence)
	normalized.SourceRuns = compactStrings(normalized.SourceRuns)
	normalized.ValidFrom = strings.TrimSpace(normalized.ValidFrom)
	normalized.ValidTo = strings.TrimSpace(normalized.ValidTo)
	normalized.CreatedAt = strings.TrimSpace(normalized.CreatedAt)
	if normalized.CreatedAt == "" {
		normalized.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	normalized.UpdatedAt = strings.TrimSpace(normalized.UpdatedAt)
	if normalized.UpdatedAt == "" {
		normalized.UpdatedAt = normalized.CreatedAt
	}
	return &normalized, nil
}

func normalizeMemorySelectors(selectors map[string]string) map[string]string {
	if len(selectors) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(selectors))
	for key, value := range selectors {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		normalized[trimmedKey] = trimmedValue
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}

func ensureJSONFile(path string, v any) error {
	info, err := os.Stat(path)
	if err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%s is not a regular file", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		if len(strings.TrimSpace(string(data))) == 0 {
			return fmt.Errorf("%s is empty", path)
		}
		target, err := zeroJSONTarget(v)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, target); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return writeJSONFile(path, v)
}

func zeroJSONTarget(v any) (any, error) {
	switch value := v.(type) {
	case nil:
		return nil, fmt.Errorf("json target is nil")
	case *MemoryQuery:
		return &MemoryQuery{}, nil
	case *MemoryContext:
		return &MemoryContext{}, nil
	case map[string]any:
		return &map[string]any{}, nil
	default:
		return nil, fmt.Errorf("unsupported json target %T", value)
	}
}
