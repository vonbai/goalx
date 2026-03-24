package cli

import (
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestExtractRunFlag(t *testing.T) {
	run, rest, err := extractRunFlag([]string{"--run", "demo", "session-2"})
	if err != nil {
		t.Fatalf("extractRunFlag: %v", err)
	}
	if run != "demo" {
		t.Fatalf("run = %q, want demo", run)
	}
	if len(rest) != 1 || rest[0] != "session-2" {
		t.Fatalf("rest = %#v, want [session-2]", rest)
	}
}

func TestExtractRunFlagMissingValue(t *testing.T) {
	_, _, err := extractRunFlag([]string{"--run"})
	if err == nil {
		t.Fatal("expected error for missing --run value")
	}
}

func TestParseStartInitArgs(t *testing.T) {
	opts, err := parseStartInitArgs([]string{
		"ship feature",
		"--research",
		"--parallel", "3",
		"--name", "demo-run",
		"--master", "codex/best",
		"--research-role", "claude-code/opus",
		"--develop-role", "codex/fast",
	})
	if err != nil {
		t.Fatalf("parseStartInitArgs: %v", err)
	}
	if opts.Objective != "ship feature" {
		t.Fatalf("objective = %q", opts.Objective)
	}
	if opts.Mode != goalx.ModeResearch {
		t.Fatalf("mode = %q, want %q", opts.Mode, goalx.ModeResearch)
	}
	if opts.Parallel != 3 {
		t.Fatalf("parallel = %d, want 3", opts.Parallel)
	}
	if opts.Name != "demo-run" {
		t.Fatalf("name = %q, want demo-run", opts.Name)
	}
	if opts.Master != "codex/best" {
		t.Fatalf("master = %q, want codex/best", opts.Master)
	}
	if opts.ResearchRole != "claude-code/opus" {
		t.Fatalf("research-role = %q", opts.ResearchRole)
	}
	if opts.DevelopRole != "codex/fast" {
		t.Fatalf("develop-role = %q", opts.DevelopRole)
	}
}

func TestParseLaunchOptionsRejectsAmbiguousTopLevelEngineFlags(t *testing.T) {
	_, err := parseLaunchOptions([]string{"audit auth", "--engine", "codex"}, goalx.ModeResearch, true)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseLaunchOptionsLeavesParallelUnsetWhenOmitted(t *testing.T) {
	opts, err := parseLaunchOptions([]string{"audit auth"}, goalx.ModeResearch, false)
	if err != nil {
		t.Fatalf("parseLaunchOptions: %v", err)
	}
	if opts.Parallel != 0 {
		t.Fatalf("parallel = %d, want 0 when omitted", opts.Parallel)
	}
}

func TestParseLaunchOptionsLeavesParallelUnsetByDefault(t *testing.T) {
	opts, err := parseLaunchOptions([]string{"audit auth"}, goalx.ModeResearch, true)
	if err != nil {
		t.Fatalf("parseLaunchOptions: %v", err)
	}
	if opts.Parallel != 0 {
		t.Fatalf("parallel = %d, want 0 when omitted", opts.Parallel)
	}
}

func TestParseLaunchOptionsKeepsAutoDefaultMode(t *testing.T) {
	opts, err := parseLaunchOptions([]string{"audit auth"}, goalx.ModeAuto, true)
	if err != nil {
		t.Fatalf("parseLaunchOptions: %v", err)
	}
	if opts.Mode != goalx.ModeAuto {
		t.Fatalf("mode = %q, want %q", opts.Mode, goalx.ModeAuto)
	}
}

func TestParseLaunchOptionsAcceptsNoSnapshotFlag(t *testing.T) {
	opts, err := parseLaunchOptions([]string{"audit auth", "--no-snapshot"}, goalx.ModeResearch, true)
	if err != nil {
		t.Fatalf("parseLaunchOptions: %v", err)
	}
	if !opts.NoSnapshot {
		t.Fatal("NoSnapshot = false, want true")
	}
}

func TestParseStatusArgs(t *testing.T) {
	run, session, err := parseStatusArgs([]string{"--run", "demo", "session-1"})
	if err != nil {
		t.Fatalf("parseStatusArgs: %v", err)
	}
	if run != "demo" || session != "session-1" {
		t.Fatalf("got run=%q session=%q", run, session)
	}
}

func TestParseSessionIndex(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "session 1", input: "session-1", want: 1},
		{name: "session 99", input: "session-99", want: 99},
		{name: "invalid", input: "invalid", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseSessionIndex(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseSessionIndex(%q) error = nil, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSessionIndex(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("parseSessionIndex(%q) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

func TestSessionCountPrefersExplicitSessions(t *testing.T) {
	cfg := &goalx.Config{
		Parallel: 1,
		Sessions: []goalx.SessionConfig{{Hint: "a"}, {Hint: "b"}},
	}
	if got := sessionCount(cfg); got != 2 {
		t.Fatalf("sessionCount = %d, want 2", got)
	}
}

func TestSessionWindowNameOmitsRunNamePrefix(t *testing.T) {
	if got := sessionWindowName("demo-run", 2); got != "session-2" {
		t.Fatalf("sessionWindowName = %q, want session-2", got)
	}
}
