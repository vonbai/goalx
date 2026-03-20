package cli

import (
	"os/exec"
	"strconv"
	"time"
)

// SessionExists returns true if a tmux session with the given name exists.
func SessionExists(name string) bool {
	return exec.Command("tmux", "has-session", "-t", name).Run() == nil
}

// NewSession creates a new detached tmux session with its first window named.
func NewSession(name, firstWindow string) error {
	return exec.Command("tmux", "new-session", "-d", "-s", name, "-n", firstWindow).Run()
}

// NewWindow creates a new window in the given tmux session.
func NewWindow(session, window, workdir string) error {
	return exec.Command("tmux", "new-window", "-t", session, "-n", window, "-c", workdir).Run()
}

// RenameWindow renames a window by index in the given tmux session.
func RenameWindow(session string, index int, name string) error {
	target := session + ":" + strconv.Itoa(index)
	return exec.Command("tmux", "rename-window", "-t", target, name).Run()
}

// SendKeys sends keystrokes to a tmux target, followed by Enter.
// Includes a short delay to ensure the target pane's shell is ready.
func SendKeys(target, keys string) error {
	time.Sleep(200 * time.Millisecond)
	return exec.Command("tmux", "send-keys", "-t", target, keys, "Enter").Run()
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

// CapturePaneOutput captures the visible content of a tmux pane.
func CapturePaneOutput(session, window string) (string, error) {
	target := session + ":" + window
	out, err := exec.Command("tmux", "capture-pane", "-t", target, "-p").Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
