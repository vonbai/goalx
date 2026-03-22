package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func EnsureProjectGoalxIgnored(projectRoot string) error {
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
	text := string(data)
	if strings.Contains(text, ".goalx/") {
		return nil
	}
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += ".goalx/\n"
	return os.WriteFile(excludePath, []byte(text), 0o644)
}
