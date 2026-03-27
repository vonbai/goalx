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

func ExtractMemoryProposals(runDir string) ([]MemoryProposal, error) {
	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		return nil, err
	}
	proposals := make([]MemoryProposal, 0)
	seen := map[string]struct{}{}
	for _, seed := range seeds {
		extracted := extractProposalsFromSeed(seed)
		for _, proposal := range extracted {
			if _, ok := seen[proposal.ID]; ok {
				continue
			}
			seen[proposal.ID] = struct{}{}
			proposals = append(proposals, proposal)
		}
	}
	sort.Slice(proposals, func(i, j int) bool { return proposals[i].ID < proposals[j].ID })
	return proposals, nil
}

func AppendExtractedMemoryProposals(runDir string, now time.Time) error {
	proposals, err := ExtractMemoryProposals(runDir)
	if err != nil {
		return err
	}
	return AppendMemoryProposals(now, proposals)
}

func AppendMemoryProposals(now time.Time, proposals []MemoryProposal) error {
	if len(proposals) == 0 {
		return nil
	}
	if err := EnsureMemoryStore(); err != nil {
		return err
	}
	path := MemoryProposalPath(now)
	existing, err := loadMemoryProposals(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		existing = nil
	}
	seen := map[string]struct{}{}
	for _, proposal := range existing {
		seen[proposal.ID] = struct{}{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, proposal := range proposals {
		normalized, err := NormalizeMemoryProposal(&proposal)
		if err != nil {
			return err
		}
		proposal = *normalized
		if _, ok := seen[proposal.ID]; ok {
			continue
		}
		data, err := json.Marshal(proposal)
		if err != nil {
			return err
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return err
		}
		seen[proposal.ID] = struct{}{}
	}
	return nil
}

func loadMemoryProposals(path string) ([]MemoryProposal, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	out := make([]MemoryProposal, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var proposal MemoryProposal
		if err := json.Unmarshal([]byte(line), &proposal); err != nil {
			return nil, fmt.Errorf("parse memory proposal %s: %w", path, err)
		}
		normalized, err := NormalizeMemoryProposal(&proposal)
		if err != nil {
			return nil, fmt.Errorf("normalize memory proposal %s: %w", path, err)
		}
		out = append(out, *normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan memory proposals %s: %w", path, err)
	}
	return out, nil
}

func extractProposalsFromSeed(seed MemorySeed) []MemoryProposal {
	if len(seed.Selectors) == 0 {
		return nil
	}
	fields := parseSeedFacts(seed.Message)
	if len(fields) == 0 {
		return nil
	}
	proposals := make([]MemoryProposal, 0, len(fields))
	for key, value := range fields {
		kind, statement, ok := proposalFromSeedField(key, value)
		if !ok {
			continue
		}
		proposal := MemoryProposal{
			State:      "proposed",
			Kind:       kind,
			Statement:  statement,
			Selectors:  cloneStringMap(seed.Selectors),
			Evidence:   append([]MemoryEvidence(nil), seed.Evidence...),
			SourceRuns: compactStrings([]string{seed.Run}),
			ValidFrom:  strings.TrimSpace(seed.CreatedAt),
			CreatedAt:  firstNonEmpty(seed.CreatedAt, time.Now().UTC().Format(time.RFC3339)),
			UpdatedAt:  firstNonEmpty(seed.CreatedAt, time.Now().UTC().Format(time.RFC3339)),
		}
		proposal.ID = stableMemoryProposalID(kind, proposal.Selectors, statement)
		normalized, err := NormalizeMemoryProposal(&proposal)
		if err != nil {
			continue
		}
		proposals = append(proposals, *normalized)
	}
	return proposals
}

func parseSeedFacts(message string) map[string]string {
	fields := map[string]string{}
	for _, token := range strings.Fields(strings.TrimSpace(message)) {
		key, value, ok := strings.Cut(token, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(strings.ToLower(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || value == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func proposalFromSeedField(key, value string) (MemoryKind, string, bool) {
	switch key {
	case "deploy_path":
		return MemoryKindFact, "deploy path is " + value, true
	case "provider":
		return MemoryKindFact, "provider is " + value, true
	case "host":
		return MemoryKindFact, "host is " + value, true
	case "container":
		return MemoryKindFact, "container is " + value, true
	case "config_source":
		return MemoryKindFact, "config source is " + value, true
	case "secret_ref":
		if looksLikeSecretValue(value) {
			return "", "", false
		}
		return MemoryKindSecretRef, "secret reference is " + value, true
	default:
		return "", "", false
	}
}

func stableMemoryProposalID(kind MemoryKind, selectors map[string]string, statement string) string {
	selectorPairs := make([]string, 0, len(selectors))
	for key, value := range selectors {
		selectorPairs = append(selectorPairs, key+"="+value)
	}
	sort.Strings(selectorPairs)
	sum := sha256.Sum256([]byte(strings.Join([]string{
		string(kind),
		statement,
		strings.Join(selectorPairs, "|"),
	}, "\n")))
	return "prop_" + hex.EncodeToString(sum[:8])
}

func looksLikeSecretValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, ":") || strings.Contains(trimmed, "1password") || strings.Contains(trimmed, "doppler") || strings.Contains(trimmed, "vault") {
		return false
	}
	if strings.HasPrefix(trimmed, "sk-") || strings.HasPrefix(trimmed, "ghp_") || strings.HasPrefix(trimmed, "AIza") {
		return true
	}
	if len(trimmed) >= 20 && !strings.ContainsAny(trimmed, " .-_") {
		return true
	}
	return false
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func compactStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
