package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	goalx "github.com/vonbai/goalx"
)

func RunHostStatePath(runDir string) string {
	return filepath.Join(ControlDir(runDir), "run-host.json")
}

func LoadRunHostState(path string) (*RunHostState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	state := &RunHostState{}
	if len(strings.TrimSpace(string(data))) == 0 {
		state.Version = 1
		return state, nil
	}
	if err := json.Unmarshal(data, state); err != nil {
		return nil, fmt.Errorf("parse run host state: %w", err)
	}
	if state.Version == 0 {
		state.Version = 1
	}
	return state, nil
}

func SaveRunHostState(path string, state *RunHostState) error {
	if state == nil {
		return fmt.Errorf("run host state is nil")
	}
	if state.Version == 0 {
		state.Version = 1
	}
	if strings.TrimSpace(state.UpdatedAt) == "" {
		state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return writeJSONFile(path, state)
}

type RuntimeHostLauncher interface {
	Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error)
	Stop(runDir string, host *RunHostState) error
	Inspect(runDir string, host *RunHostState) (*RunHostState, error)
}

var runtimeHostLauncher RuntimeHostLauncher = preferredRuntimeHostLauncher{}

type preferredRuntimeHostLauncher struct{}

func (preferredRuntimeHostLauncher) Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error) {
	if runtimeHostSystemdEnabled() {
		if host, err := (systemdRuntimeHostLauncher{}).Start(spec); err == nil {
			return host, nil
		}
	}
	return (detachedProcessRuntimeHostLauncher{}).Start(spec)
}

func (preferredRuntimeHostLauncher) Stop(runDir string, host *RunHostState) error {
	switch strings.TrimSpace(hostLauncherKind(host)) {
	case "systemd":
		return systemdRuntimeHostLauncher{}.Stop(runDir, host)
	default:
		return detachedProcessRuntimeHostLauncher{}.Stop(runDir, host)
	}
}

func (preferredRuntimeHostLauncher) Inspect(runDir string, host *RunHostState) (*RunHostState, error) {
	switch strings.TrimSpace(hostLauncherKind(host)) {
	case "systemd":
		return systemdRuntimeHostLauncher{}.Inspect(runDir, host)
	default:
		return detachedProcessRuntimeHostLauncher{}.Inspect(runDir, host)
	}
}

type detachedProcessRuntimeHostLauncher struct{}

func (detachedProcessRuntimeHostLauncher) Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error) {
	goalxBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve goalx executable: %w", err)
	}
	seconds := int(spec.Interval.Seconds())
	if seconds <= 0 {
		seconds = 300
	}
	logFile, err := os.OpenFile(filepath.Join(spec.RunDir, "runtime-host.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open runtime host log: %w", err)
	}
	defer logFile.Close()
	cmd := exec.Command(goalxBin, "runtime-host", "--run", spec.RunName, "--interval", strconv.Itoa(seconds))
	cmd.Dir = spec.ProjectRoot
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start runtime host: %w", err)
	}
	meta, _ := LoadRunMetadata(RunMetadataPath(spec.RunDir))
	epoch := 1
	if meta != nil && meta.Epoch > 0 {
		epoch = meta.Epoch
	}
	runID := ""
	if meta != nil {
		runID = meta.RunID
	}
	if err := RenewControlLease(spec.RunDir, "runtime-host", runID, epoch, spec.Interval*2, "process", cmd.Process.Pid); err != nil {
		return nil, err
	}
		host := &RunHostState{
			Version:   1,
			Kind:      "runtime_host",
			Launcher:  "process",
		RunDir:    spec.RunDir,
		RunName:   spec.RunName,
		Running:   true,
		PID:       cmd.Process.Pid,
		Transport: "process",
		RunID:     runID,
		Epoch:     epoch,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if err := cmd.Process.Release(); err != nil {
		return nil, err
	}
	return host, nil
}

func (detachedProcessRuntimeHostLauncher) Stop(runDir string, host *RunHostState) error {
	if host != nil && host.PID > 0 {
		proc, err := os.FindProcess(host.PID)
		if err == nil {
			_ = proc.Signal(syscall.SIGTERM)
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					return nil
				}
				time.Sleep(50 * time.Millisecond)
			}
			_ = proc.Signal(syscall.SIGKILL)
			return nil
		}
	}
	return defaultStopRunRuntimeHost(runDir)
}

func (detachedProcessRuntimeHostLauncher) Inspect(runDir string, host *RunHostState) (*RunHostState, error) {
	state := cloneRunHostState(host)
	if state == nil {
		state = &RunHostState{Version: 1, Kind: "runtime_host", Launcher: "process", RunDir: runDir}
	}
	if state.PID > 0 {
		state.Running = processAlive(state.PID)
		if strings.TrimSpace(state.UpdatedAt) == "" {
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		}
		if state.Transport == "" {
			state.Transport = "process"
		}
		return state, nil
	}
	lease, err := LoadControlLease(ControlLeasePath(runDir, "runtime-host"))
	if err != nil {
		return state, nil
	}
	state.Running = controlLeaseActive(runDir, "runtime-host")
	state.PID = lease.PID
	state.Transport = lease.Transport
	state.RunID = strings.TrimSpace(lease.RunID)
	state.Epoch = lease.Epoch
	state.UpdatedAt = strings.TrimSpace(lease.RenewedAt)
	if state.UpdatedAt == "" {
		state.UpdatedAt = strings.TrimSpace(lease.ExpiresAt)
	}
	return state, nil
}

type systemdRuntimeHostLauncher struct{}

func (systemdRuntimeHostLauncher) Start(spec RuntimeSupervisorStartSpec) (*RunHostState, error) {
	goalxBin, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve goalx executable: %w", err)
	}
	seconds := int(spec.Interval.Seconds())
	if seconds <= 0 {
		seconds = 300
	}
	unit := runtimeHostUnitName(spec.ProjectRoot, spec.RunName)
	logPath := filepath.Join(spec.RunDir, "runtime-host.log")
	script := fmt.Sprintf(
		"cd %s && exec %s runtime-host --run %s --interval %d >> %s 2>&1",
		shellQuote(spec.ProjectRoot),
		shellQuote(goalxBin),
		shellQuote(spec.RunName),
		seconds,
		shellQuote(logPath),
	)
	cmd := exec.Command(
		"systemd-run",
		"--user",
		"--unit", unit,
		"--service-type=simple",
		"--collect",
		"--property=KillMode=control-group",
		"/bin/bash",
		"-lc",
		script,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("systemd-run start runtime host: %w: %s", err, strings.TrimSpace(string(out)))
	}
	meta, _ := LoadRunMetadata(RunMetadataPath(spec.RunDir))
	runID := ""
	epoch := 1
	if meta != nil {
		runID = meta.RunID
		if meta.Epoch > 0 {
			epoch = meta.Epoch
		}
	}
	host := &RunHostState{
		Version:   1,
		Kind:      "runtime_host",
		Launcher:  "systemd",
		Unit:      unit,
		RunDir:    spec.RunDir,
		RunName:   spec.RunName,
		Running:   true,
		Transport: "systemd",
		RunID:     runID,
		Epoch:     epoch,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	return systemdRuntimeHostLauncher{}.Inspect(spec.RunDir, host)
}

func (systemdRuntimeHostLauncher) Stop(runDir string, host *RunHostState) error {
	unit := strings.TrimSpace(hostUnitName(host))
	if unit == "" {
		return nil
	}
	if out, err := exec.Command("systemctl", "--user", "stop", unit).CombinedOutput(); err != nil {
		text := strings.TrimSpace(string(out))
		if strings.Contains(strings.ToLower(text), "not loaded") || strings.Contains(strings.ToLower(text), "could not be found") {
			return nil
		}
		return fmt.Errorf("stop runtime host unit %s: %w: %s", unit, err, text)
	}
	return nil
}

func (systemdRuntimeHostLauncher) Inspect(runDir string, host *RunHostState) (*RunHostState, error) {
	unit := strings.TrimSpace(hostUnitName(host))
	if unit == "" {
		return cloneRunHostState(host), nil
	}
	out, err := exec.Command("systemctl", "--user", "show", unit, "--property=ActiveState", "--property=SubState", "--property=MainPID", "--property=Id").CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		lower := strings.ToLower(text)
		if strings.Contains(lower, "not loaded") || strings.Contains(lower, "could not be found") {
			state := cloneRunHostState(host)
			if state == nil {
				state = &RunHostState{Version: 1, Kind: "runtime_host", Launcher: "systemd", RunDir: runDir, Unit: unit}
			}
			state.Running = false
			state.PID = 0
			state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
			return state, nil
		}
		return nil, fmt.Errorf("inspect runtime host unit %s: %w: %s", unit, err, text)
	}
	state := cloneRunHostState(host)
	if state == nil {
		state = &RunHostState{Version: 1, Kind: "runtime_host", Launcher: "systemd", RunDir: runDir, Unit: unit}
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			state.Running = value == "active"
		case "MainPID":
			pid, _ := strconv.Atoi(strings.TrimSpace(value))
			state.PID = pid
		case "Id":
			if strings.TrimSpace(value) != "" {
				state.Unit = strings.TrimSpace(value)
			}
		}
	}
	state.Transport = "systemd"
	state.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return state, nil
}

func runtimeHostSystemdEnabled() bool {
	if strings.TrimSpace(os.Getenv("GOALX_DISABLE_SYSTEMD_HOST")) == "1" {
		return false
	}
	if strings.HasSuffix(os.Args[0], ".test") {
		return false
	}
	if _, err := exec.LookPath("systemd-run"); err != nil {
		return false
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	return exec.Command("systemctl", "--user", "show-environment").Run() == nil
}

func runtimeHostUnitName(projectRoot, runName string) string {
	projectID := goalx.ProjectID(projectRoot)
	if strings.TrimSpace(projectID) == "" {
		projectID = "project"
	}
	runSlug := goalx.Slugify(runName)
	if strings.TrimSpace(runSlug) == "" {
		runSlug = "run"
	}
	return "goalx-runtime-host-" + projectID + "-" + runSlug
}

func hostLauncherKind(host *RunHostState) string {
	if host == nil {
		return ""
	}
	return strings.TrimSpace(host.Launcher)
}

func hostUnitName(host *RunHostState) string {
	if host == nil {
		return ""
	}
	return strings.TrimSpace(host.Unit)
}

func cloneRunHostState(host *RunHostState) *RunHostState {
	if host == nil {
		return nil
	}
	copy := *host
	return &copy
}
