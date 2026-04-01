package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	MemoryPriorGovernanceKindReplayedValid = "replayed_valid"
	MemoryPriorGovernanceKindReinforced    = "reinforced"
	MemoryPriorGovernanceKindContradicted  = "contradicted"
	MemoryPriorGovernanceKindSuperseded    = "superseded"
)

type MemoryPriorGovernanceEvent struct {
	Version       int              `json:"version"`
	EntryID       string           `json:"entry_id"`
	Kind          string           `json:"kind"`
	SourceRun     string           `json:"source_run,omitempty"`
	ReplacementID string           `json:"replacement_id,omitempty"`
	Evidence      []MemoryEvidence `json:"evidence,omitempty"`
	RecordedAt    string           `json:"recorded_at,omitempty"`
}

type memoryPriorGovernanceSummary struct {
	ReplayedValidCount int
	ReinforcedCount    int
	ContradictedCount  int
	SupersededBy       string
}

func LoadMemoryPriorGovernanceEvents() ([]MemoryPriorGovernanceEvent, error) {
	path := MemoryPriorGovernancePath()
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	events := make([]MemoryPriorGovernanceEvent, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event MemoryPriorGovernanceEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, fmt.Errorf("parse memory prior governance event %s: %w", path, err)
		}
		normalized, err := normalizeMemoryPriorGovernanceEvent(event)
		if err != nil {
			return nil, fmt.Errorf("normalize memory prior governance event %s: %w", path, err)
		}
		events = append(events, normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan memory prior governance %s: %w", path, err)
	}
	return events, nil
}

func AppendMemoryPriorGovernanceEvent(event MemoryPriorGovernanceEvent) error {
	return withMemoryStoreLock(func() error {
		if err := EnsureMemoryStore(); err != nil {
			return err
		}
		return appendMemoryPriorGovernanceEventUnlocked(event)
	})
}

func ContradictMemoryEntry(entryID, sourceRun string, evidence []MemoryEvidence) error {
	return withMemoryStoreLock(func() error {
		if err := EnsureMemoryStore(); err != nil {
			return err
		}
		byKind, err := loadCanonicalMemoryByKind()
		if err != nil {
			return err
		}
		entry := findCanonicalEntryPointer(byKind[MemoryKindSuccessPrior], entryID)
		if entry == nil {
			return fmt.Errorf("success prior %q not found", entryID)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		entry.ContradictedCount++
		entry.UpdatedAt = now
		if err := saveCanonicalMemoryByKind(byKind); err != nil {
			return err
		}
		if err := appendMemoryPriorGovernanceEventUnlocked(MemoryPriorGovernanceEvent{
			EntryID:    entryID,
			Kind:       MemoryPriorGovernanceKindContradicted,
			SourceRun:  strings.TrimSpace(sourceRun),
			Evidence:   append([]MemoryEvidence(nil), evidence...),
			RecordedAt: now,
		}); err != nil {
			return err
		}
		return rebuildMemoryIndexesUnlocked()
	})
}

func appendMemoryPriorGovernanceEventUnlocked(event MemoryPriorGovernanceEvent) error {
	normalized, err := normalizeMemoryPriorGovernanceEvent(event)
	if err != nil {
		return err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	path := MemoryPriorGovernancePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(existing) > 0 && existing[len(existing)-1] != '\n' {
		existing = append(existing, '\n')
	}
	existing = append(existing, data...)
	existing = append(existing, '\n')
	return writeMemoryFileAtomic(path, existing)
}

func loadMemoryPriorGovernanceSummary() (map[string]memoryPriorGovernanceSummary, error) {
	events, err := LoadMemoryPriorGovernanceEvents()
	if err != nil {
		return nil, err
	}
	summary := make(map[string]memoryPriorGovernanceSummary, len(events))
	for _, event := range events {
		current := summary[event.EntryID]
		switch event.Kind {
		case MemoryPriorGovernanceKindReplayedValid:
			current.ReplayedValidCount++
		case MemoryPriorGovernanceKindReinforced:
			current.ReinforcedCount++
		case MemoryPriorGovernanceKindContradicted:
			current.ContradictedCount++
		case MemoryPriorGovernanceKindSuperseded:
			if strings.TrimSpace(event.ReplacementID) != "" {
				current.SupersededBy = strings.TrimSpace(event.ReplacementID)
			}
		}
		summary[event.EntryID] = current
	}
	return summary, nil
}

func normalizeMemoryPriorGovernanceEvent(event MemoryPriorGovernanceEvent) (MemoryPriorGovernanceEvent, error) {
	event.Version = 1
	event.EntryID = strings.TrimSpace(event.EntryID)
	event.Kind = strings.TrimSpace(event.Kind)
	event.SourceRun = strings.TrimSpace(event.SourceRun)
	event.ReplacementID = strings.TrimSpace(event.ReplacementID)
	event.Evidence = normalizeMemoryEvidence(event.Evidence)
	event.RecordedAt = strings.TrimSpace(event.RecordedAt)
	if event.EntryID == "" {
		return MemoryPriorGovernanceEvent{}, fmt.Errorf("memory prior governance entry_id is required")
	}
	if !isValidMemoryPriorGovernanceKind(event.Kind) {
		return MemoryPriorGovernanceEvent{}, fmt.Errorf("memory prior governance kind %q is invalid", event.Kind)
	}
	if event.Kind == MemoryPriorGovernanceKindSuperseded && event.ReplacementID == "" {
		return MemoryPriorGovernanceEvent{}, fmt.Errorf("memory prior governance superseded event requires replacement_id")
	}
	if event.RecordedAt == "" {
		event.RecordedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return event, nil
}

func isValidMemoryPriorGovernanceKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case MemoryPriorGovernanceKindReplayedValid,
		MemoryPriorGovernanceKindReinforced,
		MemoryPriorGovernanceKindContradicted,
		MemoryPriorGovernanceKindSuperseded:
		return true
	default:
		return false
	}
}

func findCanonicalEntryPointer(entries []MemoryEntry, id string) *MemoryEntry {
	for i := range entries {
		if entries[i].ID == id {
			return &entries[i]
		}
	}
	return nil
}
