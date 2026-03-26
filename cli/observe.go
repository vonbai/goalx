package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// Observe captures live tmux pane output for all windows in a run.
func Observe(projectRoot string, args []string) error {
	if len(args) == 1 && isHelpToken(args[0]) {
		fmt.Println("usage: goalx observe [NAME]")
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if runName == "" && len(rest) == 1 {
		runName = rest[0]
		rest = nil
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx observe [NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil && runName == "" {
		rc, err = findSingleRunnableRun(projectRoot)
	}
	if err != nil {
		return err
	}

	fmt.Printf("## Run: %s — Observe\n\n", rc.Name)
	printStatusControlSummary(rc)

	printObserveStatusSection("### Run runtime state", RunRuntimeStatePath(rc.RunDir))
	printObserveStatusSection("### Run status record", RunStatusPath(rc.RunDir))

	if !SessionExists(rc.TmuxSession) {
		fmt.Println("### transport")
		fmt.Println("transport degraded (no tmux session)")
		fmt.Println()

		fmt.Println("### master")
		printJournalExcerpt(filepath.Join(rc.RunDir, "master.jsonl"))
		fmt.Println()

		sessionIndexes, err := existingSessionIndexes(rc.RunDir)
		if err != nil {
			return err
		}
		for _, num := range sessionIndexes {
			fmt.Printf("### %s\n", SessionName(num))
			printJournalExcerpt(JournalPath(rc.RunDir, SessionName(num)))
			fmt.Println()
		}
		return nil
	}

	fmt.Println("### master")
	printObserveMasterQueue(rc.RunDir)
	printPaneCapture(rc.TmuxSession, "master")
	fmt.Println()

	// Sessions
	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	for _, num := range sessionIndexes {
		windowName := sessionWindowName(rc.Config.Name, num)
		fmt.Printf("### %s\n", SessionName(num))
		printObserveSessionQueue(rc.RunDir, SessionName(num))
		printPaneCapture(rc.TmuxSession, windowName)
		fmt.Println()
	}

	// Check for dynamically added sessions (windows beyond the configured count)
	// by listing all tmux windows
	out, err := exec.Command("tmux", "list-windows", "-t", rc.TmuxSession, "-F", "#{window_name}").Output()
	if err == nil {
		configured := make(map[string]bool)
		configured["master"] = true
		for _, num := range sessionIndexes {
			configured[sessionWindowName(rc.Config.Name, num)] = true
		}
		for _, w := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if w != "" && !configured[w] {
				fmt.Printf("### %s (dynamic)\n", w)
				printPaneCapture(rc.TmuxSession, w)
				fmt.Println()
			}
		}
	}

	return nil
}

func printObserveStatusSection(title, path string) {
	if strings.TrimSpace(path) == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || len(strings.TrimSpace(string(data))) == 0 {
		return
	}
	fmt.Println(title)
	fmt.Println(strings.TrimSpace(string(data)))
	fmt.Println()
}

func printObserveSessionQueue(runDir, sessionName string) {
	state := readControlInboxState(ControlInboxPath(runDir, sessionName), SessionCursorPath(runDir, sessionName))
	transport := loadTransportTargetFacts(runDir, sessionName)
	fmt.Printf("Queue: unread=%d cursor=%d/%d", state.Unread, state.LastSeenID, state.LastID)
	if hasTransportFacts(transport) {
		fmt.Print(formatTransportQueueFacts(transport))
	}
	fmt.Println()
	if launch := sessionLaunchFacts(runDir, sessionName); launch != "" {
		fmt.Printf("Launch: %s\n", launch)
	}
	printObserveTransportFacts(transport)
}

func printObserveMasterQueue(runDir string) {
	state := readControlInboxState(MasterInboxPath(runDir), MasterCursorPath(runDir))
	transport := loadTransportTargetFacts(runDir, "master")
	fmt.Printf("Queue: unread=%d cursor=%d/%d", state.Unread, state.LastSeenID, state.LastID)
	if hasTransportFacts(transport) {
		fmt.Print(formatTransportQueueFacts(transport))
	}
	fmt.Println()
	printObserveTransportFacts(transport)
}

func printObserveTransportFacts(transport TransportTargetFacts) {
	if transport.TransportState == "" && !transport.InputContainsWake && !transport.QueuedMessageVisible && !transport.WorkingVisible && !transport.ProviderDialogVisible && transport.LastSubmitMode == "" && transport.LastOutputAt == "" && transport.LastSubmitAttemptAt == "" && transport.LastTransportAcceptAt == "" && transport.LastTransportError == "" {
		return
	}
	fmt.Printf("Transport: state=%s", transport.TransportState)
	if transport.InputContainsWake {
		fmt.Printf(" input_contains_wake=true")
	}
	if transport.QueuedMessageVisible {
		fmt.Printf(" queued_message_visible=true")
	}
	if transport.WorkingVisible {
		fmt.Printf(" working_visible=true")
	}
	if transport.ProviderDialogVisible {
		fmt.Printf(" provider_dialog_visible=true")
		fmt.Printf(" provider_dialog_kind=%s", blankAsUnknown(transport.ProviderDialogKind))
		if transport.ProviderDialogHint != "" {
			fmt.Printf(" provider_dialog_hint=%q", transport.ProviderDialogHint)
		}
	}
	if transport.LastSubmitMode != "" {
		fmt.Printf(" submit_mode=%s", transport.LastSubmitMode)
	}
	if transport.LastOutputAt != "" {
		fmt.Printf(" last_output_at=%s", transport.LastOutputAt)
	}
	if transport.LastSubmitAttemptAt != "" {
		fmt.Printf(" submit_at=%s", transport.LastSubmitAttemptAt)
	}
	if transport.LastTransportAcceptAt != "" {
		fmt.Printf(" accepted_at=%s", transport.LastTransportAcceptAt)
	}
	if transport.LastTransportError != "" {
		fmt.Printf(" last_transport_error=%q", transport.LastTransportError)
	}
	fmt.Println()
}

func printPaneCapture(tmuxSession, window string) {
	out, err := exec.Command(
		"tmux", "capture-pane",
		"-t", tmuxSession+":"+window,
		"-p", "-S", "-200",
	).Output()
	if err != nil {
		fmt.Println("(window not found)")
		return
	}

	// Filter empty lines and take last 20
	lines := strings.Split(string(out), "\n")
	var nonEmpty []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			nonEmpty = append(nonEmpty, l)
		}
	}
	if len(nonEmpty) == 0 {
		fmt.Println("(no output)")
		return
	}
	start := 0
	if len(nonEmpty) > 20 {
		start = len(nonEmpty) - 20
	}
	for _, l := range nonEmpty[start:] {
		fmt.Println(l)
	}
}

func printJournalExcerpt(path string) {
	entries, err := goalx.LoadJournal(path)
	if err != nil || len(entries) == 0 {
		fmt.Println("(no journal output)")
		return
	}
	start := 0
	if len(entries) > 5 {
		start = len(entries) - 5
	}
	for _, entry := range entries[start:] {
		desc := strings.TrimSpace(entry.Desc)
		if desc == "" {
			desc = "(no description)"
		}
		if entry.Status != "" {
			fmt.Printf("[%d] %s: %s\n", entry.Round, entry.Status, desc)
			continue
		}
		fmt.Printf("[%d] %s\n", entry.Round, desc)
	}
}
