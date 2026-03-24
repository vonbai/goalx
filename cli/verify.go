package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

// Verify executes the run's acceptance command and records the result.
func Verify(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx verify [--run NAME]") {
		return nil
	}
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return err
	}
	if len(rest) > 0 {
		return fmt.Errorf("usage: goalx verify [--run NAME]")
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	goalState, err := EnsureGoalState(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load goal state: %w", err)
	}
	state, err := EnsureAcceptanceState(rc.RunDir, rc.Config, goalState.Version)
	if err != nil {
		return fmt.Errorf("load acceptance state: %w", err)
	}
	if _, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective); err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	meta, err := LoadRunMetadata(RunMetadataPath(rc.RunDir))
	if err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	charter, err := RequireRunCharter(rc.RunDir)
	if err != nil {
		return fmt.Errorf("load run charter: %w", err)
	}
	if err := ValidateRunCharterLinkage(meta, charter); err != nil {
		return fmt.Errorf("validate run charter linkage: %w", err)
	}
	if err := ValidateRunCharterCompletionRules(charter); err != nil {
		return fmt.Errorf("validate run charter completion rules: %w", err)
	}

	goalSummary, goalErr := ValidateGoalStateForVerification(goalState)
	acceptanceErr := ValidateAcceptanceStateForVerification(state, goalState)
	command := strings.TrimSpace(state.EffectiveCommand)
	if command == "" {
		return fmt.Errorf("no acceptance command configured")
	}

	timeout := rc.Config.Acceptance.Timeout
	if defaultCommand, source := goalx.ResolveAcceptanceCommandSource(rc.Config); source == "harness" && strings.TrimSpace(defaultCommand) == command && timeout <= 0 {
		timeout = rc.Config.Harness.Timeout
	}

	ctx := context.Background()
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	var output []byte
	var runErr error
	ranCommand := false
	if acceptanceErr == nil {
		ranCommand = true
		cmd := exec.CommandContext(ctx, "bash", "-lc", command)
		cmd.Dir = rc.ProjectRoot
		output, runErr = cmd.CombinedOutput()
	}
	if goalErr != nil {
		output = append(output, []byte("\n[goal]\n"+goalErr.Error()+"\n")...)
	}
	if acceptanceErr != nil {
		output = append(output, []byte("\n[acceptance]\n"+acceptanceErr.Error()+"\n")...)
	}

	evidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := os.WriteFile(evidencePath, output, 0o644); err != nil {
		return fmt.Errorf("write acceptance evidence: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	exitCode := 0
	gateStatus := acceptanceStatusFailed
	switch {
	case runErr == nil && ranCommand:
		gateStatus = acceptanceStatusPassed
	case errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded:
		exitCode = 124
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	default:
		exitCode = 3
	}
	state.LastResult = AcceptanceResult{
		Status:       gateStatus,
		CheckedAt:    now,
		ExitCode:     &exitCode,
		EvidencePath: evidencePath,
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
		return fmt.Errorf("save acceptance state: %w", err)
	}

	completion, err := DetectCompletionState(rc.ProjectRoot, rc.RunDir, goalState, state)
	if err != nil {
		return fmt.Errorf("detect completion state: %w", err)
	}
	if err := SaveCompletionState(CompletionStatePath(rc.RunDir), completion); err != nil {
		return fmt.Errorf("save completion state: %w", err)
	}

	proofErr := ValidateCompletionStateForVerification(rc.ProjectRoot, rc.RunDir, completion, goalState, state)
	if proofErr != nil {
		output = append(output, []byte("\n[proof]\n"+proofErr.Error()+"\n")...)
		if err := os.WriteFile(evidencePath, output, 0o644); err != nil {
			return fmt.Errorf("write acceptance evidence: %w", err)
		}
	}

	if err := updateRunVerificationState(rc.ProjectRoot, rc.RunDir, rc.Config, state, goalSummary, completion); err != nil {
		return fmt.Errorf("update run verification state: %w", err)
	}

	if runErr != nil {
		return fmt.Errorf("acceptance command failed (%d): %w", exitCode, runErr)
	}
	if acceptanceErr != nil {
		return acceptanceErr
	}
	if goalErr != nil {
		return goalErr
	}
	if proofErr != nil {
		return proofErr
	}

	fmt.Printf("Acceptance passed for run '%s'\n", rc.Name)
	fmt.Printf("  command: %s\n", command)
	fmt.Printf("  evidence: %s\n", evidencePath)
	return nil
}
