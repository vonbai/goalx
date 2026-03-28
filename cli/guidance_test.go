package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
)

func writeGuidanceRunFixture(t *testing.T) (string, string, *goalx.Config, *RunMetadata) {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	cfg := &goalx.Config{
		Name:      "guidance-run",
		Mode:      goalx.ModeDevelop,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapSidecarIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if _, err := EnsureSessionsRuntimeState(runDir); err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveProjectRegistry(repo, &ProjectRegistry{
		Version:    1,
		FocusedRun: cfg.Name,
		ActiveRuns: map[string]ProjectRunRef{
			cfg.Name: {Name: cfg.Name, State: "active"},
		},
	}); err != nil {
		t.Fatalf("SaveProjectRegistry: %v", err)
	}
	return repo, runDir, cfg, meta
}

func seedGuidanceSessionFixture(t *testing.T, runDir string, cfg *goalx.Config) {
	t.Helper()

	sessionName := "session-1"
	if err := EnsureSessionControl(runDir, sessionName); err != nil {
		t.Fatalf("EnsureSessionControl: %v", err)
	}
	identity := &SessionIdentity{
		Version:         1,
		SessionName:     sessionName,
		ExperimentID:    "exp_guidance_session_1",
		RoleKind:        "develop",
		Mode:            string(goalx.ModeDevelop),
		Engine:          "codex",
		Model:           "gpt-5.4-mini",
		OriginCharterID: loadCharterIDForTests(t, runDir),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := SaveSessionIdentity(SessionIdentityPath(runDir, sessionName), identity); err != nil {
		t.Fatalf("SaveSessionIdentity: %v", err)
	}
	if err := UpsertSessionRuntimeState(runDir, SessionRuntimeState{
		Name:         sessionName,
		State:        "active",
		Mode:         string(goalx.ModeDevelop),
		WorktreePath: WorktreePath(runDir, cfg.Name, 1),
	}); err != nil {
		t.Fatalf("UpsertSessionRuntimeState: %v", err)
	}
	if err := os.MkdirAll(WorktreePath(runDir, cfg.Name, 1), 0o755); err != nil {
		t.Fatalf("mkdir session worktree: %v", err)
	}
	if err := os.WriteFile(JournalPath(runDir, sessionName), []byte("{\"round\":1,\"status\":\"active\",\"desc\":\"working\"}\n"), 0o644); err != nil {
		t.Fatalf("write session journal: %v", err)
	}
}

func loadCharterIDForTests(t *testing.T, runDir string) string {
	t.Helper()

	charter, err := LoadRunCharter(RunCharterPath(runDir))
	if err != nil {
		t.Fatalf("LoadRunCharter: %v", err)
	}
	if charter == nil || charter.CharterID == "" {
		t.Fatal("run charter missing")
	}
	return charter.CharterID
}

func installGuidanceFakeTmux(t *testing.T, windows []string) {
	t.Helper()

	fakeBin := t.TempDir()
	script := `#!/bin/sh
case "$1" in
  has-session)
    exit 0
    ;;
  list-windows)
    printf '%s\n' master
    for w in ${TMUX_WINDOWS}; do
      printf '%s\n' "$w"
    done
    exit 0
    ;;
  list-panes)
    printf '%%0\tmaster\n'
    i=1
    for w in ${TMUX_WINDOWS}; do
      printf '%%%s\t%s\n' "$i" "$w"
      i=$((i+1))
    done
    exit 0
    ;;
  capture-pane)
    target=""
    while [ $# -gt 0 ]; do
      if [ "$1" = "-t" ]; then
        target="$2"
        shift 2
        continue
      fi
      shift
    done
    case "$target" in
      *:master) cat "$TMUX_MASTER_CAPTURE" ;;
      *:session-1) cat "$TMUX_SESSION1_CAPTURE" ;;
      *:session-2) cat "$TMUX_SESSION2_CAPTURE" ;;
    esac
    exit 0
    ;;
esac
exit 0
`
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TMUX_WINDOWS", "")
	if len(windows) > 0 {
		value := ""
		for i, window := range windows {
			if i > 0 {
				value += " "
			}
			value += window
		}
		t.Setenv("TMUX_WINDOWS", value)
	}
}
