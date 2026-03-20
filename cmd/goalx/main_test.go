package main

import "testing"

func TestDispatchRoutesServeCommand(t *testing.T) {
	oldServeCommand := serveCommand
	defer func() {
		serveCommand = oldServeCommand
	}()

	called := false
	serveCommand = func(projectRoot string, args []string) error {
		called = true
		if projectRoot == "" {
			t.Fatal("projectRoot is empty")
		}
		if len(args) != 2 || args[0] != "--bind" || args[1] != "100.110.196.103:18790" {
			t.Fatalf("args = %#v, want [--bind 100.110.196.103:18790]", args)
		}
		return nil
	}

	if err := dispatch(t.TempDir(), "serve", []string{"--bind", "100.110.196.103:18790"}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !called {
		t.Fatal("serve command was not called")
	}
}
