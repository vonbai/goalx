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
