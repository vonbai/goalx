package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type WorktreeSnapshot struct {
	CheckedAt string                       `json:"checked_at"`
	Root      WorktreeDiffStat            `json:"root"`
	Sessions  map[string]WorktreeDiffStat `json:"sessions,omitempty"`
}

type WorktreeDiffStat struct {
	DirtyFiles int `json:"dirty_files"`
	Insertions int `json:"insertions"`
	Deletions  int `json:"deletions"`
}

var (
	shortstatInsertionsRE = regexp.MustCompile(`(\d+)\s+insertions?\(\+\)`)
	shortstatDeletionsRE  = regexp.MustCompile(`(\d+)\s+deletions?\(-\)`)
)

func WorktreeSnapshotPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "worktree-snapshot.json")
}

func SnapshotWorktrees(runDir string) (*WorktreeSnapshot, error) {
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		return nil, err
	}
	sessionsState, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return nil, fmt.Errorf("load session runtime state: %w", err)
	}

	root, err := snapshotWorktreeDiffStat(RunWorktreePath(runDir))
	if err != nil {
		return nil, err
	}
	snapshot := &WorktreeSnapshot{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Root:      root,
	}

	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		worktreePath := resolvedSessionWorktreePath(runDir, cfg.Name, sessionName, sessionsState)
		if worktreePath == "" {
			continue
		}
		diffStat, err := snapshotWorktreeDiffStat(worktreePath)
		if err != nil {
			return nil, err
		}
		if snapshot.Sessions == nil {
			snapshot.Sessions = make(map[string]WorktreeDiffStat)
		}
		snapshot.Sessions[sessionName] = diffStat
	}
	return snapshot, nil
}

func LoadWorktreeSnapshot(path string) (*WorktreeSnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	snapshot := &WorktreeSnapshot{}
	if len(data) == 0 {
		return snapshot, nil
	}
	if err := json.Unmarshal(data, snapshot); err != nil {
		return nil, fmt.Errorf("parse worktree snapshot: %w", err)
	}
	return snapshot, nil
}

func SaveWorktreeSnapshot(runDir string, snap *WorktreeSnapshot) error {
	if snap == nil {
		return fmt.Errorf("worktree snapshot is nil")
	}
	if snap.CheckedAt == "" {
		snap.CheckedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(WorktreeSnapshotPath(runDir), snap)
}

func snapshotWorktreeDiffStat(worktreePath string) (WorktreeDiffStat, error) {
	if strings.TrimSpace(worktreePath) == "" {
		return WorktreeDiffStat{}, nil
	}
	dirtyFiles, _, err := snapshotWorktreeState(worktreePath)
	if err != nil {
		return WorktreeDiffStat{}, err
	}
	out, err := exec.Command("git", "-C", worktreePath, "diff", "--shortstat").CombinedOutput()
	if err != nil {
		if os.IsNotExist(err) || bytes.Contains(bytes.ToLower(out), []byte("not a git repository")) {
			return WorktreeDiffStat{DirtyFiles: dirtyFiles}, nil
		}
		return WorktreeDiffStat{}, fmt.Errorf("git diff --shortstat in %s: %w: %s", worktreePath, err, out)
	}
	text := string(out)
	return WorktreeDiffStat{
		DirtyFiles: dirtyFiles,
		Insertions: parseShortstatCount(shortstatInsertionsRE, text),
		Deletions:  parseShortstatCount(shortstatDeletionsRE, text),
	}, nil
}

func parseShortstatCount(pattern *regexp.Regexp, text string) int {
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0
	}
	var count int
	fmt.Sscanf(match[1], "%d", &count)
	return count
}
