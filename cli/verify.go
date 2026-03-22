package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Verify executes the run's acceptance command and records the result.
func Verify(projectRoot string, args []string) error {
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
	if _, err := EnsureRunMetadata(rc.RunDir, projectRoot, rc.Config.Objective); err != nil {
		return fmt.Errorf("load run metadata: %w", err)
	}
	goalContract, err := EnsureGoalContractState(rc.RunDir, rc.Config.Objective)
	if err != nil {
		return fmt.Errorf("load goal contract: %w", err)
	}
	completion, err := DetectCompletionState(projectRoot, rc.RunDir)
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
		cmd.Dir = projectRoot
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

	evidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := os.WriteFile(evidencePath, output, 0o644); err != nil {
		return fmt.Errorf("write acceptance evidence: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	state.CheckedAt = now
	state.EvidencePath = evidencePath
	exitCode := 0

	if runErr != nil || contractErr != nil || acceptanceErr != nil || completionErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
			exitCode = 124
		} else if runErr != nil {
			exitCode = 1
		}
		if runErr == nil && (contractErr != nil || acceptanceErr != nil || completionErr != nil) {
			exitCode = 3
		}
		state.LastExitCode = &exitCode
		state.Status = acceptanceStatusFailed
		if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
			return fmt.Errorf("save acceptance state: %w", err)
		}
		if err := updateStatusWithAcceptance(filepath.Join(projectRoot, ".goalx", "status.json"), state, contractSummary, completion); err != nil {
			return fmt.Errorf("update status: %w", err)
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
		return contractErr
	}

	state.Status = acceptanceStatusPassed
	state.LastExitCode = &exitCode
	if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
		return fmt.Errorf("save acceptance state: %w", err)
	}
	if err := updateStatusWithAcceptance(filepath.Join(projectRoot, ".goalx", "status.json"), state, contractSummary, completion); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	fmt.Printf("Acceptance passed for run '%s'\n", rc.Name)
	fmt.Printf("  command: %s\n", command)
	fmt.Printf("  evidence: %s\n", evidencePath)
	return nil
}
