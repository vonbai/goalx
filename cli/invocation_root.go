package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CanonicalProjectRoot maps a cwd back to the source project root. GoalX run
// worktree paths resolve through durable metadata; ordinary project subdirs
// resolve through explicit project markers or the enclosing git toplevel.
func CanonicalProjectRoot(projectRoot string) string {
	abs, err := filepath.Abs(strings.TrimSpace(projectRoot))
	if err != nil || abs == "" {
		return projectRoot
	}
	runDir, ok := enclosingRunDirFromWorktree(abs)
	if ok {
		if meta, err := LoadRunMetadata(RunMetadataPath(runDir)); err == nil && meta != nil && strings.TrimSpace(meta.ProjectRoot) != "" {
			return filepath.Clean(meta.ProjectRoot)
		}
		if identity, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir)); err == nil && identity != nil && strings.TrimSpace(identity.ProjectRoot) != "" {
			return filepath.Clean(identity.ProjectRoot)
		}
		return abs
	}
	if runDir, ok := resolveRunDirFromGlobalRegistry(abs); ok {
		if meta, err := LoadRunMetadata(RunMetadataPath(runDir)); err == nil && meta != nil && strings.TrimSpace(meta.ProjectRoot) != "" {
			return filepath.Clean(meta.ProjectRoot)
		}
		if identity, err := LoadControlRunIdentity(ControlRunIdentityPath(runDir)); err == nil && identity != nil && strings.TrimSpace(identity.ProjectRoot) != "" {
			return filepath.Clean(identity.ProjectRoot)
		}
	}

	goalxRoot := enclosingConfiguredProjectRoot(abs)
	gitCommonRoot := gitCommonProjectRoot(abs)
	gitRoot := gitTopLevelProjectRoot(abs)
	switch {
	case goalxRoot != "":
		return goalxRoot
	case gitCommonRoot != "":
		return gitCommonRoot
	case gitRoot != "":
		return gitRoot
	case goalxRoot != "":
		return goalxRoot
	default:
		return abs
	}
}

func resolveRunDirFromGlobalRegistry(path string) (string, bool) {
	reg, err := LoadGlobalRunRegistry()
	if err != nil || reg == nil {
		return "", false
	}
	for _, ref := range reg.Runs {
		runDir := strings.TrimSpace(ref.RunDir)
		if runDir == "" {
			continue
		}
		if pathWithinRunWorktree(path, runDir) {
			return runDir, true
		}
	}
	return "", false
}

func pathWithinRunWorktree(path, runDir string) bool {
	if pathHasPrefix(path, RunWorktreePath(runDir)) {
		return true
	}
	sessionsState, err := LoadSessionsRuntimeState(SessionsRuntimeStatePath(runDir))
	if err != nil || sessionsState == nil {
		return false
	}
	for _, session := range sessionsState.Sessions {
		if strings.TrimSpace(session.WorktreePath) == "" {
			continue
		}
		if pathHasPrefix(path, session.WorktreePath) {
			return true
		}
	}
	return false
}

func enclosingRunDirFromWorktree(path string) (string, bool) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		parent := filepath.Dir(current)
		if filepath.Base(parent) == "worktrees" {
			runDir := filepath.Dir(parent)
			if strings.TrimSpace(runDir) != "" {
				if _, err := os.Stat(RunMetadataPath(runDir)); err == nil {
					return runDir, true
				}
				if _, err := os.Stat(RunSpecPath(runDir)); err == nil {
					return runDir, true
				}
			}
			return "", false
		}
		next := filepath.Dir(current)
		if next == current {
			return "", false
		}
	}
}

func enclosingConfiguredProjectRoot(path string) string {
	home, _ := os.UserHomeDir()
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		if current != "" && current != home && hasProjectGoalxMarker(current) {
			return current
		}
		next := filepath.Dir(current)
		if next == current {
			return ""
		}
	}
}

func hasProjectGoalxMarker(projectRoot string) bool {
	for _, rel := range []string{
		filepath.Join(".goalx", "config.yaml"),
		filepath.Join(".goalx", "goalx.yaml"),
		filepath.Join(".goalx", "data", "registry.json"),
	} {
		if info, err := os.Stat(filepath.Join(projectRoot, rel)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func gitTopLevelProjectRoot(path string) string {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return ""
	}
	root := filepath.Clean(strings.TrimSpace(string(out)))
	if root == "" {
		return ""
	}
	return root
}

func gitCommonProjectRoot(path string) string {
	out, err := exec.Command("git", "-C", path, "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return ""
	}
	gitDir := filepath.Clean(strings.TrimSpace(string(out)))
	if gitDir == "" {
		return ""
	}
	if filepath.Base(gitDir) != ".git" {
		return ""
	}
	return filepath.Dir(gitDir)
}
