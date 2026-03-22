package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// Save copies run artifacts (reports, summary, config snapshot) to .goalx/runs/<name>/.
// This preserves results locally after archive/drop without polluting the git repo.
func Save(projectRoot string, args []string) error {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx save [NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	saveDir := filepath.Join(projectRoot, ".goalx", "runs", rc.Name)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}

	// Copy summary
	summaryPath := filepath.Join(rc.RunDir, "summary.md")
	if err := copyFileIfExists(summaryPath, filepath.Join(saveDir, "summary.md")); err != nil {
		return fmt.Errorf("copy summary: %w", err)
	}

	// Copy config snapshot
	cfgPath := filepath.Join(rc.RunDir, "goalx.yaml")
	if err := copyFileIfExists(cfgPath, filepath.Join(saveDir, "goalx.yaml")); err != nil {
		return fmt.Errorf("copy config: %w", err)
	}

	// Copy acceptance checklist
	acceptPath := filepath.Join(rc.RunDir, "acceptance.md")
	if err := copyFileIfExists(acceptPath, filepath.Join(saveDir, "acceptance.md")); err != nil {
		return fmt.Errorf("copy acceptance: %w", err)
	}
	acceptStatePath := AcceptanceStatePath(rc.RunDir)
	if err := copyFileIfExists(acceptStatePath, filepath.Join(saveDir, "acceptance.json")); err != nil {
		return fmt.Errorf("copy acceptance state: %w", err)
	}
	acceptEvidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := copyFileIfExists(acceptEvidencePath, filepath.Join(saveDir, "acceptance-last.txt")); err != nil {
		return fmt.Errorf("copy acceptance evidence: %w", err)
	}

	// Copy session reports
	sessions := goalx.ExpandSessions(rc.Config)
	for i := range sessions {
		num := i + 1
		wtPath := WorktreePath(rc.RunDir, rc.Config.Name, num)
		reportPath := findSessionReport(wtPath, rc.Config.Target.Files)
		destName := fmt.Sprintf("session-%d-report.md", num)
		if reportPath != "" {
			if err := copyFileIfExists(reportPath, filepath.Join(saveDir, destName)); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not copy %s report: %v\n", SessionName(num), err)
			}
		}

		// Copy journal
		jPath := JournalPath(rc.RunDir, SessionName(num))
		jDest := fmt.Sprintf("session-%d.jsonl", num)
		if err := copyFileIfExists(jPath, filepath.Join(saveDir, jDest)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not copy %s journal: %v\n", SessionName(num), err)
		}
	}

	// Copy master journal
	masterJPath := filepath.Join(rc.RunDir, "master.jsonl")
	copyFileIfExists(masterJPath, filepath.Join(saveDir, "master.jsonl"))

	fmt.Printf("Saved run '%s' to %s\n", rc.Name, saveDir)
	return nil
}

// findSessionReport locates the report file in a worktree.
// Priority: target.files[0] (if regular file) → report.md → git-added *.md files.
func findSessionReport(wtPath string, targetFiles []string) string {
	// Try target.files[0] if it looks like a file (not "." or a directory)
	if len(targetFiles) > 0 && targetFiles[0] != "" && targetFiles[0] != "." {
		candidate := filepath.Join(wtPath, targetFiles[0])
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Size() > 0 {
			return candidate
		}
	}

	// Try report.md
	candidate := filepath.Join(wtPath, "report.md")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() && info.Size() > 0 {
		return candidate
	}

	// Fallback: find .md files added in git (not tracked upstream)
	out, err := exec.Command("git", "-C", wtPath, "diff", "--name-only", "--diff-filter=A", "HEAD~10", "--", "*.md").Output()
	if err != nil {
		// If git fails (e.g. shallow history), try untracked .md files
		out, _ = exec.Command("git", "-C", wtPath, "ls-files", "--others", "--exclude-standard", "*.md").Output()
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		p := filepath.Join(wtPath, line)
		if info, err := os.Stat(p); err == nil && !info.IsDir() && info.Size() > 0 {
			return p
		}
	}
	return ""
}

func copyFileIfExists(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.IsDir() {
		return nil // silently skip directories
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
