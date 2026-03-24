package cli

import (
	"strings"
	"testing"
	"time"
)

func TestWaitReturnsWhenMasterInboxHasUnreadMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if _, err := AppendMasterInboxMessage(runDir, "tell", "user", "check control files now"); err != nil {
		t.Fatalf("AppendMasterInboxMessage: %v", err)
	}

	oldPoll := waitPollInterval
	waitPollInterval = 5 * time.Millisecond
	defer func() { waitPollInterval = oldPoll }()

	out := captureStdout(t, func() {
		if err := Wait(repo, []string{"--run", runName, "--timeout", "50ms"}); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	})

	if !strings.Contains(out, "inbox pending for master") {
		t.Fatalf("wait output = %q, want master inbox wake", out)
	}
}

func TestWaitReturnsWhenSessionInboxHasUnreadMessage(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if _, err := AppendControlInboxMessage(runDir, "session-1", "tell", "user", "focus on db race triage"); err != nil {
		t.Fatalf("AppendControlInboxMessage: %v", err)
	}

	oldPoll := waitPollInterval
	waitPollInterval = 5 * time.Millisecond
	defer func() { waitPollInterval = oldPoll }()

	out := captureStdout(t, func() {
		if err := Wait(repo, []string{"--run", runName, "session-1", "--timeout", "50ms"}); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	})

	if !strings.Contains(out, "inbox pending for session-1") {
		t.Fatalf("wait output = %q, want session inbox wake", out)
	}
}

func TestWaitTimesOutWithoutUnreadInbox(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, _ := writeLifecycleRunFixture(t, repo)

	oldPoll := waitPollInterval
	waitPollInterval = 5 * time.Millisecond
	defer func() { waitPollInterval = oldPoll }()

	out := captureStdout(t, func() {
		if err := Wait(repo, []string{"--run", runName, "session-1", "--timeout", "20ms"}); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	})

	if !strings.Contains(out, "timed out waiting for session-1") {
		t.Fatalf("wait output = %q, want timeout message", out)
	}
}

func TestParseDurationOrSeconds(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"5m", 5 * time.Minute, false},
		{"300", 300 * time.Second, false},
		{"1.5", 1500 * time.Millisecond, false},
		{"50ms", 50 * time.Millisecond, false},
		{"garbage", 0, true},
	}
	for _, tt := range tests {
		got, err := parseDurationOrSeconds(tt.input)
		if tt.err && err == nil {
			t.Errorf("parseDurationOrSeconds(%q) = %v, want error", tt.input, got)
		}
		if !tt.err && err != nil {
			t.Errorf("parseDurationOrSeconds(%q) error: %v", tt.input, err)
		}
		if !tt.err && got != tt.want {
			t.Errorf("parseDurationOrSeconds(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWaitReturnsStoppedErrorWhenRunStops(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	runName, runDir := writeLifecycleRunFixture(t, repo)

	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:        1,
		LifecycleState: "stopped",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	oldPoll := waitPollInterval
	waitPollInterval = 5 * time.Millisecond
	defer func() { waitPollInterval = oldPoll }()

	err := Wait(repo, []string{"--run", runName, "--timeout", "50ms"})
	if err == nil || !strings.Contains(err.Error(), `run "lifecycle-run" is stopped`) {
		t.Fatalf("Wait error = %v, want stopped error", err)
	}
}
