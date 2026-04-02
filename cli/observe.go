package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	if err := refreshDisplayFacts(rc); err != nil {
		return err
	}
	startup, _ := LoadRunStartupState(rc.RunDir, rc.TmuxSession)

	fmt.Printf("## Run: %s — Observe\n\n", rc.Name)
	printStatusControlSummary(rc)

	printObserveStatusSection("### Run runtime state", RunRuntimeStatePath(rc.RunDir))
	printObserveStatusSection("### Run status record", RunStatusPath(rc.RunDir))
	printObserveStatusSection("### Resource state", ResourceStatePath(rc.RunDir))
	if err := printRunAdvisoriesFull(rc); err != nil {
		return err
	}
	printObserveOperationsSection(rc.RunDir)
	printObserveEvolveSection(rc.RunDir)

	if !SessionExistsInRun(rc.RunDir, rc.TmuxSession) {
		fmt.Println("### transport")
		if startup.Launching() {
			fmt.Printf("transport launching (%s)\n", startup.Phase)
		} else {
			fmt.Println("transport degraded (no tmux session)")
		}
		fmt.Println()

		fmt.Println("### master")
		printObserveMasterQueue(rc.RunDir)
		if facts, err := LoadTargetPresenceFact(rc.RunDir, rc.TmuxSession, "master"); err == nil {
			if label := startupTargetObserveLabel("master", facts, startup); label != "" {
				fmt.Println(label)
			}
		} else if label := startupTransportObserveLabel("master", loadTransportTargetFacts(rc.RunDir, "master"), startup); label != "" {
			fmt.Println(label)
		}
		printJournalExcerpt(filepath.Join(rc.RunDir, "master.jsonl"))
		fmt.Println()

		sessionIndexes, err := existingSessionIndexes(rc.RunDir)
		if err != nil {
			return err
		}
		sessionState, _ := EnsureSessionsRuntimeState(rc.RunDir)
		for _, num := range sessionIndexes {
			fmt.Printf("### %s\n", SessionName(num))
			printObserveSessionQueue(rc.RunDir, rc.Config.Name, SessionName(num), sessionState)
			if facts, err := LoadTargetPresenceFact(rc.RunDir, rc.TmuxSession, SessionName(num)); err == nil {
				if label := startupTargetObserveLabel(SessionName(num), facts, startup); label != "" {
					fmt.Println(label)
				}
			} else if label := startupTransportObserveLabel(SessionName(num), loadTransportTargetFacts(rc.RunDir, SessionName(num)), startup); label != "" {
				fmt.Println(label)
			}
			printJournalExcerpt(JournalPath(rc.RunDir, SessionName(num)))
			fmt.Println()
		}
		return nil
	}

	fmt.Println("### master")
	printObserveMasterQueue(rc.RunDir)
	if facts, err := LoadTargetPresenceFact(rc.RunDir, rc.TmuxSession, "master"); err == nil {
		if label := startupTargetObserveLabel("master", facts, startup); label != "" {
			fmt.Println(label)
		} else {
			printPaneCapture(rc.RunDir, rc.TmuxSession, "master")
		}
	} else if label := startupTransportObserveLabel("master", loadTransportTargetFacts(rc.RunDir, "master"), startup); label != "" {
		fmt.Println(label)
	} else {
		printPaneCapture(rc.RunDir, rc.TmuxSession, "master")
	}
	fmt.Println()

	// Sessions
	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	sessionState, _ := EnsureSessionsRuntimeState(rc.RunDir)
	for _, num := range sessionIndexes {
		windowName := sessionWindowName(rc.Config.Name, num)
		fmt.Printf("### %s\n", SessionName(num))
		printObserveSessionQueue(rc.RunDir, rc.Config.Name, SessionName(num), sessionState)
		if facts, err := LoadTargetPresenceFact(rc.RunDir, rc.TmuxSession, SessionName(num)); err == nil {
			if label := startupTargetObserveLabel(SessionName(num), facts, startup); label != "" {
				fmt.Println(label)
			} else {
				printPaneCapture(rc.RunDir, rc.TmuxSession, windowName)
			}
		} else if label := startupTransportObserveLabel(SessionName(num), loadTransportTargetFacts(rc.RunDir, SessionName(num)), startup); label != "" {
			fmt.Println(label)
		} else {
			printPaneCapture(rc.RunDir, rc.TmuxSession, windowName)
		}
		fmt.Println()
	}

	// Check for dynamically added sessions (windows beyond the configured count)
	// by listing all tmux windows
	out, err := tmuxOutputWithSocketDir(resolveRunTmuxSocketDir(rc.ProjectRoot, rc.RunDir, rc.Name), "list-windows", "-t", rc.TmuxSession, "-F", "#{window_name}")
	if err == nil {
		configured := make(map[string]bool)
		configured["master"] = true
		for _, num := range sessionIndexes {
			configured[sessionWindowName(rc.Config.Name, num)] = true
		}
		for _, w := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if w != "" && !configured[w] {
				fmt.Printf("### %s (dynamic)\n", w)
				printPaneCapture(rc.RunDir, rc.TmuxSession, w)
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

func printObserveEvolveSection(runDir string) {
	facts, err := LoadCurrentEvolveFacts(runDir)
	if err != nil || facts == nil {
		return
	}
	parts := []string{
		"frontier_state=" + blankAsUnknown(facts.FrontierState),
		fmt.Sprintf("open_candidate_count=%d", facts.OpenCandidateCount),
	}
	if facts.BestExperimentID != "" {
		parts = append(parts, "best_experiment_id="+facts.BestExperimentID)
	}
	if len(facts.OpenCandidateIDs) > 0 {
		parts = append(parts, "open_candidate_ids="+strings.Join(facts.OpenCandidateIDs, ","))
	}
	if facts.LastStopReasonCode != "" {
		parts = append(parts, "last_stop_reason_code="+facts.LastStopReasonCode)
	}
	if facts.LastManagementEventAt != "" {
		parts = append(parts, "last_management_event_at="+facts.LastManagementEventAt)
	}
	if facts.ManagementGap != "" {
		parts = append(parts, "management_gap="+facts.ManagementGap)
	}
	fmt.Println("### Evolve")
	fmt.Println(strings.Join(parts, " "))
	fmt.Println()
}

func printObserveSessionQueue(runDir, runName, sessionName string, sessionState *SessionsRuntimeState) {
	state := readControlInboxState(ControlInboxPath(runDir, sessionName), SessionCursorPath(runDir, sessionName))
	transport := loadTransportTargetFacts(runDir, sessionName)
	fmt.Printf("Queue: unread=%d cursor=%d/%d", state.Unread, state.LastSeenID, state.LastID)
	if hasTransportFacts(transport) {
		fmt.Print(formatTransportQueueFacts(transport))
	}
	fmt.Println()
	if worktree := sessionWorktreeSurfaceSummary(runDir, runName, sessionName, sessionState); worktree != "" {
		fmt.Printf("Worktree: %s\n", worktree)
	}
	if launch := sessionLaunchFacts(runDir, sessionName); launch != "" {
		fmt.Printf("Launch: %s\n", launch)
	}
	if operations, err := LoadControlOperationsState(ControlOperationsPath(runDir)); err == nil && operations != nil {
		if op, ok := operations.Targets[sessionName]; ok && op.Kind == ControlOperationKindSessionDispatch {
			fmt.Printf("Operation: %s\n", formatOperationDetailLine(sessionName, op))
		}
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
	if lineage := rootWorktreeLineageSummary(runDir); lineage != "" {
		fmt.Printf("Worktree: %s\n", lineage)
	}
	if experiments := formatExperimentSurfaceSummary(runDir); experiments != "" {
		fmt.Printf("Experiments: %s\n", experiments)
	}
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

func printObserveOperationsSection(runDir string) {
	operations, err := LoadControlOperationsState(ControlOperationsPath(runDir))
	if err != nil || operations == nil || len(operations.Targets) == 0 {
		return
	}
	keys := make([]string, 0, len(operations.Targets))
	for key := range operations.Targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Println("### operations")
	for _, key := range keys {
		fmt.Println(formatOperationDetailLine(key, operations.Targets[key]))
	}
	fmt.Println()
}

func printPaneCapture(runDir, tmuxSession, window string) {
	out, err := tmuxOutputWithSocketDir(resolveRunTmuxSocketDir("", runDir, ""),
		"capture-pane",
		"-t", tmuxSession+":"+window,
		"-p", "-S", "-200",
	)
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
