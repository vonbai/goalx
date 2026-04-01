package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	goalx "github.com/vonbai/goalx"
)

func splitContextFlagValue(raw string) ([]string, error) {
	var (
		items   []string
		current strings.Builder
		escape  bool
	)

	flush := func() {
		item := strings.TrimSpace(current.String())
		if item != "" {
			items = append(items, item)
		}
		current.Reset()
	}

	for _, r := range raw {
		switch {
		case escape:
			current.WriteRune(r)
			escape = false
		case r == '\\':
			escape = true
		case r == ',':
			flush()
		default:
			current.WriteRune(r)
		}
	}
	if escape {
		return nil, fmt.Errorf("invalid --context value %q: trailing escape", raw)
	}
	flush()
	return items, nil
}

// DiscoverContextFiles expands paths relative to the current working directory.
func DiscoverContextFiles(paths []string) ([]string, error) {
	return DiscoverContextFilesFrom("", paths)
}

// DiscoverContextFilesFrom expands paths relative to baseDir when they are not
// already absolute. Directories are scanned for key files; regular files are
// included directly. All returned paths are absolute.
func DiscoverContextFilesFrom(baseDir string, paths []string) ([]string, error) {
	var result []string
	seen := make(map[string]bool)

	for _, p := range paths {
		absPath, err := resolveContextPath(baseDir, p)
		if err != nil {
			return nil, err
		}

		info, err := os.Stat(absPath)
		if err != nil {
			return nil, err
		}

		if !info.IsDir() {
			if !seen[absPath] {
				result = append(result, absPath)
				seen[absPath] = true
			}
			continue
		}

		// Directory: discover key files
		discovered := discoverKeyFiles(absPath)
		for _, f := range discovered {
			if !seen[f] {
				result = append(result, f)
				seen[f] = true
			}
		}
	}

	return result, nil
}

// ResolveContextInputs converts mixed CLI context inputs into canonical
// file and ref lists without silently guessing between the two.
//
// Accepted forms:
//   - existing files or directories
//   - URLs (stored as refs)
//   - ref:<value> or note:<value> (stored as refs without the prefix)
func ResolveContextInputs(inputs []string) (goalx.ContextConfig, error) {
	return ResolveContextInputsFrom("", inputs)
}

func ResolveContextInputsFrom(baseDir string, inputs []string) (goalx.ContextConfig, error) {
	var cfg goalx.ContextConfig
	seenFiles := make(map[string]bool)
	seenRefs := make(map[string]bool)

	addFile := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seenFiles[path] {
			return
		}
		cfg.Files = append(cfg.Files, path)
		seenFiles[path] = true
	}
	addRef := func(ref string) {
		ref = strings.TrimSpace(ref)
		if ref == "" || seenRefs[ref] {
			return
		}
		cfg.Refs = append(cfg.Refs, ref)
		seenRefs[ref] = true
	}

	for _, input := range inputs {
		raw := strings.TrimSpace(input)
		if raw == "" {
			continue
		}
		switch {
		case strings.HasPrefix(raw, "ref:"):
			ref := strings.TrimSpace(strings.TrimPrefix(raw, "ref:"))
			if ref == "" {
				return goalx.ContextConfig{}, fmt.Errorf("invalid --context item %q: empty ref", input)
			}
			addRef(ref)
			continue
		case strings.HasPrefix(raw, "note:"):
			ref := strings.TrimSpace(strings.TrimPrefix(raw, "note:"))
			if ref == "" {
				return goalx.ContextConfig{}, fmt.Errorf("invalid --context item %q: empty note", input)
			}
			addRef(ref)
			continue
		case looksLikeURL(raw):
			addRef(raw)
			continue
		}

		files, err := DiscoverContextFilesFrom(baseDir, []string{raw})
		if err != nil {
			return goalx.ContextConfig{}, fmt.Errorf("resolve --context item %q: %w", input, err)
		}
		for _, file := range files {
			addFile(file)
		}
	}

	return cfg, nil
}

func MergeContextConfigs(configs ...goalx.ContextConfig) goalx.ContextConfig {
	var merged goalx.ContextConfig
	seenFiles := make(map[string]bool)
	seenRefs := make(map[string]bool)
	for _, cfg := range configs {
		for _, file := range cfg.Files {
			file = strings.TrimSpace(file)
			if file == "" || seenFiles[file] {
				continue
			}
			merged.Files = append(merged.Files, file)
			seenFiles[file] = true
		}
		for _, ref := range cfg.Refs {
			ref = strings.TrimSpace(ref)
			if ref == "" || seenRefs[ref] {
				continue
			}
			merged.Refs = append(merged.Refs, ref)
			seenRefs[ref] = true
		}
	}
	return merged
}

func looksLikeURL(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

func resolveContextPath(baseDir, raw string) (string, error) {
	path := strings.TrimSpace(raw)
	if filepath.IsAbs(path) {
		return filepath.Abs(path)
	}
	if strings.TrimSpace(baseDir) != "" {
		return filepath.Abs(filepath.Join(baseDir, path))
	}
	return filepath.Abs(path)
}

// discoverKeyFiles finds important files in a project directory.
func discoverKeyFiles(dir string) []string {
	var files []string

	// Priority 1: documentation and config
	topLevel := []string{
		"README.md", "readme.md", "README",
		"CLAUDE.md", "AGENTS.md",
		"go.mod", "package.json", "pyproject.toml", "Cargo.toml",
		"Makefile", "Dockerfile",
	}
	for _, name := range topLevel {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}

	// Priority 2: main entry points (search common patterns)
	entryPoints := []string{
		"main.go", "cmd/*/main.go",
		"src/main.*", "src/index.*", "src/app.*",
		"lib/main.*", "app/main.*",
		"index.ts", "index.js", "app.py", "main.py",
	}
	for _, pattern := range entryPoints {
		matches, _ := filepath.Glob(filepath.Join(dir, pattern))
		for _, m := range matches {
			files = append(files, m)
		}
	}

	// Priority 3: doc directory (if exists, grab key files)
	docDirs := []string{"docs", "doc", "documentation"}
	for _, d := range docDirs {
		docDir := filepath.Join(dir, d)
		if info, err := os.Stat(docDir); err == nil && info.IsDir() {
			entries, _ := os.ReadDir(docDir)
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".rst") {
					files = append(files, filepath.Join(docDir, name))
				}
			}
			break // only first found doc dir
		}
	}

	return files
}
