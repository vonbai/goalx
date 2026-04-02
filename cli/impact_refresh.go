package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func RefreshImpactState(runDir, scopeName string) error {
	worktreePath, baselineRevision, err := resolveImpactScope(runDir, scopeName)
	if err != nil {
		return err
	}
	headRevision := ""
	if rev, err := gitRevisionIfAvailable(worktreePath, "HEAD"); err == nil {
		headRevision = rev
	}
	state := &ImpactState{
		Version:          1,
		Scope:            strings.TrimSpace(scopeName),
		BaselineRevision: baselineRevision,
		HeadRevision:     headRevision,
		ResolverKind:     "none",
	}
	if _, err := lookPathFunc("git"); err == nil && strings.TrimSpace(worktreePath) != "" {
		state.ResolverKind = "repo-native"
		files, err := gitChangedFiles(worktreePath, baselineRevision, headRevision)
		if err != nil {
			return err
		}
		state.ChangedFiles = files
	}
	return SaveImpactState(ImpactStatePath(runDir), state)
}

func resolveImpactScope(runDir, scopeName string) (worktreePath, baselineRevision string, err error) {
	scopeName = strings.TrimSpace(scopeName)
	if scopeName == "" || scopeName == "run-root" {
		meta, err := LoadRunMetadata(RunMetadataPath(runDir))
		if err != nil {
			return "", "", err
		}
		worktreePath = RunWorktreePath(runDir)
		if info, statErr := os.Stat(worktreePath); statErr != nil || !info.IsDir() {
			if meta != nil {
				worktreePath = meta.ProjectRoot
			}
		}
		if meta != nil {
			baselineRevision = strings.TrimSpace(meta.BaseRevision)
		}
		if baselineRevision == "" {
			baselineRevision, _ = gitRevisionIfAvailable(worktreePath, "HEAD")
		}
		return worktreePath, baselineRevision, nil
	}
	if strings.HasPrefix(scopeName, "session-") {
		sessionsState, err := EnsureSessionsRuntimeState(runDir)
		if err != nil {
			return "", "", err
		}
		cfg, err := LoadRunSpec(runDir)
		if err != nil {
			return "", "", err
		}
		worktreePath = resolvedSessionWorktreePath(runDir, cfg.Name, scopeName, sessionsState)
		if strings.TrimSpace(worktreePath) == "" {
			worktreePath = RunWorktreePath(runDir)
		}
		identity, err := LoadSessionIdentity(SessionIdentityPath(runDir, scopeName))
		if err == nil && identity != nil && strings.TrimSpace(identity.BaseBranch) != "" {
			baselineRevision, _ = gitRevisionIfAvailable(worktreePath, identity.BaseBranch)
		}
		if baselineRevision == "" {
			baselineRevision, _ = gitRevisionIfAvailable(worktreePath, "HEAD")
		}
		return worktreePath, baselineRevision, nil
	}
	return "", "", fmt.Errorf("unknown impact scope %q", scopeName)
}

func gitChangedFiles(worktreePath, baselineRevision, headRevision string) ([]string, error) {
	seen := map[string]struct{}{}
	files := []string{}
	if strings.TrimSpace(worktreePath) == "" {
		return nil, nil
	}
	if strings.TrimSpace(baselineRevision) != "" && strings.TrimSpace(headRevision) != "" && baselineRevision != headRevision {
		cmd := exec.Command("git", "-C", worktreePath, "diff", "--name-only", baselineRevision+".."+headRevision)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("git diff --name-only in %s: %w: %s", worktreePath, err, out)
		}
		for _, file := range splitLinesToStrings(out) {
			if _, ok := seen[file]; ok {
				continue
			}
			seen[file] = struct{}{}
			files = append(files, file)
		}
	}
	statusCmd := exec.Command("git", "-C", worktreePath, "status", "--porcelain=v1", "-uall")
	statusOut, err := statusCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status --porcelain=v1 -uall in %s: %w: %s", worktreePath, err, statusOut)
	}
	for _, line := range splitRawLines(statusOut) {
		if len(line) < 4 {
			continue
		}
		file := strings.TrimSpace(line[3:])
		if file == "" {
			continue
		}
		if _, ok := seen[file]; ok {
			continue
		}
		seen[file] = struct{}{}
		files = append(files, file)
	}
	return files, nil
}

func splitLinesToStrings(data []byte) []string {
	lines := bytes.Split(bytes.TrimSpace(data), []byte{'\n'})
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		text := strings.TrimSpace(string(line))
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

func splitRawLines(data []byte) []string {
	lines := bytes.Split(bytes.TrimRight(data, "\n"), []byte{'\n'})
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		text := string(line)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}
