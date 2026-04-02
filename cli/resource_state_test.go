package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveResourceStateRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := ResourceStatePath(runDir)
	state := &ResourceState{
		Version:   1,
		CheckedAt: "2026-04-02T00:00:00Z",
		Host: &ResourceHostFacts{
			MemTotalBytes:     34359738368,
			MemAvailableBytes: 21474836480,
			SwapTotalBytes:    17179869184,
			SwapFreeBytes:     17179869184,
		},
		PSI: &ResourcePSIFacts{
			MemorySomeAvg10:  0,
			MemorySomeAvg60:  0,
			MemorySomeAvg300: 0,
			MemoryFullAvg10:  0,
			MemoryFullAvg60:  0,
			MemoryFullAvg300: 0,
		},
		Cgroup: &ResourceCgroupFacts{
			MemoryCurrentBytes:     0,
			MemoryHighBytes:        0,
			MemoryMaxBytes:         0,
			MemorySwapCurrentBytes: 0,
			MemorySwapMaxBytes:     0,
			Events: &ResourceEventFacts{
				Low: 0, High: 0, Max: 0, OOM: 0, OOMKill: 0,
			},
		},
		GoalxProcesses: &GoalXProcessFacts{
			MasterRSSBytes:      734003200,
			RuntimeHostRSSBytes: 33554432,
			WorkerRSSBytes: map[string]int64{
				"session-1": 3221225472,
			},
			TotalGoalXRSSBytes: 4026531840,
		},
		HeadroomBytes: 17179869184,
		State:         resourceStateHealthy,
		Reasons:       []string{},
	}

	if err := SaveResourceState(path, state); err != nil {
		t.Fatalf("SaveResourceState: %v", err)
	}
	loaded, err := LoadResourceState(path)
	if err != nil {
		t.Fatalf("LoadResourceState: %v", err)
	}
	if loaded == nil || loaded.Host == nil || loaded.PSI == nil || loaded.Cgroup == nil || loaded.GoalxProcesses == nil {
		t.Fatalf("loaded resource state incomplete: %+v", loaded)
	}
	if loaded.State != resourceStateHealthy || loaded.GoalxProcesses.WorkerRSSBytes["session-1"] != 3221225472 {
		t.Fatalf("loaded resource state = %+v", loaded)
	}
}

func TestLoadResourceStateRejectsMissingHost(t *testing.T) {
	path := ResourceStatePath(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "psi": {},
  "cgroup": {},
  "goalx_processes": {},
  "headroom_bytes": 0,
  "state": "healthy"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadResourceState(path)
	if err == nil {
		t.Fatal("LoadResourceState should reject missing host")
	}
	if !strings.Contains(err.Error(), "host is required") {
		t.Fatalf("LoadResourceState error = %v", err)
	}
}

func TestLoadResourceStateRejectsInvalidState(t *testing.T) {
	path := ResourceStatePath(t.TempDir())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "host": {},
  "psi": {},
  "cgroup": {},
  "goalx_processes": {},
  "headroom_bytes": 0,
  "state": "yellow"
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadResourceState(path)
	if err == nil {
		t.Fatal("LoadResourceState should reject invalid state")
	}
	if !strings.Contains(err.Error(), "resource state") {
		t.Fatalf("LoadResourceState error = %v", err)
	}
}
