package cli

import (
	"testing"
)

func TestInterventionLogRoundTripsSuccessDeltaEvent(t *testing.T) {
	_, runDir, _, _ := writeGuidanceRunFixture(t)

	if err := AppendInterventionEvent(runDir, "user_tell", "user", InterventionEventBody{
		Run:             "guidance-run",
		Message:         "Do not stop at route cutover only.",
		AffectedTargets: []string{"master"},
		Before: InterventionBeforeState{
			GoalHash:         "sha256:goal",
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
	if event.Body.Before.GoalHash == "" || event.Body.Before.SuccessModelHash == "" {
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
	origDetailed := sendAgentNudgeDetailed
	defer func() { sendAgentNudge = orig }()
	defer func() { sendAgentNudgeDetailed = origDetailed }()
	sendAgentNudge = func(target, engine string) error { return nil }
	sendAgentNudgeDetailed = func(target, engine string) (TransportDeliveryOutcome, error) {
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
	if event.Body.Before.GoalHash == "" {
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
