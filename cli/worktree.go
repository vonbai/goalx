package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CreateWorktree creates a new git worktree with a new branch.
// Cleans up stale worktree references and branches from failed previous runs.
func CreateWorktree(projectRoot, worktreePath, branch string) error {
	// Prune stale worktree refs, but do not delete an existing branch. Branch
	// collisions should fail fast so we do not destroy prior run history.
	exec.Command("git", "-C", projectRoot, "worktree", "prune").Run()
	cmd := exec.Command("git", "-C", projectRoot, "worktree", "add", worktreePath, "-b", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// RemoveWorktree removes a git worktree forcefully.
func RemoveWorktree(projectRoot, worktreePath string) error {
	return exec.Command("git", "-C", projectRoot, "worktree", "remove", worktreePath, "--force").Run()
}

// DeleteBranch removes a local branch after its worktree has been cleaned up.
func DeleteBranch(projectRoot, branch string) error {
	exec.Command("git", "-C", projectRoot, "worktree", "prune").Run()
	out, err := exec.Command("git", "-C", projectRoot, "branch", "-D", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, out)
	}
	return nil
}

// MergeWorktree merges a branch into the current branch of projectRoot.
func MergeWorktree(projectRoot, branch string) error {
	statusOut, err := exec.Command("git", "-C", projectRoot, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git status: %w: %s", err, statusOut)
	}
	for _, line := range strings.Split(string(statusOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path := parsePorcelainPath(line)
		if isAllowedLocalConfigPath(path) {
			continue
		}
		return fmt.Errorf("project worktree is dirty; commit or stash changes before merge")
	}

	// Pre-check for conflicts using merge-tree
	head, _ := exec.Command("git", "-C", projectRoot, "rev-parse", "HEAD").Output()
	branchRev, _ := exec.Command("git", "-C", projectRoot, "rev-parse", branch).Output()
	if len(head) > 0 && len(branchRev) > 0 {
		mtOut, mtErr := exec.Command("git", "-C", projectRoot, "merge-tree",
			strings.TrimSpace(string(head)),
			strings.TrimSpace(string(head)),
			strings.TrimSpace(string(branchRev)),
		).CombinedOutput()
		if mtErr != nil && strings.Contains(string(mtOut), "<<<<<<") {
			return fmt.Errorf("merge conflict detected with %s — resolve manually or let master handle:\n%s", branch, string(mtOut)[:min(len(mtOut), 500)])
		}
	}

	out, err := exec.Command("git", "-C", projectRoot, "merge", "--ff-only", branch).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: fast-forward not possible, creating merge commit\n")
		out, err = exec.Command("git", "-C", projectRoot, "merge", "--no-ff", branch).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TagArchive creates a git tag pointing at the given branch.
func TagArchive(projectRoot, branch, tag string) error {
	return exec.Command("git", "-C", projectRoot, "tag", tag, branch).Run()
}

func parsePorcelainPath(line string) string {
	if len(line) < 4 {
		return strings.TrimSpace(line)
	}
	path := strings.TrimSpace(line[3:])
	if idx := strings.LastIndex(path, " -> "); idx >= 0 {
		path = path[idx+4:]
	}
	return strings.Trim(path, "\"")
}

func isAllowedLocalConfigPath(path string) bool {
	return path == ".goalx" || strings.HasPrefix(path, ".goalx/") ||
		path == ".goalx" || strings.HasPrefix(path, ".goalx/") ||
		path == ".claude" || strings.HasPrefix(path, ".claude/") ||
		path == ".codex" || strings.HasPrefix(path, ".codex/")
}

// hasDirtyWorktree returns true if the project has uncommitted changes
// beyond config files that are expected to be local.
func hasDirtyWorktree(projectRoot string) (bool, error) {
	out, err := exec.Command("git", "-C", projectRoot, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path := parsePorcelainPath(line)
		if isAllowedLocalConfigPath(path) {
			continue
		}
		return true, nil
	}
	return false, nil
}
