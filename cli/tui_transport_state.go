package cli

import "strings"

type TUITransportState string

const (
	TUIStateIdlePrompt     TUITransportState = "idle_prompt"
	TUIStateBufferedInput  TUITransportState = "buffered_input"
	TUIStateQueued         TUITransportState = "queued"
	TUIStateWorking        TUITransportState = "working"
	TUIStateCompacting     TUITransportState = "compacting"
	TUIStateInterrupted    TUITransportState = "interrupted"
	TUIStateProviderDialog TUITransportState = "provider_dialog"
	TUIStateBlank          TUITransportState = "blank"
	TUIStateUnknown        TUITransportState = "unknown"
)

func normalizeTUITransportState(raw string) TUITransportState {
	switch TUITransportState(strings.TrimSpace(raw)) {
	case TUIStateIdlePrompt,
		TUIStateBufferedInput,
		TUIStateQueued,
		TUIStateWorking,
		TUIStateCompacting,
		TUIStateInterrupted,
		TUIStateProviderDialog,
		TUIStateBlank,
		TUIStateUnknown:
		return TUITransportState(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func isAcceptedTUITransportState(raw string) bool {
	switch normalizeTUITransportState(raw) {
	case TUIStateQueued, TUIStateWorking, TUIStateCompacting:
		return true
	default:
		return false
	}
}

func canonicalTransportStateOrUnknown(raw string) string {
	if state := normalizeTUITransportState(raw); state != "" {
		return string(state)
	}
	return string(TUIStateUnknown)
}
