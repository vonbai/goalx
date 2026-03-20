package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateAdapter configures engine-specific adapter files in a worktree.
func GenerateAdapter(engine, worktreePath, guidancePath string) error {
	if engine != "claude-code" {
		return nil
	}

	claudeDir := filepath.Join(worktreePath, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	hooksPath := filepath.Join(claudeDir, "hooks.json")

	var doc map[string]json.RawMessage
	data, err := os.ReadFile(hooksPath)
	if err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse hooks.json: %w", err)
		}
	} else if os.IsNotExist(err) {
		doc = make(map[string]json.RawMessage)
	} else {
		return fmt.Errorf("read hooks.json: %w", err)
	}

	var hooks []map[string]string
	if raw, ok := doc["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return fmt.Errorf("parse hooks array: %w", err)
		}
	}

	quotedGuidancePath := shellQuote(guidancePath)
	stopCmd := fmt.Sprintf(
		`cat %s 2>/dev/null | grep -q . && printf '\n⚠️ MASTER GUIDANCE PENDING — read %%s and follow it NOW\n' %s || true`,
		quotedGuidancePath, quotedGuidancePath,
	)
	hooks = append(hooks, map[string]string{
		"event":   "Stop",
		"command": stopCmd,
	})

	raw, err := json.Marshal(hooks)
	if err != nil {
		return fmt.Errorf("marshal hooks: %w", err)
	}
	doc["hooks"] = raw

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal hooks.json: %w", err)
	}
	if err := os.WriteFile(hooksPath, out, 0o644); err != nil {
		return fmt.Errorf("write hooks.json: %w", err)
	}

	// Only assume-unchanged if the file is already tracked by git.
	// If it's new (not in index), it won't be committed unless explicitly added.
	if err := exec.Command("git", "-C", worktreePath, "ls-files", "--error-unmatch", ".claude/hooks.json").Run(); err == nil {
		// File is tracked — mark assume-unchanged so our edits don't show in diffs
		return exec.Command("git", "-C", worktreePath, "update-index", "--assume-unchanged", ".claude/hooks.json").Run()
	}
	// File not tracked — nothing to do, it's already invisible to git
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
