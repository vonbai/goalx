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
	"gopkg.in/yaml.v3"
)

const maxAutoIterations = 5
const maxNextConfigParallel = 10
const maxNextConfigIterations = 20
const minHeartbeatStallPolls = 10
const defaultHeartbeatCheckInterval = 2 * time.Minute
const heartbeatProgressPollInterval = 10
const heartbeatTmuxCheckPollInterval = 5

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
	autoSessionExists     = SessionExists
	autoPollUntilComplete = func(statusPath string, interval, timeout time.Duration) (*statusJSON, error) {
		return pollUntilCompleteWithHeartbeat(statusPath, interval, timeout, autoPollHeartbeatInterval)
	}
	autoPollHeartbeatInterval = defaultHeartbeatCheckInterval
	autoVerifyHarness         = verifyHarness
	autoHTTPClient            = &http.Client{Timeout: 10 * time.Second}
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
	Parallel       int                 `json:"parallel,omitempty"`
	Engine         string              `json:"engine,omitempty"`
	Model          string              `json:"model,omitempty"`
	Preset         string              `json:"preset,omitempty"`
	DiversityHints []string            `json:"diversity_hints,omitempty"`
	Strategies     []string            `json:"strategies,omitempty"`
	BudgetSeconds  int                 `json:"budget_seconds,omitempty"`
	Objective      string              `json:"objective,omitempty"`
	Harness        string              `json:"harness,omitempty"`
	Mode           string              `json:"mode,omitempty"`
	MaxIterations  int                 `json:"max_iterations,omitempty"`
	Context        []string            `json:"context,omitempty"`
	MasterEngine   string              `json:"master_engine,omitempty"`
	MasterModel    string              `json:"master_model,omitempty"`
	Sessions       []sessionConfigJSON `json:"sessions,omitempty"`
}

type sessionConfigJSON struct {
	Hint   string `json:"hint,omitempty"`
	Engine string `json:"engine,omitempty"`
	Model  string `json:"model,omitempty"`
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

// Auto starts a single master-led run and waits for it to complete.
// The master is responsible for any internal debate/implement transitions.
func Auto(projectRoot string, args []string) (err error) {
	statusPath := filepath.Join(projectRoot, ".goalx", "status.json")
	initArgs := append([]string(nil), args...)
	if len(initArgs) > 0 && !hasMode(initArgs) {
		initArgs = append(initArgs[:1:1], append([]string{"--research"}, initArgs[1:]...)...)
	}
	var finalStatus *statusJSON
	startedAt := time.Now()
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

	if err := autoInit(projectRoot, initArgs); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := autoStart(projectRoot, nil); err != nil {
		return fmt.Errorf("start: %w", err)
	}

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

	fmt.Println("Waiting for run to complete...")
	status, err := autoPollUntilComplete(statusPath, 30*time.Second, pollTimeout)
	if err != nil {
		return fmt.Errorf("poll: %w", err)
	}
	finalStatus = status

	if status.Recommendation == "done" {
		if err := autoVerifyHarness(projectRoot); err != nil {
			return fmt.Errorf("verify harness: %w", err)
		}
	}

	if err := autoSave(projectRoot, nil); err != nil {
		return fmt.Errorf("save: %w", err)
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
		printAutoResults(projectRoot, status, startedAt)
		notifyCompletion()
		return nil
	}

	if err := autoDrop(projectRoot, nil); err != nil {
		fmt.Fprintf(os.Stderr, "warning: drop failed: %v\n", err)
	}
	activeTmuxSession = ""
	switch rec {
	case "debate", "implement", "more-research":
		return fmt.Errorf("auto expects the master to finish within one run; got recommendation %q", rec)
	default:
		return fmt.Errorf("unknown recommendation %q", rec)
	}
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

	sessionIndexes, err := existingSessionIndexes(rc.RunDir)
	if err != nil {
		return err
	}
	for _, num := range sessionIndexes {
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
	validated.Mode = strings.TrimSpace(validated.Mode)
	validated.Context = normalizeNextConfigContext(validated.Context)
	validated.MasterEngine = strings.TrimSpace(validated.MasterEngine)
	validated.MasterModel = strings.TrimSpace(validated.MasterModel)
	validated.Sessions = normalizeNextConfigSessions(validated.Sessions)
	validated.DiversityHints = normalizeNextConfigHints(validated.DiversityHints, 0)
	validated.Strategies = normalizeNextConfigHints(validated.Strategies, 0)

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
	if validated.MaxIterations < 0 || validated.MaxIterations > maxNextConfigIterations {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.max_iterations=%d (must be 1-%d or 0)\n", validated.MaxIterations, maxNextConfigIterations)
		validated.MaxIterations = 0
	}
	if len(validated.DiversityHints) > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: truncating next_config.diversity_hints to %d entries\n", maxNextConfigParallel)
		validated.DiversityHints = validated.DiversityHints[:maxNextConfigParallel]
	}
	if len(validated.Strategies) > maxNextConfigParallel {
		fmt.Fprintf(os.Stderr, "warning: truncating next_config.strategies to %d entries\n", maxNextConfigParallel)
		validated.Strategies = validated.Strategies[:maxNextConfigParallel]
	}

	engines := goalx.BuiltinEngines
	if _, loadedEngines, err := goalx.LoadRawBaseConfig(projectRoot); err == nil && len(loadedEngines) > 0 {
		engines = loadedEngines
	}
	switch validated.Mode {
	case "", string(goalx.ModeResearch), string(goalx.ModeDevelop):
	default:
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.mode=%q (must be research or develop)\n", validated.Mode)
		validated.Mode = ""
	}
	if validated.Engine != "" {
		if _, ok := engines[validated.Engine]; !ok {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.engine=%q (unknown engine)\n", validated.Engine)
			validated.Engine = ""
		}
	}
	if len(validated.Strategies) > 0 {
		if _, err := goalx.ResolveStrategies(validated.Strategies); err != nil {
			fmt.Fprintf(os.Stderr, "warning: ignoring next_config.strategies: %v\n", err)
			validated.Strategies = nil
		}
	}
	validated.MasterEngine, validated.MasterModel = validateNamedEngineModelPair(engines, validated.MasterEngine, validated.MasterModel, "next_config.master")
	for i := range validated.Sessions {
		label := fmt.Sprintf("next_config.sessions[%d]", i)
		validated.Sessions[i].Engine, validated.Sessions[i].Model = validateNamedEngineModelPair(engines, validated.Sessions[i].Engine, validated.Sessions[i].Model, label)
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
	if nc == nil {
		return normalizeNextConfigHints(fallback, parallel)
	}
	strategyHints := nextConfigStrategyHints(nc)
	if len(strategyHints) == 0 && len(nc.DiversityHints) == 0 {
		return normalizeNextConfigHints(fallback, parallel)
	}
	merged := append([]string(nil), strategyHints...)
	merged = append(merged, nc.DiversityHints...)
	return normalizeNextConfigHints(merged, parallel)
}

func normalizeNextConfigHints(hints []string, parallel int) []string {
	if len(hints) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(hints))
	for _, hint := range hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		normalized = append(normalized, hint)
	}
	if len(normalized) == 0 {
		return nil
	}
	if parallel > 0 && len(normalized) > parallel {
		return normalized[:parallel]
	}
	return normalized
}

func nextConfigStrategyHints(nc *nextConfigJSON) []string {
	if nc == nil || len(nc.Strategies) == 0 {
		return nil
	}
	hints, err := goalx.ResolveStrategies(nc.Strategies)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring next_config.strategies: %v\n", err)
		return nil
	}
	return hints
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

func validateNamedEngineModelPair(engines map[string]goalx.EngineConfig, engine, model, label string) (string, string) {
	if strings.TrimSpace(engine) == "" {
		if strings.TrimSpace(model) != "" {
			fmt.Fprintf(os.Stderr, "warning: ignoring %s.model=%q (engine is required)\n", label, model)
		}
		return "", ""
	}
	if _, ok := engines[engine]; !ok {
		fmt.Fprintf(os.Stderr, "warning: ignoring %s.engine=%q (unknown engine)\n", label, engine)
		return "", ""
	}
	if err := validateNextConfigModel(engines, engine, model); err != nil {
		fmt.Fprintf(os.Stderr, "warning: ignoring %s.model=%q for engine %q: %v\n", label, model, engine, err)
		return engine, ""
	}
	return engine, model
}

func normalizeNextConfigContext(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		normalized = append(normalized, path)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeNextConfigSessions(sessions []sessionConfigJSON) []sessionConfigJSON {
	if len(sessions) == 0 {
		return nil
	}

	normalized := make([]sessionConfigJSON, 0, len(sessions))
	for _, session := range sessions {
		session.Hint = strings.TrimSpace(session.Hint)
		session.Engine = strings.TrimSpace(session.Engine)
		session.Model = strings.TrimSpace(session.Model)
		if session.Hint == "" && session.Engine == "" && session.Model == "" {
			continue
		}
		normalized = append(normalized, session)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
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

func applyGeneratedConfigNextConfig(projectRoot string, nc *nextConfigJSON) error {
	if nc == nil {
		return nil
	}

	cfgPath := filepath.Join(projectRoot, ".goalx", "goalx.yaml")
	cfg, err := goalx.LoadYAML[goalx.Config](cfgPath)
	if err != nil {
		return fmt.Errorf("load generated config: %w", err)
	}

	_, engines, err := goalx.LoadRawBaseConfig(projectRoot)
	if err != nil {
		return fmt.Errorf("load base config: %w", err)
	}

	if nc.Preset != "" {
		cfg.Preset = nc.Preset
		goalx.ApplyPreset(&cfg)
	}
	cfg.Parallel = nextConfigParallel(cfg.Parallel, nc)
	cfg.Objective = nextConfigObjective(cfg.Objective, nc)
	cfg.Budget.MaxDuration = nextConfigBudget(cfg.Budget.MaxDuration, nc)
	if nc.Harness != "" {
		cfg.Harness.Command = nc.Harness
	}
	cfg.DiversityHints = nextConfigHints(cfg.DiversityHints, cfg.Parallel, nc)
	cfg.Engine, cfg.Model = resolveNextEngineModel(engines, cfg.Engine, cfg.Model, nc)

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("marshal generated config: %w", err)
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		return fmt.Errorf("write generated config: %w", err)
	}
	return nil
}

// pollUntilComplete reads status.json every interval until phase=complete or timeout.
func pollUntilComplete(statusPath string, interval, timeout time.Duration) (*statusJSON, error) {
	return pollUntilCompleteWithHeartbeat(statusPath, interval, timeout, defaultHeartbeatCheckInterval)
}

func pollUntilCompleteWithHeartbeat(statusPath string, interval, timeout, checkInterval time.Duration) (*statusJSON, error) {
	tmuxSession := tmuxSessionForStatusPath(statusPath)
	deadline := time.Now().Add(timeout)
	startedAt := time.Now()
	lastHB := -1
	stalledPolls := 0
	heartbeatChanges := 0
	stallLimit := heartbeatStallPollLimit(checkInterval, interval)
	pollCount := 0

	for time.Now().Before(deadline) {
		pollCount++
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
				if s.Phase != "complete" && pollCount%heartbeatProgressPollInterval == 0 {
					fmt.Printf("  polling progress -- elapsed: %s phase: %s heartbeat: %d stalled: %d/%d\n", time.Since(startedAt).Round(time.Millisecond), s.Phase, s.Heartbeat, stalledPolls, stallLimit)
				}
				if tmuxSession != "" && s.Phase != "complete" && pollCount%heartbeatTmuxCheckPollInterval == 0 && !autoSessionExists(tmuxSession) {
					return nil, fmt.Errorf("tmux session %s exited while waiting for completion", tmuxSession)
				}
			}
		}
		time.Sleep(interval)
	}
	return nil, fmt.Errorf("timeout after %v waiting for completion", timeout)
}

func tmuxSessionForStatusPath(statusPath string) string {
	goalxDir := filepath.Dir(statusPath)
	if filepath.Base(goalxDir) != ".goalx" {
		return ""
	}

	projectRoot := filepath.Dir(goalxDir)
	cfg, err := goalx.LoadYAML[goalx.Config](filepath.Join(goalxDir, "goalx.yaml"))
	if err != nil || strings.TrimSpace(cfg.Name) == "" {
		return ""
	}
	return goalx.TmuxSessionName(projectRoot, cfg.Name)
}

func heartbeatStallPollLimit(checkInterval, pollInterval time.Duration) int {
	if checkInterval <= 0 {
		checkInterval = defaultHeartbeatCheckInterval
	}
	if pollInterval <= 0 {
		return minHeartbeatStallPolls
	}

	limit := int((checkInterval*8 + pollInterval - 1) / pollInterval)
	if limit < minHeartbeatStallPolls {
		return minHeartbeatStallPolls
	}
	return limit
}
