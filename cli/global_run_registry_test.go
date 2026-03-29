package cli

import (
	"fmt"
	"sync"
	"testing"

	goalx "github.com/vonbai/goalx"
)

func TestGlobalRunRegistryLookupHydratesRunIDFromRunScopedMetadata(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	cfg := &goalx.Config{
		Name:      "alpha",
		Mode:      goalx.ModeResearch,
		Objective: "audit alpha",
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}

	reg := &GlobalRunRegistry{
		Version: 1,
		Runs: map[string]GlobalRunRef{
			globalRunKey(repo, cfg.Name): {
				Name:        cfg.Name,
				ProjectID:   goalx.ProjectID(repo),
				ProjectRoot: repo,
				RunDir:      runDir,
				State:       "active",
			},
		},
	}
	if err := SaveGlobalRunRegistry(reg); err != nil {
		t.Fatalf("SaveGlobalRunRegistry: %v", err)
	}

	matches, err := LookupGlobalRuns(meta.RunID)
	if err != nil {
		t.Fatalf("LookupGlobalRuns: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches len = %d, want 1", len(matches))
	}
	if matches[0].RunID != meta.RunID {
		t.Fatalf("matches[0].RunID = %q, want %q", matches[0].RunID, meta.RunID)
	}
}

func TestUpsertGlobalRunPreservesConcurrentEntries(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")

	configs := make([]*goalx.Config, 0, 8)
	for i := 0; i < 8; i++ {
		configs = append(configs, &goalx.Config{
			Name:      fmt.Sprintf("run-%d", i),
			Mode:      goalx.ModeDevelop,
			Objective: fmt.Sprintf("ship run %d", i),
		})
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, cfg := range configs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if err := UpsertGlobalRun(repo, cfg, "active"); err != nil {
				t.Errorf("UpsertGlobalRun(%s): %v", cfg.Name, err)
			}
		}()
	}
	close(start)
	wg.Wait()

	reg, err := LoadGlobalRunRegistry()
	if err != nil {
		t.Fatalf("LoadGlobalRunRegistry: %v", err)
	}
	for _, cfg := range configs {
		if _, ok := reg.Runs[globalRunKey(repo, cfg.Name)]; !ok {
			t.Fatalf("missing global run %s in %#v", cfg.Name, reg.Runs)
		}
	}
}
