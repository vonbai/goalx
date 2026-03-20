package autoresearch

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// JournalEntry is a single line in a subagent or master journal.
type JournalEntry struct {
	// Subagent fields
	Round      int    `json:"round,omitempty"`
	Commit     string `json:"commit,omitempty"`
	Desc       string `json:"desc,omitempty"`
	Confidence string `json:"confidence,omitempty"`
	Status     string `json:"status,omitempty"`

	// Master fields
	Ts       string `json:"ts,omitempty"`
	Action   string `json:"action,omitempty"`
	Session  string `json:"session,omitempty"`
	Finding  string `json:"finding,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Guidance string `json:"guidance,omitempty"`
}

// LoadJournal reads a JSONL journal file and returns all entries.
func LoadJournal(path string) ([]JournalEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var entries []JournalEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var e JournalEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue // skip malformed lines
		}
		entries = append(entries, e)
	}
	return entries, scanner.Err()
}

// Summary returns a one-line summary of journal progress.
func Summary(entries []JournalEntry) string {
	if len(entries) == 0 {
		return "no entries"
	}
	last := entries[len(entries)-1]
	if last.Round > 0 {
		return fmt.Sprintf("round %d: %s (%s)", last.Round, last.Desc, last.Status)
	}
	if last.Action != "" {
		return fmt.Sprintf("[%s] %s: %s", last.Action, last.Session, last.Finding)
	}
	return last.Desc
}
