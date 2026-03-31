package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSessionWithCommandSyncsCurrentPATHTotmuxServer(t *testing.T) {
	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
exit 0
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+"/usr/bin")

	if err := NewSessionWithCommand("demo-session", "master", "/tmp/demo", "echo hi"); err != nil {
		t.Fatalf("NewSessionWithCommand: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) < 2 {
		t.Fatalf("tmux log = %q, want set-environment + new-session", string(logData))
	}
	if !strings.Contains(lines[0], "set-environment -g PATH "+fakeBin+":/usr/bin") {
		t.Fatalf("first tmux command = %q, want PATH sync", lines[0])
	}
	if !strings.Contains(lines[1], "new-session -d -s demo-session -n master -c /tmp/demo echo hi") {
		t.Fatalf("second tmux command = %q, want new-session", lines[1])
	}
}

func TestNewWindowWithCommandSyncsCurrentPATHTotmuxServer(t *testing.T) {
	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
exit 0
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+"/usr/bin")

	if err := NewWindowWithCommand("demo-session", "session-2", "/tmp/demo", "echo hi"); err != nil {
		t.Fatalf("NewWindowWithCommand: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) < 2 {
		t.Fatalf("tmux log = %q, want set-environment + new-window", string(logData))
	}
	if !strings.Contains(lines[0], "set-environment -g PATH "+fakeBin+":/usr/bin") {
		t.Fatalf("first tmux command = %q, want PATH sync", lines[0])
	}
	if !strings.Contains(lines[1], "new-window -t demo-session -n session-2 -c /tmp/demo echo hi") {
		t.Fatalf("second tmux command = %q, want new-window", lines[1])
	}
}

func TestNewSessionWithCommandIgnoresMissingTmuxServerDuringPathSync(t *testing.T) {
	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "$@" >> "$TMUX_LOG"
if [ "$1" = "set-environment" ]; then
  echo "error connecting to /tmp/tmux-0/default (No such file or directory)" >&2
  exit 1
fi
exit 0
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+"/usr/bin")

	if err := NewSessionWithCommand("demo-session", "master", "/tmp/demo", "echo hi"); err != nil {
		t.Fatalf("NewSessionWithCommand: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(logData)), "\n")
	if len(lines) < 2 {
		t.Fatalf("tmux log = %q, want set-environment + new-session", string(logData))
	}
	if !strings.Contains(lines[0], "set-environment -g PATH "+fakeBin+":/usr/bin") {
		t.Fatalf("first tmux command = %q, want PATH sync", lines[0])
	}
	if !strings.Contains(lines[1], "new-session -d -s demo-session -n master -c /tmp/demo echo hi") {
		t.Fatalf("second tmux command = %q, want new-session after missing-server path sync", lines[1])
	}
}

func TestNewSessionWithCommandInRunClearsAmbientTmuxAndUsesRunOwnedSocketDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	runDir := t.TempDir()
	socketDir := filepath.Join(home, ".goalx", "tmux", "demo-socket")
	if err := SaveTmuxLocator(TmuxLocatorPath(runDir), &TmuxLocator{
		Version:   1,
		Session:   "demo-session",
		SocketDir: socketDir,
	}); err != nil {
		t.Fatalf("SaveTmuxLocator: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := `#!/bin/sh
echo "TMUX=${TMUX-} TMPDIR=${TMUX_TMPDIR-} $@" >> "$TMUX_LOG"
exit 0
`
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("TMUX", "/tmp/tmux-0/default,123,0")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+"/usr/bin")

	if err := NewSessionWithCommandInRun(runDir, "demo-session", "master", "/tmp/demo", "echo hi"); err != nil {
		t.Fatalf("NewSessionWithCommandInRun: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	logText := strings.TrimSpace(string(logData))
	wantSocketDir := resolveRunTmuxSocketDir("", runDir, "")
	for _, line := range strings.Split(logText, "\n") {
		if !strings.Contains(line, "TMPDIR="+wantSocketDir) {
			t.Fatalf("tmux command should use run-owned socket dir:\n%s", logText)
		}
		if strings.Contains(line, "TMUX=/tmp/tmux-0/default,123,0") {
			t.Fatalf("tmux command inherited ambient TMUX:\n%s", logText)
		}
	}
}

func TestEnsureRunTmuxLocatorUsesShortUserScopedSocketDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectRoot := filepath.Join(home, "workspaces", "very", "long", "project")
	runName := "this-run-name-is-intentionally-long-to-prove-the-locator-does-not-embed-the-full-run-directory"
	runDir := filepath.Join(home, ".goalx", "runs", "project-id", runName)

	session, err := ensureRunTmuxLocator(projectRoot, runDir, runName)
	if err != nil {
		t.Fatalf("ensureRunTmuxLocator: %v", err)
	}
	if strings.TrimSpace(session) == "" {
		t.Fatal("ensureRunTmuxLocator returned empty session")
	}

	locator, err := LoadTmuxLocator(TmuxLocatorPath(runDir))
	if err != nil {
		t.Fatalf("LoadTmuxLocator: %v", err)
	}
	if locator == nil {
		t.Fatal("locator missing")
	}
	wantPrefix := filepath.Join(os.TempDir(), "goalx-tmux") + string(os.PathSeparator)
	if !strings.HasPrefix(locator.SocketDir, wantPrefix) {
		t.Fatalf("socket dir = %q, want prefix %q", locator.SocketDir, wantPrefix)
	}
	if strings.Contains(locator.SocketDir, runName) {
		t.Fatalf("socket dir should not embed full run name: %q", locator.SocketDir)
	}
	legacySocketPath := filepath.Join(runDir, "control", "tmux", "tmux-0", "default")
	socketPath := filepath.Join(locator.SocketDir, "tmux-0", "default")
	if len(socketPath) >= len(legacySocketPath) {
		t.Fatalf("socket path not shortened: len=%d legacy_len=%d path=%q legacy=%q", len(socketPath), len(legacySocketPath), socketPath, legacySocketPath)
	}
}
