package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	ar "github.com/vonbai/autoresearch"
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

	// Copy session reports
	sessions := ar.ExpandSessions(rc.Config)
	for i := range sessions {
		num := i + 1
		wtPath := WorktreePath(rc.RunDir, rc.Config.Name, num)
		reportFile := "report.md"
		if len(rc.Config.Target.Files) > 0 && rc.Config.Target.Files[0] != "" {
			reportFile = rc.Config.Target.Files[0]
		}
		reportPath := filepath.Join(wtPath, reportFile)
		destName := fmt.Sprintf("session-%d-report.md", num)
		if err := copyFileIfExists(reportPath, filepath.Join(saveDir, destName)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not copy %s report: %v\n", SessionName(num), err)
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

func copyFileIfExists(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
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
