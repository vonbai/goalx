package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseMeminfo(t *testing.T) {
	facts, err := parseMeminfo([]byte(`MemTotal:       32869572 kB
MemAvailable:   20739708 kB
SwapTotal:      16777212 kB
SwapFree:       16777212 kB
`))
	if err != nil {
		t.Fatalf("parseMeminfo: %v", err)
	}
	if facts.MemTotalBytes != 32869572*1024 || facts.MemAvailableBytes != 20739708*1024 {
		t.Fatalf("meminfo facts = %+v", facts)
	}
}

func TestParseMemoryPSI(t *testing.T) {
	facts, err := parseMemoryPSI([]byte(`some avg10=0.10 avg60=0.20 avg300=0.30 total=123
full avg10=0.01 avg60=0.02 avg300=0.03 total=45
`))
	if err != nil {
		t.Fatalf("parseMemoryPSI: %v", err)
	}
	if facts.MemorySomeAvg10 != 0.10 || facts.MemoryFullAvg300 != 0.03 {
		t.Fatalf("psi facts = %+v", facts)
	}
}

func TestParseCgroupMemoryValueHandlesMaxAndNumbers(t *testing.T) {
	value, err := parseCgroupIntValue([]byte("max\n"))
	if err != nil || value != 0 {
		t.Fatalf("parseCgroupIntValue(max) = %d, %v", value, err)
	}
	value, err = parseCgroupIntValue([]byte("4096\n"))
	if err != nil || value != 4096 {
		t.Fatalf("parseCgroupIntValue(number) = %d, %v", value, err)
	}
}

func TestParseCgroupMemoryEvents(t *testing.T) {
	events, err := parseCgroupMemoryEvents([]byte("low 1\nhigh 2\nmax 3\noom 4\noom_kill 5\n"))
	if err != nil {
		t.Fatalf("parseCgroupMemoryEvents: %v", err)
	}
	if events.Low != 1 || events.High != 2 || events.Max != 3 || events.OOM != 4 || events.OOMKill != 5 {
		t.Fatalf("events = %+v", events)
	}
}

func TestProbeLinuxResourceStateToleratesMissingPSIAndCgroupFiles(t *testing.T) {
	prev := resourceReadFile
	t.Cleanup(func() { resourceReadFile = prev })
	resourceReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/proc/meminfo":
			return []byte("MemTotal: 1 kB\nMemAvailable: 1 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n"), nil
		case "/proc/sys/vm/swappiness":
			return []byte("10\n"), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	state, err := ProbeLinuxResourceState()
	if err != nil {
		t.Fatalf("ProbeLinuxResourceState: %v", err)
	}
	if state.Host == nil || state.PSI == nil || state.Cgroup == nil || state.GoalxProcesses == nil {
		t.Fatalf("resource state incomplete: %+v", state)
	}
	if state.Host.Swappiness != 10 {
		t.Fatalf("swappiness = %d, want 10", state.Host.Swappiness)
	}
}

func TestProbeLinuxResourceStateReadsCgroupFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"/proc/meminfo":                      "MemTotal: 2 kB\nMemAvailable: 1 kB\nSwapTotal: 4 kB\nSwapFree: 3 kB\n",
		"/proc/sys/vm/swappiness":            "15\n",
		"/proc/pressure/memory":              "some avg10=0.00 avg60=1.00 avg300=2.00 total=3\nfull avg10=0.10 avg60=0.20 avg300=0.30 total=4\n",
		"/sys/fs/cgroup/memory.current":      "100\n",
		"/sys/fs/cgroup/memory.high":         "200\n",
		"/sys/fs/cgroup/memory.max":          "300\n",
		"/sys/fs/cgroup/memory.swap.current": "400\n",
		"/sys/fs/cgroup/memory.swap.max":     "500\n",
		"/sys/fs/cgroup/memory.events":       "low 1\nhigh 2\nmax 3\noom 4\noom_kill 5\n",
	}
	for path, body := range files {
		local := filepath.Join(root, strings.TrimPrefix(path, "/"))
		if err := os.MkdirAll(filepath.Dir(local), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", local, err)
		}
		if err := os.WriteFile(local, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", local, err)
		}
	}
	prev := resourceReadFile
	t.Cleanup(func() { resourceReadFile = prev })
	resourceReadFile = func(path string) ([]byte, error) {
		local := filepath.Join(root, strings.TrimPrefix(path, "/"))
		return os.ReadFile(local)
	}

	state, err := ProbeLinuxResourceState()
	if err != nil {
		t.Fatalf("ProbeLinuxResourceState: %v", err)
	}
	if state.Cgroup.MemoryCurrentBytes != 100 || state.Cgroup.MemorySwapMaxBytes != 500 {
		t.Fatalf("cgroup facts = %+v", state.Cgroup)
	}
	if state.Cgroup.Events == nil || state.Cgroup.Events.OOMKill != 5 {
		t.Fatalf("cgroup events = %+v", state.Cgroup.Events)
	}
	if state.PSI.MemorySomeAvg60 != 1 || state.PSI.MemoryFullAvg300 != 0.30 {
		t.Fatalf("psi facts = %+v", state.PSI)
	}
	if state.Host.Swappiness != 15 {
		t.Fatalf("swappiness = %d, want 15", state.Host.Swappiness)
	}
}

func TestParsePSIRejectsMalformedNumbers(t *testing.T) {
	_, err := parseMemoryPSI([]byte("some avg10=NaN avg60=0 avg300=0 total=1\n"))
	if err == nil {
		t.Fatal("parseMemoryPSI should reject malformed numbers")
	}
}

func TestProbeLinuxResourceStateReportsMeminfoReadFailure(t *testing.T) {
	prev := resourceReadFile
	t.Cleanup(func() { resourceReadFile = prev })
	resourceReadFile = func(path string) ([]byte, error) {
		return nil, fmt.Errorf("boom")
	}
	_, err := ProbeLinuxResourceState()
	if err == nil || !strings.Contains(err.Error(), "/proc/meminfo") {
		t.Fatalf("ProbeLinuxResourceState error = %v", err)
	}
}

func TestRefreshResourceStateCollectsGoalXRSSFacts(t *testing.T) {
	prev := resourceReadFile
	t.Cleanup(func() { resourceReadFile = prev })

	repo, runDir, cfg, meta := writeGuidanceRunFixture(t)
	seedGuidanceSessionFixture(t, runDir, cfg)
	_ = repo
	if err := RenewControlLease(runDir, "master", meta.RunID, meta.Epoch, time.Minute, "tmux", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease master: %v", err)
	}
	if err := RenewControlLease(runDir, "runtime-host", meta.RunID, meta.Epoch, time.Minute, "process", os.Getpid()); err != nil {
		t.Fatalf("RenewControlLease runtime-host: %v", err)
	}
	if err := seedGuidanceSessionLease(runDir, meta.RunID, meta.Epoch, os.Getpid()); err != nil {
		t.Fatalf("seedGuidanceSessionLease: %v", err)
	}
	resourceReadFile = func(path string) ([]byte, error) {
		switch path {
		case "/proc/meminfo":
			return []byte("MemTotal: 2 kB\nMemAvailable: 1 kB\nSwapTotal: 4 kB\nSwapFree: 3 kB\n"), nil
		case "/proc/sys/vm/swappiness":
			return []byte("10\n"), nil
		case "/proc/pressure/memory":
			return []byte("some avg10=0 avg60=0 avg300=0 total=0\nfull avg10=0 avg60=0 avg300=0 total=0\n"), nil
		case "/sys/fs/cgroup/memory.current", "/sys/fs/cgroup/memory.high", "/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.swap.current", "/sys/fs/cgroup/memory.swap.max":
			return []byte("0\n"), nil
		case "/sys/fs/cgroup/memory.events":
			return []byte("low 0\nhigh 0\nmax 0\noom 0\noom_kill 0\n"), nil
		}
		if strings.HasSuffix(path, "/status") {
			return []byte("Name:\tgoalx\nVmRSS:\t1024 kB\n"), nil
		}
		return nil, os.ErrNotExist
	}

	if err := RefreshResourceState(runDir); err != nil {
		t.Fatalf("RefreshResourceState: %v", err)
	}
	state, err := LoadResourceState(ResourceStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadResourceState: %v", err)
	}
	if state == nil || state.GoalxProcesses == nil {
		t.Fatalf("resource state missing process facts: %+v", state)
	}
	if state.GoalxProcesses.MasterRSSBytes == 0 || state.GoalxProcesses.RuntimeHostRSSBytes == 0 || state.GoalxProcesses.WorkerRSSBytes["session-1"] == 0 {
		t.Fatalf("goalx process facts incomplete: %+v", state.GoalxProcesses)
	}
	if state.GoalxProcesses.TotalGoalXRSSBytes == 0 {
		t.Fatalf("total goalx rss should be populated: %+v", state.GoalxProcesses)
	}
	_ = cfg
}

func seedGuidanceSessionLease(runDir, runID string, epoch, pid int) error {
	return RenewControlLease(runDir, "session-1", runID, epoch, time.Minute, "tmux", pid)
}
