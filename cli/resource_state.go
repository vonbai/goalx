package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	resourceStateHealthy  = "healthy"
	resourceStateTight    = "tight"
	resourceStateCritical = "critical"
	resourceStateUnknown  = "unknown"
)

type ResourceState struct {
	Version        int                  `json:"version"`
	CheckedAt      string               `json:"checked_at,omitempty"`
	Host           *ResourceHostFacts   `json:"host,omitempty"`
	PSI            *ResourcePSIFacts    `json:"psi,omitempty"`
	Cgroup         *ResourceCgroupFacts `json:"cgroup,omitempty"`
	GoalxProcesses *GoalXProcessFacts   `json:"goalx_processes,omitempty"`
	HeadroomBytes  int64                `json:"headroom_bytes"`
	State          string               `json:"state"`
	Reasons        []string             `json:"reasons,omitempty"`
}

type ResourceHostFacts struct {
	MemTotalBytes     int64 `json:"mem_total_bytes"`
	MemAvailableBytes int64 `json:"mem_available_bytes"`
	SwapTotalBytes    int64 `json:"swap_total_bytes"`
	SwapFreeBytes     int64 `json:"swap_free_bytes"`
	Swappiness        int64 `json:"swappiness,omitempty"`
}

type ResourcePSIFacts struct {
	MemorySomeAvg10  float64 `json:"memory_some_avg10"`
	MemorySomeAvg60  float64 `json:"memory_some_avg60"`
	MemorySomeAvg300 float64 `json:"memory_some_avg300"`
	MemoryFullAvg10  float64 `json:"memory_full_avg10"`
	MemoryFullAvg60  float64 `json:"memory_full_avg60"`
	MemoryFullAvg300 float64 `json:"memory_full_avg300"`
}

type ResourceCgroupFacts struct {
	MemoryCurrentBytes     int64               `json:"memory_current_bytes"`
	MemoryHighBytes        int64               `json:"memory_high_bytes"`
	MemoryMaxBytes         int64               `json:"memory_max_bytes"`
	MemorySwapCurrentBytes int64               `json:"memory_swap_current_bytes"`
	MemorySwapMaxBytes     int64               `json:"memory_swap_max_bytes"`
	Events                 *ResourceEventFacts `json:"events,omitempty"`
}

type ResourceEventFacts struct {
	Low     int64 `json:"low"`
	High    int64 `json:"high"`
	Max     int64 `json:"max"`
	OOM     int64 `json:"oom"`
	OOMKill int64 `json:"oom_kill"`
}

type GoalXProcessFacts struct {
	MasterRSSBytes      int64            `json:"master_rss_bytes"`
	RuntimeHostRSSBytes int64            `json:"runtime_host_rss_bytes"`
	WorkerRSSBytes      map[string]int64 `json:"worker_rss_bytes,omitempty"`
	TotalGoalXRSSBytes  int64            `json:"total_goalx_rss_bytes"`
}

func ResourceStatePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "resource-state.json")
}

func LoadResourceState(path string) (*ResourceState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseResourceState(data)
	if err != nil {
		return nil, fmt.Errorf("parse resource state: %w", err)
	}
	return state, nil
}

func SaveResourceState(path string, state *ResourceState) error {
	if state == nil {
		return fmt.Errorf("resource state is nil")
	}
	if err := validateResourceStateInput(state); err != nil {
		return err
	}
	normalizeResourceState(state)
	return writeJSONFile(path, state)
}

func parseResourceState(data []byte) (*ResourceState, error) {
	var state ResourceState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceResourceState, err)
	}
	if err := validateResourceStateInput(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceResourceState, err)
	}
	normalizeResourceState(&state)
	return &state, nil
}

func validateResourceStateInput(state *ResourceState) error {
	if state == nil {
		return fmt.Errorf("resource state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("resource state version must be positive")
	}
	if state.Host == nil {
		return fmt.Errorf("resource state host is required")
	}
	if state.PSI == nil {
		return fmt.Errorf("resource state psi is required")
	}
	if state.Cgroup == nil {
		return fmt.Errorf("resource state cgroup is required")
	}
	if state.GoalxProcesses == nil {
		return fmt.Errorf("resource state goalx_processes is required")
	}
	switch strings.TrimSpace(state.State) {
	case resourceStateHealthy, resourceStateTight, resourceStateCritical, resourceStateUnknown:
	default:
		return fmt.Errorf("resource state %q is invalid", state.State)
	}
	return nil
}

func normalizeResourceState(state *ResourceState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	state.CheckedAt = strings.TrimSpace(state.CheckedAt)
	state.State = strings.TrimSpace(state.State)
	state.Reasons = compactStrings(state.Reasons)
	if state.GoalxProcesses != nil {
		if state.GoalxProcesses.WorkerRSSBytes == nil {
			state.GoalxProcesses.WorkerRSSBytes = map[string]int64{}
		}
		normalized := make(map[string]int64, len(state.GoalxProcesses.WorkerRSSBytes))
		for rawName, rss := range state.GoalxProcesses.WorkerRSSBytes {
			name := strings.TrimSpace(rawName)
			if name == "" {
				continue
			}
			normalized[name] = rss
		}
		state.GoalxProcesses.WorkerRSSBytes = normalized
	}
	classifyResourceState(state)
}
