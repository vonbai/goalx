package cli

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const memoryLLMProposalLimit = 2
const memoryLLMExtractTimeout = 20 * time.Second

type memoryLLMExtractTarget struct {
	Engine  string
	Model   string
	Effort  goalx.EffortLevel
	ModelID string
}

type memoryLLMExtractRequest struct {
	RunDir      string
	ProjectRoot string
	Target      memoryLLMExtractTarget
	Bundle      string
	Schema      string
	Timeout     time.Duration
	EvidenceMap map[string]MemoryEvidence
	Selectors   map[string]string
	SourceRuns  []string
	ObservedAt  string
}

type memoryLLMExtractResponse struct {
	Proposals []memoryLLMExtractItem `json:"proposals,omitempty"`
}

type memoryLLMExtractItem struct {
	Kind          MemoryKind `json:"kind,omitempty"`
	Statement     string     `json:"statement,omitempty"`
	EvidencePaths []string   `json:"evidence_paths,omitempty"`
}

var runMemoryLLMExtract = defaultRunMemoryLLMExtract
var memoryLLMCommandExists = toolAvailable

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
	llmProposals, err := extractLLMMemoryProposals(runDir, seeds)
	if err != nil {
		appendAuditLog(runDir, "memory llm extraction skipped: %v", err)
	} else {
		for _, proposal := range llmProposals {
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
	return withMemoryStoreLock(func() error {
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
		merged := append([]MemoryProposal(nil), existing...)
		seen := map[string]struct{}{}
		for _, proposal := range existing {
			seen[proposal.ID] = struct{}{}
		}
		for _, proposal := range proposals {
			normalized, err := NormalizeMemoryProposal(&proposal)
			if err != nil {
				return err
			}
			proposal = *normalized
			if _, ok := seen[proposal.ID]; ok {
				continue
			}
			merged = append(merged, proposal)
			seen[proposal.ID] = struct{}{}
		}
		return saveMemoryProposalShard(path, merged)
	})
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

func saveMemoryProposalShard(path string, proposals []MemoryProposal) error {
	lines := make([]byte, 0)
	for _, proposal := range proposals {
		normalized, err := NormalizeMemoryProposal(&proposal)
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
	return writeMemoryFileAtomic(path, lines)
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

func extractLLMMemoryProposals(runDir string, seeds []MemorySeed) ([]MemoryProposal, error) {
	if !memoryLLMExtractionEligible(runDir, seeds) {
		return nil, nil
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		return nil, err
	}
	if meta == nil || strings.TrimSpace(meta.ProjectRoot) == "" {
		return nil, nil
	}
	engines, err := loadEngineCatalog(meta.ProjectRoot)
	if err != nil {
		return nil, err
	}
	target, ok := selectMemoryLLMExtractTarget(engines)
	if !ok {
		return nil, nil
	}
	query, err := BuildMemoryQuery(runDir)
	if err != nil {
		return nil, err
	}
	req, err := buildMemoryLLMExtractRequest(runDir, meta.ProjectRoot, target, query, seeds)
	if err != nil {
		return nil, err
	}
	if req == nil {
		return nil, nil
	}
	resp, err := runMemoryLLMExtract(*req)
	if err != nil {
		return nil, err
	}
	return llmResponseToMemoryProposals(*req, resp), nil
}

func memoryLLMExtractionEligible(runDir string, seeds []MemorySeed) bool {
	if len(seeds) == 0 {
		return false
	}
	if !fileExists(SummaryPath(runDir)) && len(collectReportEvidencePaths(runDir)) == 0 {
		return false
	}
	for _, seed := range seeds {
		if len(seed.Evidence) == 0 {
			continue
		}
		switch strings.TrimSpace(seed.Kind) {
		case "verify_result", "transport_error", "provider_dialog_visible", "summary_present", "report_present", "saved_summary_present", "saved_report_present", "saved_acceptance_evidence_present":
			return true
		}
	}
	return false
}

func selectMemoryLLMExtractTarget(engines map[string]goalx.EngineConfig) (memoryLLMExtractTarget, bool) {
	if memoryLLMCommandExists("codex") {
		if target, ok := buildMemoryTargetForEngine(engines, "codex", []string{"fast", "gpt-5.4", "best", "balanced"}, goalx.EffortMinimal); ok {
			return target, true
		}
	}
	if memoryLLMCommandExists("claude") {
		if target, ok := buildMemoryTargetForEngine(engines, "claude-code", []string{"haiku", "sonnet", "opus"}, goalx.EffortLow); ok {
			return target, true
		}
	}
	return memoryLLMExtractTarget{}, false
}

func buildMemoryTargetForEngine(engines map[string]goalx.EngineConfig, engine string, modelCandidates []string, effort goalx.EffortLevel) (memoryLLMExtractTarget, bool) {
	engineConfig, ok := engines[engine]
	if !ok {
		return memoryLLMExtractTarget{}, false
	}
	for _, model := range modelCandidates {
		modelID, ok := resolveMemoryModelID(engineConfig, model)
		if !ok {
			continue
		}
		return memoryLLMExtractTarget{
			Engine:  engine,
			Model:   model,
			Effort:  effort,
			ModelID: modelID,
		}, true
	}
	return memoryLLMExtractTarget{}, false
}

func resolveMemoryModelID(engine goalx.EngineConfig, model string) (string, bool) {
	model = strings.TrimSpace(model)
	if model == "" {
		return "", false
	}
	if engine.Models == nil {
		return model, true
	}
	modelID, ok := engine.Models[model]
	if !ok || strings.TrimSpace(modelID) == "" {
		return "", false
	}
	return strings.TrimSpace(modelID), true
}

func buildMemoryLLMExtractRequest(runDir, projectRoot string, target memoryLLMExtractTarget, query MemoryQuery, seeds []MemorySeed) (*memoryLLMExtractRequest, error) {
	evidenceMap := buildMemoryLLMEvidenceMap(runDir, seeds)
	if len(evidenceMap) == 0 {
		return nil, nil
	}
	bundle, sourceRuns, observedAt, err := buildMemoryLLMExtractBundle(runDir, query, seeds, evidenceMap)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(bundle) == "" {
		return nil, nil
	}
	return &memoryLLMExtractRequest{
		RunDir:      runDir,
		ProjectRoot: projectRoot,
		Target:      target,
		Bundle:      bundle,
		Schema:      memoryLLMExtractSchema(),
		Timeout:     memoryLLMExtractTimeout,
		EvidenceMap: evidenceMap,
		Selectors:   buildMemoryLLMSelectors(query, seeds),
		SourceRuns:  sourceRuns,
		ObservedAt:  observedAt,
	}, nil
}

func buildMemoryLLMEvidenceMap(runDir string, seeds []MemorySeed) map[string]MemoryEvidence {
	evidenceMap := map[string]MemoryEvidence{}
	add := func(item MemoryEvidence) {
		path := strings.TrimSpace(item.Path)
		if path == "" {
			return
		}
		item.Kind = strings.TrimSpace(item.Kind)
		item.Path = path
		evidenceMap[path] = item
	}
	if fileExists(SummaryPath(runDir)) {
		add(MemoryEvidence{Kind: "summary", Path: SummaryPath(runDir)})
	}
	for _, path := range collectReportEvidencePaths(runDir) {
		add(MemoryEvidence{Kind: "report", Path: path})
	}
	for _, seed := range seeds {
		for _, item := range seed.Evidence {
			add(item)
		}
	}
	return evidenceMap
}

func collectReportEvidencePaths(runDir string) []string {
	entries, err := os.ReadDir(ReportsDir(runDir))
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		paths = append(paths, filepath.Join(ReportsDir(runDir), entry.Name()))
	}
	sort.Strings(paths)
	return paths
}

func buildMemoryLLMExtractBundle(runDir string, query MemoryQuery, seeds []MemorySeed, evidenceMap map[string]MemoryEvidence) (string, []string, string, error) {
	var sections []string
	selectors := buildMemoryLLMSelectors(query, seeds)
	if len(selectors) > 0 {
		pairs := make([]string, 0, len(selectors))
		for key, value := range selectors {
			pairs = append(pairs, key+"="+value)
		}
		sort.Strings(pairs)
		sections = append(sections, "selectors:\n"+strings.Join(pairs, "\n"))
	}

	sourceRuns := make([]string, 0)
	observedAt := ""

	if fileExists(SummaryPath(runDir)) {
		data, err := os.ReadFile(SummaryPath(runDir))
		if err != nil {
			return "", nil, "", err
		}
		sections = append(sections, "summary:\n"+truncateForMemoryLLM(string(data), 4000))
	}
	reportPaths := collectReportEvidencePaths(runDir)
	if len(reportPaths) > 0 {
		reportParts := make([]string, 0, len(reportPaths))
		for i, path := range reportPaths {
			if i >= 2 {
				break
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return "", nil, "", err
			}
			reportParts = append(reportParts, filepath.Base(path)+":\n"+truncateForMemoryLLM(string(data), 2500))
		}
		sections = append(sections, "reports:\n"+strings.Join(reportParts, "\n\n"))
	}

	seedLines := make([]string, 0)
	for _, seed := range seeds {
		if len(seed.Evidence) == 0 {
			continue
		}
		sourceRuns = mergeStringSets(sourceRuns, compactStrings([]string{seed.Run}))
		observedAt = earliestRFC3339(observedAt, seed.CreatedAt)
		evidencePaths := make([]string, 0, len(seed.Evidence))
		for _, item := range seed.Evidence {
			if _, ok := evidenceMap[strings.TrimSpace(item.Path)]; ok {
				evidencePaths = append(evidencePaths, strings.TrimSpace(item.Path))
			}
		}
		if len(evidencePaths) == 0 {
			continue
		}
		sort.Strings(evidencePaths)
		seedSelectors := buildSortedSelectorPairs(seed.Selectors)
		line := strings.TrimSpace(seed.Kind) + " | " + strings.TrimSpace(seed.Message)
		if len(seedSelectors) > 0 {
			line += " | selectors=" + strings.Join(seedSelectors, ",")
		}
		line += " | evidence=" + strings.Join(evidencePaths, ",")
		seedLines = append(seedLines, line)
	}
	if len(seedLines) == 0 {
		return "", nil, "", nil
	}
	sections = append(sections, "grounded_seeds:\n"+strings.Join(seedLines, "\n"))

	evidencePaths := make([]string, 0, len(evidenceMap))
	for path := range evidenceMap {
		evidencePaths = append(evidencePaths, path)
	}
	sort.Strings(evidencePaths)
	sections = append(sections, "allowed_evidence_paths:\n"+strings.Join(evidencePaths, "\n"))

	return strings.Join(sections, "\n\n"), sourceRuns, observedAt, nil
}

func buildMemoryLLMSelectors(query MemoryQuery, seeds []MemorySeed) map[string]string {
	selectors := normalizeMemorySelectors(querySelectorMap(query))
	if len(seeds) == 0 {
		return selectors
	}
	common := map[string]string{}
	initialized := false
	for _, seed := range seeds {
		if len(seed.Selectors) == 0 {
			continue
		}
		normalized := normalizeMemorySelectors(seed.Selectors)
		if len(normalized) == 0 {
			continue
		}
		if !initialized {
			for key, value := range normalized {
				common[key] = value
			}
			initialized = true
			continue
		}
		for key, value := range common {
			if normalized[key] != value {
				delete(common, key)
			}
		}
	}
	if selectors == nil {
		selectors = map[string]string{}
	}
	for key, value := range common {
		if _, ok := selectors[key]; !ok {
			selectors[key] = value
		}
	}
	if len(selectors) == 0 {
		return nil
	}
	return selectors
}

func buildSortedSelectorPairs(selectors map[string]string) []string {
	pairs := make([]string, 0, len(selectors))
	for key, value := range normalizeMemorySelectors(selectors) {
		pairs = append(pairs, key+"="+value)
	}
	sort.Strings(pairs)
	return pairs
}

func truncateForMemoryLLM(text string, limit int) string {
	text = strings.TrimSpace(text)
	if limit <= 0 || len(text) <= limit {
		return text
	}
	return strings.TrimSpace(text[:limit]) + "\n...[truncated]"
}

func llmResponseToMemoryProposals(req memoryLLMExtractRequest, resp memoryLLMExtractResponse) []MemoryProposal {
	limit := memoryLLMProposalLimit
	if len(resp.Proposals) < limit {
		limit = len(resp.Proposals)
	}
	out := make([]MemoryProposal, 0, limit)
	for i := 0; i < limit; i++ {
		item := resp.Proposals[i]
		statement := strings.TrimSpace(item.Statement)
		if statement == "" {
			continue
		}
		if item.Kind != MemoryKindProcedure && item.Kind != MemoryKindPitfall {
			continue
		}
		evidence := make([]MemoryEvidence, 0, len(item.EvidencePaths))
		for _, path := range item.EvidencePaths {
			if known, ok := req.EvidenceMap[strings.TrimSpace(path)]; ok {
				evidence = append(evidence, known)
			}
		}
		evidence = normalizeMemoryEvidence(evidence)
		if len(evidence) == 0 {
			continue
		}
		proposal := MemoryProposal{
			State:      "proposed",
			Kind:       item.Kind,
			Statement:  statement,
			Selectors:  cloneStringMap(req.Selectors),
			Evidence:   evidence,
			SourceRuns: append([]string(nil), req.SourceRuns...),
			ValidFrom:  req.ObservedAt,
			CreatedAt:  firstNonEmpty(req.ObservedAt, time.Now().UTC().Format(time.RFC3339)),
			UpdatedAt:  firstNonEmpty(req.ObservedAt, time.Now().UTC().Format(time.RFC3339)),
		}
		proposal.ID = stableMemoryProposalID(proposal.Kind, proposal.Selectors, proposal.Statement)
		normalized, err := NormalizeMemoryProposal(&proposal)
		if err != nil {
			continue
		}
		out = append(out, *normalized)
	}
	return out
}

func memoryLLMExtractSchema() string {
	return `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "proposals": {
      "type": "array",
      "maxItems": 2,
      "items": {
        "type": "object",
        "additionalProperties": false,
        "properties": {
          "kind": { "type": "string", "enum": ["procedure", "pitfall"] },
          "statement": { "type": "string", "minLength": 1 },
          "evidence_paths": {
            "type": "array",
            "minItems": 1,
            "maxItems": 4,
            "items": { "type": "string", "minLength": 1 }
          }
        },
        "required": ["kind", "statement", "evidence_paths"]
      }
    }
  },
  "required": ["proposals"]
}`
}

func defaultRunMemoryLLMExtract(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
	switch req.Target.Engine {
	case "codex":
		return runCodexMemoryLLMExtract(req)
	case "claude-code":
		return runClaudeMemoryLLMExtract(req)
	default:
		return memoryLLMExtractResponse{}, fmt.Errorf("unsupported memory llm extractor %q", req.Target.Engine)
	}
}

func runCodexMemoryLLMExtract(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()

	schemaFile, err := os.CreateTemp("", "goalx-memory-schema-*.json")
	if err != nil {
		return memoryLLMExtractResponse{}, err
	}
	schemaPath := schemaFile.Name()
	if _, err := schemaFile.WriteString(req.Schema); err != nil {
		_ = schemaFile.Close()
		return memoryLLMExtractResponse{}, err
	}
	if err := schemaFile.Close(); err != nil {
		return memoryLLMExtractResponse{}, err
	}
	defer os.Remove(schemaPath)

	outputFile, err := os.CreateTemp("", "goalx-memory-output-*.json")
	if err != nil {
		return memoryLLMExtractResponse{}, err
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return memoryLLMExtractResponse{}, err
	}
	defer os.Remove(outputPath)

	args := []string{
		"exec",
		"--cd", req.ProjectRoot,
		"--skip-git-repo-check",
		"--sandbox", "read-only",
		"-a", "never",
		"--ephemeral",
		"--output-schema", schemaPath,
		"-o", outputPath,
		"-m", req.Target.ModelID,
	}
	if effortArg := codexMemoryEffortArg(req.Target.Effort); effortArg != "" {
		args = append(args, "-c", effortArg)
	}
	args = append(args, "-")

	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = req.ProjectRoot
	cmd.Stdin = strings.NewReader(buildMemoryLLMPrompt(req.Bundle))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return memoryLLMExtractResponse{}, fmt.Errorf("codex exec: %w: %s", err, strings.TrimSpace(string(output)))
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		return memoryLLMExtractResponse{}, err
	}
	return parseMemoryLLMExtractResponse(data)
}

func codexMemoryEffortArg(effort goalx.EffortLevel) string {
	switch effort {
	case goalx.EffortMinimal, goalx.EffortLow:
		return `model_reasoning_effort="low"`
	case goalx.EffortMedium:
		return `model_reasoning_effort="medium"`
	case goalx.EffortHigh:
		return `model_reasoning_effort="high"`
	case goalx.EffortMax:
		return `model_reasoning_effort="xhigh"`
	default:
		return ""
	}
}

func runClaudeMemoryLLMExtract(req memoryLLMExtractRequest) (memoryLLMExtractResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()

	args := []string{
		"-p",
		"--bare",
		"--tools", "",
		"--model", req.Target.ModelID,
		"--json-schema", req.Schema,
		buildMemoryLLMPrompt(req.Bundle),
	}
	if effort := claudeMemoryEffort(req.Target.Effort); effort != "" {
		args = append(args[:len(args)-1], "--effort", effort, args[len(args)-1])
	}

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = req.ProjectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return memoryLLMExtractResponse{}, fmt.Errorf("claude print: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseMemoryLLMExtractResponse(output)
}

func claudeMemoryEffort(effort goalx.EffortLevel) string {
	switch effort {
	case goalx.EffortMinimal, goalx.EffortLow:
		return "low"
	case goalx.EffortMedium:
		return "medium"
	case goalx.EffortHigh, goalx.EffortMax:
		return "high"
	default:
		return ""
	}
}

func buildMemoryLLMPrompt(bundle string) string {
	return strings.TrimSpace(`You extract at most 2 bounded operational lessons from grounded run artifacts.

Rules:
- Output valid JSON only.
- Return only proposals of kind "procedure" or "pitfall".
- Do not restate facts already obvious from deterministic key=value extraction.
- Use only evidence paths listed in allowed_evidence_paths.
- Every proposal must cite at least one evidence path.
- Prefer short, concrete operational lessons.
- If there is no good bounded lesson, return {"proposals":[]}.

Grounded bundle:
` + bundle)
}

func parseMemoryLLMExtractResponse(data []byte) (memoryLLMExtractResponse, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return memoryLLMExtractResponse{}, nil
	}
	var resp memoryLLMExtractResponse
	if err := json.Unmarshal([]byte(trimmed), &resp); err == nil {
		return resp, nil
	}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err != nil {
		return memoryLLMExtractResponse{}, err
	}
	for _, key := range []string{"result", "response", "content", "text"} {
		raw, ok := wrapper[key]
		if !ok {
			continue
		}
		var nested memoryLLMExtractResponse
		if err := json.Unmarshal(raw, &nested); err == nil {
			return nested, nil
		}
		var text string
		if err := json.Unmarshal(raw, &text); err == nil {
			if err := json.Unmarshal([]byte(strings.TrimSpace(text)), &nested); err == nil {
				return nested, nil
			}
		}
	}
	return memoryLLMExtractResponse{}, fmt.Errorf("unrecognized memory llm response")
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
