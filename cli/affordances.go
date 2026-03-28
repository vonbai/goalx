package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type AffordancesDocument struct {
	Version    int              `json:"version"`
	CheckedAt  string           `json:"checked_at,omitempty"`
	RunName    string           `json:"run_name,omitempty"`
	Target     string           `json:"target,omitempty"`
	RunDir     string           `json:"run_dir,omitempty"`
	ControlDir string           `json:"control_dir,omitempty"`
	Items      []AffordanceItem `json:"items"`
}

type AffordanceItem struct {
	ID      string   `json:"id,omitempty"`
	Kind    string   `json:"kind,omitempty"`
	Summary string   `json:"summary,omitempty"`
	Command string   `json:"command,omitempty"`
	Facts   []string `json:"facts,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

func AffordancesJSONPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "affordances.json")
}

func AffordancesMarkdownPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "affordances.md")
}

func LoadAffordances(path string) (*AffordancesDocument, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	doc := &AffordancesDocument{}
	if len(strings.TrimSpace(string(data))) == 0 {
		doc.Version = 1
		return doc, nil
	}
	if err := json.Unmarshal(data, doc); err != nil {
		return nil, err
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	return doc, nil
}

func SaveAffordances(runDir string, doc *AffordancesDocument) error {
	if doc == nil {
		return nil
	}
	if doc.Version == 0 {
		doc.Version = 1
	}
	if doc.CheckedAt == "" {
		doc.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := writeJSONFile(AffordancesJSONPath(runDir), doc); err != nil {
		return err
	}
	return writeFileAtomic(AffordancesMarkdownPath(runDir), []byte(RenderAffordancesMarkdown(doc)), 0o644)
}

func BuildAffordances(projectRoot, runName, runDir, target string) (*AffordancesDocument, error) {
	index, err := BuildContextIndex(projectRoot, runName, runDir)
	if err != nil {
		return nil, err
	}
	normalizedTarget := normalizedAffordanceTarget(target)
	doc := &AffordancesDocument{
		Version:    1,
		CheckedAt:  time.Now().UTC().Format(time.RFC3339),
		RunName:    runName,
		Target:     normalizedTarget,
		RunDir:     runDir,
		ControlDir: ControlDir(runDir),
	}
	doc.Items = []AffordanceItem{
		{
			ID:      "status",
			Kind:    "observe",
			Summary: "Read the current run progress and control summary.",
			Command: fmt.Sprintf("goalx status --run %s", runName),
			Paths:   []string{ActivityPath(runDir), TransportFactsPath(runDir)},
		},
		{
			ID:      "observe",
			Kind:    "observe",
			Summary: "Read the live transport capture plus current run facts.",
			Command: fmt.Sprintf("goalx observe --run %s", runName),
			Paths:   []string{ActivityPath(runDir), TransportFactsPath(runDir)},
		},
		{
			ID:      "context",
			Kind:    "context",
			Summary: "Read the structural context index for this run.",
			Command: fmt.Sprintf("goalx context --run %s", runName),
			Paths:   []string{ContextIndexPath(runDir)},
		},
		{
			ID:      "afford",
			Kind:    "context",
			Summary: "Read the GoalX command and path affordances for this run.",
			Command: buildAffordanceCommand(runName, normalizedTarget),
			Paths:   []string{AffordancesJSONPath(runDir), AffordancesMarkdownPath(runDir)},
		},
		{
			ID:      "tell",
			Kind:    "control",
			Summary: "Dispatch or redirect durable session work through the control plane.",
			Command: buildTellCommand(runName, normalizedTarget),
			Paths:   []string{ControlInboxDir(runDir)},
		},
		{
			ID:      "durable-replace",
			Kind:    "control",
			Summary: "Replace a machine-consumed structured durable surface after validating canonical JSON shape.",
			Command: fmt.Sprintf("goalx durable replace status --run %s --file /abs/path.json", runName),
			Paths:   []string{GoalPath(runDir), AcceptanceStatePath(runDir), CoordinationPath(runDir), RunStatusPath(runDir)},
		},
		{
			ID:      "durable-append",
			Kind:    "control",
			Summary: "Append a machine-consumed durable event log using the canonical JSONL envelope.",
			Command: fmt.Sprintf("goalx durable append goal-log --run %s --file /abs/path.jsonl", runName),
			Paths:   []string{GoalLogPath(runDir), ExperimentsLogPath(runDir)},
		},
		{
			ID:      "attach",
			Kind:    "control",
			Summary: "Attach to a tmux window for inspection or emergency manual intervention.",
			Command: buildAttachCommand(runName, normalizedTarget),
		},
		{
			ID:      "add-research",
			Kind:    "control",
			Summary: "Launch a research worker using the current selection policy.",
			Command: fmt.Sprintf(`goalx add --run %s --mode research --effort high --worktree "sub-goal"`, runName),
		},
		{
			ID:      "add-develop",
			Kind:    "control",
			Summary: "Launch a develop worker using the current selection policy.",
			Command: fmt.Sprintf(`goalx add --run %s --mode develop --effort medium --worktree "sub-goal"`, runName),
		},
		{
			ID:      "add-override",
			Kind:    "control",
			Summary: "Launch an explicit engine/model override worker that bypasses the current selection policy.",
			Command: fmt.Sprintf(`goalx add --run %s --mode research --engine ENGINE --model MODEL --effort LEVEL --worktree "sub-goal"`, runName),
		},
		{
			ID:      "replace",
			Kind:    "control",
			Summary: "Replace a stale or unsuitable durable worker.",
			Command: fmt.Sprintf("goalx replace --run %s session-N --mode research --effort high", runName),
		},
		{
			ID:      "keep-session",
			Kind:    "control",
			Summary: "Merge a reviewed develop session branch into the run worktree only. goalx keep only merges committed session branch history relative to that session's recorded parent/base ref; dirty uncommitted work must be committed first. If you need partial adoption or conflict resolution, inspect the session worktree and integrate manually. This does not merge into the source root yet.",
			Command: fmt.Sprintf("goalx keep --run %s session-N", runName),
		},
		{
			ID:      "integrate",
			Kind:    "control",
			Summary: "Record the lineage of a manual run-root integration after master has already merged, cherry-picked, or partially adopted work in the run-root worktree. goalx integrate requires a clean run-root worktree and does not perform the merge itself.",
			Command: fmt.Sprintf("goalx integrate --run %s --method partial_adopt --from session-1,session-2", runName),
		},
		{
			ID:      "keep-run",
			Kind:    "control",
			Summary: "Merge the run worktree into the source root when source HEAD still descends from the run base revision; skips if already integrated. This is distinct from goalx keep session-N, which only merges a committed session branch into the run worktree.",
			Command: fmt.Sprintf("goalx keep --run %s", runName),
		},
	}
	if index != nil {
		if item := buildSelectionFactsAffordance(index); item != nil {
			doc.Items = append(doc.Items, *item)
		}
		if item := buildWorktreeBoundaryAffordance(index, normalizedTarget); item != nil {
			doc.Items = append(doc.Items, *item)
		}
		if facts := providerFactsForTarget(index.ProviderFacts, normalizedTarget); len(facts) > 0 {
			doc.Items = append(doc.Items, AffordanceItem{
				ID:      "provider-facts",
				Kind:    "fact",
				Summary: providerFactsSummary(normalizedTarget, facts),
				Facts:   renderProviderFactLines(normalizedTarget, facts),
				Paths:   []string{ContextIndexPath(runDir)},
			})
		}
		doc.Items = append(doc.Items, AffordanceItem{
			ID:      "paths",
			Kind:    "path",
			Summary: "Absolute run paths for durable state and reports.",
			Command: "",
			Paths:   []string{index.RunDir, index.ControlDir, index.CharterPath, index.GoalPath, index.ExperimentsLogPath, index.IntegrationStatePath},
		})
	}
	return doc, nil
}

func buildSelectionFactsAffordance(index *ContextIndex) *AffordanceItem {
	if index == nil || index.Selection == nil {
		return nil
	}
	item := &AffordanceItem{
		ID:      "selection-facts",
		Kind:    "fact",
		Summary: "Selection candidate pools and disabled targets recorded for this run.",
	}
	if len(index.Selection.MasterCandidates) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Master candidates: `%s`.", strings.Join(index.Selection.MasterCandidates, ", ")))
	}
	if len(index.Selection.ResearchCandidates) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Research candidates: `%s`.", strings.Join(index.Selection.ResearchCandidates, ", ")))
	}
	if len(index.Selection.DevelopCandidates) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Develop candidates: `%s`.", strings.Join(index.Selection.DevelopCandidates, ", ")))
	}
	if len(index.Selection.DisabledEngines) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Disabled engines: `%s`.", strings.Join(index.Selection.DisabledEngines, ", ")))
	}
	if len(index.Selection.DisabledTargets) > 0 {
		item.Facts = append(item.Facts, fmt.Sprintf("Disabled targets: `%s`.", strings.Join(index.Selection.DisabledTargets, ", ")))
	}
	if index.SelectionSnapshotPath != "" {
		item.Paths = []string{index.SelectionSnapshotPath}
	}
	return item
}

func buildWorktreeBoundaryAffordance(index *ContextIndex, target string) *AffordanceItem {
	if index == nil {
		return nil
	}
	item := &AffordanceItem{
		ID:   "worktree-boundary",
		Kind: "fact",
	}
	if strings.TrimSpace(target) == "" || target == "master" {
		item.Summary = "Understand source-root, run-root, and session lineage boundaries before inspecting, keeping, or manually integrating work."
		if index.ProjectRoot != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Source root: `%s`.", index.ProjectRoot))
			item.Paths = append(item.Paths, index.ProjectRoot)
		}
		if index.RunWorktree != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Run-root integration boundary: `%s`.", index.RunWorktree))
			item.Paths = append(item.Paths, index.RunWorktree)
		}
		item.Facts = append(item.Facts, "`goalx keep session-N` merges committed session branch history into the run-root worktree only.")
		item.Facts = append(item.Facts, "`goalx integrate` records the current run-root result after master manually merged, cherry-picked, or partially adopted work there.")
		item.Facts = append(item.Facts, "`goalx keep --run NAME` is the separate source-root merge step.")
		for _, session := range index.Sessions {
			fact := fmt.Sprintf("%s", session.Name)
			if session.WorktreePath != "" {
				fact += fmt.Sprintf(": worktree `%s`", session.WorktreePath)
				item.Paths = append(item.Paths, session.WorktreePath)
			} else {
				fact += ": shared run-root worktree"
			}
			if session.BaseBranchSelector != "" {
				fact += fmt.Sprintf(", base selector `%s`", session.BaseBranchSelector)
			}
			if session.BaseBranch != "" {
				fact += fmt.Sprintf(", base ref `%s`", session.BaseBranch)
			}
			item.Facts = append(item.Facts, fact+".")
		}
		item.Paths = dedupeStrings(item.Paths)
		return item
	}
	for _, session := range index.Sessions {
		if session.Name != target {
			continue
		}
		item.Summary = "Understand this session's default edit boundary and recorded lineage before changing files."
		if session.WorktreePath != "" {
			item.Facts = append(item.Facts, "Default edit boundary is this session's dedicated worktree.")
			item.Paths = append(item.Paths, session.WorktreePath)
		} else {
			item.Facts = append(item.Facts, "This session shares the run-root worktree; assume overlap with other no-worktree sessions.")
		}
		if index.RunWorktree != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Run-root integration boundary: `%s`.", index.RunWorktree))
			item.Paths = append(item.Paths, index.RunWorktree)
		}
		if index.ProjectRoot != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Source root: `%s`.", index.ProjectRoot))
			item.Paths = append(item.Paths, index.ProjectRoot)
		}
		if session.WorktreePath != "" {
			item.Facts = append(item.Facts, "Do not edit the source root or run-root worktree from a dedicated session unless master explicitly redirects you to inspect or integrate there.")
		}
		if session.BaseBranchSelector != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Recorded parent/base selector: `%s`.", session.BaseBranchSelector))
		}
		if session.BaseBranch != "" {
			item.Facts = append(item.Facts, fmt.Sprintf("Recorded parent/base ref: `%s`.", session.BaseBranch))
		}
		if session.WorktreePath != "" {
			item.Facts = append(item.Facts, "If you discover accidental edits outside your assigned worktree, stop, record the boundary violation, and migrate or revert those edits before continuing.")
		}
		item.Paths = dedupeStrings(item.Paths)
		return item
	}
	return nil
}

func RenderAffordancesMarkdown(doc *AffordancesDocument) string {
	if doc == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# GoalX Affordances\n\n")
	b.WriteString(fmt.Sprintf("- Run: `%s`\n", doc.RunName))
	if doc.Target != "" {
		b.WriteString(fmt.Sprintf("- Target: `%s`\n", doc.Target))
	}
	if doc.RunDir != "" {
		b.WriteString(fmt.Sprintf("- Run dir: `%s`\n", doc.RunDir))
	}
	if doc.ControlDir != "" {
		b.WriteString(fmt.Sprintf("- Control dir: `%s`\n", doc.ControlDir))
	}
	b.WriteString("\n")
	for _, item := range doc.Items {
		b.WriteString(fmt.Sprintf("## %s\n\n", item.ID))
		if item.Summary != "" {
			b.WriteString(item.Summary + "\n\n")
		}
		if item.Command != "" {
			b.WriteString("```bash\n" + item.Command + "\n```\n\n")
		}
		for _, fact := range item.Facts {
			b.WriteString("- " + fact + "\n")
		}
		if len(item.Facts) > 0 {
			b.WriteString("\n")
		}
		for _, path := range item.Paths {
			b.WriteString("- `" + path + "`\n")
		}
		if len(item.Paths) > 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func RefreshRunGuidance(projectRoot, runName, runDir string) error {
	if err := RefreshSessionRuntimeProjection(runDir, runName); err != nil {
		return err
	}
	if err := RefreshWorktreeSnapshot(runDir); err != nil {
		return err
	}
	if err := RefreshRunMemoryContext(runDir); err != nil {
		return err
	}
	activity, err := BuildActivitySnapshot(projectRoot, runName, runDir)
	if err != nil {
		return err
	}
	if err := SaveActivitySnapshot(runDir, activity); err != nil {
		return err
	}
	index, err := BuildContextIndex(projectRoot, runName, runDir)
	if err != nil {
		return err
	}
	if err := SaveContextIndex(runDir, index); err != nil {
		return err
	}
	affordances, err := BuildAffordances(projectRoot, runName, runDir, "")
	if err != nil {
		return err
	}
	return SaveAffordances(runDir, affordances)
}

func buildAffordanceCommand(runName, target string) string {
	args := []string{"goalx afford", "--run", runName}
	if normalized := normalizedAffordanceTarget(target); normalized != "" {
		args = append(args, normalized)
	}
	return strings.Join(args, " ")
}

func normalizedAffordanceTarget(target string) string {
	if strings.TrimSpace(target) == "" {
		return ""
	}
	return strings.TrimSpace(target)
}

func buildTellCommand(runName, target string) string {
	args := []string{"goalx", "tell", "--run", runName}
	if normalized := normalizedAffordanceTarget(target); normalized != "" {
		args = append(args, normalized)
	} else {
		args = append(args, "session-N")
	}
	args = append(args, "\"message\"")
	return strings.Join(args, " ")
}

func buildAttachCommand(runName, target string) string {
	args := []string{"goalx", "attach", "--run", runName}
	if normalized := normalizedAffordanceTarget(target); normalized != "" {
		args = append(args, normalized)
	} else {
		args = append(args, "session-N")
	}
	return strings.Join(args, " ")
}

func providerFactsForTarget(facts []ProviderFact, target string) []ProviderFact {
	if len(facts) == 0 {
		return nil
	}
	if strings.TrimSpace(target) == "" {
		return facts
	}
	filtered := make([]ProviderFact, 0, len(facts))
	for _, fact := range facts {
		if fact.Target == target {
			filtered = append(filtered, fact)
		}
	}
	return filtered
}

func providerFactsSummary(target string, facts []ProviderFact) string {
	if len(facts) == 0 {
		return ""
	}
	if strings.TrimSpace(target) == "" {
		return "Provider-native capability facts for this run."
	}
	engine := facts[0].Engine
	if strings.TrimSpace(engine) == "" {
		return fmt.Sprintf("Provider-native capability facts for `%s`.", target)
	}
	return fmt.Sprintf("Provider-native capability facts for `%s` (`%s`).", target, engine)
}

func renderProviderFactLines(target string, facts []ProviderFact) []string {
	lines := make([]string, 0, len(facts))
	for _, fact := range facts {
		line := fact.Fact
		if strings.TrimSpace(target) == "" {
			prefix := fact.Target
			if strings.TrimSpace(prefix) == "" {
				prefix = "run"
			}
			if fact.Engine != "" {
				prefix += " (" + fact.Engine + ")"
			}
			line = prefix + ": " + line
		}
		lines = append(lines, line)
	}
	return lines
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
