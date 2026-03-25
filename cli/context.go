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
	writeContextLine("Acceptance", index.AcceptanceStatePath)
	writeContextLine("Completion proof", index.CompletionProofPath)
	writeContextLine("Coordination", index.CoordinationPath)
	writeContextLine("Summary", index.SummaryPath)
	writeContextLine("Activity", index.ActivityPath)
	writeContextLine("Context index", index.ContextIndexPath)
	writeContextLine("Affordances", index.AffordancesMarkdown)
	b.WriteString("\n## Master\n\n")
	writeContextLine("Engine", index.Master.Engine)
	writeContextLine("Model", index.Master.Model)
	writeContextLine("Mode", index.Master.Mode)
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
