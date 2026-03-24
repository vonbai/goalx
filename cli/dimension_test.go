package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestDimensionSetAppliesToAllSessions(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &goalx.Config{
		Name: "dimension-run",
		Mode: goalx.ModeDevelop,
	}
	runDir := writeRunSpecFixture(t, projectRoot, cfg)
	for _, sessionName := range []string{"session-1", "session-2"} {
		if err := os.WriteFile(JournalPath(runDir, sessionName), nil, 0o644); err != nil {
			t.Fatalf("seed %s journal: %v", sessionName, err)
		}
	}

	if err := Dimension(projectRoot, []string{"--run", cfg.Name, "all", "--set", "depth, adversarial"}); err != nil {
		t.Fatalf("Dimension set all: %v", err)
	}

	state, err := LoadDimensionsState(ControlDimensionsPath(runDir))
	if err != nil {
		t.Fatalf("LoadDimensionsState: %v", err)
	}
	if state == nil {
		t.Fatal("dimensions state missing")
	}
	for _, sessionName := range []string{"session-1", "session-2"} {
		got := state.Sessions[sessionName]
		want := []string{"depth", "adversarial"}
		if !slices.Equal(got, want) {
			t.Fatalf("%s dimensions = %#v, want %#v", sessionName, got, want)
		}
	}
	if state.UpdatedAt == "" {
		t.Fatal("UpdatedAt empty")
	}
}

func TestDimensionAddAndRemoveUpdateNamedSession(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &goalx.Config{
		Name: "dimension-run",
		Mode: goalx.ModeDevelop,
	}
	runDir := writeRunSpecFixture(t, projectRoot, cfg)
	if err := os.WriteFile(JournalPath(runDir, "session-1"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}
	if err := SaveDimensionsState(ControlDimensionsPath(runDir), &DimensionsState{
		Version: 1,
		Sessions: map[string][]string{
			"session-1": []string{"depth", "evidence"},
			"session-2": []string{"comparative"},
		},
	}); err != nil {
		t.Fatalf("SaveDimensionsState: %v", err)
	}

	if err := Dimension(projectRoot, []string{"--run", cfg.Name, "session-1", "--add", "creative"}); err != nil {
		t.Fatalf("Dimension add: %v", err)
	}
	if err := Dimension(projectRoot, []string{"--run", cfg.Name, "session-1", "--add", "creative"}); err != nil {
		t.Fatalf("Dimension add duplicate: %v", err)
	}
	if err := Dimension(projectRoot, []string{"--run", cfg.Name, "session-1", "--remove", "depth"}); err != nil {
		t.Fatalf("Dimension remove: %v", err)
	}

	state, err := LoadDimensionsState(ControlDimensionsPath(runDir))
	if err != nil {
		t.Fatalf("LoadDimensionsState: %v", err)
	}
	if got, want := state.Sessions["session-1"], []string{"evidence", "creative"}; !slices.Equal(got, want) {
		t.Fatalf("session-1 dimensions = %#v, want %#v", got, want)
	}
	if got, want := state.Sessions["session-2"], []string{"comparative"}; !slices.Equal(got, want) {
		t.Fatalf("session-2 dimensions = %#v, want %#v", got, want)
	}
}

func TestDimensionRejectsUnsupportedTargetMutationCombination(t *testing.T) {
	projectRoot := t.TempDir()
	cfg := &goalx.Config{
		Name: "dimension-run",
		Mode: goalx.ModeDevelop,
	}
	runDir := writeRunSpecFixture(t, projectRoot, cfg)
	if err := os.WriteFile(filepath.Join(runDir, "journals", "session-1.jsonl"), nil, 0o644); err != nil {
		t.Fatalf("seed session-1 journal: %v", err)
	}

	err := Dimension(projectRoot, []string{"--run", cfg.Name, "all", "--add", "creative"})
	if err == nil {
		t.Fatal("Dimension(all --add) succeeded, want error")
	}
	if got, want := err.Error(), "usage: goalx dimension"; len(got) < len(want) || got[:len(want)] != want {
		t.Fatalf("Dimension(all --add) error = %q, want usage prefix %q", got, want)
	}
}
