package cli

import (
	"strings"
	"testing"
	"time"
)

func TestInterventionLogRoundTripsSuccessDeltaEvent(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)

	if err := AppendInterventionEvent(runDir, "user_tell", "user", InterventionEventBody{
		Run:             "guidance-run",
		Message:         "Do not stop at route cutover only.",
		AffectedTargets: []string{"master"},
		Before: InterventionBeforeState{
			ObligationModelHash:         "sha256:goal",
			StatusHash:       "sha256:status",
			CoordinationHash: "sha256:coordination",
			SuccessModelHash: "sha256:success",
		},
	}); err != nil {
		t.Fatalf("AppendInterventionEvent: %v", err)
	}

	events, err := LoadInterventionLog(InterventionLogPath(runDir))
	if err != nil {
		t.Fatalf("LoadInterventionLog: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	event := events[0]
	if event.Kind != "user_tell" || event.Actor != "user" {
		t.Fatalf("event = %+v, want user_tell by user", event)
	}
	if event.Body.Run != "guidance-run" || event.Body.Message != "Do not stop at route cutover only." {
		t.Fatalf("event body = %+v", event.Body)
	}
	if len(event.Body.AffectedTargets) != 1 || event.Body.AffectedTargets[0] != "master" {
		t.Fatalf("affected_targets = %+v, want [master]", event.Body.AffectedTargets)
	}
	if event.Body.Before.ObligationModelHash == "" || event.Body.Before.SuccessModelHash == "" {
		t.Fatalf("before hashes = %+v, want preserved hashes", event.Body.Before)
	}
}

func TestTellAppendsSuccessDeltaInterventionEvent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	installFakePresenceTmux(t, true, "master session-1", "%0\\tmaster\\n%1\\tsession-1\\n")

	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	meta, err := LoadRunMetadata(RunMetadataPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunMetadata: %v", err)
	}
	if err := EnsureSuccessCompilation(repo, runDir, cfg, meta); err != nil {
		t.Fatalf("EnsureSuccessCompilation: %v", err)
	}
	requiredRemaining := 0
	if err := SaveRunStatusRecord(RunStatusPath(runDir), &RunStatusRecord{
		Version:           1,
		Phase:             runStatusPhaseWorking,
		RequiredRemaining: &requiredRemaining,
	}); err != nil {
		t.Fatalf("SaveRunStatusRecord: %v", err)
	}

	orig := sendAgentNudge
	origDetailed := sendAgentNudgeDetailedInRunFunc
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailedInRunFunc = origDetailed }()
	sendAgentNudge = func(target, engine string) error { return nil }
	sendAgentNudgeDetailedInRunFunc = func(_ string, target, engine string) (TransportDeliveryOutcome, error) {
		return TransportDeliveryOutcome{SubmitMode: "payload_enter", TransportState: "queued"}, nil
	}

	if err := Tell(repo, []string{"--run", runName, "session-1", "Do not stop at route cutover only."}); err != nil {
		t.Fatalf("Tell: %v", err)
	}

	events, err := LoadInterventionLog(InterventionLogPath(runDir))
	if err != nil {
		t.Fatalf("LoadInterventionLog: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	event := events[0]
	if event.Kind != "user_redirect" {
		t.Fatalf("kind = %q, want user_redirect", event.Kind)
	}
	if event.Body.Run != runName || event.Body.Message != "Do not stop at route cutover only." {
		t.Fatalf("event body = %+v", event.Body)
	}
	if len(event.Body.AffectedTargets) != 1 || event.Body.AffectedTargets[0] != "session-1" {
		t.Fatalf("affected_targets = %+v, want [session-1]", event.Body.AffectedTargets)
	}
	if event.Body.Before.ObligationModelHash == "" {
		t.Fatalf("goal hash missing: %+v", event.Body.Before)
	}
	if event.Body.Before.StatusHash == "" {
		t.Fatalf("status hash missing: %+v", event.Body.Before)
	}
	if event.Body.Before.CoordinationHash == "" {
		t.Fatalf("coordination hash missing: %+v", event.Body.Before)
	}
	if event.Body.Before.SuccessModelHash == "" {
		t.Fatalf("success model hash missing: %+v", event.Body.Before)
	}
}

func TestBudgetMutationAppendsStructuredInterventionEvent(t *testing.T) {
	repo, runDir, cfg, _ := writeGuidanceRunFixture(t)
	cfg.Budget.MaxDuration = time.Hour
	if err := SaveRunSpec(runDir, cfg); err != nil {
		t.Fatalf("SaveRunSpec: %v", err)
	}

	if err := Budget(repo, []string{"--run", cfg.Name, "--extend", "2h"}); err != nil {
		t.Fatalf("Budget: %v", err)
	}

	events, err := LoadInterventionLog(InterventionLogPath(runDir))
	if err != nil {
		t.Fatalf("LoadInterventionLog: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events len = %d, want 1", len(events))
	}
	event := events[0]
	if event.Kind != "budget_extend" {
		t.Fatalf("kind = %q, want budget_extend", event.Kind)
	}
	if event.Body.BudgetAction != "extend" {
		t.Fatalf("budget_action = %q, want extend", event.Body.BudgetAction)
	}
	if event.Body.BudgetBeforeSeconds != int64(time.Hour/time.Second) {
		t.Fatalf("budget_before_seconds = %d, want %d", event.Body.BudgetBeforeSeconds, int64(time.Hour/time.Second))
	}
	if event.Body.BudgetAfterSeconds != int64((3*time.Hour)/time.Second) {
		t.Fatalf("budget_after_seconds = %d, want %d", event.Body.BudgetAfterSeconds, int64((3*time.Hour)/time.Second))
	}
	if !strings.Contains(event.Body.Message, "3h0m0s") {
		t.Fatalf("message = %q, want new budget summary", event.Body.Message)
	}
}
