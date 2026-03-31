package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const memoryContextCategoryLimit = 3

type memoryProjectProfile struct {
	Environment string `json:"environment,omitempty"`
	Service     string `json:"service,omitempty"`
	Provider    string `json:"provider,omitempty"`
	Tool        string `json:"tool,omitempty"`
}

type memoryEntryRank struct {
	ProjectSpecific  bool
	MatchedSelectors int
	LexicalMatches   int
	SelectorCount    int
	TrustWeight      int
	Freshness        int64
}

type memoryRankedEntry struct {
	Entry MemoryEntry
	Rank  memoryEntryRank
}

func RefreshRunMemoryContext(runDir string) error {
	if err := EnsureMemoryControl(runDir); err != nil {
		return err
	}
	query, err := BuildMemoryQuery(runDir)
	if err != nil {
		return err
	}
	if err := writeJSONFile(MemoryQueryPath(runDir), &query); err != nil {
		return err
	}
	context, err := BuildMemoryContext(query)
	if err != nil {
		return err
	}
	return writeJSONFile(MemoryContextPath(runDir), context)
}

func LoadMemoryQueryFile(path string) (*MemoryQuery, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	query := &MemoryQuery{}
	if err := json.Unmarshal(data, query); err != nil {
		return nil, err
	}
	return query, nil
}

func LoadMemoryContextFile(path string) (*MemoryContext, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	context := &MemoryContext{}
	if err := json.Unmarshal(data, context); err != nil {
		return nil, err
	}
	return context, nil
}

func BuildMemoryQuery(runDir string) (MemoryQuery, error) {
	query := MemoryQuery{}

	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return query, err
	}
	if meta == nil {
		return query, nil
	}

	projectRoot := strings.TrimSpace(meta.ProjectRoot)
	if projectRoot != "" {
		query.ProjectID = goalx.ProjectID(projectRoot)
	}
	query.Intent = strings.TrimSpace(meta.Intent)

	if strings.TrimSpace(query.ProjectID) == "" {
		return query, nil
	}
	profile, err := loadMemoryProjectProfile(query.ProjectID)
	if err != nil {
		return query, err
	}
	if profile == nil {
		return query, nil
	}
	query.Environment = profile.Environment
	query.Service = profile.Service
	query.Provider = profile.Provider
	query.Tool = profile.Tool
	return query, nil
}

func RetrieveMemory(query MemoryQuery) ([]MemoryEntry, error) {
	entries, err := loadCanonicalMemoryEntries()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	entryIDs := stableMemoryEntryIDs(entries)
	entryByNumericID := make(map[uint32]MemoryEntry, len(entries))
	for _, entry := range entries {
		entryByNumericID[entryIDs[entry.ID]] = entry
	}

	candidates, err := selectorRecallEntries(query, entryByNumericID)
	if err != nil {
		return nil, err
	}
	requireSelectorMatch := len(candidates) > 0
	if len(candidates) == 0 {
		candidates, err = lexicalRecallEntries(query, entryByNumericID)
		if err != nil {
			return nil, err
		}
	}
	if len(candidates) == 0 {
		candidates = entries
	}

	ranked := make([]memoryRankedEntry, 0, len(candidates))
	for _, entry := range candidates {
		if strings.TrimSpace(entry.SupersededBy) != "" {
			continue
		}
		match, rank := rankMemoryEntry(entry, query, requireSelectorMatch)
		if !match {
			continue
		}
		ranked = append(ranked, memoryRankedEntry{Entry: entry, Rank: rank})
	}

	sort.Slice(ranked, func(i, j int) bool {
		return compareMemoryRank(ranked[i], ranked[j])
	})

	results := make([]MemoryEntry, 0, len(ranked))
	for _, rankedEntry := range ranked {
		results = append(results, rankedEntry.Entry)
	}
	return results, nil
}

func BuildMemoryContext(query MemoryQuery) (*MemoryContext, error) {
	entries, err := RetrieveMemory(query)
	if err != nil {
		return nil, err
	}

	context := &MemoryContext{
		BuiltAt: time.Now().UTC().Format(time.RFC3339),
	}
	for _, entry := range entries {
		switch entry.Kind {
		case MemoryKindFact:
			if len(context.Facts) < memoryContextCategoryLimit {
				context.Facts = append(context.Facts, entry.Statement)
			}
		case MemoryKindProcedure:
			if len(context.Procedures) < memoryContextCategoryLimit {
				context.Procedures = append(context.Procedures, entry.Statement)
			}
		case MemoryKindPitfall:
			if len(context.Pitfalls) < memoryContextCategoryLimit {
				context.Pitfalls = append(context.Pitfalls, entry.Statement)
			}
		case MemoryKindSecretRef:
			if len(context.SecretRefs) < memoryContextCategoryLimit {
				context.SecretRefs = append(context.SecretRefs, entry.Statement)
			}
		case MemoryKindSuccessPrior:
			if len(context.SuccessPriors) < memoryContextCategoryLimit {
				context.SuccessPriors = append(context.SuccessPriors, entry.Statement)
			}
		}
	}
	return context, nil
}

func loadMemoryProjectProfile(projectID string) (*memoryProjectProfile, error) {
	path := filepath.Join(MemoryProjectsDir(), projectID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	profile := &memoryProjectProfile{}
	if err := json.Unmarshal(data, profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func selectorRecallEntries(query MemoryQuery, entryByNumericID map[uint32]MemoryEntry) ([]MemoryEntry, error) {
	index, err := BuildMemorySelectorIndex()
	if err != nil {
		return nil, err
	}
	keys := querySelectorKeys(query)
	if len(keys) == 0 {
		return nil, nil
	}

	candidateIDs := make(map[uint32]struct{})
	for _, key := range keys {
		for _, numericID := range index.Postings[key] {
			candidateIDs[numericID] = struct{}{}
		}
	}
	return entriesForNumericIDs(candidateIDs, entryByNumericID), nil
}

func lexicalRecallEntries(query MemoryQuery, entryByNumericID map[uint32]MemoryEntry) ([]MemoryEntry, error) {
	index, err := BuildMemoryTokenIndex()
	if err != nil {
		return nil, err
	}
	tokens := queryTokens(query)
	if len(tokens) == 0 {
		return nil, nil
	}

	candidateIDs := make(map[uint32]struct{})
	for _, token := range tokens {
		for _, numericID := range index.Postings[token] {
			candidateIDs[numericID] = struct{}{}
		}
	}
	return entriesForNumericIDs(candidateIDs, entryByNumericID), nil
}

func entriesForNumericIDs(candidateIDs map[uint32]struct{}, entryByNumericID map[uint32]MemoryEntry) []MemoryEntry {
	if len(candidateIDs) == 0 {
		return nil
	}
	numericIDs := make([]uint32, 0, len(candidateIDs))
	for numericID := range candidateIDs {
		numericIDs = append(numericIDs, numericID)
	}
	sort.Slice(numericIDs, func(i, j int) bool { return numericIDs[i] < numericIDs[j] })

	entries := make([]MemoryEntry, 0, len(numericIDs))
	for _, numericID := range numericIDs {
		entry, ok := entryByNumericID[numericID]
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func rankMemoryEntry(entry MemoryEntry, query MemoryQuery, requireSelectorMatch bool) (bool, memoryEntryRank) {
	rank := memoryEntryRank{
		SelectorCount: len(entry.Selectors),
		TrustWeight:   memoryTrustWeight(entry),
		Freshness:     memoryEntryFreshness(entry),
	}

	querySelectors := querySelectorMap(query)
	for key, value := range entry.Selectors {
		queryValue := querySelectors[key]
		if queryValue == "" {
			continue
		}
		if queryValue != value {
			return false, memoryEntryRank{}
		}
		rank.MatchedSelectors++
		if key == "project_id" {
			rank.ProjectSpecific = true
		}
	}

	if requireSelectorMatch && len(entry.Selectors) > 0 && rank.MatchedSelectors == 0 && len(querySelectorKeys(query)) > 0 {
		return false, memoryEntryRank{}
	}

	statementTokens := make(map[string]struct{})
	for _, token := range tokenizeMemoryStatement(entry.Statement) {
		statementTokens[token] = struct{}{}
	}
	for _, token := range queryTokens(query) {
		if _, ok := statementTokens[token]; ok {
			rank.LexicalMatches++
		}
	}
	return true, rank
}

func compareMemoryRank(left, right memoryRankedEntry) bool {
	if left.Rank.ProjectSpecific != right.Rank.ProjectSpecific {
		return left.Rank.ProjectSpecific
	}
	if left.Rank.MatchedSelectors != right.Rank.MatchedSelectors {
		return left.Rank.MatchedSelectors > right.Rank.MatchedSelectors
	}
	if left.Rank.LexicalMatches != right.Rank.LexicalMatches {
		return left.Rank.LexicalMatches > right.Rank.LexicalMatches
	}
	if left.Rank.TrustWeight != right.Rank.TrustWeight {
		return left.Rank.TrustWeight > right.Rank.TrustWeight
	}
	if left.Rank.Freshness != right.Rank.Freshness {
		return left.Rank.Freshness > right.Rank.Freshness
	}
	if left.Rank.SelectorCount != right.Rank.SelectorCount {
		return left.Rank.SelectorCount > right.Rank.SelectorCount
	}
	return left.Entry.ID < right.Entry.ID
}

func querySelectorMap(query MemoryQuery) map[string]string {
	return map[string]string{
		"project_id":  strings.TrimSpace(query.ProjectID),
		"environment": strings.TrimSpace(query.Environment),
		"infra_group": strings.TrimSpace(query.InfraGroup),
		"host":        strings.TrimSpace(query.Host),
		"service":     strings.TrimSpace(query.Service),
		"provider":    strings.TrimSpace(query.Provider),
		"tool":        strings.TrimSpace(query.Tool),
		"intent":      strings.TrimSpace(query.Intent),
	}
}

func querySelectorKeys(query MemoryQuery) []string {
	selectorMap := querySelectorMap(query)
	keys := make([]string, 0, len(selectorMap))
	for key, value := range selectorMap {
		if value == "" {
			continue
		}
		keys = append(keys, key+":"+value)
	}
	sort.Strings(keys)
	return keys
}

func queryTokens(query MemoryQuery) []string {
	tokenSet := map[string]struct{}{}
	for _, value := range querySelectorMap(query) {
		for _, token := range tokenizeMemoryStatement(value) {
			tokenSet[token] = struct{}{}
		}
	}
	tokens := make([]string, 0, len(tokenSet))
	for token := range tokenSet {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	return tokens
}

func memoryTrustWeight(entry MemoryEntry) int {
	weight := 0
	switch strings.TrimSpace(entry.VerificationState) {
	case "validated":
		weight += 30
	case "repeated":
		weight += 20
	case "unverified":
		weight += 10
	}
	switch strings.TrimSpace(entry.Confidence) {
	case "grounded":
		weight += 10
	case "high":
		weight += 8
	case "medium":
		weight += 4
	case "heuristic":
		weight += 1
	}
	return weight
}

func memoryEntryFreshness(entry MemoryEntry) int64 {
	for _, value := range []string{entry.UpdatedAt, entry.ValidFrom} {
		if strings.TrimSpace(value) == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err == nil {
			return parsed.Unix()
		}
	}
	return 0
}
