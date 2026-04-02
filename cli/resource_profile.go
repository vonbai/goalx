package cli

import "strings"

const (
	resourceHostReserveBytes         int64   = 4 * 1024 * 1024 * 1024
	resourceCriticalReserveBytes     int64   = 2 * 1024 * 1024 * 1024
	resourceMaxSwapCreditBytes       int64   = 4 * 1024 * 1024 * 1024
	resourcePSISomeTightThreshold    float64 = 5.0
	resourcePSIFullTightThreshold    float64 = 1.0
	resourcePSISomeCriticalThreshold float64 = 10.0
	resourcePSIFullCriticalThreshold float64 = 2.5
)

type EngineMemoryProfile struct {
	ExpectedRSSBytes int64
}

var builtinEngineMemoryProfiles = map[string]EngineMemoryProfile{
	"codex/codex":                   {ExpectedRSSBytes: 3 * 1024 * 1024 * 1024},
	"codex/best":                    {ExpectedRSSBytes: 3 * 1024 * 1024 * 1024},
	"codex/balanced":                {ExpectedRSSBytes: 3 * 1024 * 1024 * 1024},
	"codex/gpt-5.4":                 {ExpectedRSSBytes: 3 * 1024 * 1024 * 1024},
	"codex/fast":                    {ExpectedRSSBytes: int64(1.5 * 1024 * 1024 * 1024)},
	"codex/gpt-5.4-mini":            {ExpectedRSSBytes: int64(1.5 * 1024 * 1024 * 1024)},
	"claude-code/sonnet":            {ExpectedRSSBytes: int64(3.5 * 1024 * 1024 * 1024)},
	"claude-code/haiku":             {ExpectedRSSBytes: 2 * 1024 * 1024 * 1024},
	"claude-code/claude-sonnet-4-6": {ExpectedRSSBytes: int64(3.5 * 1024 * 1024 * 1024)},
	"claude-code/opus":              {ExpectedRSSBytes: 6 * 1024 * 1024 * 1024},
	"claude-code/claude-haiku-4-5":  {ExpectedRSSBytes: 2 * 1024 * 1024 * 1024},
	"claude-code/claude-opus-4-6":   {ExpectedRSSBytes: 6 * 1024 * 1024 * 1024},
	"aider/sonnet":                  {ExpectedRSSBytes: int64(3.5 * 1024 * 1024 * 1024)},
	"aider/claude-sonnet-4-6":       {ExpectedRSSBytes: int64(3.5 * 1024 * 1024 * 1024)},
	"aider/opus":                    {ExpectedRSSBytes: 6 * 1024 * 1024 * 1024},
	"aider/claude-opus-4-6":         {ExpectedRSSBytes: 6 * 1024 * 1024 * 1024},
}

func lookupEngineMemoryProfile(engine, model string) (EngineMemoryProfile, bool) {
	key := strings.TrimSpace(engine) + "/" + strings.TrimSpace(model)
	profile, ok := builtinEngineMemoryProfiles[key]
	return profile, ok
}

func classifyResourceState(state *ResourceState) {
	if state == nil {
		return
	}
	headroom := deriveResourceHeadroom(state)
	state.HeadroomBytes = headroom
	reasons := []string{}

	if state.Cgroup != nil && state.Cgroup.Events != nil && state.Cgroup.Events.OOMKill > 0 {
		reasons = append(reasons, "cgroup_oom_kill_detected")
	}
	if state.PSI != nil {
		switch {
		case state.PSI.MemoryFullAvg10 >= resourcePSIFullCriticalThreshold:
			reasons = append(reasons, "psi_memory_full_critical")
		case state.PSI.MemorySomeAvg10 >= resourcePSISomeCriticalThreshold:
			reasons = append(reasons, "psi_memory_some_critical")
		case state.PSI.MemoryFullAvg10 >= resourcePSIFullTightThreshold:
			reasons = append(reasons, "psi_memory_full_tight")
		case state.PSI.MemorySomeAvg10 >= resourcePSISomeTightThreshold:
			reasons = append(reasons, "psi_memory_some_tight")
		}
	}
	switch {
	case headroom <= resourceCriticalReserveBytes:
		reasons = append(reasons, "host_available_below_critical_reserve")
	case headroom <= resourceHostReserveBytes:
		reasons = append(reasons, "host_available_below_reserve")
	}
	if state.Cgroup != nil && state.Cgroup.MemoryHighBytes > 0 && state.Cgroup.MemoryCurrentBytes >= state.Cgroup.MemoryHighBytes {
		reasons = append(reasons, "cgroup_current_near_high")
	}

	state.Reasons = compactStrings(reasons)
	switch {
	case len(reasons) == 0:
		state.State = resourceStateHealthy
	case hasAnyReason(reasons, "cgroup_oom_kill_detected", "psi_memory_full_critical", "psi_memory_some_critical", "host_available_below_critical_reserve"):
		state.State = resourceStateCritical
	default:
		state.State = resourceStateTight
	}
}

func resourceStateNeedsAttention(state *ResourceState) bool {
	if state == nil {
		return false
	}
	if strings.TrimSpace(state.State) == "" || state.State == resourceStateHealthy {
		return len(state.Reasons) > 0
	}
	return true
}

func deriveResourceHeadroom(state *ResourceState) int64 {
	if state == nil || state.Host == nil {
		return 0
	}
	headroom := state.Host.MemAvailableBytes
	if state.Cgroup != nil {
		if state.Cgroup.MemoryMaxBytes > 0 {
			limitHeadroom := state.Cgroup.MemoryMaxBytes - state.Cgroup.MemoryCurrentBytes
			if headroom == 0 || (limitHeadroom >= 0 && limitHeadroom < headroom) {
				headroom = limitHeadroom
			}
		}
		if state.Cgroup.MemoryHighBytes > 0 {
			softHeadroom := state.Cgroup.MemoryHighBytes - state.Cgroup.MemoryCurrentBytes
			if headroom == 0 || (softHeadroom >= 0 && softHeadroom < headroom) {
				headroom = softHeadroom
			}
		}
	}
	return headroom + deriveSwapCredit(state)
}

func deriveSwapCredit(state *ResourceState) int64 {
	if state == nil || state.Host == nil || state.Host.SwapFreeBytes <= 0 {
		return 0
	}
	if state.Host.Swappiness == 0 {
		return 0
	}
	if state.PSI != nil && (state.PSI.MemorySomeAvg10 >= resourcePSISomeTightThreshold || state.PSI.MemoryFullAvg10 >= resourcePSIFullTightThreshold) {
		return 0
	}
	if state.Cgroup != nil && state.Cgroup.MemoryHighBytes > 0 && state.Cgroup.MemoryCurrentBytes >= state.Cgroup.MemoryHighBytes {
		return 0
	}
	swapFree := state.Host.SwapFreeBytes
	if state.Cgroup != nil && state.Cgroup.MemorySwapMaxBytes > 0 {
		cgroupSwapHeadroom := state.Cgroup.MemorySwapMaxBytes - state.Cgroup.MemorySwapCurrentBytes
		if cgroupSwapHeadroom <= 0 {
			return 0
		}
		if cgroupSwapHeadroom < swapFree {
			swapFree = cgroupSwapHeadroom
		}
	}
	credit := swapFree / 4
	if credit > resourceMaxSwapCreditBytes {
		credit = resourceMaxSwapCreditBytes
	}
	if credit < 0 {
		return 0
	}
	return credit
}

func hasAnyReason(reasons []string, values ...string) bool {
	for _, reason := range reasons {
		for _, value := range values {
			if reason == value {
				return true
			}
		}
	}
	return false
}
