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

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = projectRoot
	output, runErr := cmd.CombinedOutput()

	evidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := os.WriteFile(evidencePath, output, 0o644); err != nil {
		return fmt.Errorf("write acceptance evidence: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	state.CheckedAt = now
	state.EvidencePath = evidencePath
	exitCode := 0

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded {
			exitCode = 124
		} else {
			exitCode = 1
		}
		state.LastExitCode = &exitCode
		state.Status = acceptanceStatusFailed
		if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
			return fmt.Errorf("save acceptance state: %w", err)
		}
		if err := updateStatusWithAcceptance(filepath.Join(projectRoot, ".goalx", "status.json"), state); err != nil {
			return fmt.Errorf("update status: %w", err)
		}
		return fmt.Errorf("acceptance command failed (%d): %w", exitCode, runErr)
	}

	state.Status = acceptanceStatusPassed
	state.LastExitCode = &exitCode
	if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
		return fmt.Errorf("save acceptance state: %w", err)
	}
	if err := updateStatusWithAcceptance(filepath.Join(projectRoot, ".goalx", "status.json"), state); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	fmt.Printf("Acceptance passed for run '%s'\n", rc.Name)
	fmt.Printf("  command: %s\n", command)
	fmt.Printf("  evidence: %s\n", evidencePath)
	return nil
}
