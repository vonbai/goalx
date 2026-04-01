package cli

import (
	"os"
	"strings"
	"testing"
)

func TestSaveCognitionStateRoundTrip(t *testing.T) {
	runDir := t.TempDir()
	path := CognitionStatePath(runDir)
	state := &CognitionState{
		Version: 1,
		Scopes: []CognitionScopeState{
			{
				Scope:        "run-root",
				WorktreePath: "/abs/run-root",
				Providers: []CognitionProviderState{
					{
						Name:           "repo-native",
						InvocationKind: "builtin",
						Available:      true,
						IndexState:     "fresh",
						HeadRevision:   "def456",
						Capabilities:   []string{"file_search", "git_diff"},
					},
					{
						Name:                    "gitnexus",
						InvocationKind:          "binary",
						Available:               true,
						Command:                 "gitnexus",
						Version:                 "1.5.0",
						ReadTransportsSupported: []string{"cli", "mcp"},
						MCPServerCommand:        "gitnexus mcp",
						MCPToolsSupported:       []string{"list_repos", "query", "context", "impact", "detect_changes", "rename"},
						MCPResourcesSupported:   []string{"gitnexus://repos", "gitnexus://repo/{name}/context"},
						RepoRoot:                "/abs/run-root",
						StoragePath:             "/abs/run-root/.gitnexus",
						RegistryName:            "demo-repo",
						IndexState:              "stale",
						IndexProvenance:         "seeded",
						IndexedRevision:         "abc123",
						HeadRevision:            "def456",
						StaleCommits:            2,
						LastRefreshError:        "status parse warning",
						AnalyzedInScopeAt:       "2026-04-01T00:00:00Z",
						Capabilities:            []string{"query", "context", "impact"},
						CheckedAt:               "2026-04-01T00:00:00Z",
					},
				},
			},
		},
	}

	if err := SaveCognitionState(path, state); err != nil {
		t.Fatalf("SaveCognitionState: %v", err)
	}
	loaded, err := LoadCognitionState(path)
	if err != nil {
		t.Fatalf("LoadCognitionState: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadCognitionState returned nil state")
	}
	if len(loaded.Scopes) != 1 || len(loaded.Scopes[0].Providers) != 2 {
		t.Fatalf("scopes = %#v, want one scope with two providers", loaded.Scopes)
	}
	got := loaded.Scopes[0].Providers[1]
	if got.IndexState != "stale" || got.RegistryName != "demo-repo" || got.LastRefreshError == "" || got.IndexProvenance != "seeded" || got.AnalyzedInScopeAt == "" {
		t.Fatalf("gitnexus provider = %+v, want enriched provider state facts", got)
	}
	if len(got.ReadTransportsSupported) != 2 || got.MCPServerCommand == "" || len(got.MCPToolsSupported) == 0 || len(got.MCPResourcesSupported) == 0 {
		t.Fatalf("gitnexus mcp capability facts missing: %+v", got)
	}
}

func TestLoadCognitionStateRejectsProviderWithoutCapabilities(t *testing.T) {
	path := CognitionStatePath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "scopes": [
    {
      "scope": "run-root",
      "providers": [
        {
          "name": "repo-native",
          "invocation_kind": "builtin",
          "available": true
        }
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCognitionState(path)
	if err == nil {
		t.Fatal("LoadCognitionState should reject provider without capabilities")
	}
	if !strings.Contains(err.Error(), "capabilities") {
		t.Fatalf("LoadCognitionState error = %v, want capabilities hint", err)
	}
}

func TestLoadCognitionStateRejectsInvalidIndexState(t *testing.T) {
	path := CognitionStatePath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "scopes": [
    {
      "scope": "run-root",
      "providers": [
        {
          "name": "gitnexus",
          "invocation_kind": "binary",
          "available": true,
          "index_state": "ready-ish",
          "capabilities": ["query"]
        }
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCognitionState(path)
	if err == nil {
		t.Fatal("LoadCognitionState should reject invalid index_state")
	}
	if !strings.Contains(err.Error(), "index_state") {
		t.Fatalf("LoadCognitionState error = %v, want index_state hint", err)
	}
}

func TestLoadCognitionStateRejectsInvalidIndexProvenance(t *testing.T) {
	path := CognitionStatePath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "scopes": [
    {
      "scope": "run-root",
      "providers": [
        {
          "name": "gitnexus",
          "invocation_kind": "binary",
          "available": true,
          "index_state": "fresh",
          "index_provenance": "borrowed",
          "capabilities": ["query"]
        }
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCognitionState(path)
	if err == nil {
		t.Fatal("LoadCognitionState should reject invalid index_provenance")
	}
	if !strings.Contains(err.Error(), "index_provenance") {
		t.Fatalf("LoadCognitionState error = %v, want index_provenance hint", err)
	}
}

func TestLoadCognitionStateRejectsInvalidReadTransport(t *testing.T) {
	path := CognitionStatePath(t.TempDir())
	if err := os.WriteFile(path, []byte(`{
  "version": 1,
  "scopes": [
    {
      "scope": "run-root",
      "providers": [
        {
          "name": "gitnexus",
          "invocation_kind": "binary",
          "available": true,
          "index_state": "fresh",
          "read_transports_supported": ["cli", "socket_magic"],
          "capabilities": ["query"]
        }
      ]
    }
  ]
}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCognitionState(path)
	if err == nil {
		t.Fatal("LoadCognitionState should reject invalid read transport")
	}
	if !strings.Contains(err.Error(), "read_transports_supported") {
		t.Fatalf("LoadCognitionState error = %v, want read_transports_supported hint", err)
	}
}
