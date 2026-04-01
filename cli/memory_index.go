package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

type MemorySelectorIndex struct {
	Version  int                 `json:"version"`
	EntryIDs map[string]uint32   `json:"entry_ids,omitempty"`
	Postings map[string][]uint32 `json:"postings,omitempty"`
}

type MemoryTokenIndex struct {
	Version           int                 `json:"version"`
	EntryIDs          map[string]uint32   `json:"entry_ids,omitempty"`
	Postings          map[string][]uint32 `json:"postings,omitempty"`
	DocumentFrequency map[string]int      `json:"document_frequency,omitempty"`
}

type MemoryTrustRecord struct {
	VerificationState  string `json:"verification_state,omitempty"`
	Confidence         string `json:"confidence,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
	ValidFrom          string `json:"valid_from,omitempty"`
	SupersededBy       string `json:"superseded_by,omitempty"`
	ContradictedCount  int    `json:"contradicted_count,omitempty"`
	ReinforcedCount    int    `json:"reinforced_count,omitempty"`
	ReplayedValidCount int    `json:"replayed_valid_count,omitempty"`
}

type MemoryTrustIndex struct {
	Version int                          `json:"version"`
	Records map[string]MemoryTrustRecord `json:"records,omitempty"`
}

type MemoryIndexStats struct {
	Version    int    `json:"version"`
	EntryCount int    `json:"entry_count,omitempty"`
	BuiltAt    string `json:"built_at,omitempty"`
}

func BuildMemorySelectorIndex() (*MemorySelectorIndex, error) {
	entries, err := loadCanonicalMemoryEntries()
	if err != nil {
		return nil, err
	}
	index := &MemorySelectorIndex{
		Version:  1,
		EntryIDs: stableMemoryEntryIDs(entries),
		Postings: map[string][]uint32{},
	}
	for _, entry := range entries {
		entryID := index.EntryIDs[entry.ID]
		for key, value := range entry.Selectors {
			postingKey := strings.TrimSpace(key) + ":" + strings.TrimSpace(value)
			index.Postings[postingKey] = append(index.Postings[postingKey], entryID)
		}
		index.Postings["kind:"+string(entry.Kind)] = append(index.Postings["kind:"+string(entry.Kind)], entryID)
	}
	return index, nil
}

func BuildMemoryTokenIndex() (*MemoryTokenIndex, error) {
	entries, err := loadCanonicalMemoryEntries()
	if err != nil {
		return nil, err
	}
	index := &MemoryTokenIndex{
		Version:           1,
		EntryIDs:          stableMemoryEntryIDs(entries),
		Postings:          map[string][]uint32{},
		DocumentFrequency: map[string]int{},
	}
	for _, entry := range entries {
		entryID := index.EntryIDs[entry.ID]
		seen := map[string]struct{}{}
		for _, token := range tokenizeMemoryStatement(entry.Statement) {
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			index.Postings[token] = append(index.Postings[token], entryID)
			index.DocumentFrequency[token]++
		}
	}
	return index, nil
}

func BuildMemoryTrustIndex() (*MemoryTrustIndex, error) {
	entries, err := loadCanonicalMemoryEntries()
	if err != nil {
		return nil, err
	}
	governance, err := loadMemoryPriorGovernanceSummary()
	if err != nil {
		return nil, err
	}
	index := &MemoryTrustIndex{
		Version: 1,
		Records: make(map[string]MemoryTrustRecord, len(entries)),
	}
	for _, entry := range entries {
		summary := governance[entry.ID]
		supersededBy := firstNonEmpty(summary.SupersededBy, entry.SupersededBy)
		index.Records[entry.ID] = MemoryTrustRecord{
			VerificationState:  entry.VerificationState,
			Confidence:         entry.Confidence,
			UpdatedAt:          entry.UpdatedAt,
			ValidFrom:          entry.ValidFrom,
			SupersededBy:       supersededBy,
			ContradictedCount:  entry.ContradictedCount + summary.ContradictedCount,
			ReinforcedCount:    summary.ReinforcedCount,
			ReplayedValidCount: summary.ReplayedValidCount,
		}
	}
	return index, nil
}

func RebuildMemoryIndexes() error {
	return withMemoryStoreLock(rebuildMemoryIndexesUnlocked)
}

func rebuildMemoryIndexesUnlocked() error {
	selectorIndex, err := BuildMemorySelectorIndex()
	if err != nil {
		return err
	}
	tokenIndex, err := BuildMemoryTokenIndex()
	if err != nil {
		return err
	}
	trustIndex, err := BuildMemoryTrustIndex()
	if err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(MemoryIndexesDir(), "selectors.json"), selectorIndex); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(MemoryIndexesDir(), "tokens.json"), tokenIndex); err != nil {
		return err
	}
	if err := writeJSONFile(filepath.Join(MemoryIndexesDir(), "trust.json"), trustIndex); err != nil {
		return err
	}
	stats := &MemoryIndexStats{
		Version:    1,
		EntryCount: len(selectorIndex.EntryIDs),
		BuiltAt:    time.Now().UTC().Format(time.RFC3339),
	}
	return writeJSONFile(filepath.Join(MemoryIndexesDir(), "stats.json"), stats)
}

func loadMemorySelectorIndex(path string) (*MemorySelectorIndex, error) {
	index := &MemorySelectorIndex{}
	if err := loadMemoryIndex(path, index); err != nil {
		return nil, err
	}
	if index.EntryIDs == nil {
		index.EntryIDs = map[string]uint32{}
	}
	if index.Postings == nil {
		index.Postings = map[string][]uint32{}
	}
	return index, nil
}

func loadMemoryIndexStats(path string) (*MemoryIndexStats, error) {
	stats := &MemoryIndexStats{}
	if err := loadMemoryIndex(path, stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func loadMemoryIndex(path string, dest any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil
	}
	return json.Unmarshal(data, dest)
}

func loadCanonicalMemoryEntries() ([]MemoryEntry, error) {
	entries := make([]MemoryEntry, 0)
	for _, kind := range []MemoryKind{
		MemoryKindFact,
		MemoryKindProcedure,
		MemoryKindPitfall,
		MemoryKindSecretRef,
		MemoryKindSuccessPrior,
	} {
		path := MemoryEntryPath(kind)
		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var entry MemoryEntry
			if err := json.Unmarshal([]byte(line), &entry); err != nil {
				_ = file.Close()
				return nil, fmt.Errorf("parse memory entry %s: %w", path, err)
			}
			normalized, err := NormalizeMemoryEntry(&entry)
			if err != nil {
				_ = file.Close()
				return nil, fmt.Errorf("normalize memory entry %s: %w", path, err)
			}
			entries = append(entries, *normalized)
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("scan memory entries %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	return entries, nil
}

func stableMemoryEntryIDs(entries []MemoryEntry) map[string]uint32 {
	ids := make(map[string]uint32, len(entries))
	for i, entry := range entries {
		ids[entry.ID] = uint32(i + 1)
	}
	return ids
}

func tokenizeMemoryStatement(statement string) []string {
	normalized := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			return unicode.ToLower(r)
		case unicode.IsSpace(r):
			return ' '
		default:
			return ' '
		}
	}, statement)
	return strings.Fields(normalized)
}
