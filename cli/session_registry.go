package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
)

var sessionJournalPattern = regexp.MustCompile(`^session-(\d+)\.jsonl$`)

func existingSessionIndexes(runDir string) ([]int, error) {
	entries, err := os.ReadDir(filepath.Join(runDir, "journals"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session journals: %w", err)
	}

	seen := make(map[int]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		m := sessionJournalPattern.FindStringSubmatch(entry.Name())
		if len(m) != 2 {
			continue
		}
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx <= 0 {
			continue
		}
		seen[idx] = struct{}{}
	}

	indexes := make([]int, 0, len(seen))
	for idx := range seen {
		indexes = append(indexes, idx)
	}
	slices.Sort(indexes)
	return indexes, nil
}

func nextSessionIndex(runDir string) (int, error) {
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return 0, err
	}
	if len(indexes) == 0 {
		return 1, nil
	}
	return indexes[len(indexes)-1] + 1, nil
}

func hasSessionIndex(runDir string, idx int) (bool, error) {
	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return false, err
	}
	return slices.Contains(indexes, idx), nil
}
