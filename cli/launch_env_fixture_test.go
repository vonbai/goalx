package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func writeTestLaunchEnvSnapshotFromCurrent(t *testing.T, runDir string) {
	t.Helper()

	env := map[string]string{}
	for _, key := range []string{"HOME", "PATH", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if value, ok := os.LookupEnv(key); ok {
			env[key] = value
		}
	}
	writeTestLaunchEnvSnapshot(t, runDir, env)
}

func writeTestLaunchEnvSnapshot(t *testing.T, runDir string, env map[string]string) {
	t.Helper()

	path := filepath.Join(runDir, "control", "launch-env.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir launch env dir: %v", err)
	}
	payload := map[string]any{
		"version":     1,
		"captured_at": "2026-03-24T00:00:00Z",
		"env":         env,
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal launch env snapshot: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write launch env snapshot: %v", err)
	}
}
