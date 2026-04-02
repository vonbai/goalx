package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	goalx "github.com/vonbai/goalx"
)

type TargetRunnerLaunchSpec struct {
	LaunchEnv map[string]string
	GoalxBin  string
	RunName   string
	RunDir    string
	Holder    string
	RunID     string
	Epoch     int
	TTL       time.Duration
	EngineCmd string
	Prompt    string
}

type TargetRunner interface {
	BuildCommand(spec TargetRunnerLaunchSpec) (string, error)
}

var targetRunner TargetRunner = processTargetRunner{}

type processTargetRunner struct{}

func (processTargetRunner) BuildCommand(spec TargetRunnerLaunchSpec) (string, error) {
	ttlSeconds := int(spec.TTL.Seconds())
	if ttlSeconds <= 0 {
		ttlSeconds = 30
	}
	args := []string{
		shellQuote(spec.GoalxBin),
		"target-runner",
		"--run", shellQuote(spec.RunName),
		"--run-dir", shellQuote(spec.RunDir),
		"--holder", shellQuote(spec.Holder),
		"--run-id", shellQuote(spec.RunID),
		"--epoch", strconv.Itoa(spec.Epoch),
		"--ttl-seconds", strconv.Itoa(ttlSeconds),
		"--transport", "tmux",
		"--engine-command", shellQuote(spec.EngineCmd),
		"--prompt", shellQuote(spec.Prompt),
	}
	return buildLaunchEnvPrefix(spec.LaunchEnv) + " " + strings.Join(args, " "), nil
}

const targetRunnerUsage = "usage: goalx target-runner (--run RUN | --run-dir PATH) --holder HOLDER --run-id RUN_ID --epoch N --ttl-seconds N --transport NAME --engine-command CMD --prompt PATH"

func parseTargetRunnerArgs(projectRoot string, args []string) (runDir, holder, runID string, epoch int, ttl time.Duration, transport, engineCmd, prompt string, err error) {
	runName := ""
	runDir = ""
	holder = ""
	runID = ""
	epoch = 0
	ttlSeconds := 0
	transport = ""
	engineCmd = ""
	prompt = ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			runName = strings.TrimSpace(args[i])
		case "--run-dir":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			runDir = strings.TrimSpace(args[i])
		case "--holder":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			holder = strings.TrimSpace(args[i])
		case "--run-id":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			runID = strings.TrimSpace(args[i])
		case "--epoch":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(args[i]))
			if parseErr != nil {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			epoch = parsed
		case "--ttl-seconds":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(args[i]))
			if parseErr != nil {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			ttlSeconds = parsed
		case "--transport":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			transport = strings.TrimSpace(args[i])
		case "--engine-command":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			engineCmd = args[i]
		case "--prompt":
			if i+1 >= len(args) {
				return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
			}
			i++
			prompt = args[i]
		default:
			return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
		}
	}
	if runDir == "" && runName != "" {
		runDir = goalx.RunDir(projectRoot, runName)
	}
	if strings.TrimSpace(runDir) == "" || holder == "" || runID == "" || epoch <= 0 || ttlSeconds <= 0 || transport == "" || strings.TrimSpace(engineCmd) == "" || strings.TrimSpace(prompt) == "" {
		return "", "", "", 0, 0, "", "", "", fmt.Errorf(targetRunnerUsage)
	}
	return runDir, holder, runID, epoch, time.Duration(ttlSeconds) * time.Second, transport, engineCmd, prompt, nil
}

func TargetRunnerCommand(projectRoot string, args []string) error {
	runDir, holder, runID, epoch, ttl, transport, engineCmd, prompt, err := parseTargetRunnerArgs(projectRoot, args)
	if err != nil {
		return err
	}
	child := exec.Command("/bin/bash", "-lc", buildEngineExecCommand(engineCmd, prompt))
	child.Dir = projectRoot
	child.Stdin = os.Stdin
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		return fmt.Errorf("start target runner child: %w", err)
	}
	applyOOMPriorityBestEffort(runDir, holder, child.Process.Pid)
	ctx, cancel := context.WithCancel(context.Background())
	leaseDone := make(chan error, 1)
	go func() {
		leaseDone <- runLeaseLoop(ctx, runDir, holder, runID, epoch, ttl, transport, child.Process.Pid)
	}()
	waitErr := child.Wait()
	cancel()
	leaseErr := <-leaseDone
	if waitErr != nil {
		return waitErr
	}
	return leaseErr
}
