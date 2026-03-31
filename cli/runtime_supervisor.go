package cli

import (
	"os"
	"strings"
	"time"
)

type RunHostState struct {
	Version   int    `json:"version"`
	Kind      string `json:"kind,omitempty"`
	Launcher  string `json:"launcher,omitempty"`
	Unit      string `json:"unit,omitempty"`
	RunDir    string `json:"run_dir,omitempty"`
	RunName   string `json:"run_name,omitempty"`
	Running   bool   `json:"running"`
	PID       int    `json:"pid,omitempty"`
	Transport string `json:"transport,omitempty"`
	RunID     string `json:"run_id,omitempty"`
	Epoch     int    `json:"epoch,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type RuntimeSupervisorStartSpec struct {
	ProjectRoot string
	RunName     string
	RunDir      string
	Interval    time.Duration
}

type RuntimeSupervisor interface {
	Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error)
	Stop(runDir string) error
	Inspect(runDir string) (*RunHostState, error)
}

var runtimeSupervisor RuntimeSupervisor = defaultRuntimeSupervisor{}

type defaultRuntimeSupervisor struct{}

func (defaultRuntimeSupervisor) Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error) {
	host, err := runtimeHostLauncher.Start(spec)
	if err != nil {
		return nil, err
	}
	if host == nil {
		host = &RunHostState{Version: 1, RunDir: spec.RunDir, RunName: spec.RunName, UpdatedAt: time.Now().UTC().Format(time.RFC3339)}
	}
	if err := SaveRunHostState(RunHostStatePath(spec.RunDir), host); err != nil {
		return nil, err
	}
	return defaultRuntimeSupervisor{}.Inspect(spec.RunDir)
}

func (defaultRuntimeSupervisor) Stop(runDir string) error {
	host, err := LoadRunHostState(RunHostStatePath(runDir))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if host == nil && os.IsNotExist(err) {
		return runtimeHostLauncher.Stop(runDir, nil)
	}
	if err := runtimeHostLauncher.Stop(runDir, host); err != nil {
		return err
	}
	inspected, inspectErr := runtimeHostLauncher.Inspect(runDir, host)
	if inspectErr == nil && inspected != nil {
		return SaveRunHostState(RunHostStatePath(runDir), inspected)
	}
	if host != nil {
		host.Running = false
		host.PID = 0
		host.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return SaveRunHostState(RunHostStatePath(runDir), host)
	}
	return nil
}

func (defaultRuntimeSupervisor) Inspect(runDir string) (*RunHostState, error) {
	host, err := LoadRunHostState(RunHostStatePath(runDir))
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		lease, leaseErr := LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
		if leaseErr != nil {
			return nil, leaseErr
		}
		state := &RunHostState{
			Version:   1,
			Kind:      "runtime_host",
			Launcher:  "process",
			RunDir:    runDir,
			Running:   controlLeaseActive(runDir, "runtime-host"),
			UpdatedAt: lease.RenewedAt,
		}
		if lease != nil {
			state.PID = lease.PID
			state.Transport = lease.Transport
			state.RunID = strings.TrimSpace(lease.RunID)
			state.Epoch = lease.Epoch
			if strings.TrimSpace(state.UpdatedAt) == "" {
				state.UpdatedAt = strings.TrimSpace(lease.ExpiresAt)
			}
		}
		return state, nil
	}
	inspected, err := runtimeHostLauncher.Inspect(runDir, host)
	if err != nil {
		return nil, err
	}
	if inspected != nil {
		_ = SaveRunHostState(RunHostStatePath(runDir), inspected)
	}
	return inspected, nil
}
