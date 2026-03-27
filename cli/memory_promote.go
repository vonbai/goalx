package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type memoryProposalAggregate struct {
	Kind             MemoryKind
	Statement        string
	Selectors        map[string]string
	Evidence         []MemoryEvidence
	SourceRuns       []string
	Verification     string
	Confidence       string
	ValidFrom        string
	CreatedAt        string
	UpdatedAt        string
	FailureEvidence  bool
	RecoveryEvidence bool
}

func PromoteMemoryProposals() error {
	return withMemoryStoreLock(promoteMemoryProposalsLocked)
}

func SupersedeMemoryEntry(oldID, newID string) error {
	return withMemoryStoreLock(func() error {
		if err := EnsureMemoryStore(); err != nil {
			return err
		}
		byKind, err := loadCanonicalMemoryByKind()
		if err != nil {
			return err
		}
		changed, err := supersedeMemoryEntryInStore(byKind, oldID, newID, time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}
		if err := saveCanonicalMemoryByKind(byKind); err != nil {
			return err
		}
		return rebuildMemoryIndexesUnlocked()
	})
}

func promoteMemoryProposalsLocked() error {
	if err := EnsureMemoryStore(); err != nil {
		return err
	}
	proposals, err := loadAllMemoryProposals()
	if err != nil {
		return err
	}
	if len(proposals) == 0 {
		return nil
	}

	byKind, err := loadCanonicalMemoryByKind()
	if err != nil {
		return err
	}
	aggregates := aggregateMemoryProposals(proposals)
	changed := false
	now := time.Now().UTC().Format(time.RFC3339)
	for _, aggregate := range aggregates {
		if !memoryAggregatePromotable(aggregate) {
			continue
		}
		if promoteAggregateIntoCanonical(byKind, aggregate, now) {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := saveCanonicalMemoryByKind(byKind); err != nil {
		return err
	}
	return rebuildMemoryIndexesUnlocked()
}

func loadAllMemoryProposals() ([]MemoryProposal, error) {
	entries, err := os.ReadDir(MemoryProposalsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		paths = append(paths, filepath.Join(MemoryProposalsDir(), entry.Name()))
	}
	sort.Strings(paths)

	out := make([]MemoryProposal, 0)
	for _, path := range paths {
		proposals, err := loadMemoryProposals(path)
		if err != nil {
			return nil, err
		}
		out = append(out, proposals...)
	}
	return out, nil
}

func aggregateMemoryProposals(proposals []MemoryProposal) []memoryProposalAggregate {
	byKey := map[string]*memoryProposalAggregate{}
	for _, proposal := range proposals {
		if strings.TrimSpace(proposal.State) == "rejected" || strings.TrimSpace(proposal.State) == "expired" {
			continue
		}
		key := memoryAggregateKey(proposal.Kind, proposal.Selectors, proposal.Statement)
		aggregate := byKey[key]
		if aggregate == nil {
			aggregate = &memoryProposalAggregate{
				Kind:       proposal.Kind,
				Statement:  strings.TrimSpace(proposal.Statement),
				Selectors:  cloneStringMap(proposal.Selectors),
				ValidFrom:  strings.TrimSpace(proposal.ValidFrom),
				CreatedAt:  strings.TrimSpace(proposal.CreatedAt),
				UpdatedAt:  strings.TrimSpace(proposal.UpdatedAt),
				Confidence: "grounded",
			}
			byKey[key] = aggregate
		}
		aggregate.Evidence = mergeMemoryEvidence(aggregate.Evidence, proposal.Evidence)
		aggregate.SourceRuns = mergeStringSets(aggregate.SourceRuns, proposal.SourceRuns)
		aggregate.ValidFrom = earliestRFC3339(aggregate.ValidFrom, proposal.ValidFrom, proposal.CreatedAt, proposal.UpdatedAt)
		aggregate.CreatedAt = earliestRFC3339(aggregate.CreatedAt, proposal.CreatedAt, proposal.ValidFrom, proposal.UpdatedAt)
		aggregate.UpdatedAt = latestRFC3339(aggregate.UpdatedAt, proposal.UpdatedAt, proposal.CreatedAt, proposal.ValidFrom)
		if proposalHasFailureEvidence(proposal) {
			aggregate.FailureEvidence = true
		}
		if proposalHasRecoveryEvidence(proposal) {
			aggregate.RecoveryEvidence = true
		}
	}

	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]memoryProposalAggregate, 0, len(keys))
	for _, key := range keys {
		aggregate := byKey[key]
		switch aggregate.Kind {
		case MemoryKindFact, MemoryKindSecretRef:
			aggregate.Verification = "validated"
		case MemoryKindProcedure:
			if aggregate.FailureEvidence && aggregate.RecoveryEvidence {
				aggregate.Verification = "validated"
			} else if len(aggregate.SourceRuns) >= 2 {
				aggregate.Verification = "repeated"
			}
		case MemoryKindPitfall:
			if aggregate.FailureEvidence {
				aggregate.Verification = "validated"
			} else if len(aggregate.SourceRuns) >= 2 {
				aggregate.Verification = "repeated"
			}
		}
		out = append(out, *aggregate)
	}
	return out
}

func memoryAggregatePromotable(aggregate memoryProposalAggregate) bool {
	if len(aggregate.Selectors) == 0 || !hasGroundedEvidence(aggregate.Evidence) {
		return false
	}
	switch aggregate.Kind {
	case MemoryKindFact, MemoryKindSecretRef:
		return true
	case MemoryKindProcedure:
		return len(aggregate.SourceRuns) >= 2 || (aggregate.FailureEvidence && aggregate.RecoveryEvidence)
	case MemoryKindPitfall:
		return len(aggregate.SourceRuns) >= 2 || aggregate.FailureEvidence
	default:
		return false
	}
}

func promoteAggregateIntoCanonical(byKind map[MemoryKind][]MemoryEntry, aggregate memoryProposalAggregate, now string) bool {
	entryID := stableMemoryEntryID(aggregate.Kind, aggregate.Selectors, aggregate.Statement)
	if existing := findActiveCanonicalEntry(byKind[aggregate.Kind], entryID); existing != nil {
		return mergeAggregateIntoEntry(existing, aggregate, now)
	}

	entry := MemoryEntry{
		ID:                entryID,
		Kind:              aggregate.Kind,
		Statement:         aggregate.Statement,
		Selectors:         cloneStringMap(aggregate.Selectors),
		VerificationState: aggregate.Verification,
		Confidence:        aggregate.Confidence,
		Evidence:          append([]MemoryEvidence(nil), aggregate.Evidence...),
		SourceRuns:        append([]string(nil), aggregate.SourceRuns...),
		ValidFrom:         firstNonEmpty(aggregate.ValidFrom, aggregate.CreatedAt, now),
		CreatedAt:         firstNonEmpty(aggregate.CreatedAt, aggregate.ValidFrom, now),
		UpdatedAt:         firstNonEmpty(aggregate.UpdatedAt, aggregate.CreatedAt, now),
	}
	if normalized, err := NormalizeMemoryEntry(&entry); err == nil {
		entry = *normalized
	}
	byKind[aggregate.Kind] = append(byKind[aggregate.Kind], entry)

	if aggregate.Kind == MemoryKindFact || aggregate.Kind == MemoryKindSecretRef {
		if prior := findConflictingCanonicalEntry(byKind[aggregate.Kind], entry); prior != nil {
			_, _ = supersedeMemoryEntryInStore(byKind, prior.ID, entry.ID, now)
		}
	}
	return true
}

func mergeAggregateIntoEntry(entry *MemoryEntry, aggregate memoryProposalAggregate, now string) bool {
	changed := false

	mergedEvidence := mergeMemoryEvidence(entry.Evidence, aggregate.Evidence)
	if !memoryEvidenceEqual(entry.Evidence, mergedEvidence) {
		entry.Evidence = mergedEvidence
		changed = true
	}
	mergedRuns := mergeStringSets(entry.SourceRuns, aggregate.SourceRuns)
	if !stringSliceEqual(entry.SourceRuns, mergedRuns) {
		entry.SourceRuns = mergedRuns
		changed = true
	}

	if strongerVerificationState(aggregate.Verification, entry.VerificationState) == aggregate.Verification && strings.TrimSpace(aggregate.Verification) != strings.TrimSpace(entry.VerificationState) {
		entry.VerificationState = aggregate.Verification
		changed = true
	}
	if strongerConfidence(aggregate.Confidence, entry.Confidence) == aggregate.Confidence && strings.TrimSpace(aggregate.Confidence) != strings.TrimSpace(entry.Confidence) {
		entry.Confidence = aggregate.Confidence
		changed = true
	}

	validFrom := earliestRFC3339(entry.ValidFrom, aggregate.ValidFrom, aggregate.CreatedAt)
	if validFrom != strings.TrimSpace(entry.ValidFrom) {
		entry.ValidFrom = validFrom
		changed = true
	}
	createdAt := earliestRFC3339(entry.CreatedAt, aggregate.CreatedAt, aggregate.ValidFrom)
	if createdAt != strings.TrimSpace(entry.CreatedAt) {
		entry.CreatedAt = createdAt
		changed = true
	}
	updatedAt := latestRFC3339(entry.UpdatedAt, aggregate.UpdatedAt, aggregate.CreatedAt, now)
	if updatedAt != strings.TrimSpace(entry.UpdatedAt) {
		entry.UpdatedAt = updatedAt
		changed = true
	}
	return changed
}

func loadCanonicalMemoryByKind() (map[MemoryKind][]MemoryEntry, error) {
	out := map[MemoryKind][]MemoryEntry{}
	for _, kind := range []MemoryKind{
		MemoryKindFact,
		MemoryKindProcedure,
		MemoryKindPitfall,
		MemoryKindSecretRef,
	} {
		entries, err := loadCanonicalEntriesForKind(kind)
		if err != nil {
			return nil, err
		}
		out[kind] = entries
	}
	return out, nil
}

func loadCanonicalEntriesForKind(kind MemoryKind) ([]MemoryEntry, error) {
	path := MemoryEntryPath(kind)
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	entries := make([]MemoryEntry, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var entry MemoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("parse canonical memory %s: %w", path, err)
		}
		normalized, err := NormalizeMemoryEntry(&entry)
		if err != nil {
			return nil, fmt.Errorf("normalize canonical memory %s: %w", path, err)
		}
		entries = append(entries, *normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan canonical memory %s: %w", path, err)
	}
	return entries, nil
}

func saveCanonicalMemoryByKind(byKind map[MemoryKind][]MemoryEntry) error {
	for _, kind := range []MemoryKind{
		MemoryKindFact,
		MemoryKindProcedure,
		MemoryKindPitfall,
		MemoryKindSecretRef,
	} {
		path := MemoryEntryPath(kind)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		lines := make([]byte, 0)
		for _, entry := range byKind[kind] {
			normalized, err := NormalizeMemoryEntry(&entry)
			if err != nil {
				return err
			}
			data, err := json.Marshal(normalized)
			if err != nil {
				return err
			}
			lines = append(lines, data...)
			lines = append(lines, '\n')
		}
		if err := writeMemoryFileAtomic(path, lines); err != nil {
			return err
		}
	}
	return nil
}

func supersedeMemoryEntryInStore(byKind map[MemoryKind][]MemoryEntry, oldID, newID, now string) (bool, error) {
	var oldEntry *MemoryEntry
	for kind := range byKind {
		for i := range byKind[kind] {
			if byKind[kind][i].ID == oldID {
				oldEntry = &byKind[kind][i]
				break
			}
		}
	}
	if oldEntry == nil {
		return false, fmt.Errorf("memory entry %q not found", oldID)
	}
	changed := false
	if strings.TrimSpace(oldEntry.SupersededBy) != strings.TrimSpace(newID) {
		oldEntry.SupersededBy = strings.TrimSpace(newID)
		changed = true
	}
	if strings.TrimSpace(oldEntry.ValidTo) == "" {
		oldEntry.ValidTo = now
		changed = true
	}
	oldEntry.ContradictedCount++
	oldEntry.UpdatedAt = now
	changed = true
	return changed, nil
}

func findActiveCanonicalEntry(entries []MemoryEntry, entryID string) *MemoryEntry {
	for i := range entries {
		if entries[i].ID == entryID && strings.TrimSpace(entries[i].SupersededBy) == "" {
			return &entries[i]
		}
	}
	return nil
}

func findConflictingCanonicalEntry(entries []MemoryEntry, entry MemoryEntry) *MemoryEntry {
	scopeKey := memoryScopeKey(entry.Kind, entry.Selectors)
	for i := range entries {
		if entries[i].ID == entry.ID || strings.TrimSpace(entries[i].SupersededBy) != "" {
			continue
		}
		if memoryScopeKey(entries[i].Kind, entries[i].Selectors) != scopeKey {
			continue
		}
		if strings.TrimSpace(entries[i].Statement) == strings.TrimSpace(entry.Statement) {
			continue
		}
		return &entries[i]
	}
	return nil
}

func mergeMemoryEvidence(left, right []MemoryEvidence) []MemoryEvidence {
	seen := map[string]MemoryEvidence{}
	for _, item := range append(append([]MemoryEvidence(nil), left...), right...) {
		item.Kind = strings.TrimSpace(item.Kind)
		item.Path = strings.TrimSpace(item.Path)
		if item.Kind == "" && item.Path == "" {
			continue
		}
		seen[item.Kind+":"+item.Path] = item
	}
	if len(seen) == 0 {
		return nil
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]MemoryEvidence, 0, len(keys))
	for _, key := range keys {
		out = append(out, seen[key])
	}
	return out
}

func mergeStringSets(left, right []string) []string {
	seen := map[string]struct{}{}
	for _, value := range append(append([]string(nil), left...), right...) {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasGroundedEvidence(evidence []MemoryEvidence) bool {
	for _, item := range evidence {
		switch strings.TrimSpace(item.Kind) {
		case "summary", "report", "saved_summary", "saved_report", "acceptance_state", "acceptance_output", "saved_acceptance_output", "transport_facts", "verify_failure", "verify_recovery", "verify_success":
			return true
		}
	}
	return false
}

func proposalHasFailureEvidence(proposal MemoryProposal) bool {
	for _, item := range proposal.Evidence {
		switch strings.TrimSpace(item.Kind) {
		case "verify_failure", "acceptance_failure", "transport_error", "failure":
			return true
		}
	}
	return false
}

func proposalHasRecoveryEvidence(proposal MemoryProposal) bool {
	for _, item := range proposal.Evidence {
		switch strings.TrimSpace(item.Kind) {
		case "verify_recovery", "verify_success", "acceptance_success", "recovery":
			return true
		}
	}
	return false
}

func memoryAggregateKey(kind MemoryKind, selectors map[string]string, statement string) string {
	return strings.Join([]string{
		string(kind),
		strings.TrimSpace(statement),
		selectorPairsString(selectors),
	}, "\n")
}

func memoryScopeKey(kind MemoryKind, selectors map[string]string) string {
	return strings.Join([]string{string(kind), selectorPairsString(selectors)}, "\n")
}

func selectorPairsString(selectors map[string]string) string {
	pairs := make([]string, 0, len(selectors))
	for key, value := range selectors {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "|")
}

func stableMemoryEntryID(kind MemoryKind, selectors map[string]string, statement string) string {
	sum := sha256.Sum256([]byte(memoryAggregateKey(kind, selectors, statement)))
	return "mem_" + hex.EncodeToString(sum[:8])
}

func earliestRFC3339(values ...string) string {
	var best time.Time
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			continue
		}
		if best.IsZero() || parsed.Before(best) {
			best = parsed
		}
	}
	if best.IsZero() {
		return ""
	}
	return best.UTC().Format(time.RFC3339)
}

func latestRFC3339(values ...string) string {
	var best time.Time
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			continue
		}
		if best.IsZero() || parsed.After(best) {
			best = parsed
		}
	}
	if best.IsZero() {
		return ""
	}
	return best.UTC().Format(time.RFC3339)
}

func strongerVerificationState(left, right string) string {
	rank := map[string]int{
		"":           0,
		"unverified": 1,
		"repeated":   2,
		"validated":  3,
	}
	if rank[strings.TrimSpace(left)] >= rank[strings.TrimSpace(right)] {
		return strings.TrimSpace(left)
	}
	return strings.TrimSpace(right)
}

func strongerConfidence(left, right string) string {
	rank := map[string]int{
		"":          0,
		"heuristic": 1,
		"medium":    2,
		"high":      3,
		"grounded":  4,
	}
	if rank[strings.TrimSpace(left)] >= rank[strings.TrimSpace(right)] {
		return strings.TrimSpace(left)
	}
	return strings.TrimSpace(right)
}

func memoryEvidenceEqual(left, right []MemoryEvidence) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func stringSliceEqual(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
