package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
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

	state, err := EnsureAcceptanceState(rc.RunDir, rc.Config)
	if err != nil {
		return fmt.Errorf("load acceptance state: %w", err)
	}
	if _, err := EnsureRunMetadata(rc.RunDir, rc.ProjectRoot, rc.Config.Objective); err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	goalContract, err := EnsureGoalContractState(rc.RunDir, rc.Config.Objective)
	if err != nil {
		return fmt.Errorf("load goal contract: %w", err)
	}
	completion, err := DetectCompletionState(rc.ProjectRoot, rc.RunDir, goalContract, state)
	if err != nil {
		return fmt.Errorf("detect completion state: %w", err)
	}
	if err := SaveCompletionState(CompletionStatePath(rc.RunDir), completion); err != nil {
		return fmt.Errorf("save completion state: %w", err)
	}
	acceptanceErr := ValidateAcceptanceStateForVerification(state, goalContract)
	command := strings.TrimSpace(state.Command)
	if command == "" {
		return fmt.Errorf("no acceptance command configured")
	}

	timeout := rc.Config.Acceptance.Timeout
	if state.CommandSource == "harness" && timeout <= 0 {
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
	if acceptanceErr == nil {
		cmd := exec.CommandContext(ctx, "bash", "-lc", command)
		cmd.Dir = rc.ProjectRoot
		output, runErr = cmd.CombinedOutput()
	}
	contractSummary, contractErr := ValidateGoalContractForCompletion(goalContract)
	if contractErr != nil {
		output = append(output, []byte("\n[goal-contract]\n"+contractErr.Error()+"\n")...)
	}
	if acceptanceErr != nil {
		output = append(output, []byte("\n[acceptance]\n"+acceptanceErr.Error()+"\n")...)
	}
	completionErr := ValidateGoalContractAgainstCompletion(goalContract, completion)
	if completionErr != nil {
		output = append(output, []byte("\n[completion]\n"+completionErr.Error()+"\n")...)
	}
	proofErr := ValidateCompletionStateForVerification(rc.ProjectRoot, completion, goalContract, state)
	if proofErr != nil {
		output = append(output, []byte("\n[proof]\n"+proofErr.Error()+"\n")...)
	}

	evidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := os.WriteFile(evidencePath, output, 0o644); err != nil {
		return fmt.Errorf("write acceptance evidence: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	state.CheckedAt = now
	state.EvidencePath = evidencePath
	exitCode := 0

	if runErr != nil || contractErr != nil || acceptanceErr != nil || completionErr != nil || proofErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
			exitCode = 124
		} else if runErr != nil {
			exitCode = 1
		}
		if runErr == nil && (contractErr != nil || acceptanceErr != nil || completionErr != nil || proofErr != nil) {
			exitCode = 3
		}
		state.LastExitCode = &exitCode
		state.Status = acceptanceStatusFailed
		if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
			return fmt.Errorf("save acceptance state: %w", err)
		}
		completion.AcceptanceStatus = state.Status
		completion.AcceptanceCheckedAt = state.CheckedAt
		completion.AcceptanceEvidence = state.EvidencePath
		if err := SaveCompletionState(CompletionStatePath(rc.RunDir), completion); err != nil {
			return fmt.Errorf("save completion state: %w", err)
		}
		if err := updateRunVerificationState(rc.ProjectRoot, rc.RunDir, rc.Config, state, contractSummary, completion); err != nil {
			return fmt.Errorf("update run verification state: %w", err)
		}
		if runErr != nil {
			return fmt.Errorf("acceptance command failed (%d): %w", exitCode, runErr)
		}
		if acceptanceErr != nil {
			return acceptanceErr
		}
		if completionErr != nil {
			return completionErr
		}
		if proofErr != nil {
			return proofErr
		}
		return contractErr
	}

	state.Status = acceptanceStatusPassed
	state.LastExitCode = &exitCode
	if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
		return fmt.Errorf("save acceptance state: %w", err)
	}
	completion.AcceptanceStatus = state.Status
	completion.AcceptanceCheckedAt = state.CheckedAt
	completion.AcceptanceEvidence = state.EvidencePath
	if err := SaveCompletionState(CompletionStatePath(rc.RunDir), completion); err != nil {
		return fmt.Errorf("save completion state: %w", err)
	}
	if err := updateRunVerificationState(rc.ProjectRoot, rc.RunDir, rc.Config, state, contractSummary, completion); err != nil {
		return fmt.Errorf("update run verification state: %w", err)
	}

	fmt.Printf("Acceptance passed for run '%s'\n", rc.Name)
	fmt.Printf("  command: %s\n", command)
	fmt.Printf("  evidence: %s\n", evidencePath)
	return nil
}
