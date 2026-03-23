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

	quotedGuidancePath := shellQuote(guidancePath)
	quotedGuidanceMessage := shellQuote("GUIDANCE PENDING: read " + guidancePath + " and follow it now")
	stopCmd := fmt.Sprintf(
		`if cat %s 2>/dev/null | grep -q .; then printf '%%s\n' %s >&2; exit 2; fi`,
		quotedGuidancePath, quotedGuidanceMessage,
	)
	return appendClaudeHook(worktreePath, map[string]string{
		"event":   "Stop",
		"command": stopCmd,
	})
}

// GenerateMasterAdapter configures a project-level Stop hook for the master
// agent so it does not exit before the run reaches a verified completion state.
func GenerateMasterAdapter(engine, projectRoot, runStatePath, proofPath string) error {
	if engine != "claude-code" {
		return nil
	}

	quotedRunStatePath := shellQuote(runStatePath)
	quotedProofPath := shellQuote(proofPath)
	quotedRunStateMessage := shellQuote("RUN INCOMPLETE: keep waiting until " + runStatePath + ` reports "phase":"complete"`)
	quotedVerifyMessage := shellQuote("RUN NOT VERIFIED: done/implement require acceptance_status=passed in " + proofPath)
	quotedContractMessage := shellQuote("RUN CONTRACT INCOMPLETE: done/implement require goal_contract_status=satisfied and goal_required_remaining=0 in " + proofPath)
	quotedCompletionMessage := shellQuote("RUN COMPLETION PROVENANCE MISSING: done/implement require completion_mode and code_changed in " + proofPath)
	stopCmd := fmt.Sprintf(
		`if [ ! -f %s ] || ! grep -Eq '"phase"[[:space:]]*:[[:space:]]*"complete"' %s; then printf '%%s\n' %s >&2; exit 2; fi; if grep -Eq '"recommendation"[[:space:]]*:[[:space:]]*"(done|implement)"' %s && { [ ! -f %s ] || ! grep -Eq '"acceptance_status"[[:space:]]*:[[:space:]]*"passed"' %s; }; then printf '%%s\n' %s >&2; exit 2; fi; if grep -Eq '"recommendation"[[:space:]]*:[[:space:]]*"(done|implement)"' %s && { [ ! -f %s ] || ! grep -Eq '"goal_contract_status"[[:space:]]*:[[:space:]]*"satisfied"' %s || ! grep -Eq '"goal_required_remaining"[[:space:]]*:[[:space:]]*0([[:space:]]*[,}])' %s; }; then printf '%%s\n' %s >&2; exit 2; fi; if grep -Eq '"recommendation"[[:space:]]*:[[:space:]]*"(done|implement)"' %s && { [ ! -f %s ] || ! grep -Eq '"completion_mode"[[:space:]]*:[[:space:]]*"(verification_only|implementation_and_verification)"' %s || ! grep -Eq '"code_changed"[[:space:]]*:[[:space:]]*(true|false)([[:space:]]*[,}])' %s; }; then printf '%%s\n' %s >&2; exit 2; fi`,
		quotedRunStatePath, quotedRunStatePath, quotedRunStateMessage, quotedRunStatePath, quotedProofPath, quotedProofPath, quotedVerifyMessage, quotedRunStatePath, quotedProofPath, quotedProofPath, quotedProofPath, quotedContractMessage, quotedRunStatePath, quotedProofPath, quotedProofPath, quotedProofPath, quotedCompletionMessage,
	)
	return appendClaudeHook(projectRoot, map[string]string{
		"event":   "Stop",
		"command": stopCmd,
	})
}

func appendClaudeHook(root string, hook map[string]string) error {
	claudeDir := filepath.Join(root, ".claude")
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
	for _, existing := range hooks {
		if existing["event"] == hook["event"] && existing["command"] == hook["command"] {
			return markHookFileAssumeUnchanged(root)
		}
	}
	hooks = append(hooks, hook)

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
	return markHookFileAssumeUnchanged(root)
}

func markHookFileAssumeUnchanged(root string) error {
	// Only assume-unchanged if the file is already tracked by git.
	// If it's new (not in index), it won't be committed unless explicitly added.
	if err := exec.Command("git", "-C", root, "ls-files", "--error-unmatch", ".claude/hooks.json").Run(); err == nil {
		// File is tracked — mark assume-unchanged so our edits don't show in diffs
		return exec.Command("git", "-C", root, "update-index", "--assume-unchanged", ".claude/hooks.json").Run()
	}
	// File not tracked — nothing to do, it's already invisible to git
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
