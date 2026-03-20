package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	goalx "github.com/vonbai/goalx"
)

const maxAutoIterations = 5

// statusJSON matches the structure master writes to .goalx/status.json
type statusJSON struct {
	Phase          string `json:"phase"`
	Recommendation string `json:"recommendation"`
	Heartbeat      int    `json:"heartbeat"`
	AcceptanceMet  bool   `json:"acceptance_met"`
	KeepSession    string `json:"keep_session"`
	NextObjective  string `json:"next_objective"`
}

// Auto runs the full goalx pipeline as a goal-driven loop (max 5 iterations).
// Each iteration: init+start → poll → save → read recommendation → route.
func Auto(projectRoot string, args []string) error {
	statusPath := filepath.Join(projectRoot, ".goalx", "status.json")
	initArgs := args // first iteration uses the user's original args

	for i := 0; i < maxAutoIterations; i++ {
		fmt.Printf("\n=== auto iteration %d/%d ===\n", i+1, maxAutoIterations)

		// Init + Start
		if err := Init(projectRoot, initArgs); err != nil {
			return fmt.Errorf("init (iter %d): %w", i, err)
		}
		if err := Start(projectRoot, nil); err != nil {
			return fmt.Errorf("start (iter %d): %w", i, err)
		}

		// Poll until complete
		fmt.Println("Waiting for run to complete...")
		status, err := pollUntilComplete(statusPath, 30*time.Second, 4*time.Hour)
		if err != nil {
			return fmt.Errorf("poll (iter %d): %w", i, err)
		}

		// Save
		if err := Save(projectRoot, nil); err != nil {
			return fmt.Errorf("save (iter %d): %w", i, err)
		}

		// Keep session if master requested it
		if status.KeepSession != "" {
			fmt.Printf("Keeping session %s...\n", status.KeepSession)
			if err := Keep(projectRoot, []string{status.KeepSession}); err != nil {
				fmt.Fprintf(os.Stderr, "warning: keep failed: %v\n", err)
			}
		}

		// Drop the completed run
		if err := Drop(projectRoot, nil); err != nil {
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
			initArgs = nil // subsequent Start uses goalx.yaml as-is

		case "implement":
			fmt.Println("Starting implementation...")
			if err := Implement(projectRoot, nil); err != nil {
				return fmt.Errorf("implement (iter %d): %w", i, err)
			}
			initArgs = nil

		case "more-research":
			obj := status.NextObjective
			if obj == "" {
				fmt.Println("more-research recommended but no next_objective provided. Stopping.")
				return nil
			}
			fmt.Printf("Re-initializing with new objective: %s\n", obj)
			initArgs = []string{"--research", obj}

		default:
			fmt.Printf("Unknown recommendation %q. Stopping.\n", rec)
			return nil
		}
	}

	fmt.Printf("Reached max iterations (%d). Stopping.\n", maxAutoIterations)
	return nil
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

func notifyAutoCompletion(projectRoot string, status *statusJSON) error {
	cfg, _, err := goalx.LoadConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load config for notification: %w", err)
	}
	if cfg.Serve.NotificationURL == "" {
		return nil
	}

	payload := map[string]any{
		"event":          "auto_complete",
		"run":            cfg.Name,
		"objective":      cfg.Objective,
		"phase":          status.Phase,
		"recommendation": status.Recommendation,
		"acceptance_met": status.AcceptanceMet,
		"keep_session":   status.KeepSession,
		"next_objective": status.NextObjective,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification payload: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(cfg.Serve.NotificationURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("post notification: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notification webhook returned %s", resp.Status)
	}

	return nil
}
