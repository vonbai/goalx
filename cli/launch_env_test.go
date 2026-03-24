package cli

import (
	"strings"
	"testing"
)

func TestBuildEngineLaunchCommandInjectsRuntimeEnv(t *testing.T) {
	t.Setenv("HOME", "/tmp/goalx-home")
	t.Setenv("PATH", "/tmp/goalx-bin:/usr/bin")
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("SSH_AUTH_SOCK", "/tmp/ssh.sock")
	t.Setenv("FOO_TOOLCHAIN_ROOT", "/opt/tools")
	t.Setenv("TMUX", "/tmp/tmux-should-not-propagate")
	t.Setenv("TMUX_PANE", "%42")
	t.Setenv("CODEX_THREAD_ID", "thread-should-not-propagate")

	cmd := buildEngineLaunchCommand("codex -m gpt-5.4 -a never -s danger-full-access", "/tmp/run/master.md")

	for _, want := range []string{
		"env ",
		"FOO_TOOLCHAIN_ROOT='/opt/tools'",
		"HOME='/tmp/goalx-home'",
		"PATH='/tmp/goalx-bin:/usr/bin'",
		"OPENAI_API_KEY='sk-test'",
		"SSH_AUTH_SOCK='/tmp/ssh.sock'",
		"codex -m gpt-5.4 -a never -s danger-full-access",
		"'/tmp/run/master.md'",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("launch command missing %q:\n%s", want, cmd)
		}
	}
	if strings.Contains(cmd, "TMUX='/tmp/tmux-should-not-propagate'") {
		t.Fatalf("launch command should not propagate TMUX:\n%s", cmd)
	}
	if strings.Contains(cmd, "TMUX_PANE='%42'") {
		t.Fatalf("launch command should not propagate TMUX_PANE:\n%s", cmd)
	}
	if strings.Contains(cmd, "CODEX_THREAD_ID='thread-should-not-propagate'") {
		t.Fatalf("launch command should not propagate Codex session vars:\n%s", cmd)
	}
}
