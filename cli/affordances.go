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
	return os.WriteFile(AffordancesMarkdownPath(runDir), []byte(RenderAffordancesMarkdown(doc)), 0o644)
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
			ID:      "attach",
			Kind:    "control",
			Summary: "Attach to a tmux window for inspection or emergency manual intervention.",
			Command: buildAttachCommand(runName, normalizedTarget),
		},
		{
			ID:      "add-research",
			Kind:    "control",
			Summary: "Launch a route-first research worker.",
			Command: fmt.Sprintf(`goalx add --run %s --mode research --effort high --worktree "sub-goal"`, runName),
		},
		{
			ID:      "add-develop",
			Kind:    "control",
			Summary: "Launch a route-first develop worker.",
			Command: fmt.Sprintf(`goalx add --run %s --mode develop --effort medium --worktree "sub-goal"`, runName),
		},
		{
			ID:      "add-override",
			Kind:    "control",
			Summary: "Launch an explicit engine/model override worker.",
			Command: fmt.Sprintf(`goalx add --run %s --mode research --engine ENGINE --model MODEL --effort LEVEL --worktree "sub-goal"`, runName),
		},
		{
			ID:      "replace",
			Kind:    "control",
			Summary: "Replace a stale or unsuitable durable worker.",
			Command: fmt.Sprintf("goalx replace --run %s session-N --mode research --effort high", runName),
		},
	}
	if index != nil {
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
			Paths:   []string{index.RunDir, index.ControlDir, index.CharterPath, index.GoalPath},
		})
	}
	return doc, nil
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
