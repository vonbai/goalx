package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "goalx-cli-home-")
	if err != nil {
		panic(err)
	}
	pathDir, err := os.MkdirTemp("", "goalx-cli-path-")
	if err != nil {
		panic(err)
	}
	for _, name := range []string{"claude", "codex"} {
		path := filepath.Join(pathDir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			panic(err)
		}
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("PATH", pathDir+":"+os.Getenv("PATH"))
	code := m.Run()
	_ = os.RemoveAll(home)
	_ = os.RemoveAll(pathDir)
	os.Exit(code)
}
