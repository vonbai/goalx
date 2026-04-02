package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	goalx "github.com/vonbai/goalx"
	"gopkg.in/yaml.v3"
)

func TestEvaluateResourceAdmissionAllowsHealthyLaunch(t *testing.T) {
	runDir := t.TempDir()
	withStubbedResourceRead(t, healthyResourceFixture)

	decision, err := evaluateResourceAdmission(runDir, "codex", "gpt-5.4-mini")
	if err != nil {
		t.Fatalf("evaluateResourceAdmission: %v", err)
	}
	if decision == nil || !decision.Allowed {
		t.Fatalf("decision = %+v, want allowed", decision)
	}
}

func TestEvaluateResourceAdmissionRejectsCriticalLaunch(t *testing.T) {
	runDir := t.TempDir()
	withStubbedResourceRead(t, criticalResourceFixture)

	decision, err := evaluateResourceAdmission(runDir, "codex", "gpt-5.4")
	if err != nil {
		t.Fatalf("evaluateResourceAdmission: %v", err)
	}
	if decision == nil || decision.Allowed {
		t.Fatalf("decision = %+v, want blocked", decision)
	}
	if decision.State != resourceStateCritical {
		t.Fatalf("decision state = %q, want critical", decision.State)
	}
	if !slicesContains(decision.Reasons, "resource_state_critical") {
		t.Fatalf("decision reasons = %#v, want resource_state_critical", decision.Reasons)
	}
}

func TestRequireResourceAdmissionRejectsUnknownProfile(t *testing.T) {
	runDir := t.TempDir()
	withStubbedResourceRead(t, healthyResourceFixture)

	err := requireResourceAdmission(runDir, "mystery", "ghost", "session launch")
	if err == nil || !strings.Contains(err.Error(), "no built-in memory profile") {
		t.Fatalf("requireResourceAdmission error = %v, want unknown profile failure", err)
	}
}

func TestAddRejectsUnsafeLaunchByResourceAdmission(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	logPath := installFakeTmux(t, "master session-1")
	runName, _ := writeLifecycleRunFixture(t, repo)
	withStubbedResourceRead(t, criticalResourceFixture)

	err := Add(repo, []string{"new slice", "--run", runName})
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked session launch") {
		t.Fatalf("Add error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n session-2") {
		t.Fatalf("add should not launch session-2 when resource admission blocks it:\n%s", string(logData))
	}
}

func TestResumeRejectsUnsafeLaunchByResourceAdmission(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	logPath := installFakeTmux(t, "master")
	runName, _ := writeLifecycleRunFixture(t, repo)
	withStubbedResourceRead(t, criticalResourceFixture)

	err := Resume(repo, []string{"--run", runName, "session-1"})
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked session resume") {
		t.Fatalf("Resume error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n session-1") {
		t.Fatalf("resume should not relaunch session-1 when resource admission blocks it:\n%s", string(logData))
	}
}

func TestReplaceRejectsUnsafeLaunchBeforeParking(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	logPath := installFakeTmux(t, "master session-1")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	withStubbedResourceRead(t, criticalResourceFixture)

	err := Replace(repo, []string{"--run", runName, "session-1"})
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked session replacement") {
		t.Fatalf("Replace error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "kill-window -t "+goalx.TmuxSessionName(repo, runName)+":session-1") {
		t.Fatalf("replace should not park session-1 before resource admission passes:\n%s", string(logData))
	}
	state, err := EnsureSessionsRuntimeState(runDir)
	if err != nil {
		t.Fatalf("EnsureSessionsRuntimeState: %v", err)
	}
	if got := state.Sessions["session-1"].State; got == "parked" {
		t.Fatalf("session-1 should not be parked after blocked replacement: %+v", state.Sessions["session-1"])
	}
}

func TestRecoverRejectsUnsafeMasterRecovery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	logPath, _ := installRecoverFakeTmux(t, false)
	runName, runDir := writeLifecycleRunFixture(t, repo)
	withStubbedResourceRead(t, criticalResourceFixture)

	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{
		Version:         1,
		GoalState:       "open",
		ContinuityState: "stopped",
		UpdatedAt:       "2026-03-29T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}
	if err := SaveRunRuntimeState(RunRuntimeStatePath(runDir), &RunRuntimeState{
		Version:   1,
		Run:       runName,
		Mode:      string(goalx.ModeWorker),
		Active:    false,
		StartedAt: "2026-03-29T00:00:00Z",
		StoppedAt: "2026-03-29T00:10:00Z",
		UpdatedAt: "2026-03-29T00:10:00Z",
	}); err != nil {
		t.Fatalf("SaveRunRuntimeState: %v", err)
	}

	err := Recover(repo, []string{"--run", runName})
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked master recovery") {
		t.Fatalf("Recover error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-session -d -s "+goalx.TmuxSessionName(repo, runName)+" -n master") {
		t.Fatalf("recover should not relaunch master when resource admission blocks it:\n%s", string(logData))
	}
}

func TestStartRejectsUnsafeFreshMasterLaunch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "demo", "base commit")
	goalxDir := filepath.Join(repo, ".goalx")
	if err := os.MkdirAll(goalxDir, 0o755); err != nil {
		t.Fatalf("mkdir .goalx: %v", err)
	}
	cfg := goalx.Config{
		Name:      "demo",
		Mode:      goalx.ModeWorker,
		Objective: "audit auth flow",
		Roles: goalx.RoleDefaultsConfig{
			Worker: goalx.SessionConfig{Engine: "codex", Model: "gpt-5.4"},
		},
		Target:          goalx.TargetConfig{Files: []string{"README.md"}},
		LocalValidation: goalx.LocalValidationConfig{Command: "test -f README.md"},
		Master:          goalx.MasterConfig{Engine: "codex", Model: "gpt-5.4"},
	}
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goalxDir, "goalx.yaml"), data, 0o644); err != nil {
		t.Fatalf("write goalx.yaml: %v", err)
	}
	logPath := installBlockedStartFakeTmux(t)
	withStubbedResourceRead(t, criticalResourceFixture)

	err = Start(repo, []string{"--config", filepath.Join(goalxDir, "goalx.yaml")})
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked fresh start") {
		t.Fatalf("Start error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-session -d -s "+goalx.TmuxSessionName(repo, cfg.Name)+" -n master") {
		t.Fatalf("start should not launch master when resource admission blocks it:\n%s", string(logData))
	}
}

func TestRelaunchMasterRejectsUnsafeLaunchByResourceAdmission(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	logPath := installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	withStubbedResourceRead(t, criticalResourceFixture)

	err = relaunchMaster(repo, runDir, goalx.TmuxSessionName(repo, runName), cfg)
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked master relaunch") {
		t.Fatalf("relaunchMaster error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n master") {
		t.Fatalf("relaunchMaster should not relaunch master window when blocked:\n%s", string(logData))
	}
}

func TestRelaunchMissingMasterWindowRejectsUnsafeLaunchByResourceAdmission(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "base.txt", "base", "base commit")
	logPath := installFakeTmux(t, "master")
	runName, runDir := writeLifecycleRunFixture(t, repo)
	cfg, err := LoadRunSpec(runDir)
	if err != nil {
		t.Fatalf("LoadRunSpec: %v", err)
	}
	withStubbedResourceRead(t, criticalResourceFixture)

	err = relaunchMissingMasterWindow(repo, runDir, goalx.TmuxSessionName(repo, runName), cfg)
	if err == nil || !strings.Contains(err.Error(), "resource admission blocked master relaunch") {
		t.Fatalf("relaunchMissingMasterWindow error = %v, want resource admission failure", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-window -t "+goalx.TmuxSessionName(repo, runName)+" -n master") {
		t.Fatalf("relaunchMissingMasterWindow should not recreate master window when blocked:\n%s", string(logData))
	}
}

func TestRunRuntimeHostTickKeepsRunningWhenMasterRelaunchBlockedByResources(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$TMUX_LOG\"\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 1; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '4321\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	withStubbedResourceRead(t, criticalResourceFixture)

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("runRuntimeHostTick should stay non-fatal on blocked relaunch, got: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Contains(string(logData), "new-session -d -s "+goalx.TmuxSessionName(repo, cfg.Name)+" -n master") {
		t.Fatalf("runtime host should not create master session when relaunch is blocked:\n%s", string(logData))
	}
	auditData, err := os.ReadFile(auditLogPath(runDir))
	if err != nil {
		t.Fatalf("read runtime-host audit log: %v", err)
	}
	if !strings.Contains(string(auditData), "resource admission blocked master relaunch") {
		t.Fatalf("runtime-host audit log missing blocked resource relaunch record:\n%s", string(auditData))
	}
}

func TestRunRuntimeHostTickBacksOffBlockedMasterRelaunch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repo := initGitRepo(t)
	writeAndCommit(t, repo, "README.md", "base", "base commit")
	cfg := &goalx.Config{
		Name:      "runtime-host-run",
		Mode:      goalx.ModeWorker,
		Objective: "ship feature",
		Master:    goalx.MasterConfig{Engine: "codex", Model: "codex"},
	}
	runDir := writeRunSpecFixture(t, repo, cfg)
	meta, err := EnsureRunMetadata(runDir, repo, cfg.Objective)
	if err != nil {
		t.Fatalf("EnsureRunMetadata: %v", err)
	}
	bootstrapRuntimeHostIdentityFixture(t, runDir, repo, cfg, meta)
	if _, err := EnsureRuntimeState(runDir, cfg); err != nil {
		t.Fatalf("EnsureRuntimeState: %v", err)
	}
	if err := EnsureControlState(runDir); err != nil {
		t.Fatalf("EnsureControlState: %v", err)
	}
	if err := SaveControlRunState(ControlRunStatePath(runDir), &ControlRunState{Version: 1, GoalState: "open", ContinuityState: "running"}); err != nil {
		t.Fatalf("SaveControlRunState: %v", err)
	}

	fakeBin := t.TempDir()
	logPath := filepath.Join(fakeBin, "tmux.log")
	tmuxPath := filepath.Join(fakeBin, "tmux")
	script := "#!/bin/sh\n" +
		"echo \"$@\" >> \"$TMUX_LOG\"\n" +
		"if [ \"$1\" = \"has-session\" ]; then exit 1; fi\n" +
		"if [ \"$1\" = \"list-windows\" ]; then exit 0; fi\n" +
		"if [ \"$1\" = \"list-panes\" ]; then printf '4321\n'; exit 0; fi\n" +
		"if [ \"$1\" = \"capture-pane\" ]; then exit 0; fi\n" +
		"exit 0\n"
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	withStubbedResourceRead(t, criticalResourceFixture)

	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("first runRuntimeHostTick: %v", err)
	}
	if err := runRuntimeHostTick(repo, cfg.Name, runDir, meta.RunID, meta.Epoch, time.Minute, os.Getpid()); err != nil {
		t.Fatalf("second runRuntimeHostTick: %v", err)
	}

	logData, err := os.ReadFile(logPath)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("read tmux log: %v", err)
	}
	if strings.Count(string(logData), "new-session -d -s "+goalx.TmuxSessionName(repo, cfg.Name)+" -n master") > 0 {
		t.Fatalf("runtime host should not retry blocked master relaunch immediately:\n%s", string(logData))
	}
	recovery := loadTransportRecoveryTarget(runDir, "master")
	if recovery.CurrentMissingRelaunchAttempts != 1 {
		t.Fatalf("relaunch attempts = %d, want 1 with immediate backoff", recovery.CurrentMissingRelaunchAttempts)
	}
}

func withStubbedResourceRead(t *testing.T, fn func(path string) ([]byte, error)) {
	t.Helper()
	prev := resourceReadFile
	resourceReadFile = fn
	t.Cleanup(func() { resourceReadFile = prev })
}

func healthyResourceFixture(path string) ([]byte, error) {
	switch path {
	case "/proc/meminfo":
		return []byte("MemTotal: 32768 kB\nMemAvailable: 20971520 kB\nSwapTotal: 16384 kB\nSwapFree: 16384 kB\n"), nil
	case "/proc/pressure/memory":
		return []byte("some avg10=0 avg60=0 avg300=0 total=0\nfull avg10=0 avg60=0 avg300=0 total=0\n"), nil
	case "/sys/fs/cgroup/memory.current", "/sys/fs/cgroup/memory.high", "/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.swap.current", "/sys/fs/cgroup/memory.swap.max":
		return []byte("0\n"), nil
	case "/sys/fs/cgroup/memory.events":
		return []byte("low 0\nhigh 0\nmax 0\noom 0\noom_kill 0\n"), nil
	}
	if strings.HasSuffix(path, "/status") {
		return []byte("Name:\tgoalx\nVmRSS:\t1024 kB\n"), nil
	}
	return nil, os.ErrNotExist
}

func criticalResourceFixture(path string) ([]byte, error) {
	switch path {
	case "/proc/meminfo":
		return []byte("MemTotal: 32768 kB\nMemAvailable: 1048576 kB\nSwapTotal: 16384 kB\nSwapFree: 16384 kB\n"), nil
	case "/proc/pressure/memory":
		return []byte("some avg10=12 avg60=0 avg300=0 total=0\nfull avg10=3 avg60=0 avg300=0 total=0\n"), nil
	case "/sys/fs/cgroup/memory.current", "/sys/fs/cgroup/memory.high", "/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.swap.current", "/sys/fs/cgroup/memory.swap.max":
		return []byte("0\n"), nil
	case "/sys/fs/cgroup/memory.events":
		return []byte("low 0\nhigh 0\nmax 0\noom 0\noom_kill 0\n"), nil
	}
	if strings.HasSuffix(path, "/status") {
		return []byte("Name:\tgoalx\nVmRSS:\t1024 kB\n"), nil
	}
	return nil, os.ErrNotExist
}

func slicesContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func installBlockedStartFakeTmux(t *testing.T) string {
	t.Helper()

	fakeBin := t.TempDir()
	stateDir := t.TempDir()
	logPath := filepath.Join(stateDir, "tmux.log")
	script := `#!/bin/sh
set -eu
log="${TMUX_LOG:?}"
cmd="$1"
shift
echo "$cmd $*" >> "$log"
case "$cmd" in
  has-session)
    exit 1
    ;;
  list-panes)
    printf '4321\n'
    exit 0
    ;;
  *)
    exit 0
    ;;
esac
`
	tmuxPath := filepath.Join(fakeBin, "tmux")
	if err := os.WriteFile(tmuxPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("TMUX_LOG", logPath)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return logPath
}
