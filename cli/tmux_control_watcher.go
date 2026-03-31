package cli

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

type tmuxPaneRef struct {
	PaneID     string
	WindowName string
}

type tmuxControlOutputNotification struct {
	PaneID string
	Value  string
}

type TmuxControlWatcher struct {
	runDir       string
	session      string
	masterEngine string

	cmd   *exec.Cmd
	stdin io.WriteCloser

	closeOnce        sync.Once
	writeMu          sync.Mutex
	mu               sync.RWMutex
	paneLastOutputAt map[string]time.Time
	lastError        string
	dirty            bool
	closed           bool

	done chan struct{}
}

var startTmuxControlWatcher = StartTmuxControlWatcher
var listTmuxSessionPanes = defaultListTmuxSessionPanes
var launchTmuxControlClient = defaultLaunchTmuxControlClient

func StartTmuxControlWatcher(runDir, session, masterEngine string) (*TmuxControlWatcher, error) {
	cmd, stdin, stdout, err := launchTmuxControlClient(runDir, session)
	if err != nil {
		return nil, err
	}
	w := &TmuxControlWatcher{
		runDir:           runDir,
		session:          session,
		masterEngine:     masterEngine,
		cmd:              cmd,
		stdin:            stdin,
		paneLastOutputAt: map[string]time.Time{},
		dirty:            true,
		done:             make(chan struct{}),
	}
	go w.readLoop(stdout)
	go w.flushLoop()
	return w, nil
}

func (w *TmuxControlWatcher) Close() error {
	if w == nil {
		return nil
	}
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.closed = true
		close(w.done)
		stdin := w.stdin
		cmd := w.cmd
		w.mu.Unlock()
		if stdin != nil {
			_, _ = io.WriteString(stdin, "detach-client\n")
			_ = stdin.Close()
		}
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})
	return nil
}

func (w *TmuxControlWatcher) Alive() bool {
	if w == nil {
		return false
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	return !w.closed
}

func (w *TmuxControlWatcher) snapshotLastOutputAt() map[string]time.Time {
	if w == nil {
		return nil
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[string]time.Time, len(w.paneLastOutputAt))
	for paneID, at := range w.paneLastOutputAt {
		out[paneID] = at
	}
	return out
}

func (w *TmuxControlWatcher) recordOutput(paneID, _ string) {
	if w == nil || strings.TrimSpace(paneID) == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.paneLastOutputAt[strings.TrimSpace(paneID)] = time.Now().UTC()
	w.dirty = true
}

func (w *TmuxControlWatcher) readLoop(stdout io.Reader) {
	defer func() {
		_ = w.Close()
	}()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		select {
		case <-w.done:
			return
		default:
		}
		if notification, ok := parseTmuxControlOutputNotification(scanner.Text()); ok {
			w.recordOutput(notification.PaneID, notification.Value)
		}
	}
	if err := scanner.Err(); err != nil {
		w.mu.Lock()
		w.lastError = err.Error()
		w.dirty = true
		w.mu.Unlock()
	}
}

func (w *TmuxControlWatcher) flushLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-w.done:
			return
		case <-ticker.C:
			if !w.consumeDirty() {
				continue
			}
			if err := w.writeSnapshot(); err != nil {
				w.mu.Lock()
				w.lastError = err.Error()
				w.mu.Unlock()
			}
		}
	}
}

func (w *TmuxControlWatcher) consumeDirty() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.dirty {
		return false
	}
	w.dirty = false
	return true
}

func (w *TmuxControlWatcher) writeSnapshot() error {
	if w == nil {
		return nil
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	facts, err := BuildTransportFactsWithPaneOutputTimes(w.runDir, w.session, w.masterEngine, w.snapshotLastOutputAt())
	if err != nil {
		return err
	}
	w.mu.RLock()
	lastError := w.lastError
	w.mu.RUnlock()
	if lastError != "" && facts != nil {
		for target, targetFacts := range facts.Targets {
			if targetFacts.LastTransportError == "" {
				targetFacts.LastTransportError = lastError
			}
			facts.Targets[target] = targetFacts
		}
	}
	return SaveTransportFacts(w.runDir, facts)
}

func defaultLaunchTmuxControlClient(runDir, session string) (*exec.Cmd, io.WriteCloser, io.Reader, error) {
	cmd := tmuxCommandWithSocketDir(resolveRunTmuxSocketDir("", runDir, ""), "-C", "attach-session", "-t", session)
	cmd.Stderr = io.Discard
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("control-mode stdout: %w", err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("control-mode stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start control-mode client: %w", err)
	}
	return cmd, stdin, stdout, nil
}

func defaultListTmuxSessionPanes(runDir, session string) ([]tmuxPaneRef, error) {
	out, err := tmuxOutputWithSocketDir(resolveRunTmuxSocketDir("", runDir, ""), "list-panes", "-a", "-F", "#{session_name}\t#{pane_id}\t#{window_name}")
	if err != nil {
		return nil, err
	}
	var panes []tmuxPaneRef
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		ref := tmuxPaneRef{}
		switch len(parts) {
		case 3:
			if strings.TrimSpace(parts[0]) != session {
				continue
			}
			ref.PaneID = strings.TrimSpace(parts[1])
			ref.WindowName = strings.TrimSpace(parts[2])
		case 2:
			// Test fixtures may still emit pane/window pairs without a session column.
			ref.PaneID = strings.TrimSpace(parts[0])
			ref.WindowName = strings.TrimSpace(parts[1])
		default:
			continue
		}
		panes = append(panes, ref)
	}
	return panes, nil
}

func parseTmuxControlOutputNotification(line string) (tmuxControlOutputNotification, bool) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "%output "):
		rest := strings.TrimPrefix(line, "%output ")
		paneID, payload, ok := splitPanePayload(rest)
		if !ok {
			return tmuxControlOutputNotification{}, false
		}
		return tmuxControlOutputNotification{PaneID: paneID, Value: decodeTmuxControlValue(payload)}, true
	case strings.HasPrefix(line, "%extended-output "):
		rest := strings.TrimPrefix(line, "%extended-output ")
		paneID, payload, ok := splitExtendedPanePayload(rest)
		if !ok {
			return tmuxControlOutputNotification{}, false
		}
		return tmuxControlOutputNotification{PaneID: paneID, Value: decodeTmuxControlValue(payload)}, true
	default:
		return tmuxControlOutputNotification{}, false
	}
}

func splitPanePayload(rest string) (string, string, bool) {
	parts := strings.SplitN(rest, " ", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), parts[1], true
}

func splitExtendedPanePayload(rest string) (string, string, bool) {
	parts := strings.SplitN(rest, " ", 3)
	if len(parts) != 3 {
		return "", "", false
	}
	payload := strings.TrimSpace(parts[2])
	if strings.HasPrefix(payload, ": ") {
		payload = strings.TrimPrefix(payload, ": ")
	}
	return strings.TrimSpace(parts[0]), payload, true
}

func decodeTmuxControlValue(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		if s[i+1] == '\\' {
			b.WriteByte('\\')
			i++
			continue
		}
		if i+3 < len(s) && isOctalDigit(s[i+1]) && isOctalDigit(s[i+2]) && isOctalDigit(s[i+3]) {
			value, err := strconv.ParseInt(s[i+1:i+4], 8, 32)
			if err == nil {
				b.WriteByte(byte(value))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isOctalDigit(b byte) bool {
	return b >= '0' && b <= '7'
}
