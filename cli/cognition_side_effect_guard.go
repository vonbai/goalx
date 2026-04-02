package cli

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type guardedPathSnapshot struct {
	targetPath string
	backupPath string
	existed    bool
	isDir      bool
	mode       fs.FileMode
}

func withGitNexusSideEffectGuard(scopePath string, fn func() error) error {
	scopePath = strings.TrimSpace(scopePath)
	if scopePath == "" {
		return fn()
	}
	targets := []string{
		filepath.Join(scopePath, "AGENTS.md"),
		filepath.Join(scopePath, "CLAUDE.md"),
		filepath.Join(scopePath, ".claude", "skills", "gitnexus"),
	}
	snapshots := make([]guardedPathSnapshot, 0, len(targets))
	for _, target := range targets {
		snapshot, err := snapshotGuardedPath(target)
		if err != nil {
			return err
		}
		snapshots = append(snapshots, snapshot)
	}
	defer cleanupGuardedSnapshots(snapshots)

	runErr := fn()
	restoreErr := restoreGuardedSnapshots(snapshots)
	switch {
	case runErr != nil && restoreErr != nil:
		return fmt.Errorf("%v; restore gitnexus side effects: %w", runErr, restoreErr)
	case runErr != nil:
		return runErr
	default:
		return restoreErr
	}
}

func snapshotGuardedPath(targetPath string) (guardedPathSnapshot, error) {
	snapshot := guardedPathSnapshot{targetPath: targetPath}
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return snapshot, nil
		}
		return snapshot, err
	}
	snapshot.existed = true
	snapshot.isDir = info.IsDir()
	snapshot.mode = info.Mode()
	backupRoot, err := os.MkdirTemp("", "goalx-gitnexus-guard-*")
	if err != nil {
		return snapshot, err
	}
	snapshot.backupPath = filepath.Join(backupRoot, "payload")
	if snapshot.isDir {
		if err := copyDirRecursive(targetPath, snapshot.backupPath); err != nil {
			_ = os.RemoveAll(backupRoot)
			return snapshot, err
		}
		return snapshot, nil
	}
	if err := copyFileWithMode(targetPath, snapshot.backupPath, info.Mode()); err != nil {
		_ = os.RemoveAll(backupRoot)
		return snapshot, err
	}
	return snapshot, nil
}

func restoreGuardedSnapshots(snapshots []guardedPathSnapshot) error {
	for _, snapshot := range snapshots {
		if !snapshot.existed {
			if err := os.RemoveAll(snapshot.targetPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			continue
		}
		if snapshot.isDir {
			if err := os.RemoveAll(snapshot.targetPath); err != nil && !os.IsNotExist(err) {
				return err
			}
			if err := copyDirRecursive(snapshot.backupPath, snapshot.targetPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFileWithMode(snapshot.backupPath, snapshot.targetPath, snapshot.mode); err != nil {
			return err
		}
	}
	return nil
}

func cleanupGuardedSnapshots(snapshots []guardedPathSnapshot) {
	for _, snapshot := range snapshots {
		if snapshot.backupPath == "" {
			continue
		}
		_ = os.RemoveAll(filepath.Dir(snapshot.backupPath))
	}
}

func copyDirRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		entryInfo, err := entry.Info()
		if err != nil {
			return err
		}
		if entryInfo.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFileWithMode(srcPath, dstPath, entryInfo.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFileWithMode(src, dst string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(mode.Perm())
}
