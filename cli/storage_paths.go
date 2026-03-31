package cli

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

func ProjectDataDir(projectRoot string) string {
	return filepath.Join(userGoalxDir(), "runs", goalx.ProjectID(projectRoot))
}

func userGoalxDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".goalx")
}

func MemoryRootDir() string {
	return filepath.Join(userGoalxDir(), "memory")
}

func MemoryLockPath() string {
	return filepath.Join(MemoryRootDir(), ".lock")
}

func MemoryEntriesDir() string {
	return filepath.Join(MemoryRootDir(), "entries")
}

func MemoryProposalsDir() string {
	return filepath.Join(MemoryRootDir(), "proposals")
}

func MemoryIndexesDir() string {
	return filepath.Join(MemoryRootDir(), "indexes")
}

func MemoryProjectsDir() string {
	return filepath.Join(MemoryRootDir(), "projects")
}

func MemoryGCPath() string {
	return filepath.Join(MemoryRootDir(), "gc.json")
}

func MemoryEntryPath(kind MemoryKind) string {
	switch kind {
	case MemoryKindFact:
		return filepath.Join(MemoryEntriesDir(), "facts.jsonl")
	case MemoryKindProcedure:
		return filepath.Join(MemoryEntriesDir(), "procedures.jsonl")
	case MemoryKindPitfall:
		return filepath.Join(MemoryEntriesDir(), "pitfalls.jsonl")
	case MemoryKindSecretRef:
		return filepath.Join(MemoryEntriesDir(), "secret_refs.jsonl")
	case MemoryKindSuccessPrior:
		return filepath.Join(MemoryEntriesDir(), "success_priors.jsonl")
	default:
		panic("unknown memory kind")
	}
}

func MemoryProposalPath(now time.Time) string {
	return filepath.Join(MemoryProposalsDir(), now.UTC().Format("2006-01-02")+".jsonl")
}

func MemorySeedsPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "memory-seeds.jsonl")
}

func MemoryQueryPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "memory-query.json")
}

func MemoryContextPath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "memory-context.json")
}

func SavedRunsDir(projectRoot string) string {
	return filepath.Join(ProjectDataDir(projectRoot), "saved")
}

func SavedRunDir(projectRoot, runName string) string {
	return filepath.Join(SavedRunsDir(projectRoot), runName)
}

func LegacySavedRunsDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".goalx", "runs")
}

func LegacySavedRunDir(projectRoot, runName string) string {
	return filepath.Join(LegacySavedRunsDir(projectRoot), runName)
}

func RunStatusPath(runDir string) string {
	return filepath.Join(runDir, "status.json")
}

type SavedRunLocation struct {
	Name   string
	Dir    string
	Legacy bool
}

func ResolveSavedRunLocation(projectRoot, runName string) (SavedRunLocation, error) {
	runName = filepath.Clean(strings.TrimSpace(runName))
	if runName == "" || runName == "." {
		locations, err := ListSavedRunLocations(projectRoot)
		if err != nil {
			return SavedRunLocation{}, err
		}
		switch len(locations) {
		case 0:
			return SavedRunLocation{}, os.ErrNotExist
		case 1:
			return locations[0], nil
		default:
			names := make([]string, 0, len(locations))
			for _, loc := range locations {
				names = append(names, loc.Name)
			}
			sort.Strings(names)
			return SavedRunLocation{}, MultipleSavedRunsError{Names: names}
		}
	}

	for _, candidate := range []SavedRunLocation{
		{Name: runName, Dir: SavedRunDir(projectRoot, runName), Legacy: false},
		{Name: runName, Dir: LegacySavedRunDir(projectRoot, runName), Legacy: true},
	} {
		if info, err := os.Stat(candidate.Dir); err == nil && info.IsDir() {
			return candidate, nil
		} else if err != nil && !os.IsNotExist(err) {
			return SavedRunLocation{}, err
		}
	}
	return SavedRunLocation{}, os.ErrNotExist
}

func ListSavedRunLocations(projectRoot string) ([]SavedRunLocation, error) {
	seen := map[string]bool{}
	locations := make([]SavedRunLocation, 0)
	for _, root := range []struct {
		dir    string
		legacy bool
	}{
		{dir: SavedRunsDir(projectRoot), legacy: false},
		{dir: LegacySavedRunsDir(projectRoot), legacy: true},
	} {
		entries, err := os.ReadDir(root.dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if !entry.IsDir() || seen[entry.Name()] {
				continue
			}
			seen[entry.Name()] = true
			locations = append(locations, SavedRunLocation{
				Name:   entry.Name(),
				Dir:    filepath.Join(root.dir, entry.Name()),
				Legacy: root.legacy,
			})
		}
	}
	sort.Slice(locations, func(i, j int) bool {
		return locations[i].Name < locations[j].Name
	})
	return locations, nil
}

type MultipleSavedRunsError struct {
	Names []string
}

func (e MultipleSavedRunsError) Error() string {
	return "multiple saved runs: " + strings.Join(e.Names, ", ")
}
