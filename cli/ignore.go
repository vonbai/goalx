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
	managed := []string{
		goalxExcludeBegin,
		".goalx/goalx.yaml",
		".gitnexus/",
	}
	if worktreeIgnore := worktreeIgnorePattern(projectRoot, cfg); worktreeIgnore != "" {
		managed = append(managed, worktreeIgnore)
	}
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

func worktreeIgnorePattern(projectRoot string, cfg *goalx.Config) string {
	if cfg == nil || strings.TrimSpace(cfg.WorktreeRoot) == "" {
		return ""
	}
	root := strings.TrimSpace(cfg.WorktreeRoot)
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
