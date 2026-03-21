package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const maxAutoIterations = 5

var (
	autoInit              = Init
	autoStart             = Start
	autoSave              = Save
	autoKeep              = Keep
	autoDrop              = Drop
	autoPollUntilComplete = pollUntilComplete
	autoHTTPClient        = &http.Client{Timeout: 10 * time.Second}
)

// statusJSON matches the structure master writes to .goalx/status.json
type statusJSON struct {
	Phase          string `json:"phase"`
	Recommendation string `json:"recommendation"`
	Heartbeat      int    `json:"heartbeat"`
	AcceptanceMet  bool   `json:"acceptance_met"`
	KeepSession    string `json:"keep_session"`
	NextObjective  string `json:"next_objective"`
}

type autoCompletionPayload struct {
	Event          string `json:"event"`
	Run            string `json:"run"`
	Objective      string `json:"objective,omitempty"`
	Phase          string `json:"phase"`
	Recommendation string `json:"recommendation"`
	AcceptanceMet  bool   `json:"acceptance_met"`
	KeepSession    string `json:"keep_session,omitempty"`
	NextObjective  string `json:"next_objective,omitempty"`
	CompletedAt    string `json:"completed_at"`
}

// Auto runs the full goalx pipeline as a goal-driven loop (max 5 iterations).
// Each iteration: init+start → poll → save → read recommendation → route.
func Auto(projectRoot string, args []string) (err error) {
	statusPath := filepath.Join(projectRoot, ".goalx", "status.json")
	initArgs := append([]string(nil), args...) // first iteration uses the user's original args
	if len(initArgs) > 0 && !hasMode(initArgs) {
		initArgs = append(initArgs[:1:1], append([]string{"--research"}, initArgs[1:]...)...)
	}
	needsInit := true
	var finalStatus *statusJSON

	defer func() {
		if err != nil || finalStatus == nil {
			return
		}
		if notifyErr := notifyAutoCompletion(projectRoot, finalStatus); notifyErr != nil {
			fmt.Fprintf(os.Stderr, "warning: completion webhook failed: %v\n", notifyErr)
		}
	}()

	for i := 0; i < maxAutoIterations; i++ {
		fmt.Printf("\n=== auto iteration %d/%d ===\n", i+1, maxAutoIterations)

		// Init + Start
		if needsInit {
			if err := autoInit(projectRoot, initArgs); err != nil {
				return fmt.Errorf("init (iter %d): %w", i, err)
			}
		}
		if err := autoStart(projectRoot, nil); err != nil {
			return fmt.Errorf("start (iter %d): %w", i, err)
		}

		// Poll until complete
		fmt.Println("Waiting for run to complete...")
		status, err := autoPollUntilComplete(statusPath, 30*time.Second, 4*time.Hour)
		if err != nil {
			return fmt.Errorf("poll (iter %d): %w", i, err)
		}
		finalStatus = status

		// Save
		if err := autoSave(projectRoot, nil); err != nil {
			return fmt.Errorf("save (iter %d): %w", i, err)
		}

		// Keep session if master requested it
		if status.KeepSession != "" {
			fmt.Printf("Keeping session %s...\n", status.KeepSession)
			if err := autoKeep(projectRoot, []string{status.KeepSession}); err != nil {
				fmt.Fprintf(os.Stderr, "warning: keep failed: %v\n", err)
			}
		}

		// Drop the completed run
		if err := autoDrop(projectRoot, nil); err != nil {
			fmt.Fprintf(os.Stderr, "warning: drop failed: %v\n", err)
		}

		rec := status.Recommendation
		fmt.Printf("Master recommendation: %s (acceptance_met=%v)\n", rec, status.AcceptanceMet)

		// Terminal conditions
		if status.AcceptanceMet || rec == "done" {
			fmt.Println("Objective achieved. Results saved.")
			if err := notifyAutoCompletion(projectRoot, status); err != nil {
				fmt.Fprintf(os.Stderr, "warning: notify failed: %v\n", err)
			}
			return nil
		}

		// Route to next iteration
		switch rec {
		case "debate":
			fmt.Println("Starting debate round...")
			if err := Debate(projectRoot, nil); err != nil {
				return fmt.Errorf("debate (iter %d): %w", i, err)
			}
			needsInit = false

		case "implement":
			fmt.Println("Starting implementation...")
			if err := Implement(projectRoot, nil); err != nil {
				return fmt.Errorf("implement (iter %d): %w", i, err)
			}
			needsInit = false

		case "more-research":
			obj := status.NextObjective
			if obj == "" {
				fmt.Println("more-research recommended but no next_objective provided. Stopping.")
				return nil
			}
			fmt.Printf("Re-initializing with new objective: %s\n", obj)
			initArgs = []string{obj, "--research"}
			needsInit = true

		default:
			fmt.Printf("Unknown recommendation %q. Stopping.\n", rec)
			return nil
		}
	}

	fmt.Printf("Reached max iterations (%d). Stopping.\n", maxAutoIterations)
	return nil
}

func notifyAutoCompletion(projectRoot string, status *statusJSON) error {
	cfg, _, err := goalx.LoadConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	url := strings.TrimSpace(cfg.Serve.NotificationURL)
	if url == "" {
		return nil
	}

	body, err := json.Marshal(autoCompletionPayload{
		Event:          "goalx.auto.complete",
		Run:            cfg.Name,
		Objective:      cfg.Objective,
		Phase:          status.Phase,
		Recommendation: status.Recommendation,
		AcceptanceMet:  status.AcceptanceMet,
		KeepSession:    status.KeepSession,
		NextObjective:  status.NextObjective,
		CompletedAt:    time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "goalx/auto-webhook")

	resp, err := autoHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	msg = bytes.TrimSpace(msg)
	if len(msg) == 0 {
		return fmt.Errorf("status %s", resp.Status)
	}
	return fmt.Errorf("status %s: %s", resp.Status, string(msg))
}

func hasMode(args []string) bool {
	for _, arg := range args {
		if arg == "--research" || arg == "--develop" {
			return true
		}
	}
	return false
}

// pollUntilComplete reads status.json every interval until phase=complete or timeout.
func pollUntilComplete(statusPath string, interval, timeout time.Duration) (*statusJSON, error) {
	deadline := time.Now().Add(timeout)
	lastHB := -1

	for time.Now().Before(deadline) {
		data, err := os.ReadFile(statusPath)
		if err == nil && len(data) > 0 {
			var s statusJSON
			if json.Unmarshal(data, &s) == nil {
				if s.Heartbeat != lastHB {
					fmt.Printf("  heartbeat %d -- phase: %s\n", s.Heartbeat, s.Phase)
					lastHB = s.Heartbeat
				}
				if s.Phase == "complete" {
					return &s, nil
				}
			}
		}
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("timeout after %v waiting for completion", timeout)
}

// duplicate removed — kept first declaration above
