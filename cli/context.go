package cli

import (
	"encoding/json"
	"fmt"
	"strings"
)

const contextUsage = "usage: goalx context [--run NAME] [--json]"

func Context(projectRoot string, args []string) error {
	if printUsageIfHelp(args, contextUsage) {
		return nil
	}
	runName, jsonOut, err := parseContextArgs(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}
	if err := RefreshRunMemoryContext(rc.RunDir); err != nil {
		return err
	}

	index, err := BuildContextIndex(projectRoot, rc.Name, rc.RunDir)
	if err != nil {
		return err
	}
	if jsonOut {
		data, err := json.MarshalIndent(index, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Print(renderContextIndex(index))
	return nil
}

func parseContextArgs(args []string) (runName string, jsonOut bool, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", false, err
	}
	positional := make([]string, 0, len(rest))
	for _, arg := range rest {
		switch arg {
		case "--json":
			jsonOut = true
		case "--help", "-h", "help":
			return "", false, fmt.Errorf(contextUsage)
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) > 1 {
		return "", false, fmt.Errorf(contextUsage)
	}
	if len(positional) == 1 {
		if runName != "" {
			return "", false, fmt.Errorf(contextUsage)
		}
		runName = positional[0]
	}
	return runName, jsonOut, nil
}

func renderContextIndex(index *ContextIndex) string {
	if index == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("# GoalX Context\n\n")
	writeContextLine := func(label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		b.WriteString(fmt.Sprintf("- %s: `%s`\n", label, value))
	}
	writeContextLine("Project root", index.ProjectRoot)
	writeContextLine("Run dir", index.RunDir)
	writeContextLine("Run worktree", index.RunWorktree)
	writeContextLine("Control dir", index.ControlDir)
	writeContextLine("Reports dir", index.ReportsDir)
	writeContextLine("Charter", index.CharterPath)
	writeContextLine("Goal", index.GoalPath)
	writeContextLine("Experiment ledger", index.ExperimentsLogPath)
	writeContextLine("Integration state", index.IntegrationStatePath)
	writeContextLine("Acceptance", index.AcceptanceStatePath)
	writeContextLine("Closeout/evidence surface", index.CompletionProofPath)
	writeContextLine("Coordination", index.CoordinationPath)
	writeContextLine("Result", index.SummaryPath)
	writeContextLine("Activity", index.ActivityPath)
	writeContextLine("Worktree snapshot", index.WorktreeSnapshotPath)
	writeContextLine("Selection snapshot", index.SelectionSnapshotPath)
	writeContextLine("Memory query", index.MemoryQueryPath)
	writeContextLine("Memory context", index.MemoryContextPath)
	writeContextLine("Context index", index.ContextIndexPath)
	writeContextLine("Affordances", index.AffordancesMarkdown)
	if identity := index.RunIdentity; identity.RunID != "" || identity.Objective != "" {
		b.WriteString("\n## Run Identity\n\n")
		writeContextLine("Objective", identity.Objective)
		writeContextLine("Run ID", identity.RunID)
		writeContextLine("Root run ID", identity.RootRunID)
		writeContextLine("Charter ID", identity.CharterID)
		writeContextLine("Intent", identity.Intent)
		writeContextLine("Mode", identity.Mode)
		writeContextLine("Phase kind", identity.PhaseKind)
		if identity.RoleContracts.Master != nil {
			writeContextLine("Master role", identity.RoleContracts.Master.Mandate)
		}
		if identity.RoleContracts.ResearchSubagent != nil {
			writeContextLine("Research role", identity.RoleContracts.ResearchSubagent.Mandate)
		}
		if identity.RoleContracts.DevelopSubagent != nil {
			writeContextLine("Develop role", identity.RoleContracts.DevelopSubagent.Mandate)
		}
	}
	if len(index.ProviderFacts) > 0 {
		b.WriteString("\n## Provider Facts\n\n")
		for _, fact := range index.ProviderFacts {
			target := fact.Target
			if strings.TrimSpace(target) == "" {
				target = "run"
			}
			label := target
			if fact.Engine != "" {
				label += " (" + fact.Engine + ")"
			}
			b.WriteString(fmt.Sprintf("- %s: %s\n", label, fact.Fact))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n## Master\n\n")
	writeContextLine("Engine", index.Master.Engine)
	writeContextLine("Model", index.Master.Model)
	writeContextLine("Mode", index.Master.Mode)
	if index.Selection != nil {
		b.WriteString("\n## Selection\n\n")
		if len(index.Selection.MasterCandidates) > 0 {
			writeContextLine("Master candidates", strings.Join(index.Selection.MasterCandidates, ", "))
		}
		if len(index.Selection.ResearchCandidates) > 0 {
			writeContextLine("Research candidates", strings.Join(index.Selection.ResearchCandidates, ", "))
		}
		if len(index.Selection.DevelopCandidates) > 0 {
			writeContextLine("Develop candidates", strings.Join(index.Selection.DevelopCandidates, ", "))
		}
		if len(index.Selection.DisabledEngines) > 0 {
			writeContextLine("Disabled engines", strings.Join(index.Selection.DisabledEngines, ", "))
		}
		if len(index.Selection.DisabledTargets) > 0 {
			writeContextLine("Disabled targets", strings.Join(index.Selection.DisabledTargets, ", "))
		}
	}
	if index.ClaudeCodeAvailable || index.CodexAvailable || index.GitAvailable || index.TmuxAvailable {
		b.WriteString("## Capabilities\n\n")
		if index.ClaudeCodeAvailable {
			b.WriteString("- claude-code available\n")
		}
		if index.CodexAvailable {
			b.WriteString("- codex available\n")
		}
		if index.GitAvailable {
			b.WriteString("- git available\n")
		}
		if index.TmuxAvailable {
			b.WriteString("- tmux available\n")
		}
		b.WriteString("\n")
	}
	if len(index.Sessions) > 0 {
		b.WriteString("## Sessions\n\n")
		for _, sess := range index.Sessions {
			b.WriteString(fmt.Sprintf("- %s", sess.Name))
			if sess.Mode != "" {
				b.WriteString(fmt.Sprintf(" (%s)", sess.Mode))
			}
			b.WriteString("\n")
			writeIndentedContextLine(&b, "window", sess.WindowName)
			writeIndentedContextLine(&b, "worktree", sess.WorktreePath)
			writeIndentedContextLine(&b, "branch", sess.Branch)
			writeIndentedContextLine(&b, "base selector", sess.BaseBranchSelector)
			writeIndentedContextLine(&b, "base branch", sess.BaseBranch)
			writeIndentedContextLine(&b, "journal", sess.JournalPath)
			writeIndentedContextLine(&b, "inbox", sess.InboxPath)
			writeIndentedContextLine(&b, "cursor", sess.CursorPath)
		}
	}
	return b.String()
}

func writeIndentedContextLine(b *strings.Builder, label, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	b.WriteString(fmt.Sprintf("  - %s: `%s`\n", label, value))
}
