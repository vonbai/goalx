package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Save copies run artifacts to user-scoped durable storage.
func Save(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx save [NAME]") {
		return nil
	}
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
	meta, err := LoadRunMetadata(RunMetadataPath(rc.RunDir))
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	if meta == nil {
		return fmt.Errorf("run metadata missing at %s", RunMetadataPath(rc.RunDir))
	}
	charter, err := RequireRunCharter(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load run charter: %w", err)
	}
	if err := ValidateRunCharterLinkage(meta, charter); err != nil {
		return fmt.Errorf("validate run charter linkage: %w", err)
	}

	saveDir := SavedRunDir(rc.ProjectRoot, rc.Name)
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return fmt.Errorf("create save dir: %w", err)
	}
	manifest, manifestFromFile, err := ResolveRunArtifacts(rc.RunDir, rc.Config)
	if err != nil {
		return fmt.Errorf("resolve run artifacts: %w", err)
	}

	// Copy summary
	summaryPath := filepath.Join(rc.RunDir, "summary.md")
	if err := copyFileIfExists(summaryPath, filepath.Join(saveDir, "summary.md")); err != nil {
		return fmt.Errorf("copy summary: %w", err)
	}

	// Copy immutable run spec + runtime state.
	if err := copyFileIfExists(RunSpecPath(rc.RunDir), filepath.Join(saveDir, "run-spec.yaml")); err != nil {
		return fmt.Errorf("copy run spec: %w", err)
	}
	if err := copyFileIfExists(RunRuntimeStatePath(rc.RunDir), filepath.Join(saveDir, "run.json")); err != nil {
		return fmt.Errorf("copy run state: %w", err)
	}
	if err := copyFileIfExists(SessionsRuntimeStatePath(rc.RunDir), filepath.Join(saveDir, "sessions.json")); err != nil {
		return fmt.Errorf("copy session state: %w", err)
	}
	if err := copyFileIfExists(RunStatusPath(rc.RunDir), filepath.Join(saveDir, "status.json")); err != nil {
		return fmt.Errorf("copy run status: %w", err)
	}

	if err := copyFileIfExists(GoalPath(rc.RunDir), filepath.Join(saveDir, "goal.json")); err != nil {
		return fmt.Errorf("copy goal state: %w", err)
	}
	if err := copyFileIfExists(GoalLogPath(rc.RunDir), filepath.Join(saveDir, "goal-log.jsonl")); err != nil {
		return fmt.Errorf("copy goal log: %w", err)
	}
	runMetadataPath := RunMetadataPath(rc.RunDir)
	if err := copyFileIfExists(runMetadataPath, filepath.Join(saveDir, "run-metadata.json")); err != nil {
		return fmt.Errorf("copy run metadata: %w", err)
	}
	if err := copyFileIfExists(RunCharterPath(rc.RunDir), filepath.Join(saveDir, "run-charter.json")); err != nil {
		return fmt.Errorf("copy run charter: %w", err)
	}
	completionStatePath := CompletionStatePath(rc.RunDir)
	if err := copyFileIfExists(completionStatePath, filepath.Join(saveDir, "proof", "completion.json")); err != nil {
		return fmt.Errorf("copy completion state: %w", err)
	}
	acceptStatePath := AcceptanceStatePath(rc.RunDir)
	if err := copyFileIfExists(acceptStatePath, filepath.Join(saveDir, "acceptance.json")); err != nil {
		return fmt.Errorf("copy acceptance state: %w", err)
	}
	acceptEvidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := copyFileIfExists(acceptEvidencePath, filepath.Join(saveDir, "acceptance-last.txt")); err != nil {
		return fmt.Errorf("copy acceptance evidence: %w", err)
	}

	savedManifest := &ArtifactsManifest{
		Run:     rc.Name,
		Version: 1,
	}

	// Copy session artifacts + journals
	sessionState, err := EnsureSessionsRuntimeState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load session runtime state: %w", err)
	}
	sessionList := sortedSessionStates(sessionState)
	if len(sessionList) == 0 {
		indexes, err := existingSessionIndexes(rc.RunDir)
		if err != nil {
			return err
		}
		for _, num := range indexes {
			sName := SessionName(num)
			identity, err := RequireSessionIdentity(rc.RunDir, sName)
			if err != nil {
				return fmt.Errorf("load %s identity: %w", sName, err)
			}
			sessionList = append(sessionList, SessionRuntimeState{
				Name:         sName,
				Mode:         identity.Mode,
				WorktreePath: resolvedSessionWorktreePath(rc.RunDir, rc.Config.Name, sName, sessionState),
			})
		}
	}
	for _, sess := range sessionList {
		sName := sess.Name
		identity, err := RequireSessionIdentity(rc.RunDir, sName)
		if err != nil {
			return fmt.Errorf("load %s identity: %w", sName, err)
		}
		if err := copyFileIfExists(SessionIdentityPath(rc.RunDir, sName), filepath.Join(saveDir, "sessions", sName, "identity.json")); err != nil {
			return fmt.Errorf("copy %s identity: %w", sName, err)
		}
		sessionMode := identity.Mode
		if sessionMode == "" {
			sessionMode = sess.Mode
		}
		targetFiles := append([]string(nil), identity.Target.Files...)
		reportSource := ""
		declaredSession := FindSessionArtifacts(manifest, sName)
		artifact := FindSessionArtifact(manifest, sName, "report")
		if artifact != nil && artifact.Path != "" {
			reportSource = artifact.Path
		} else if runReport := findRunScopedReport(rc.RunDir, sName); runReport != "" {
			reportSource = runReport
		} else if !manifestFromFile || declaredSession == nil {
			reportRoot := sess.WorktreePath
			if reportRoot == "" {
				reportRoot = RunWorktreePath(rc.RunDir)
			}
			if reportRoot != "" {
				reportSource = findSessionReport(reportRoot, targetFiles)
			}
		}
		if reportSource != "" {
			destName := ""
			if artifact != nil {
				destName = artifact.DurableName
			}
			if destName == "" {
				destName = fmt.Sprintf("%s-report.md", sName)
			}
			destPath := filepath.Join(saveDir, destName)
			if err := copyFileIfExists(reportSource, destPath); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not copy %s report: %v\n", sName, err)
			} else {
				savedSession := ensureSessionArtifactsEntry(savedManifest, sName, sessionMode)
				upsertArtifact(savedSession, ArtifactMeta{
					Kind:        "report",
					Path:        destPath,
					RelPath:     destName,
					DurableName: destName,
				})
			}
		}

		jPath := JournalPath(rc.RunDir, sName)
		jDest := fmt.Sprintf("%s.jsonl", sName)
		if err := copyFileIfExists(jPath, filepath.Join(saveDir, jDest)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not copy %s journal: %v\n", sName, err)
		}
	}
	if err := SaveArtifacts(filepath.Join(saveDir, "artifacts.json"), savedManifest); err != nil {
		return fmt.Errorf("copy artifacts manifest: %w", err)
	}

	// Copy master journal
	masterJPath := filepath.Join(rc.RunDir, "master.jsonl")
	copyFileIfExists(masterJPath, filepath.Join(saveDir, "master.jsonl"))
	if err := RegisterSavedRun(rc.ProjectRoot, rc.Config); err != nil {
		return fmt.Errorf("register saved run: %w", err)
	}

	fmt.Printf("Saved run '%s' to %s\n", rc.Name, saveDir)
	return nil
}

func parseSessionNumber(name string) (int, error) {
	var num int
	if _, err := fmt.Sscanf(name, "session-%d", &num); err != nil {
		return 0, fmt.Errorf("parse session number from %q: %w", name, err)
	}
	return num, nil
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

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
