# Project-Local Run Roots Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add project-level `run_root` and `saved_run_root` support so GoalX can store active run state and saved run artifacts inside the project while preserving compatibility with legacy `~/.goalx/...` runs.

**Architecture:** Introduce unified active-run and saved-run path resolvers, thread them through run creation and run lookup, and keep the global registry user-scoped but pointed at the actual run directory. New configured runs become project-local; legacy runs remain readable through fallback resolution.

**Tech Stack:** Go, YAML config loading, filesystem path resolution, git worktrees, tmux-backed run lifecycle, Go test.

---

### Task 1: Extend Config Schema and Central Path Resolvers

**Files:**
- Modify: `config.go`
- Modify: `config_layers.go`
- Modify: `cli/storage_paths.go`
- Test: `config_test.go`
- Test: `cli/storage_model_test.go`

- [ ] **Step 1: Write the failing config parsing tests**

Add tests covering:

```go
cfg := goalx.Config{
    WorktreeRoot: ".worktrees",
    RunRoot: ".goalx/runs",
    SavedRunRoot: ".goalx/saved",
}
```

and resolver expectations such as:

```go
wantRunDir := filepath.Join(projectRoot, ".goalx", "runs", "demo")
wantSaveDir := filepath.Join(projectRoot, ".goalx", "saved", "demo")
```

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./... -run 'Test(RunDir|SavedRunDir|LoadConfigLayers.*RunRoot)'
```

Expected: FAIL because `Config` does not yet expose `RunRoot` / `SavedRunRoot` and resolvers still point to legacy paths.

- [ ] **Step 3: Add config fields and resolver helpers**

Implement fields in `goalx.Config`:

```go
RunRoot      string `yaml:"run_root,omitempty"`
SavedRunRoot string `yaml:"saved_run_root,omitempty"`
```

Add helper functions that centralize behavior instead of open-coding paths:

```go
func ResolveRunRoot(projectRoot string, cfg *Config) string
func ResolveSavedRunRoot(projectRoot string, cfg *Config) string
func ResolveRunDir(projectRoot, runName string, cfg *Config) string
func ResolveSavedRunDir(projectRoot, runName string, cfg *Config) string
```

Rules:
- relative roots resolve against `projectRoot`
- absolute roots pass through
- empty values preserve current legacy behavior

- [ ] **Step 4: Run tests to verify pass**

Run:

```bash
go test ./... -run 'Test(RunDir|SavedRunDir|LoadConfigLayers.*RunRoot)'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add config.go config_layers.go cli/storage_paths.go config_test.go cli/storage_model_test.go
git commit -m "feat: add configurable run and save roots"
```

### Task 2: Move New Active Runs to Configured Run Root

**Files:**
- Modify: `cli/start.go`
- Modify: `cli/runctx.go`
- Modify: `cli/project_registry.go`
- Modify: `cli/sidecar.go`
- Test: `cli/start_test.go`
- Test: `cli/runctx_test.go`
- Test: `cli/focus_test.go`

- [ ] **Step 1: Write failing lifecycle tests**

Add tests for:

```go
cfg.RunRoot = ".goalx/runs"
runDir := filepath.Join(repo, ".goalx", "runs", "demo")
```

Verify:
- `startWithConfig()` creates `run-spec.yaml` under the configured run root
- `ResolveRun()` finds the run there
- focus/default-run resolution still works

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./cli -run 'Test(Start|ResolveRun|Focus).*ConfiguredRunRoot'
```

Expected: FAIL because `newStartRunState()` and `resolveLocalRun()` still call the legacy `goalx.RunDir(...)`.

- [ ] **Step 3: Implement active-run path plumbing**

Change `start.go` to compute the run directory from the merged config snapshot, for example:

```go
runDir := goalx.ResolveRunDir(projectRoot, runName, cfg)
```

Update run lookup paths in:
- `cli/runctx.go`
- `cli/project_registry.go`
- `cli/sidecar.go`

Ensure `run-spec.yaml` includes:

```yaml
run_root: .goalx/runs
saved_run_root: .goalx/saved
```

- [ ] **Step 4: Run tests to verify pass**

Run:

```bash
go test ./cli -run 'Test(Start|ResolveRun|Focus).*ConfiguredRunRoot'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cli/start.go cli/runctx.go cli/project_registry.go cli/sidecar.go cli/start_test.go cli/runctx_test.go cli/focus_test.go
git commit -m "feat: create and resolve active runs in project-local roots"
```

### Task 3: Move Saved Runs to Configured Saved Root and Keep Fallbacks

**Files:**
- Modify: `cli/storage_paths.go`
- Modify: `cli/save.go`
- Modify: `cli/result.go`
- Modify: `cli/integration_state.go`
- Test: `cli/save_test.go`
- Test: `cli/result_test.go`
- Test: `cli/storage_model_test.go`

- [ ] **Step 1: Write failing save/result tests**

Add tests that expect:

```go
saveDir := filepath.Join(projectRoot, ".goalx", "saved", runName)
```

Verify:
- `goalx save` writes there when `saved_run_root` is configured
- `goalx result` resolves saved runs from there first
- legacy user-scoped and legacy project-local save paths still resolve

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./cli -run 'Test(Save|Result|ResolveSavedRunLocation).*ConfiguredSavedRunRoot'
```

Expected: FAIL because `SavedRunDir(...)` and `ResolveSavedRunLocation(...)` still derive from the user-scoped root.

- [ ] **Step 3: Implement saved-run resolver changes**

Update save/read resolution to use config-aware helpers:

```go
saveDir := ResolveSavedRunDir(rc.ProjectRoot, rc.Name, rc.Config)
```

Keep resolver order:
1. configured saved root
2. current user-scoped saved root
3. legacy project-local saved root

- [ ] **Step 4: Run tests to verify pass**

Run:

```bash
go test ./cli -run 'Test(Save|Result|ResolveSavedRunLocation).*ConfiguredSavedRunRoot'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cli/storage_paths.go cli/save.go cli/result.go cli/integration_state.go cli/save_test.go cli/result_test.go cli/storage_model_test.go
git commit -m "feat: store saved runs in configurable project-local roots"
```

### Task 4: Update Discovery, Registry, and Invocation Mapping

**Files:**
- Modify: `cli/global_run_registry.go`
- Modify: `cli/list.go`
- Modify: `cli/invocation_root.go`
- Modify: `cli/claude_hook.go`
- Modify: `cli/wait.go`
- Modify: `cli/control_cleanup.go`
- Test: `cli/context_afford_test.go`
- Test: `cli/sidecar_test.go`
- Test: `cli/result_test.go`
- Test: `cli/runctx_test.go`

- [ ] **Step 1: Write failing compatibility tests**

Add tests for:
- registry entries storing actual configured `RunDir`
- `list` showing project-local runs
- invocation root mapping still recovering the source project from project-local run/worktree layouts
- `wait` / cleanup finalization still resolving completed runs in the configured root

- [ ] **Step 2: Run tests to verify failure**

Run:

```bash
go test ./cli -run 'Test(GlobalRunRegistry|List|CanonicalProjectRoot|RepairCompletedRunFinalization).*ConfiguredRunRoot'
```

Expected: FAIL because multiple call sites still derive run paths from legacy assumptions.

- [ ] **Step 3: Implement registry and discovery changes**

Update writers to store actual paths:

```go
RunDir: resolvedRunDir,
```

Update readers to prefer:
- explicit registry `RunDir`
- configured run root
- legacy fallback

Adjust invocation-root helpers so they do not assume all durable run dirs live under `~/.goalx/runs/...`.

- [ ] **Step 4: Run tests to verify pass**

Run:

```bash
go test ./cli -run 'Test(GlobalRunRegistry|List|CanonicalProjectRoot|RepairCompletedRunFinalization).*ConfiguredRunRoot'
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cli/global_run_registry.go cli/list.go cli/invocation_root.go cli/claude_hook.go cli/wait.go cli/control_cleanup.go cli/context_afford_test.go cli/sidecar_test.go cli/result_test.go cli/runctx_test.go
git commit -m "fix: honor project-local run roots across discovery and recovery"
```

### Task 5: Documentation, Examples, and Full Verification

**Files:**
- Modify: `README.md`
- Modify: `guides/configuration.md`
- Modify: `guides/cli-reference.md`
- Modify: `cmd/goalx/main.go`

- [ ] **Step 1: Update docs to describe the new storage model**

Document:

```yaml
worktree_root: .worktrees
run_root: .goalx/runs
saved_run_root: .goalx/saved
```

Explain:
- what each root controls
- that the global registry remains user-scoped
- that legacy runs are still supported

- [ ] **Step 2: Run doc/build sanity checks**

Run:

```bash
git diff --check
go test ./... -run '^$'
```

Expected:
- no diff formatting errors
- compile-only test pass

- [ ] **Step 3: Run targeted behavior regression tests**

Run:

```bash
go test ./cli -run 'Test(Start|ResolveRun|Save|Result|GlobalRunRegistry|CanonicalProjectRoot|List).*Configured'
```

Expected: PASS

- [ ] **Step 4: Run broad package verification**

Run:

```bash
go test ./cli ./cmd/goalx ./...
```

Expected: PASS or a clearly documented pre-existing unrelated failure

- [ ] **Step 5: Commit**

```bash
git add README.md guides/configuration.md guides/cli-reference.md cmd/goalx/main.go
git commit -m "docs: document project-local run and save roots"
```
