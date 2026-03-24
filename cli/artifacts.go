package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	goalx "github.com/vonbai/goalx"
)

type ArtifactMeta struct {
	Kind        string `json:"kind"`
	Path        string `json:"path"`
	RelPath     string `json:"rel_path,omitempty"`
	DurableName string `json:"durable_name,omitempty"`
}

type SessionArtifacts struct {
	Name      string         `json:"name"`
	Mode      string         `json:"mode,omitempty"`
	Artifacts []ArtifactMeta `json:"artifacts,omitempty"`
}

type ArtifactsManifest struct {
	Run      string             `json:"run"`
	Version  int                `json:"version"`
	Sessions []SessionArtifacts `json:"sessions,omitempty"`
}

func ArtifactsPath(runDir string) string {
	return filepath.Join(runDir, "artifacts.json")
}

func LoadArtifacts(path string) (*ArtifactsManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var manifest ArtifactsManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &manifest, nil
}

func SaveArtifacts(path string, manifest *ArtifactsManifest) error {
	if manifest == nil {
		return fmt.Errorf("artifacts manifest is nil")
	}
	if manifest.Version <= 0 {
		manifest.Version = 1
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EnsureArtifactsManifest(runDir string) (*ArtifactsManifest, error) {
	path := ArtifactsPath(runDir)
	manifest, err := LoadArtifacts(path)
	if err != nil {
		return nil, err
	}
	if manifest == nil {
		manifest = &ArtifactsManifest{
			Run:     filepath.Base(runDir),
			Version: 1,
		}
		if err := SaveArtifacts(path, manifest); err != nil {
			return nil, err
		}
		return manifest, nil
	}
	if manifest.Run == "" {
		manifest.Run = filepath.Base(runDir)
	}
	if manifest.Version <= 0 {
		manifest.Version = 1
	}
	if err := SaveArtifacts(path, manifest); err != nil {
		return nil, err
	}
	return manifest, nil
}

func RegisterSessionArtifact(runDir, session string, meta ArtifactMeta) error {
	if strings.TrimSpace(session) == "" {
		return fmt.Errorf("session name is required")
	}
	manifest, err := EnsureArtifactsManifest(runDir)
	if err != nil {
		return err
	}
	sessionState := ensureSessionArtifactsEntry(manifest, session, "")
	upsertArtifact(sessionState, meta)
	return SaveArtifacts(ArtifactsPath(runDir), manifest)
}

func CopyArtifactsManifest(runDir, saveDir string) error {
	manifest, err := EnsureArtifactsManifest(runDir)
	if err != nil {
		return err
	}
	return SaveArtifacts(filepath.Join(saveDir, "artifacts.json"), manifest)
}

func ResolveRunArtifacts(runDir string, cfg *goalx.Config) (*ArtifactsManifest, bool, error) {
	manifest, err := LoadArtifacts(ArtifactsPath(runDir))
	if err != nil {
		return nil, false, err
	}
	if manifest != nil {
		if manifest.Run == "" {
			manifest.Run = filepath.Base(runDir)
		}
		if manifest.Version <= 0 {
			manifest.Version = 1
		}
		return manifest, true, nil
	}
	return buildRunArtifactsManifest(runDir, cfg), false, nil
}

func artifactKey(meta ArtifactMeta) string {
	if meta.DurableName != "" {
		return "durable:" + meta.DurableName
	}
	if meta.RelPath != "" {
		return "rel:" + meta.RelPath
	}
	return "path:" + meta.Path
}

func EnsureRunArtifacts(runDir string, cfg *goalx.Config) (*ArtifactsManifest, error) {
	manifest, err := EnsureArtifactsManifest(runDir)
	if err != nil {
		return nil, err
	}
	built := buildRunArtifactsManifest(runDir, cfg)
	if built == nil {
		return manifest, nil
	}
	changed := false
	for _, session := range built.Sessions {
		sessionState := ensureSessionArtifactsEntry(manifest, session.Name, session.Mode)
		for _, artifact := range session.Artifacts {
			if upsertArtifact(sessionState, artifact) {
				changed = true
			}
		}
	}
	if changed {
		if err := SaveArtifacts(ArtifactsPath(runDir), manifest); err != nil {
			return nil, err
		}
	}
	return manifest, nil
}

func buildRunArtifactsManifest(runDir string, cfg *goalx.Config) *ArtifactsManifest {
	manifest := &ArtifactsManifest{
		Run:     filepath.Base(runDir),
		Version: 1,
	}
	if cfg == nil {
		return manifest
	}

	indexes, err := existingSessionIndexes(runDir)
	if err != nil {
		return manifest
	}
	sessionsState, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		return manifest
	}
	for _, num := range indexes {
		sessionName := SessionName(num)
		identity, err := RequireSessionIdentity(runDir, sessionName)
		if err != nil {
			continue
		}
		sessionArtifacts := ensureSessionArtifactsEntry(manifest, sessionName, identity.Mode)
		if goalx.Mode(identity.Mode) != goalx.ModeResearch {
			continue
		}

		reportPath, relPath := resolveSessionReportArtifact(runDir, cfg.Name, sessionName, identity.Target.Files, sessionsState)
		if reportPath == "" {
			continue
		}
		upsertArtifact(sessionArtifacts, ArtifactMeta{
			Kind:        "report",
			Path:        reportPath,
			RelPath:     relPath,
			DurableName: fmt.Sprintf("%s-report.md", sessionName),
		})
	}
	return manifest
}

func resolveSessionReportArtifact(runDir, runName, sessionName string, targetFiles []string, sessionsState *SessionsRuntimeState) (string, string) {
	if reportPath := findRunScopedReport(runDir, sessionName); reportPath != "" {
		return reportPath, filepath.Base(reportPath)
	}

	reportRoot := resolvedSessionWorktreePath(runDir, runName, sessionName, sessionsState)
	if reportRoot == "" {
		reportRoot = RunWorktreePath(runDir)
	}
	reportPath := findSessionReport(reportRoot, targetFiles)
	if reportPath == "" {
		return "", ""
	}
	relPath, err := filepath.Rel(reportRoot, reportPath)
	if err != nil {
		relPath = filepath.Base(reportPath)
	}
	return reportPath, relPath
}

func findRunScopedReport(runDir, sessionName string) string {
	candidate := filepath.Join(ReportsDir(runDir), fmt.Sprintf("%s-report.md", sessionName))
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() || info.Size() == 0 {
		return ""
	}
	return candidate
}

func FindSessionArtifacts(manifest *ArtifactsManifest, sessionName string) *SessionArtifacts {
	if manifest == nil {
		return nil
	}
	for i := range manifest.Sessions {
		if manifest.Sessions[i].Name == sessionName {
			return &manifest.Sessions[i]
		}
	}
	return nil
}

func FindSessionArtifact(manifest *ArtifactsManifest, sessionName, kind string) *ArtifactMeta {
	if manifest == nil {
		return nil
	}
	for _, session := range manifest.Sessions {
		if session.Name != sessionName {
			continue
		}
		for _, artifact := range session.Artifacts {
			if artifact.Kind == kind {
				copy := artifact
				return &copy
			}
		}
	}
	return nil
}

func CollectSavedResearchContext(runDir string) ([]string, []string, error) {
	manifest, err := LoadArtifacts(filepath.Join(runDir, "artifacts.json"))
	if err != nil {
		return nil, nil, err
	}

	var contextFiles []string
	var sessionNames []string
	seen := map[string]bool{}
	addPath := func(path string) {
		if path == "" || seen[path] {
			return
		}
		contextFiles = append(contextFiles, path)
		seen[path] = true
	}

	summaryPath := filepath.Join(runDir, "summary.md")
	if info, err := os.Stat(summaryPath); err == nil && !info.IsDir() && info.Size() > 0 {
		addPath(summaryPath)
	}
	if manifest != nil {
		for _, session := range manifest.Sessions {
			for _, artifact := range session.Artifacts {
				if artifact.Kind != "report" {
					continue
				}
				addPath(artifact.Path)
				if session.Name != "" {
					sessionNames = append(sessionNames, session.Name)
				}
			}
		}
	}
	if len(contextFiles) == 0 {
		absRunDir, _ := filepath.Abs(runDir)
		entries, _ := os.ReadDir(runDir)
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, "-report.md") || name == "summary.md" {
				addPath(filepath.Join(absRunDir, name))
			}
			if strings.HasSuffix(name, "-report.md") {
				sessionNames = append(sessionNames, strings.TrimSuffix(name, "-report.md"))
			}
		}
	}

	sort.Strings(sessionNames)
	return contextFiles, compactSorted(sessionNames), nil
}

func ensureSessionArtifactsEntry(manifest *ArtifactsManifest, session, mode string) *SessionArtifacts {
	for i := range manifest.Sessions {
		if manifest.Sessions[i].Name == session {
			if manifest.Sessions[i].Mode == "" && mode != "" {
				manifest.Sessions[i].Mode = mode
			}
			return &manifest.Sessions[i]
		}
	}
	manifest.Sessions = append(manifest.Sessions, SessionArtifacts{Name: session, Mode: mode})
	return &manifest.Sessions[len(manifest.Sessions)-1]
}

func upsertArtifact(session *SessionArtifacts, meta ArtifactMeta) bool {
	artifacts := session.Artifacts
	key := artifactKey(meta)
	for i := range artifacts {
		if artifactKey(artifacts[i]) == key {
			if artifacts[i] == meta {
				return false
			}
			artifacts[i] = meta
			session.Artifacts = artifacts
			return true
		}
	}
	session.Artifacts = append(artifacts, meta)
	return true
}

func compactSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
