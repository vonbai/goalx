package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CreateWorktree creates a new git worktree with a new branch.
// If baseBranch is non-empty, the new branch starts from that branch
// instead of HEAD. This enables forking from a previous run's worktree.
// Cleans up stale branch collisions from failed previous runs, but refuses to
// delete branches that are still checked out in another worktree.
func CreateWorktree(projectRoot, worktreePath, branch string, baseBranch ...string) error {
	exec.Command("git", "-C", projectRoot, "worktree", "prune").Run()
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return fmt.Errorf("mkdir worktree parent for %s: %w", worktreePath, err)
	}
	exists, err := branchExists(projectRoot, branch)
	if err != nil {
		return err
	}
	if exists {
		inUse, err := branchCheckedOutInAnyWorktree(projectRoot, branch)
		if err != nil {
			return err
		}
		if inUse {
			return fmt.Errorf("branch %s is already checked out in another worktree", branch)
		}
		if err := DeleteBranch(projectRoot, branch); err != nil {
			return fmt.Errorf("delete stale branch %s: %w", branch, err)
		}
	}
	args := []string{"-C", projectRoot, "worktree", "add", worktreePath, "-b", branch}
	if len(baseBranch) > 0 && baseBranch[0] != "" {
		args = append(args, baseBranch[0])
	}
	cmd := exec.Command("git", args...)
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

// MergeWorktree merges a branch into the current branch of targetDir.
func MergeWorktree(targetDir, branch string) error {
	statusOut, err := exec.Command("git", "-C", targetDir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git status: %w: %s", err, statusOut)
	}
	dirtyPaths := make([]string, 0)
	for _, line := range strings.Split(string(statusOut), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path := parsePorcelainPath(line)
		if isAllowedLocalConfigPath(path) {
			continue
		}
		dirtyPaths = append(dirtyPaths, path)
	}
	if len(dirtyPaths) > 0 {
		return fmt.Errorf("merge target %s has uncommitted changes (%s); commit or stash changes before merge", targetDir, summarizeDirtyPaths(dirtyPaths))
	}

	// Pre-check for conflicts using merge-tree
	head, _ := exec.Command("git", "-C", targetDir, "rev-parse", "HEAD").Output()
	branchRev, _ := exec.Command("git", "-C", targetDir, "rev-parse", branch).Output()
	if len(head) > 0 && len(branchRev) > 0 {
		mergeBase, mergeBaseErr := exec.Command("git", "-C", targetDir, "merge-base",
			strings.TrimSpace(string(head)),
			strings.TrimSpace(string(branchRev)),
		).CombinedOutput()
		if mergeBaseErr != nil {
			return fmt.Errorf("git merge-base: %w: %s", mergeBaseErr, mergeBase)
		}
		mtOut, mtErr := exec.Command("git", "-C", targetDir, "merge-tree",
			strings.TrimSpace(string(mergeBase)),
			strings.TrimSpace(string(head)),
			strings.TrimSpace(string(branchRev)),
		).CombinedOutput()
		if mtErr != nil {
			return fmt.Errorf("git merge-tree: %w: %s", mtErr, mtOut)
		}
		if hasMergeConflictMarkers(string(mtOut)) {
			return fmt.Errorf("merge conflict detected with %s — resolve manually or let master handle:\n%s", branch, string(mtOut)[:min(len(mtOut), 500)])
		}
	}

	out, err := exec.Command("git", "-C", targetDir, "merge", "--ff-only", branch).CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: fast-forward not possible, creating merge commit\n")
		out, err = exec.Command("git", "-C", targetDir, "merge", "--no-ff", branch).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, out)
		}
	}
	return nil
}

func summarizeDirtyPaths(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	const limit = 5
	if len(paths) <= limit {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s, +%d more", strings.Join(paths[:limit], ", "), len(paths)-limit)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func hasMergeConflictMarkers(out string) bool {
	return strings.Contains(out, "<<<<<<<") &&
		strings.Contains(out, "=======") &&
		strings.Contains(out, ">>>>>>>")
}

func branchExists(projectRoot, branch string) (bool, error) {
	out, err := exec.Command("git", "-C", projectRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).CombinedOutput()
	if err == nil {
		return true, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git show-ref %s: %w: %s", branch, err, out)
}

func branchCheckedOutInAnyWorktree(projectRoot, branch string) (bool, error) {
	out, err := exec.Command("git", "-C", projectRoot, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("git worktree list: %w: %s", err, out)
	}

	target := "branch refs/heads/" + branch
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == target {
			return true, nil
		}
	}
	return false, nil
}

func gitIsAncestor(projectRoot, ancestor, descendant string) (bool, error) {
	ancestor = strings.TrimSpace(ancestor)
	descendant = strings.TrimSpace(descendant)
	if ancestor == "" || descendant == "" {
		return false, fmt.Errorf("ancestor and descendant revisions are required")
	}
	out, err := exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", ancestor, descendant).CombinedOutput()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, nil
	}
	return false, fmt.Errorf("git merge-base --is-ancestor %s %s: %w: %s", ancestor, descendant, err, out)
}

func gitTreesEqual(projectRoot, revA, revB string) (bool, error) {
	treeA, err := gitTreeRevision(projectRoot, revA)
	if err != nil {
		return false, err
	}
	treeB, err := gitTreeRevision(projectRoot, revB)
	if err != nil {
		return false, err
	}
	return treeA == treeB, nil
}

func gitTreeRevision(projectRoot, rev string) (string, error) {
	out, err := exec.Command("git", "-C", projectRoot, "rev-parse", strings.TrimSpace(rev)+"^{tree}").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git rev-parse %s^{tree}: %w: %s", rev, err, out)
	}
	return strings.TrimSpace(string(out)), nil
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

// CopyGitignoredFiles copies ignored-but-existing regular files from sourceDir
// into targetDir, preserving relative paths so worktrees inherit local project
// state such as CLAUDE.md, docs/, and other ignored artifacts.
func CopyGitignoredFiles(sourceDir, targetDir string) error {
	out, err := exec.Command(
		"git", "-c", "core.quotePath=false",
		"-C", sourceDir,
		"ls-files", "--others", "--ignored", "--exclude-standard", "-z",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git ls-files ignored in %s: %w: %s", sourceDir, err, out)
	}

	for _, raw := range bytes.Split(out, []byte{0}) {
		if len(raw) == 0 {
			continue
		}

		rel := filepath.Clean(string(raw))
		if rel == "." || rel == ".git" || strings.HasPrefix(rel, ".git"+string(filepath.Separator)) {
			continue
		}
		if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			fmt.Fprintf(os.Stderr, "warning: skip invalid ignored path %q\n", rel)
			continue
		}

		sourcePath := filepath.Join(sourceDir, rel)
		info, statErr := os.Lstat(sourcePath)
		if errors.Is(statErr, os.ErrNotExist) {
			continue
		}
		if statErr != nil {
			fmt.Fprintf(os.Stderr, "warning: stat ignored path %s: %v\n", sourcePath, statErr)
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		if info.IsDir() {
			if err := os.MkdirAll(filepath.Join(targetDir, rel), info.Mode().Perm()); err != nil {
				fmt.Fprintf(os.Stderr, "warning: mkdir mirrored ignored dir %s: %v\n", rel, err)
			}
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := copyIgnoredRegularFile(sourcePath, filepath.Join(targetDir, rel), info.Mode().Perm()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: copy ignored file %s: %v\n", rel, err)
		}
	}

	return nil
}

func copyIgnoredRegularFile(sourcePath, targetPath string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	src, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	return os.Chmod(targetPath, mode)
}
