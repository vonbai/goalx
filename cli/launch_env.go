package cli

import (
	"os"
	"sort"
	"strings"
)

var launchEnvKeys = map[string]struct{}{
	"ALL_PROXY":         {},
	"ANTHROPIC_API_KEY": {},
	"COLORTERM":         {},
	"DISPLAY":           {},
	"HOME":              {},
	"HTTP_PROXY":        {},
	"HTTPS_PROXY":       {},
	"LANG":              {},
	"LC_ALL":            {},
	"LOGNAME":           {},
	"NO_PROXY":          {},
	"OPENAI_API_KEY":    {},
	"PATH":              {},
	"SHELL":             {},
	"SSH_AUTH_SOCK":     {},
	"TERM":              {},
	"USER":              {},
	"XDG_RUNTIME_DIR":   {},
	"http_proxy":        {},
	"https_proxy":       {},
	"no_proxy":          {},
}

var launchEnvPrefixes = []string{
	"ANTHROPIC_",
	"BUN_",
	"FNM_",
	"NPM_",
	"NVM_",
	"OPENAI_",
}

func buildEngineLaunchCommand(engineCmd, prompt string) string {
	parts := []string{"env"}
	env := currentLaunchEnv()
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts = append(parts, key+"="+shellQuote(env[key]))
	}
	parts = append(parts, engineCmd, shellQuote(prompt))
	return strings.Join(parts, " ")
}

func currentLaunchEnv() map[string]string {
	env := make(map[string]string)
	for _, item := range os.Environ() {
		key, value, ok := strings.Cut(item, "=")
		if !ok || value == "" || !shouldPropagateLaunchEnv(key) {
			continue
		}
		env[key] = value
	}
	return env
}

func shouldPropagateLaunchEnv(key string) bool {
	if _, ok := launchEnvKeys[key]; ok {
		return true
	}
	for _, prefix := range launchEnvPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
