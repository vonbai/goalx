package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

const (
	goalxExcludeBegin = "# goalx-managed-begin"
	goalxExcludeEnd   = "# goalx-managed-end"
)

func EnsureProjectGoalxIgnored(projectRoot string) error {
	var cfg *goalx.Config
	if layers, err := goalx.LoadConfigLayers(projectRoot); err == nil {
		cfg = &layers.Config
	}
	return EnsureProjectGoalxIgnoredWithConfig(projectRoot, cfg)
}

func EnsureProjectGoalxIgnoredWithConfig(projectRoot string, cfg *goalx.Config) error {
	gitDirOut, err := exec.Command("git", "-C", projectRoot, "rev-parse", "--git-dir").CombinedOutput()
	if err != nil {
		return nil
	}
	gitDir := strings.TrimSpace(string(gitDirOut))
	if gitDir == "" {
		return nil
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(projectRoot, gitDir)
	}

	excludePath := filepath.Join(gitDir, "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	managed := []string{goalxExcludeBegin}
	managed = append(managed, managedProjectIgnorePatterns(projectRoot, cfg)...)
	managed = append(managed, goalxExcludeEnd)
	out := make([]string, 0, len(lines)+len(managed))
	skippingManaged := false
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case goalxExcludeBegin:
			skippingManaged = true
			if !inserted {
				out = append(out, managed...)
				inserted = true
			}
			continue
		case goalxExcludeEnd:
			skippingManaged = false
			continue
		}
		if skippingManaged {
			continue
		}
		if trimmed == ".goalx/" {
			if !inserted {
				out = append(out, managed...)
				inserted = true
			}
			continue
		}
		out = append(out, line)
	}
	if !inserted {
		if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
		out = append(out, managed...)
	}
	text := strings.TrimRight(strings.Join(out, "\n"), "\n") + "\n"
	return os.WriteFile(excludePath, []byte(text), 0o644)
}

func managedProjectIgnorePatterns(projectRoot string, cfg *goalx.Config) []string {
	patterns := []string{".goalx/goalx.yaml"}
	seen := map[string]bool{
		".goalx/goalx.yaml": true,
	}
	for _, pattern := range []string{
		configuredRootIgnorePattern(projectRoot, resolveConfiguredWorktreeRoot(projectRoot, cfg.WorktreeRoot)),
		configuredRootIgnorePattern(projectRoot, goalx.ResolveRunRoot(projectRoot, cfg)),
		configuredRootIgnorePattern(projectRoot, goalx.ResolveSavedRunRoot(projectRoot, cfg)),
	} {
		if pattern == "" || seen[pattern] {
			continue
		}
		seen[pattern] = true
		patterns = append(patterns, pattern)
	}
	return patterns
}

func configuredRootIgnorePattern(projectRoot, resolvedRoot string) string {
	root := strings.TrimSpace(resolvedRoot)
	if root == "" {
		return ""
	}
	if filepath.IsAbs(root) {
		rel, err := filepath.Rel(projectRoot, root)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return ""
		}
		root = rel
	}
	root = filepath.ToSlash(filepath.Clean(root))
	if root == "." || root == "" || strings.HasPrefix(root, "../") {
		return ""
	}
	if !strings.HasSuffix(root, "/") {
		root += "/"
	}
	return root
}
