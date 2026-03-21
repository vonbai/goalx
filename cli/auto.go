package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

const maxAutoIterations = 5
const maxNextConfigParallel = 10
const minHeartbeatStallPolls = 10
const defaultHeartbeatCheckInterval = 2 * time.Minute

var (
	autoInit              = Init
	autoStart             = Start
	autoSave              = Save
	autoKeep              = Keep
	autoDrop              = Drop
	autoDebate            = Debate
	autoImplement         = Implement
	autoResolveRun        = ResolveRun
	autoKillSession       = KillSession
	autoPollUntilComplete = func(statusPath string, interval, timeout time.Duration) (*statusJSON, error) {
		return pollUntilCompleteWithHeartbeat(statusPath, interval, timeout, autoPollHeartbeatInterval)
	}
	autoPollHeartbeatInterval = defaultHeartbeatCheckInterval
	autoVerifyHarness     = verifyHarness
	autoHTTPClient        = &http.Client{Timeout: 10 * time.Second}
)

// statusJSON matches the structure master writes to .goalx/status.json
type statusJSON struct {
	Phase          string          `json:"phase"`
	Recommendation string          `json:"recommendation"`
	Heartbeat      int             `json:"heartbeat"`
	AcceptanceMet  bool            `json:"acceptance_met"`
	KeepSession    string          `json:"keep_session"`
	NextObjective  string          `json:"next_objective"`
	NextConfig     *nextConfigJSON `json:"next_config,omitempty"`
}

type nextConfigJSON struct {
	Parallel       int      `json:"parallel,omitempty"`
	Engine         string   `json:"engine,omitempty"`
	Model          string   `json:"model,omitempty"`
	Preset         string   `json:"preset,omitempty"`
	DiversityHints []string `json:"diversity_hints,omitempty"`
	Strategies     []string `json:"strategies,omitempty"`
	BudgetSeconds  int      `json:"budget_seconds,omitempty"`
	Objective      string   `json:"objective,omitempty"`
	Harness        string   `json:"harness,omitempty"`
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
	originalArgs := append([]string(nil), args...)
	initArgs := append([]string(nil), args...) // first iteration uses the user's original args
	if len(initArgs) > 0 && !hasMode(initArgs) {
		initArgs = append(initArgs[:1:1], append([]string{"--research"}, initArgs[1:]...)...)
	}
	needsInit := true
	var finalStatus *statusJSON
	var lastPhaseStartedAt time.Time
	notified := false
	activeTmuxSession := ""
	prevHeartbeatInterval := autoPollHeartbeatInterval
	autoPollHeartbeatInterval = defaultHeartbeatCheckInterval

	notifyCompletion := func() {
		if notified || err != nil || finalStatus == nil {
			return
		}
		notified = true
		if notifyErr := notifyAutoCompletion(projectRoot, finalStatus); notifyErr != nil {
			fmt.Fprintf(os.Stderr, "warning: completion webhook failed: %v\n", notifyErr)
		}
	}

	defer func() {
		autoPollHeartbeatInterval = prevHeartbeatInterval
		if err != nil && activeTmuxSession != "" {
			if killErr := autoKillSession(activeTmuxSession); killErr != nil {
				fmt.Fprintf(os.Stderr, "warning: cleanup tmux session %s: %v\n", activeTmuxSession, killErr)
			}
		}
		notifyCompletion()
	}()

	for i := 0; i < maxAutoIterations; i++ {
		fmt.Printf("\n=== auto iteration %d/%d ===\n", i+1, maxAutoIterations)
		lastPhaseStartedAt = time.Now()

		// Init + Start
		if needsInit {
			if err := autoInit(projectRoot, initArgs); err != nil {
				return fmt.Errorf("init (iter %d): %w", i, err)
			}
		}
		if err := autoStart(projectRoot, nil); err != nil {
			return fmt.Errorf("start (iter %d): %w", i, err)
		}
		activeTmuxSession = ""
		autoPollHeartbeatInterval = defaultHeartbeatCheckInterval
		pollTimeout := 4 * time.Hour
		if rc, resolveErr := autoResolveRun(projectRoot, ""); resolveErr == nil && rc != nil {
			activeTmuxSession = rc.TmuxSession
			autoPollHeartbeatInterval = rc.Config.Master.CheckInterval
			if autoPollHeartbeatInterval <= 0 {
				autoPollHeartbeatInterval = defaultHeartbeatCheckInterval
			}
			if rc.Config.Budget.MaxDuration > 0 {
				pollTimeout = rc.Config.Budget.MaxDuration
			}
		}

		// Poll until complete
		fmt.Println("Waiting for run to complete...")
		status, err := autoPollUntilComplete(statusPath, 30*time.Second, pollTimeout)
		if err != nil {
			return fmt.Errorf("poll (iter %d): %w", i, err)
		}
		status.NextConfig = validateNextConfig(projectRoot, status.NextConfig)
		finalStatus = status

		if status.Recommendation == "done" {
			if err := autoVerifyHarness(projectRoot); err != nil {
				return fmt.Errorf("verify harness (iter %d): %w", i, err)
			}
		}

		// Save
		if err := autoSave(projectRoot, nil); err != nil {
			return fmt.Errorf("save (iter %d): %w", i, err)
		}

		rec := status.Recommendation
		fmt.Printf("Master recommendation: %s (acceptance_met=%v)\n", rec, status.AcceptanceMet)

		if rec == "done" {
			if status.KeepSession != "" {
				fmt.Printf("Keeping session %s...\n", status.KeepSession)
				if err := autoKeep(projectRoot, []string{status.KeepSession}); err != nil {
					fmt.Fprintf(os.Stderr, "warning: keep failed: %v\n", err)
				}
			}
			if err := autoDrop(projectRoot, nil); err != nil {
				fmt.Fprintf(os.Stderr, "warning: drop failed: %v\n", err)
			}
			activeTmuxSession = ""
			fmt.Println("Objective achieved. Results saved.")
			printAutoResults(projectRoot, status, lastPhaseStartedAt)
			notifyCompletion()
			return nil
		}

		if err := autoDrop(projectRoot, nil); err != nil {
			fmt.Fprintf(os.Stderr, "warning: drop failed: %v\n", err)
		}
		activeTmuxSession = ""

		// Route to next iteration
		switch rec {
		case "debate":
			fmt.Println("Starting debate round...")
			if err := autoDebate(projectRoot, nil, status.NextConfig); err != nil {
				return fmt.Errorf("debate (iter %d): %w", i, err)
			}
			needsInit = false

		case "implement":
			fmt.Println("Starting implementation...")
			if err := autoImplement(projectRoot, nil, status.NextConfig); err != nil {
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
			initArgs = []string{obj}
			if len(originalArgs) > 1 {
				initArgs = append(initArgs, originalArgs[1:]...)
			}
			if status.NextConfig != nil && status.NextConfig.Parallel > 0 {
				initArgs = append(initArgs, "--parallel", fmt.Sprint(status.NextConfig.Parallel))
			}
			if status.NextConfig != nil && status.NextConfig.Preset != "" {
				initArgs = append(initArgs, "--preset", status.NextConfig.Preset)
			}
			if !hasMode(initArgs) {
				initArgs = append(initArgs, "--research")
			}
			needsInit = true

		default:
			return fmt.Errorf("unknown recommendation %q", rec)
		}
	}

	printAutoResults(projectRoot, finalStatus, lastPhaseStartedAt)
	fmt.Printf("Reached max iterations (%d). Stopping.\n", maxAutoIterations)
	return nil
}

func verifyHarness(projectRoot string) error {
	rc, err := ResolveRun(projectRoot, "")
	if err != nil {
		return fmt.Errorf("resolve run: %w", err)
	}

	command := strings.TrimSpace(rc.Config.Harness.Command)
	if command == "" {
		return nil
	}

	for num := 1; num <= sessionCount(rc.Config); num++ {
		worktreePath := WorktreePath(rc.RunDir, rc.Config.Name, num)
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return fmt.Errorf("stat worktree %s: %w", SessionName(num), err)
		}

		resolved := ResolveHarness(command, worktreePath)
		fmt.Printf("Verifying harness in %s: %s\n", SessionName(num), resolved)
		cmd := exec.Command("sh", "-c", resolved)
		cmd.Dir = worktreePath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("harness failed in %s:\n%s%w", SessionName(num), out, err)
		}
	}

	return nil
}

func printAutoResults(projectRoot string, status *statusJSON, startedAt time.Time) {
	if status == nil {
		return
	}

	cfg, _, err := goalx.LoadConfig(projectRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: load config for results: %v\n", err)
		return
	}

	fmt.Println("=== Results ===")
	if cfg.Mode == goalx.ModeDevelop && status.KeepSession != "" {
		fmt.Printf("Merged %s into main\n", status.KeepSession)
		diffOut, err := exec.Command("git", "-C", projectRoot, "diff", "--stat", "HEAD~1..HEAD").CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: git diff summary failed: %v\n", err)
			return
		}
		fmt.Print(string(diffOut))
		return
	}

	summaryPath, err := resolveAutoSummaryPath(projectRoot, cfg.Name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: resolve summary path: %v\n", err)
	} else if relPath, err := filepath.Rel(projectRoot, summaryPath); err == nil {
		fmt.Printf("Summary: %s\n", filepath.ToSlash(relPath))
		if data, readErr := os.ReadFile(summaryPath); readErr != nil {
			fmt.Fprintf(os.Stderr, "warning: read summary for display: %v\n", readErr)
		} else if summary := strings.TrimSpace(renderResearchSummary(data)); summary != "" {
			fmt.Println(summary)
		}
	}

	duration := time.Since(startedAt).Round(time.Second)
	if duration < 0 {
		duration = 0
	}
	sessions := len(goalx.ExpandSessions(cfg))
	if sessions == 0 {
		sessions = 1
	}
	fmt.Printf("Duration: %s | Sessions: %d | Heartbeats: %d\n", duration, sessions, status.Heartbeat)
	fmt.Printf("Recommendation: %s\n", status.Recommendation)
}

func resolveAutoSummaryPath(projectRoot, runName string) (string, error) {
	if runName != "" {
		path := filepath.Join(projectRoot, ".goalx", "runs", runName, "summary.md")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	_, runDir, err := findLatestSavedRun(filepath.Join(projectRoot, ".goalx", "runs"), "")
	if err != nil {
		return "", err
	}
	return filepath.Join(runDir, "summary.md"), nil
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

func validateNextConfig(projectRoot string, nc *nextConfigJSON) *nextConfigJSON {
	if nc == nil {
		return nil
	}

	validated := *nc
	validated.Engine = strings.TrimSpace(validated.Engine)
	validated.Model = strings.TrimSpace(validated.Model)
	validated.Preset = strings.TrimSpace(validated.Preset)
	validated.Objective = strings.TrimSpace(validated.Objective)
	validated.Harness = strings.TrimSpace(validated.Harness)
	validated.DiversityHints = normalizeNextConfigHints(validated.DiversityHints, 0)

	if validated.Parallel < 0 {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.parallel=%d (must be >= 0)\n", validated.Parallel)
		validated.Parallel = 0
	} else if validated.Parallel > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: capping next_config.parallel=%d to %d\n", validated.Parallel, maxNextConfigParallel)
		validated.Parallel = maxNextConfigParallel
	}
	if validated.BudgetSeconds < 0 {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.budget_seconds=%d (must be >= 0)\n", validated.BudgetSeconds)
		validated.BudgetSeconds = 0
	}
	if len(validated.DiversityHints) > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: truncating next_config.diversity_hints to %d entries\n", maxNextConfigParallel)
		validated.DiversityHints = validated.DiversityHints[:maxNextConfigParallel]
	}

	engines := goalx.BuiltinEngines
	if _, loadedEngines, err := goalx.LoadRawBaseConfig(projectRoot); err == nil && len(loadedEngines) > 0 {
		engines = loadedEngines
	}
	if validated.Engine != "" {
		if _, ok := engines[validated.Engine]; !ok {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.engine=%q (unknown engine)\n", validated.Engine)
			validated.Engine = ""
		}
	}

	return &validated
}

func nextConfigParallel(fallback int, nc *nextConfigJSON) int {
	if nc != nil && nc.Parallel > 0 {
		return nc.Parallel
	}
	return fallback
}

func nextConfigObjective(fallback string, nc *nextConfigJSON) string {
	if nc != nil && nc.Objective != "" {
		return nc.Objective
	}
	return fallback
}

func nextConfigBudget(fallback time.Duration, nc *nextConfigJSON) time.Duration {
	if nc != nil && nc.BudgetSeconds > 0 {
		return time.Duration(nc.BudgetSeconds) * time.Second
	}
	return fallback
}

func nextConfigHints(fallback []string, parallel int, nc *nextConfigJSON) []string {
	if nc == nil || len(nc.DiversityHints) == 0 {
		return normalizeNextConfigHints(fallback, parallel)
	}
	return normalizeNextConfigHints(nc.DiversityHints, parallel)
}

func normalizeNextConfigHints(hints []string, parallel int) []string {
	if len(hints) == 0 {
		return nil
	}

	normalized := make([]string, len(hints))
	for i, hint := range hints {
		normalized[i] = strings.TrimSpace(hint)
	}
	if parallel > 0 && len(normalized) > parallel {
		return normalized[:parallel]
	}
	return normalized
}

func resolveNextEngineModel(engines map[string]goalx.EngineConfig, defaultEngine, defaultModel string, nc *nextConfigJSON) (string, string) {
	if len(engines) == 0 {
		engines = goalx.BuiltinEngines
	}

	engine := defaultEngine
	model := defaultModel
	if nc == nil {
		return engine, model
	}

	if nc.Engine != "" {
		if _, ok := engines[nc.Engine]; ok {
			engine = nc.Engine
		} else {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.engine=%q (unknown engine)\n", nc.Engine)
		}
	}
	if nc.Model != "" {
		if err := validateNextConfigModel(engines, engine, nc.Model); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.model=%q for engine %q: %v\n", nc.Model, engine, err)
		} else {
			model = nc.Model
		}
	}

	return engine, model
}

func validateNextConfigModel(engines map[string]goalx.EngineConfig, engine, model string) error {
	if strings.TrimSpace(model) == "" {
		return nil
	}
	if strings.TrimSpace(engine) == "" {
		return fmt.Errorf("engine is required")
	}
	if modelAliasBelongsToOtherEngine(engines, engine, model) {
		return fmt.Errorf("model alias belongs to a different engine")
	}
	if _, err := goalx.ResolveEngineCommand(engines, engine, model); err != nil {
		return err
	}

	modelID := model
	if ec, ok := engines[engine]; ok {
		if resolved, ok := ec.Models[model]; ok {
			modelID = resolved
		}
	}
	if engine == "codex" && (modelID == "gpt-5.3-codex" || modelID == "gpt-5.2") {
		return fmt.Errorf("model resolves to an interactive Codex migration target")
	}
	return nil
}

func modelAliasBelongsToOtherEngine(engines map[string]goalx.EngineConfig, engine, model string) bool {
	ec, ok := engines[engine]
	if !ok {
		return false
	}
	if _, ok := ec.Models[model]; ok {
		return false
	}
	for otherEngine, other := range engines {
		if otherEngine == engine {
			continue
		}
		if _, ok := other.Models[model]; ok {
			return true
		}
	}
	return false
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
	return pollUntilCompleteWithHeartbeat(statusPath, interval, timeout, defaultHeartbeatCheckInterval)
}

func pollUntilCompleteWithHeartbeat(statusPath string, interval, timeout, checkInterval time.Duration) (*statusJSON, error) {
	deadline := time.Now().Add(timeout)
	lastHB := -1
	stalledPolls := 0
	heartbeatChanges := 0
	stallLimit := heartbeatStallPollLimit(checkInterval, interval)

	for time.Now().Before(deadline) {
		data, err := os.ReadFile(statusPath)
		if err == nil && len(data) > 0 {
			var s statusJSON
			if json.Unmarshal(data, &s) == nil {
				if s.Heartbeat != lastHB {
					fmt.Printf("  heartbeat %d -- phase: %s\n", s.Heartbeat, s.Phase)
					lastHB = s.Heartbeat
					heartbeatChanges++
					stalledPolls = 0
				} else if heartbeatChanges >= 2 {
					stalledPolls++
					if s.Phase != "complete" && stalledPolls >= stallLimit {
						return nil, fmt.Errorf("heartbeat stalled at %d while phase=%s", s.Heartbeat, s.Phase)
					}
				}
				if s.Phase == "complete" && s.Recommendation != "" {
					return &s, nil
				}
			}
		}
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("timeout after %v waiting for completion", timeout)
}

func heartbeatStallPollLimit(checkInterval, pollInterval time.Duration) int {
	if checkInterval <= 0 {
		checkInterval = defaultHeartbeatCheckInterval
	}
	if pollInterval <= 0 {
		return minHeartbeatStallPolls
	}

	limit := int((checkInterval*4 + pollInterval - 1) / pollInterval)
	if limit < minHeartbeatStallPolls {
		return minHeartbeatStallPolls
	}
	return limit
}
