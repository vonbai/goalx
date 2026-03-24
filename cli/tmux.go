package cli

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SessionExists returns true if a tmux session with the given name exists.
func SessionExists(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// NewSession creates a new detached tmux session with its first window named.
func NewSession(name, firstWindow string) error {
	return NewSessionWithCommand(name, firstWindow, "", "")
}

// NewSessionWithCommand creates a new detached tmux session with its first window named.
func NewSessionWithCommand(name, firstWindow, workdir, command string) error {
	args := []string{"new-session", "-d", "-s", name, "-n", firstWindow}
	if workdir != "" {
		args = append(args, "-c", workdir)
	}
	if command != "" {
		args = append(args, command)
	}
	return exec.Command("tmux", args...).Run()
}

// NewWindow creates a new window in the given tmux session.
func NewWindow(session, window, workdir string) error {
	return NewWindowWithCommand(session, window, workdir, "")
}

// NewWindowWithCommand creates a new window in the given tmux session.
func NewWindowWithCommand(session, window, workdir, command string) error {
	args := []string{"new-window", "-t", session, "-n", window}
	if workdir != "" {
		args = append(args, "-c", workdir)
	}
	if command != "" {
		args = append(args, command)
	}
	return exec.Command("tmux", args...).Run()
}

// RenameWindow renames a window by index in the given tmux session.
func RenameWindow(session string, index int, name string) error {
	target := session + ":" + strconv.Itoa(index)
	return exec.Command("tmux", "rename-window", "-t", target, name).Run()
}

// SendKeys sends keystrokes to a tmux target, followed by Enter.
// Includes a short delay to ensure the target pane's shell is ready.
func SendKeys(target, keys string) error {
	return sendKeysWithSubmit(target, keys, "Enter")
}

// SendEscape sends Escape to a tmux target without submitting Enter.
func SendEscape(target string) error {
	return exec.Command("tmux", "send-keys", "-t", target, "Escape").Run()
}

func sendKeysWithSubmit(target, keys, submitKey string) error {
	time.Sleep(200 * time.Millisecond)
	args := []string{"send-keys", "-t", target}
	if keys != "" {
		args = append(args, keys)
	}
	if submitKey != "" {
		args = append(args, submitKey)
	}
	return exec.Command("tmux", args...).Run()
}

// AttachSession attaches to a tmux session at the specified window.
func AttachSession(session string, window string) error {
	target := session + ":" + window
	return exec.Command("tmux", "attach-session", "-t", target).Run()
}

// KillSession destroys a tmux session.
func KillSession(session string) error {
	return exec.Command("tmux", "kill-session", "-t", session).Run()
}

// KillWindow destroys a single window in a tmux session.
func KillWindow(session, window string) error {
	target := session + ":" + window
	return exec.Command("tmux", "kill-window", "-t", target).Run()
}

// WindowExists returns true if a tmux window with the given name exists.
func WindowExists(session, window string) bool {
	out, err := exec.Command("tmux", "list-windows", "-t", session, "-F", "#{window_name}").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == window {
			return true
		}
	}
	return false
}

// CapturePaneOutput captures the visible content of a tmux pane.
func CapturePaneOutput(session, window string) (string, error) {
	target := session + ":" + window
	return CapturePaneTargetOutput(target)
}

// CapturePaneTargetOutput captures the visible content of a tmux pane target.
func CapturePaneTargetOutput(target string) (string, error) {
	out, err := exec.Command("tmux", "capture-pane", "-t", target, "-p").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
