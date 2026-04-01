package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CognitionState struct {
	Version   int                   `json:"version"`
	Scopes    []CognitionScopeState `json:"scopes"`
	UpdatedAt string                `json:"updated_at,omitempty"`
}

type CognitionScopeState struct {
	Scope        string                   `json:"scope"`
	WorktreePath string                   `json:"worktree_path,omitempty"`
	Providers    []CognitionProviderState `json:"providers"`
}

type CognitionProviderState struct {
	Name                    string   `json:"name"`
	InvocationKind          string   `json:"invocation_kind"`
	Available               bool     `json:"available"`
	Command                 string   `json:"command,omitempty"`
	Version                 string   `json:"version,omitempty"`
	ReadTransportsSupported []string `json:"read_transports_supported,omitempty"`
	MCPServerCommand        string   `json:"mcp_server_command,omitempty"`
	MCPToolsSupported       []string `json:"mcp_tools_supported,omitempty"`
	MCPResourcesSupported   []string `json:"mcp_resources_supported,omitempty"`
	RepoRoot                string   `json:"repo_root,omitempty"`
	StoragePath             string   `json:"storage_path,omitempty"`
	RegistryName            string   `json:"registry_name,omitempty"`
	IndexState              string   `json:"index_state,omitempty"`
	IndexProvenance         string   `json:"index_provenance,omitempty"`
	IndexedRevision         string   `json:"indexed_revision,omitempty"`
	HeadRevision            string   `json:"head_revision,omitempty"`
	StaleCommits            int      `json:"stale_commits,omitempty"`
	LastRefreshError        string   `json:"last_refresh_error,omitempty"`
	AnalyzedInScopeAt       string   `json:"analyzed_in_scope_at,omitempty"`
	Capabilities            []string `json:"capabilities"`
	CheckedAt               string   `json:"checked_at,omitempty"`
}

func CognitionStatePath(runDir string) string {
	return filepath.Join(runDir, "cognition-state.json")
}

func LoadCognitionState(path string) (*CognitionState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	state, err := parseCognitionState(data)
	if err != nil {
		return nil, fmt.Errorf("parse cognition state: %w", err)
	}
	return state, nil
}

func SaveCognitionState(path string, state *CognitionState) error {
	if state == nil {
		return fmt.Errorf("cognition state is nil")
	}
	if err := validateCognitionStateInput(state); err != nil {
		return err
	}
	normalizeCognitionState(state)
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONFile(path, state)
}

func parseCognitionState(data []byte) (*CognitionState, error) {
	var state CognitionState
	if err := decodeStrictJSON(data, &state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCognitionState, err)
	}
	if err := validateCognitionStateInput(&state); err != nil {
		return nil, durableSchemaHintError(DurableSurfaceCognitionState, err)
	}
	normalizeCognitionState(&state)
	return &state, nil
}

func validateCognitionStateInput(state *CognitionState) error {
	if state == nil {
		return fmt.Errorf("cognition state is nil")
	}
	if state.Version <= 0 {
		return fmt.Errorf("cognition state version must be positive")
	}
	if len(state.Scopes) == 0 {
		return fmt.Errorf("cognition state scopes are required")
	}
	seenScopes := map[string]struct{}{}
	for _, scope := range state.Scopes {
		if strings.TrimSpace(scope.Scope) == "" {
			return fmt.Errorf("cognition state scope is required")
		}
		if len(scope.Providers) == 0 {
			return fmt.Errorf("cognition state scope %s providers are required", scope.Scope)
		}
		if _, ok := seenScopes[scope.Scope]; ok {
			return fmt.Errorf("duplicate cognition state scope %q", scope.Scope)
		}
		seenScopes[scope.Scope] = struct{}{}
		seenProviders := map[string]struct{}{}
		for _, provider := range scope.Providers {
			if strings.TrimSpace(provider.Name) == "" {
				return fmt.Errorf("cognition provider name is required")
			}
			if strings.TrimSpace(provider.InvocationKind) == "" {
				return fmt.Errorf("cognition provider %s invocation_kind is required", provider.Name)
			}
			switch strings.TrimSpace(provider.IndexState) {
			case "", "missing", "fresh", "stale", "unknown":
			default:
				return fmt.Errorf("cognition provider %s index_state %q is invalid", provider.Name, provider.IndexState)
			}
			switch strings.TrimSpace(provider.IndexProvenance) {
			case "", "seeded", "local":
			default:
				return fmt.Errorf("cognition provider %s index_provenance %q is invalid", provider.Name, provider.IndexProvenance)
			}
			for _, transport := range compactStrings(provider.ReadTransportsSupported) {
				switch transport {
				case "cli", "mcp":
				default:
					return fmt.Errorf("cognition provider %s read_transports_supported contains invalid value %q", provider.Name, transport)
				}
			}
			if len(compactStrings(provider.Capabilities)) == 0 {
				return fmt.Errorf("cognition provider %s capabilities are required", provider.Name)
			}
			if _, ok := seenProviders[provider.Name]; ok {
				return fmt.Errorf("duplicate cognition provider %q in scope %s", provider.Name, scope.Scope)
			}
			seenProviders[provider.Name] = struct{}{}
		}
	}
	return nil
}

func normalizeCognitionState(state *CognitionState) {
	if state.Version <= 0 {
		state.Version = 1
	}
	state.UpdatedAt = strings.TrimSpace(state.UpdatedAt)
	if state.Scopes == nil {
		state.Scopes = []CognitionScopeState{}
	}
	for i := range state.Scopes {
		state.Scopes[i].Scope = strings.TrimSpace(state.Scopes[i].Scope)
		state.Scopes[i].WorktreePath = strings.TrimSpace(state.Scopes[i].WorktreePath)
		if state.Scopes[i].Providers == nil {
			state.Scopes[i].Providers = []CognitionProviderState{}
		}
		for j := range state.Scopes[i].Providers {
			provider := &state.Scopes[i].Providers[j]
			provider.Name = strings.TrimSpace(provider.Name)
			provider.InvocationKind = strings.TrimSpace(provider.InvocationKind)
			provider.Command = strings.TrimSpace(provider.Command)
			provider.Version = strings.TrimSpace(provider.Version)
			provider.ReadTransportsSupported = compactStrings(provider.ReadTransportsSupported)
			provider.MCPServerCommand = strings.TrimSpace(provider.MCPServerCommand)
			provider.MCPToolsSupported = compactStrings(provider.MCPToolsSupported)
			provider.MCPResourcesSupported = compactStrings(provider.MCPResourcesSupported)
			provider.RepoRoot = strings.TrimSpace(provider.RepoRoot)
			provider.StoragePath = strings.TrimSpace(provider.StoragePath)
			provider.RegistryName = strings.TrimSpace(provider.RegistryName)
			provider.IndexState = strings.TrimSpace(provider.IndexState)
			provider.IndexProvenance = strings.TrimSpace(provider.IndexProvenance)
			provider.IndexedRevision = strings.TrimSpace(provider.IndexedRevision)
			provider.HeadRevision = strings.TrimSpace(provider.HeadRevision)
			provider.LastRefreshError = strings.TrimSpace(provider.LastRefreshError)
			provider.AnalyzedInScopeAt = strings.TrimSpace(provider.AnalyzedInScopeAt)
			provider.Capabilities = compactStrings(provider.Capabilities)
			provider.CheckedAt = strings.TrimSpace(provider.CheckedAt)
		}
	}
}
