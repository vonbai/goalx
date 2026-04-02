package cli

import "testing"

func TestLookupEngineMemoryProfile(t *testing.T) {
	profile, ok := lookupEngineMemoryProfile("codex", "gpt-5.4")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup codex/gpt-5.4 = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("codex", "gpt-5.4-mini")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup codex/gpt-5.4-mini = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("claude-code", "sonnet")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup claude-code/sonnet = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("claude-code", "opus")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup claude-code/opus = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("codex", "best")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup codex/best = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("codex", "fast")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup codex/fast = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("claude-code", "claude-opus-4-6")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup claude-code/claude-opus-4-6 = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("claude-code", "haiku")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup claude-code/haiku = %+v, %t", profile, ok)
	}
	profile, ok = lookupEngineMemoryProfile("claude-code", "claude-haiku-4-5")
	if !ok || profile.ExpectedRSSBytes <= 0 {
		t.Fatalf("lookup claude-code/claude-haiku-4-5 = %+v, %t", profile, ok)
	}
	if _, ok := lookupEngineMemoryProfile("mystery", "ghost"); ok {
		t.Fatal("unexpected profile for unknown engine/model")
	}
}

func TestClassifyResourceStateHealthy(t *testing.T) {
	state := &ResourceState{
		Version: 1,
		Host: &ResourceHostFacts{
			MemAvailableBytes: 20 * 1024 * 1024 * 1024,
		},
		PSI: &ResourcePSIFacts{},
		Cgroup: &ResourceCgroupFacts{
			Events: &ResourceEventFacts{},
		},
		GoalxProcesses: &GoalXProcessFacts{WorkerRSSBytes: map[string]int64{}},
		State:          resourceStateUnknown,
	}
	classifyResourceState(state)
	if state.State != resourceStateHealthy {
		t.Fatalf("state = %+v, want healthy", state)
	}
}

func TestClassifyResourceStateTight(t *testing.T) {
	state := &ResourceState{
		Version: 1,
		Host: &ResourceHostFacts{
			MemAvailableBytes: 3 * 1024 * 1024 * 1024,
		},
		PSI: &ResourcePSIFacts{},
		Cgroup: &ResourceCgroupFacts{
			Events: &ResourceEventFacts{},
		},
		GoalxProcesses: &GoalXProcessFacts{WorkerRSSBytes: map[string]int64{}},
		State:          resourceStateUnknown,
	}
	classifyResourceState(state)
	if state.State != resourceStateTight {
		t.Fatalf("state = %+v, want tight", state)
	}
}

func TestClassifyResourceStateCritical(t *testing.T) {
	state := &ResourceState{
		Version: 1,
		Host: &ResourceHostFacts{
			MemAvailableBytes: 8 * 1024 * 1024 * 1024,
		},
		PSI: &ResourcePSIFacts{
			MemoryFullAvg10: 3.0,
		},
		Cgroup: &ResourceCgroupFacts{
			Events: &ResourceEventFacts{},
		},
		GoalxProcesses: &GoalXProcessFacts{WorkerRSSBytes: map[string]int64{}},
		State:          resourceStateUnknown,
	}
	classifyResourceState(state)
	if state.State != resourceStateCritical {
		t.Fatalf("state = %+v, want critical", state)
	}
}

func TestDeriveResourceHeadroomIncludesDiscountedSwapCreditWhenPressureLow(t *testing.T) {
	state := &ResourceState{
		Host: &ResourceHostFacts{
			MemAvailableBytes: 1 * 1024 * 1024 * 1024,
			SwapFreeBytes:     16 * 1024 * 1024 * 1024,
			Swappiness:        10,
		},
		PSI: &ResourcePSIFacts{},
		Cgroup: &ResourceCgroupFacts{
			Events: &ResourceEventFacts{},
		},
	}
	if got, want := deriveResourceHeadroom(state), int64(5*1024*1024*1024); got != want {
		t.Fatalf("deriveResourceHeadroom = %d, want %d", got, want)
	}
}

func TestDeriveResourceHeadroomIgnoresSwapCreditUnderPressure(t *testing.T) {
	state := &ResourceState{
		Host: &ResourceHostFacts{
			MemAvailableBytes: 1 * 1024 * 1024 * 1024,
			SwapFreeBytes:     16 * 1024 * 1024 * 1024,
			Swappiness:        10,
		},
		PSI: &ResourcePSIFacts{
			MemorySomeAvg10: resourcePSISomeTightThreshold,
		},
		Cgroup: &ResourceCgroupFacts{
			Events: &ResourceEventFacts{},
		},
	}
	if got, want := deriveResourceHeadroom(state), int64(1*1024*1024*1024); got != want {
		t.Fatalf("deriveResourceHeadroom = %d, want %d", got, want)
	}
}
