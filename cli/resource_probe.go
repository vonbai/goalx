package cli

import (
	"bufio"
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var resourceReadFile = os.ReadFile

func ProbeLinuxResourceState() (*ResourceState, error) {
	host, err := probeHostResourceFacts()
	if err != nil {
		return nil, err
	}
	psi, err := probePSIResourceFacts()
	if err != nil {
		return nil, err
	}
	cgroup, err := probeCgroupResourceFacts()
	if err != nil {
		return nil, err
	}
	return &ResourceState{
		Version:   1,
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Host:      host,
		PSI:       psi,
		Cgroup:    cgroup,
		GoalxProcesses: &GoalXProcessFacts{
			WorkerRSSBytes: map[string]int64{},
		},
		State:   resourceStateUnknown,
		Reasons: []string{},
	}, nil
}

func RefreshResourceState(runDir string) error {
	state, err := ProbeLinuxResourceState()
	if err != nil {
		return err
	}
	processes, err := collectGoalXProcessFacts(runDir)
	if err != nil {
		return err
	}
	state.GoalxProcesses = processes
	classifyResourceState(state)
	return SaveResourceState(ResourceStatePath(runDir), state)
}

func probeHostResourceFacts() (*ResourceHostFacts, error) {
	data, err := resourceReadFile("/proc/meminfo")
	if err != nil {
		return nil, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	facts, err := parseMeminfo(data)
	if err != nil {
		return nil, err
	}
	swappinessData, err := resourceReadFile("/proc/sys/vm/swappiness")
	if err == nil {
		swappiness, parseErr := parseCgroupIntValue(swappinessData)
		if parseErr != nil {
			return nil, fmt.Errorf("read /proc/sys/vm/swappiness: %w", parseErr)
		}
		facts.Swappiness = swappiness
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read /proc/sys/vm/swappiness: %w", err)
	}
	return facts, nil
}

func probePSIResourceFacts() (*ResourcePSIFacts, error) {
	data, err := resourceReadFile("/proc/pressure/memory")
	if err != nil {
		if os.IsNotExist(err) {
			return &ResourcePSIFacts{}, nil
		}
		return nil, fmt.Errorf("read /proc/pressure/memory: %w", err)
	}
	return parseMemoryPSI(data)
}

func probeCgroupResourceFacts() (*ResourceCgroupFacts, error) {
	readInt := func(path string) (int64, error) {
		data, err := resourceReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return 0, nil
			}
			return 0, err
		}
		return parseCgroupIntValue(data)
	}
	current, err := readInt("/sys/fs/cgroup/memory.current")
	if err != nil {
		return nil, fmt.Errorf("read cgroup memory.current: %w", err)
	}
	high, err := readInt("/sys/fs/cgroup/memory.high")
	if err != nil {
		return nil, fmt.Errorf("read cgroup memory.high: %w", err)
	}
	maxValue, err := readInt("/sys/fs/cgroup/memory.max")
	if err != nil {
		return nil, fmt.Errorf("read cgroup memory.max: %w", err)
	}
	swapCurrent, err := readInt("/sys/fs/cgroup/memory.swap.current")
	if err != nil {
		return nil, fmt.Errorf("read cgroup memory.swap.current: %w", err)
	}
	swapMax, err := readInt("/sys/fs/cgroup/memory.swap.max")
	if err != nil {
		return nil, fmt.Errorf("read cgroup memory.swap.max: %w", err)
	}
	eventsData, err := resourceReadFile("/sys/fs/cgroup/memory.events")
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read cgroup memory.events: %w", err)
	}
	events := &ResourceEventFacts{}
	if len(eventsData) > 0 {
		parsed, err := parseCgroupMemoryEvents(eventsData)
		if err != nil {
			return nil, fmt.Errorf("parse cgroup memory.events: %w", err)
		}
		events = parsed
	}
	return &ResourceCgroupFacts{
		MemoryCurrentBytes:     current,
		MemoryHighBytes:        high,
		MemoryMaxBytes:         maxValue,
		MemorySwapCurrentBytes: swapCurrent,
		MemorySwapMaxBytes:     swapMax,
		Events:                 events,
	}, nil
}

func parseMeminfo(data []byte) (*ResourceHostFacts, error) {
	facts := &ResourceHostFacts{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value, err := parseMeminfoKB(parts[1])
		if err != nil {
			return nil, fmt.Errorf("parse meminfo %s: %w", key, err)
		}
		switch key {
		case "MemTotal":
			facts.MemTotalBytes = value
		case "MemAvailable":
			facts.MemAvailableBytes = value
		case "SwapTotal":
			facts.SwapTotalBytes = value
		case "SwapFree":
			facts.SwapFreeBytes = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func parseMeminfoKB(raw string) (int64, error) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return 0, nil
	}
	value, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, err
	}
	return value * 1024, nil
}

func parseMemoryPSI(data []byte) (*ResourcePSIFacts, error) {
	facts := &ResourcePSIFacts{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		label := fields[0]
		values := map[string]float64{}
		for _, field := range fields[1:] {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) != 2 {
				continue
			}
			value, err := strconv.ParseFloat(parts[1], 64)
			if err != nil {
				return nil, fmt.Errorf("parse psi field %q: %w", field, err)
			}
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("parse psi field %q: invalid non-finite float", field)
			}
			values[parts[0]] = value
		}
		switch label {
		case "some":
			facts.MemorySomeAvg10 = values["avg10"]
			facts.MemorySomeAvg60 = values["avg60"]
			facts.MemorySomeAvg300 = values["avg300"]
		case "full":
			facts.MemoryFullAvg10 = values["avg10"]
			facts.MemoryFullAvg60 = values["avg60"]
			facts.MemoryFullAvg300 = values["avg300"]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return facts, nil
}

func parseCgroupIntValue(data []byte) (int64, error) {
	value := strings.TrimSpace(string(data))
	switch value {
	case "", "max":
		return 0, nil
	default:
		return strconv.ParseInt(value, 10, 64)
	}
}

func parseCgroupMemoryEvents(data []byte) (*ResourceEventFacts, error) {
	events := &ResourceEventFacts{}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("invalid cgroup memory event line %q", line)
		}
		value, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, err
		}
		switch fields[0] {
		case "low":
			events.Low = value
		case "high":
			events.High = value
		case "max":
			events.Max = value
		case "oom":
			events.OOM = value
		case "oom_kill":
			events.OOMKill = value
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func collectGoalXProcessFacts(runDir string) (*GoalXProcessFacts, error) {
	facts := &GoalXProcessFacts{WorkerRSSBytes: map[string]int64{}}
	loadRSS := func(holder string) (int64, error) {
		lease, err := LoadControlLease(ControlLeasePath(runDir, holder))
		if err != nil {
			if os.IsNotExist(err) {
				return 0, nil
			}
			return 0, err
		}
		if lease == nil || lease.PID <= 0 || !processAlive(lease.PID) {
			return 0, nil
		}
		return rssBytesForPID(lease.PID)
	}
	masterRSS, err := loadRSS("master")
	if err != nil {
		return nil, fmt.Errorf("load master rss: %w", err)
	}
	facts.MasterRSSBytes = masterRSS
	runtimeHostRSS, err := loadRSS("runtime-host")
	if err != nil {
		return nil, fmt.Errorf("load runtime-host rss: %w", err)
	}
	facts.RuntimeHostRSSBytes = runtimeHostRSS

	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return nil, err
	}
	for _, idx := range indexes {
		sessionName := SessionName(idx)
		rss, err := loadRSS(sessionName)
		if err != nil {
			return nil, fmt.Errorf("load %s rss: %w", sessionName, err)
		}
		if rss > 0 {
			facts.WorkerRSSBytes[sessionName] = rss
		}
	}
	facts.TotalGoalXRSSBytes = facts.MasterRSSBytes + facts.RuntimeHostRSSBytes
	for _, rss := range facts.WorkerRSSBytes {
		facts.TotalGoalXRSSBytes += rss
	}
	return facts, nil
}

func rssBytesForPID(pid int) (int64, error) {
	data, err := resourceReadFile(filepath.Join("/proc", strconv.Itoa(pid), "status"))
	if err != nil {
		return 0, err
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		value, err := parseMeminfoKB(strings.TrimPrefix(line, "VmRSS:"))
		if err != nil {
			return 0, err
		}
		return value, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	return 0, nil
}
