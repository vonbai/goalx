package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func PanePIDsDir(runDir string) string {
	return filepath.Join(ControlDir(runDir), "pane-pids")
}

func PanePIDPath(runDir, holder string) string {
	return filepath.Join(PanePIDsDir(runDir), holder)
}

func PersistPanePIDsFromTmux(runDir, holder, target string) error {
	out, err := exec.Command("tmux", "list-panes", "-t", target, "-F", "#{pane_pid}").Output()
	if err != nil {
		return err
	}
	if len(parsePIDs(out)) == 0 {
		return nil
	}
	if err := os.MkdirAll(PanePIDsDir(runDir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(PanePIDPath(runDir, holder), out, 0o644)
}

func listTmuxPanePIDs(tmuxSession string) ([]int, error) {
	out, err := exec.Command("tmux", "list-panes", "-s", "-t", tmuxSession, "-F", "#{pane_pid}").Output()
	if err != nil {
		return nil, err
	}
	return parsePIDs(out), nil
}

func loadPersistedPanePIDs(runDir string) ([]int, error) {
	entries, err := os.ReadDir(PanePIDsDir(runDir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var pids []int
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(PanePIDsDir(runDir), entry.Name()))
		if err != nil {
			return nil, err
		}
		pids = append(pids, parsePIDs(data)...)
	}
	return uniquePIDs(pids), nil
}

func killRunPaneProcessTrees(runDir, tmuxSession string) {
	if tmuxSession != "" && SessionExists(tmuxSession) {
		if pids, err := listTmuxPanePIDs(tmuxSession); err == nil && len(pids) > 0 {
			killProcessTrees(pids)
			return
		}
	}
	if pids, err := loadPersistedPanePIDs(runDir); err == nil {
		killProcessTrees(pids)
	}
}

func killProcessTrees(pids []int) {
	for _, pid := range uniquePIDs(pids) {
		KillProcessTree(pid)
	}
}

func parsePIDs(data []byte) []int {
	fields := strings.Fields(string(data))
	pids := make([]int, 0, len(fields))
	for _, field := range fields {
		pid, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || pid <= 0 {
			continue
		}
		pids = append(pids, pid)
	}
	return uniquePIDs(pids)
}

func uniquePIDs(pids []int) []int {
	seen := make(map[int]struct{}, len(pids))
	out := make([]int, 0, len(pids))
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		out = append(out, pid)
	}
	return out
}
