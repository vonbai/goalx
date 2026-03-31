package cli

import (
	"testing"
	"time"
)

type runtimeSupervisorStub struct {
	startCalls    int
	lastStartSpec RuntimeSupervisorStartSpec
	startErr      error
	startState    *RunHostState

	stopCalls      int
	lastStopRunDir string
	stopErr        error

	inspectCalls      int
	lastInspectRunDir string
	inspectErr        error
	inspectState      *RunHostState
}

func (s *runtimeSupervisorStub) Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error) {
	s.startCalls++
	s.lastStartSpec = spec
	if s.startErr != nil {
		return nil, s.startErr
	}
	if s.startState != nil {
		return s.startState, nil
	}
	return &RunHostState{
		Kind:      "runtime_host",
		Running:   true,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *runtimeSupervisorStub) Stop(runDir string) error {
	s.stopCalls++
	s.lastStopRunDir = runDir
	return s.stopErr
}

func (s *runtimeSupervisorStub) Inspect(runDir string) (*RunHostState, error) {
	s.inspectCalls++
	s.lastInspectRunDir = runDir
	if s.inspectErr != nil {
		return nil, s.inspectErr
	}
	return s.inspectState, nil
}

type runtimeHostLauncherStub struct {
	startCalls    int
	lastStartSpec RuntimeSupervisorStartSpec
	startState    *RunHostState
	startErr      error

	stopCalls    int
	lastStopRun  string
	lastStopHost *RunHostState
	stopErr      error

	inspectCalls    int
	lastInspectRun  string
	lastInspectHost *RunHostState
	inspectState    *RunHostState
	inspectErr      error
}

func (s *runtimeHostLauncherStub) Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error) {
	s.startCalls++
	s.lastStartSpec = spec
	if s.startErr != nil {
		return nil, s.startErr
	}
	return cloneRunHostState(s.startState), nil
}

func (s *runtimeHostLauncherStub) Stop(runDir string, host *RunHostState) error {
	s.stopCalls++
	s.lastStopRun = runDir
	s.lastStopHost = cloneRunHostState(host)
	return s.stopErr
}

func (s *runtimeHostLauncherStub) Inspect(runDir string, host *RunHostState) (*RunHostState, error) {
	s.inspectCalls++
	s.lastInspectRun = runDir
	s.lastInspectHost = cloneRunHostState(host)
	if s.inspectErr != nil {
		return nil, s.inspectErr
	}
	return cloneRunHostState(s.inspectState), nil
}

func stubRuntimeSupervisor(t *testing.T) *runtimeSupervisorStub {
	t.Helper()
	origRuntimeSupervisor := runtimeSupervisor
	stub := &runtimeSupervisorStub{}
	runtimeSupervisor = stub
	t.Cleanup(func() { runtimeSupervisor = origRuntimeSupervisor })
	return stub
}

func stubRuntimeSupervisorWithError(t *testing.T, err error) *runtimeSupervisorStub {
	t.Helper()
	origRuntimeSupervisor := runtimeSupervisor
	stub := &runtimeSupervisorStub{startErr: err, stopErr: err}
	runtimeSupervisor = stub
	t.Cleanup(func() { runtimeSupervisor = origRuntimeSupervisor })
	return stub
}

func TestDefaultRuntimeSupervisorPersistsRunHostState(t *testing.T) {
	runDir := t.TempDir()
	launcher := &runtimeHostLauncherStub{
		startState: &RunHostState{
			Version:   1,
			Kind:      "runtime_host",
			Launcher:  "process",
			RunDir:    runDir,
			RunName:   "demo",
			Running:   true,
			PID:       4242,
			Transport: "process",
			RunID:     "run_demo",
			Epoch:     3,
			UpdatedAt: "2026-03-31T00:00:00Z",
		},
		inspectState: &RunHostState{
			Version:   1,
			Kind:      "runtime_host",
			Launcher:  "process",
			RunDir:    runDir,
			RunName:   "demo",
			Running:   true,
			PID:       4242,
			Transport: "process",
			RunID:     "run_demo",
			Epoch:     3,
			UpdatedAt: "2026-03-31T00:00:01Z",
		},
	}
	origLauncher := runtimeHostLauncher
	defer func() { runtimeHostLauncher = origLauncher }()
	runtimeHostLauncher = launcher

	host, err := defaultRuntimeSupervisor{}.Start(RuntimeSupervisorStartSpec{
		ProjectRoot: "/tmp/project",
		RunName:     "demo",
		RunDir:      runDir,
		Interval:    5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if host == nil || host.PID != 4242 || !host.Running {
		t.Fatalf("returned host = %+v", host)
	}
	saved, err := LoadRunHostState(RunHostStatePath(runDir))
	if err != nil {
		t.Fatalf("LoadRunHostState: %v", err)
	}
	if saved == nil || saved.PID != 4242 || saved.Launcher != "process" {
		t.Fatalf("saved host = %+v", saved)
	}
}

func TestDefaultRuntimeSupervisorInspectUsesSavedHostMetadata(t *testing.T) {
	runDir := t.TempDir()
	if err := SaveRunHostState(RunHostStatePath(runDir), &RunHostState{
		Version:   1,
		Kind:      "runtime_host",
		Launcher:  "systemd",
		Unit:      "goalx-runtime-host-demo",
		RunDir:    runDir,
		RunName:   "demo",
		Running:   true,
		PID:       111,
		UpdatedAt: "2026-03-31T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveRunHostState: %v", err)
	}
	launcher := &runtimeHostLauncherStub{
		inspectState: &RunHostState{
			Version:   1,
			Kind:      "runtime_host",
			Launcher:  "systemd",
			Unit:      "goalx-runtime-host-demo",
			RunDir:    runDir,
			RunName:   "demo",
			Running:   false,
			PID:       0,
			UpdatedAt: "2026-03-31T00:00:10Z",
		},
	}
	origLauncher := runtimeHostLauncher
	defer func() { runtimeHostLauncher = origLauncher }()
	runtimeHostLauncher = launcher

	host, err := defaultRuntimeSupervisor{}.Inspect(runDir)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if launcher.inspectCalls != 1 {
		t.Fatalf("inspect calls = %d, want 1", launcher.inspectCalls)
	}
	if launcher.lastInspectHost == nil || launcher.lastInspectHost.Unit != "goalx-runtime-host-demo" {
		t.Fatalf("inspect host = %+v", launcher.lastInspectHost)
	}
	if host == nil || host.Running {
		t.Fatalf("inspected host = %+v, want stopped host", host)
	}
}
