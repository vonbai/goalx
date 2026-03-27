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
// It does not detect completion, validate proof, or update state —
// the master agent reads the recorded result and decides what it means.
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

	command := strings.TrimSpace(state.EffectiveCommand)
	if command == "" {
		return fmt.Errorf("no acceptance command configured")
	}

	timeout := rc.Config.Acceptance.Timeout

	ctx := context.Background()
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = RunWorktreePath(rc.RunDir)
	if info, err := os.Stat(cmd.Dir); err != nil || !info.IsDir() {
		cmd.Dir = rc.ProjectRoot
	}
	output, runErr := cmd.CombinedOutput()

	evidencePath := AcceptanceEvidencePath(rc.RunDir)
	if err := os.WriteFile(evidencePath, output, 0o644); err != nil {
		return fmt.Errorf("write acceptance evidence: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	exitCode := 0
	switch {
	case runErr == nil:
		// exit code 0
	case errors.Is(runErr, context.DeadlineExceeded) || ctx.Err() == context.DeadlineExceeded:
		exitCode = 124
	case runErr != nil:
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	state.LastResult = AcceptanceResult{
		CheckedAt:    now,
		Command:      command,
		ExitCode:     &exitCode,
		EvidencePath: evidencePath,
	}
	if err := SaveAcceptanceState(AcceptanceStatePath(rc.RunDir), state); err != nil {
		return fmt.Errorf("save acceptance state: %w", err)
	}
	if err := AppendMemorySeedFromVerifyResult(rc.RunDir); err != nil {
		return fmt.Errorf("append memory seed from verify result: %w", err)
	}

	if runErr != nil {
		return fmt.Errorf("acceptance command failed (%d): %w", exitCode, runErr)
	}

	fmt.Printf("Acceptance passed for run '%s'\n", rc.Name)
	fmt.Printf("  command: %s\n", command)
	fmt.Printf("  evidence: %s\n", evidencePath)
	return nil
}
