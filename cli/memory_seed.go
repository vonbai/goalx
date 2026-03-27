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
)

type MemorySeed struct {
	Kind      string            `json:"kind,omitempty"`
	Run       string            `json:"run,omitempty"`
	Selectors map[string]string `json:"selectors,omitempty"`
	Message   string            `json:"message,omitempty"`
	Evidence  []MemoryEvidence  `json:"evidence,omitempty"`
	CreatedAt string            `json:"created_at,omitempty"`
}

func LoadMemorySeeds(path string) ([]MemorySeed, error) {
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
	seeds := make([]MemorySeed, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var seed MemorySeed
		if err := json.Unmarshal([]byte(line), &seed); err != nil {
			return nil, fmt.Errorf("parse memory seed %s: %w", path, err)
		}
		normalized, err := NormalizeMemorySeed(&seed)
		if err != nil {
			return nil, fmt.Errorf("normalize memory seed %s: %w", path, err)
		}
		seeds = append(seeds, *normalized)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan memory seeds %s: %w", path, err)
	}
	return seeds, nil
}

func AppendMemorySeed(runDir string, seed MemorySeed) error {
	normalized, err := NormalizeMemorySeed(&seed)
	if err != nil {
		return err
	}
	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		return err
	}
	seeds = appendIfMissingSeed(seeds, *normalized)
	return SaveMemorySeeds(MemorySeedsPath(runDir), seeds)
}

func SaveMemorySeeds(path string, seeds []MemorySeed) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var lines []byte
	for _, seed := range dedupeMemorySeeds(seeds) {
		data, err := json.Marshal(seed)
		if err != nil {
			return err
		}
		lines = append(lines, data...)
		lines = append(lines, '\n')
	}
	return os.WriteFile(path, lines, 0o644)
}

func NormalizeMemorySeed(seed *MemorySeed) (*MemorySeed, error) {
	if seed == nil {
		return nil, fmt.Errorf("memory seed is nil")
	}
	normalized := *seed
	normalized.Kind = strings.TrimSpace(normalized.Kind)
	normalized.Run = strings.TrimSpace(normalized.Run)
	normalized.Message = strings.TrimSpace(normalized.Message)
	normalized.Selectors = normalizeMemorySelectors(normalized.Selectors)
	normalized.Evidence = normalizeMemoryEvidence(normalized.Evidence)
	if normalized.Kind == "" {
		return nil, fmt.Errorf("memory seed kind is required")
	}
	if normalized.Message == "" {
		return nil, fmt.Errorf("memory seed message is required")
	}
	if normalized.CreatedAt == "" {
		normalized.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return &normalized, nil
}

func AppendMemorySeedFromVerifyResult(runDir string) error {
	state, err := LoadAcceptanceState(AcceptanceStatePath(runDir))
	if err != nil {
		return err
	}
	if state == nil || strings.TrimSpace(state.LastResult.CheckedAt) == "" {
		return nil
	}
	exitCode := "unknown"
	if state.LastResult.ExitCode != nil {
		exitCode = fmt.Sprintf("%d", *state.LastResult.ExitCode)
	}
	seed := MemorySeed{
		Kind:    "verify_result",
		Run:     filepath.Base(runDir),
		Message: fmt.Sprintf("acceptance command recorded exit_code=%s", exitCode),
		Evidence: []MemoryEvidence{
			{Kind: "acceptance_state", Path: AcceptanceStatePath(runDir)},
		},
		CreatedAt: state.LastResult.CheckedAt,
	}
	if strings.TrimSpace(state.LastResult.EvidencePath) != "" {
		seed.Evidence = append(seed.Evidence, MemoryEvidence{Kind: "acceptance_output", Path: state.LastResult.EvidencePath})
	}
	return AppendMemorySeed(runDir, seed)
}

func CollectRunMemorySeeds(runDir string) ([]MemorySeed, error) {
	seeds, err := LoadMemorySeeds(MemorySeedsPath(runDir))
	if err != nil {
		return nil, err
	}
	for _, seed := range collectSummarySeeds(runDir) {
		seeds = appendIfMissingSeed(seeds, seed)
	}
	for _, seed := range collectReportSeeds(runDir) {
		seeds = appendIfMissingSeed(seeds, seed)
	}
	for _, seed := range collectSavedArtifactSeeds(runDir) {
		seeds = appendIfMissingSeed(seeds, seed)
	}
	transportSeeds, err := collectTransportSeeds(runDir)
	if err != nil {
		return nil, err
	}
	for _, seed := range transportSeeds {
		seeds = appendIfMissingSeed(seeds, seed)
	}
	return dedupeMemorySeeds(seeds), nil
}

func RefreshRunMemorySeeds(runDir string) error {
	seeds, err := CollectRunMemorySeeds(runDir)
	if err != nil {
		return err
	}
	return SaveMemorySeeds(MemorySeedsPath(runDir), seeds)
}

func collectSummarySeeds(runDir string) []MemorySeed {
	var seeds []MemorySeed
	if info, err := os.Stat(SummaryPath(runDir)); err == nil && !info.IsDir() && info.Size() > 0 {
		seeds = append(seeds, MemorySeed{
			Kind:      "summary_present",
			Run:       filepath.Base(runDir),
			Message:   "closeout summary present",
			Evidence:  []MemoryEvidence{{Kind: "summary", Path: SummaryPath(runDir)}},
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return seeds
}

func collectReportSeeds(runDir string) []MemorySeed {
	entries, err := os.ReadDir(ReportsDir(runDir))
	if err != nil {
		return nil
	}
	seeds := make([]MemorySeed, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(ReportsDir(runDir), entry.Name())
		info, err := entry.Info()
		if err != nil || info.Size() == 0 {
			continue
		}
		seeds = append(seeds, MemorySeed{
			Kind:      "report_present",
			Run:       filepath.Base(runDir),
			Message:   fmt.Sprintf("report artifact present file=%s", entry.Name()),
			Evidence:  []MemoryEvidence{{Kind: "report", Path: path}},
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	return seeds
}

func collectSavedArtifactSeeds(runDir string) []MemorySeed {
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil || meta == nil || strings.TrimSpace(meta.ProjectRoot) == "" {
		return nil
	}
	saveDir := SavedRunDir(meta.ProjectRoot, filepath.Base(runDir))
	if info, err := os.Stat(saveDir); err != nil || !info.IsDir() {
		return nil
	}

	seeds := make([]MemorySeed, 0, 3)
	if info, err := os.Stat(SummaryPath(saveDir)); err == nil && !info.IsDir() && info.Size() > 0 {
		seeds = append(seeds, MemorySeed{
			Kind:      "saved_summary_present",
			Run:       filepath.Base(runDir),
			Message:   "saved closeout summary present",
			Evidence:  []MemoryEvidence{{Kind: "saved_summary", Path: SummaryPath(saveDir)}},
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	if info, err := os.Stat(filepath.Join(saveDir, "acceptance-last.txt")); err == nil && !info.IsDir() && info.Size() > 0 {
		seeds = append(seeds, MemorySeed{
			Kind:      "saved_acceptance_evidence_present",
			Run:       filepath.Base(runDir),
			Message:   "saved acceptance evidence present",
			Evidence:  []MemoryEvidence{{Kind: "saved_acceptance_output", Path: filepath.Join(saveDir, "acceptance-last.txt")}},
			CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	reportEntries, err := os.ReadDir(filepath.Join(saveDir, "reports"))
	if err == nil {
		for _, entry := range reportEntries {
			if entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil || info.Size() == 0 {
				continue
			}
			seeds = append(seeds, MemorySeed{
				Kind:      "saved_report_present",
				Run:       filepath.Base(runDir),
				Message:   fmt.Sprintf("saved report artifact present file=%s", entry.Name()),
				Evidence:  []MemoryEvidence{{Kind: "saved_report", Path: filepath.Join(saveDir, "reports", entry.Name())}},
				CreatedAt: info.ModTime().UTC().Format(time.RFC3339),
			})
		}
	}
	return seeds
}

func collectTransportSeeds(runDir string) ([]MemorySeed, error) {
	facts, err := LoadTransportFacts(TransportFactsPath(runDir))
	if err != nil || facts == nil {
		return nil, err
	}
	seeds := make([]MemorySeed, 0)
	for target, targetFacts := range facts.Targets {
		if targetFacts.ProviderDialogVisible {
			createdAt := firstNonEmpty(targetFacts.LastSampleAt, facts.CheckedAt, time.Now().UTC().Format(time.RFC3339))
			seeds = append(seeds, MemorySeed{
				Kind: "provider_dialog_visible",
				Run:  filepath.Base(runDir),
				Selectors: map[string]string{
					"target": target,
					"engine": strings.TrimSpace(targetFacts.Engine),
				},
				Message: fmt.Sprintf("provider dialog visible target=%s engine=%s kind=%s hint=%s",
					target,
					blankAsUnknown(targetFacts.Engine),
					blankAsUnknown(targetFacts.ProviderDialogKind),
					blankAsUnknown(targetFacts.ProviderDialogHint),
				),
				Evidence:  []MemoryEvidence{{Kind: "transport_facts", Path: TransportFactsPath(runDir)}},
				CreatedAt: createdAt,
			})
		}
		if strings.TrimSpace(targetFacts.LastTransportError) != "" {
			createdAt := firstNonEmpty(targetFacts.LastSubmitAttemptAt, facts.CheckedAt, time.Now().UTC().Format(time.RFC3339))
			seeds = append(seeds, MemorySeed{
				Kind: "transport_error",
				Run:  filepath.Base(runDir),
				Selectors: map[string]string{
					"target": target,
					"engine": strings.TrimSpace(targetFacts.Engine),
				},
				Message:   fmt.Sprintf("transport error target=%s error=%s", target, strings.TrimSpace(targetFacts.LastTransportError)),
				Evidence:  []MemoryEvidence{{Kind: "transport_facts", Path: TransportFactsPath(runDir)}},
				CreatedAt: createdAt,
			})
		}
	}
	return seeds, nil
}

func appendIfMissingSeed(seeds []MemorySeed, seed MemorySeed) []MemorySeed {
	key := memorySeedKey(seed)
	for _, existing := range seeds {
		if memorySeedKey(existing) == key {
			return seeds
		}
	}
	return append(seeds, seed)
}

func dedupeMemorySeeds(seeds []MemorySeed) []MemorySeed {
	if len(seeds) == 0 {
		return nil
	}
	out := make([]MemorySeed, 0, len(seeds))
	seen := map[string]struct{}{}
	for _, seed := range seeds {
		key := memorySeedKey(seed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, seed)
	}
	return out
}

func memorySeedKey(seed MemorySeed) string {
	selectorPairs := make([]string, 0, len(seed.Selectors))
	for key, value := range seed.Selectors {
		selectorPairs = append(selectorPairs, key+"="+value)
	}
	sort.Strings(selectorPairs)
	evidencePaths := make([]string, 0, len(seed.Evidence))
	for _, evidence := range seed.Evidence {
		evidencePaths = append(evidencePaths, evidence.Kind+":"+evidence.Path)
	}
	sort.Strings(evidencePaths)
	return strings.Join([]string{
		strings.TrimSpace(seed.Kind),
		strings.TrimSpace(seed.Run),
		strings.TrimSpace(seed.Message),
		strings.Join(selectorPairs, "|"),
		strings.Join(evidencePaths, "|"),
	}, "\n")
}

func normalizeMemoryEvidence(evidence []MemoryEvidence) []MemoryEvidence {
	if len(evidence) == 0 {
		return nil
	}
	out := make([]MemoryEvidence, 0, len(evidence))
	for _, item := range evidence {
		kind := strings.TrimSpace(item.Kind)
		path := strings.TrimSpace(item.Path)
		if kind == "" && path == "" {
			continue
		}
		out = append(out, MemoryEvidence{Kind: kind, Path: path})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
