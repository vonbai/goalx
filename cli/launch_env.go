package cli

import (
	"os"
	"sort"
	"strings"
	"time"
)

var launchEnvDenylist = map[string]struct{}{
	"CLAUDE_SESSION_ID":    {},
	"CODEX_SESSION_ID":     {},
	"CODEX_THREAD_ID":      {},
	"OLDPWD":               {},
	"PROMPT_COMMAND":       {},
	"PS1":                  {},
	"PWD":                  {},
	"SHLVL":                {},
	"TERM":                 {},
	"TERM_PROGRAM":         {},
	"TERM_PROGRAM_VERSION": {},
	"TMUX":                 {},
	"TMUX_PANE":            {},
}

func buildEngineLaunchCommand(engineCmd, prompt string) string {
	return buildEngineLaunchCommandWithEnv(currentLaunchEnv(), engineCmd, prompt)
}

func buildEngineLaunchCommandWithEnv(launchEnv map[string]string, engineCmd, prompt string) string {
	parts := []string{buildLaunchEnvPrefix(launchEnv), buildEngineExecCommand(engineCmd, prompt)}
	return strings.Join(parts, " ")
}

func buildMasterLaunchCommand(goalxBin, runName, runDir, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	return buildMasterLaunchCommandWithEnv(currentLaunchEnv(), goalxBin, runName, runDir, runID, epoch, ttl, engineCmd, prompt)
}

func buildMasterLaunchCommandWithEnv(launchEnv map[string]string, goalxBin, runName, runDir, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	return buildLeaseWrappedLaunchCommandWithEnv(launchEnv, goalxBin, runName, runDir, "master", runID, epoch, ttl, engineCmd, prompt)
}

func buildLeaseWrappedLaunchCommand(goalxBin, runName, runDir, holder, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	return buildLeaseWrappedLaunchCommandWithEnv(currentLaunchEnv(), goalxBin, runName, runDir, holder, runID, epoch, ttl, engineCmd, prompt)
}

func buildLeaseWrappedLaunchCommandWithEnv(launchEnv map[string]string, goalxBin, runName, runDir, holder, runID string, epoch int, ttl time.Duration, engineCmd, prompt string) string {
	command, err := targetRunner.BuildCommand(TargetRunnerLaunchSpec{
		LaunchEnv: launchEnv,
		GoalxBin:  goalxBin,
		RunName:   runName,
		RunDir:    runDir,
		Holder:    holder,
		RunID:     runID,
		Epoch:     epoch,
		TTL:       ttl,
		EngineCmd: engineCmd,
		Prompt:    prompt,
	})
	if err != nil {
		panic(err)
	}
	return command
}

func buildEngineExecCommand(engineCmd, prompt string) string {
	return engineCmd + " " + shellQuote(prompt)
}

func buildLaunchEnvPrefix(env map[string]string) string {
	parts := []string{"env"}
	denyKeys := make([]string, 0, len(launchEnvDenylist))
	for key := range launchEnvDenylist {
		denyKeys = append(denyKeys, key)
	}
	sort.Strings(denyKeys)
	for _, key := range denyKeys {
		parts = append(parts, "-u", key)
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+shellQuote(env[key]))
	}
	return strings.Join(parts, " ")
}

func currentLaunchEnv() map[string]string {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || !shouldPropagateLaunchEnv(key) {
			continue
		}
		env[key] = value
	}
	return env
}

func shouldPropagateLaunchEnv(key string) bool {
	_, denied := launchEnvDenylist[key]
	return !denied
}
