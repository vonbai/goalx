package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

// Verify executes the run's assurance lane and records the result.
// It does not detect completion, validate proof, or update state —
// the master agent reads the recorded result and decides what it means.
func Verify(projectRoot string, args []string) error {
	if printUsageIfHelp(args, "usage: goalx verify [--run NAME] [--lane quick|required|full]") {
		return nil
	}
	runName, lane, err := parseVerifyArgs(args)
	if err != nil {
		return err
	}

	rc, err := ResolveRun(projectRoot, runName)
	if err != nil {
		return err
	}

	if plan, err := LoadAssurancePlan(AssurancePlanPath(rc.RunDir)); err != nil {
		return fmt.Errorf("load assurance plan: %w", err)
	} else if plan != nil && len(plan.Scenarios) > 0 {
		return verifyAssurancePlan(rc.ProjectRoot, rc.RunDir, plan, lane)
	} else if plan == nil {
		bootstrapPlan, err := EnsureAssurancePlan(rc.RunDir, NewAcceptanceState(rc.Config, 0))
		if err != nil {
			return fmt.Errorf("ensure assurance plan: %w", err)
		}
		if bootstrapPlan != nil && len(bootstrapPlan.Scenarios) > 0 {
			return verifyAssurancePlan(rc.ProjectRoot, rc.RunDir, bootstrapPlan, lane)
		}
	}
	return fmt.Errorf("no assurance scenarios configured")
}

func intPtr(v int) *int {
	return &v
}

func parseVerifyArgs(args []string) (runName, lane string, err error) {
	runName, rest, err := extractRunFlag(args)
	if err != nil {
		return "", "", err
	}
	lane = "required"
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case "--lane":
			if i+1 >= len(rest) {
				return "", "", fmt.Errorf("usage: goalx verify [--run NAME] [--lane quick|required|full]")
			}
			lane = strings.TrimSpace(rest[i+1])
			i++
		default:
			return "", "", fmt.Errorf("usage: goalx verify [--run NAME] [--lane quick|required|full]")
		}
	}
	switch lane {
	case "quick", "required", "full":
	default:
		return "", "", fmt.Errorf("usage: goalx verify [--run NAME] [--lane quick|required|full]")
	}
	return runName, lane, nil
}

func verifyAssurancePlan(projectRoot, runDir string, plan *AssurancePlan, lane string) error {
	selected := selectAssuranceScenarios(plan, lane)
	if len(selected) == 0 {
		return fmt.Errorf("no assurance scenarios configured for lane %s", lane)
	}
	exitCode := 0
	for _, scenario := range selected {
		result, err := executeAssuranceScenario(projectRoot, runDir, scenario)
		if err != nil {
			return err
		}
		if err := AppendEvidenceLogEvent(EvidenceLogPath(runDir), "scenario.executed", "master", result); err != nil {
			return fmt.Errorf("append evidence log: %w", err)
		}
		if code, ok := result.OracleResult["exit_code"].(int); ok && exitCode == 0 && code != 0 {
			exitCode = code
		}
	}
	if err := AppendMemorySeedFromVerifyResult(runDir); err != nil {
		return fmt.Errorf("append memory seed from verify result: %w", err)
	}
	if exitCode != 0 {
		return fmt.Errorf("assurance scenarios failed (%d)", exitCode)
	}
	fmt.Printf("Assurance passed for lane '%s'\n", lane)
	fmt.Printf("  scenarios: %d\n", len(selected))
	fmt.Printf("  evidence log: %s\n", EvidenceLogPath(runDir))
	return nil
}

func selectAssuranceScenarios(plan *AssurancePlan, lane string) []AssuranceScenario {
	if plan == nil {
		return nil
	}
	selected := make([]AssuranceScenario, 0, len(plan.Scenarios))
	for _, scenario := range plan.Scenarios {
		verifyLane := strings.TrimSpace(scenario.GatePolicy.VerifyLane)
		if verifyLane == "" {
			verifyLane = "required"
		}
		if verifyLane == lane {
			selected = append(selected, scenario)
		}
	}
	return selected
}

func executeAssuranceScenario(projectRoot, runDir string, scenario AssuranceScenario) (EvidenceEventBody, error) {
	if strings.TrimSpace(scenario.Harness.Kind) != "cli" {
		return EvidenceEventBody{}, fmt.Errorf("unsupported assurance harness kind %q", scenario.Harness.Kind)
	}
	cmdDir := RunWorktreePath(runDir)
	if info, err := os.Stat(cmdDir); err != nil || !info.IsDir() {
		cmdDir = projectRoot
	}
	cmd := exec.Command("bash", "-lc", scenario.Harness.Command)
	cmd.Dir = cmdDir
	output, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}
	artifactPath := filepath.Join(runDir, fmt.Sprintf("evidence-%s.txt", goalx.Slugify(scenario.ID)))
	if writeErr := os.WriteFile(artifactPath, output, 0o644); writeErr != nil {
		return EvidenceEventBody{}, fmt.Errorf("write assurance evidence for %s: %w", scenario.ID, writeErr)
	}
	if err := verifyScenarioOracle(scenario, exitCode); err != nil && exitCode == 0 {
		exitCode = 1
	}
	headRevision := ""
	if rev, revErr := gitRevisionIfAvailable(cmdDir, "HEAD"); revErr == nil {
		headRevision = rev
	}
	return EvidenceEventBody{
		ScenarioID:  scenario.ID,
		Scope:       "run-root",
		Revision:    headRevision,
		HarnessKind: scenario.Harness.Kind,
		OracleResult: map[string]any{
			"exit_code": exitCode,
		},
		ArtifactRefs: []string{artifactPath},
	}, nil
}

func verifyScenarioOracle(scenario AssuranceScenario, exitCode int) error {
	switch strings.TrimSpace(scenario.Oracle.Kind) {
	case "exit_code", "compound":
		for _, check := range scenario.Oracle.CheckDefinitions {
			if strings.TrimSpace(check.Kind) != "exit_code" {
				continue
			}
			if strings.TrimSpace(check.Equals) == "0" && exitCode != 0 {
				return fmt.Errorf("scenario %s exit code = %d, want 0", scenario.ID, exitCode)
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported assurance oracle kind %q", scenario.Oracle.Kind)
	}
}
