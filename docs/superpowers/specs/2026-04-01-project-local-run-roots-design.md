# Project-Local Run Roots Design

## Summary

GoalX currently supports relocating Git worktrees into a project-local directory with `worktree_root`, but active run state and saved run artifacts still live under the user-scoped `~/.goalx/...` tree. This design adds explicit project-level configuration for the active run root and saved run root so a project can keep `summary.md`, `reports/`, `status.json`, `coordination.json`, `sessions/`, and `goalx save` output under the project directory as well.

Recommended project-level configuration:

```yaml
worktree_root: .worktrees
run_root: .goalx/runs
saved_run_root: .goalx/saved
```

## Problem

Today the codebase has a split storage model:

- `worktree_root` can relocate Git worktrees into a project-local directory.
- Active run state still resolves through `goalx.RunDir(projectRoot, name)` in `config.go`, which is hardcoded to `~/.goalx/runs/{projectID}/{name}`.
- Saved runs still resolve through `cli/SavedRunDir(projectRoot, runName)` in `cli/storage_paths.go`, which is derived from the same user-scoped root.

That split creates an inconsistent operator experience:

1. Git worktrees appear in the project.
2. Durable run surfaces remain in the user home directory.
3. Users cannot keep one self-contained project-local GoalX workspace even when the project config explicitly opts into local roots.

The user requirement is stricter: if the project config specifies local roots, both active run state and saved run artifacts should honor those roots, not just worktrees.

## Goals

1. Allow project-level config to relocate active run state into the project directory.
2. Allow project-level config to relocate saved run artifacts into the project directory.
3. Preserve existing `worktree_root` behavior without changing its meaning.
4. Keep legacy runs under `~/.goalx/...` readable and recoverable.
5. Ensure `start`, `status`, `observe`, `result`, `recover`, `save`, `list`, `focus`, `report`, `drop`, `keep`, `wait`, and cross-project lookup continue to work with both local-root and legacy runs.
6. Snapshot resolved roots into run metadata/spec so old runs remain stable even if the project config changes later.

## Non-Goals

1. Do not move global memory storage out of `~/.goalx/memory`.
2. Do not relocate the global run registry file itself in this change. It may remain user-scoped as a global index.
3. Do not auto-migrate or move existing legacy run directories on disk.
4. Do not change `worktree_root` semantics to implicitly control durable run state.

## Current State

The current path model is split across two packages.

### Active Run Root

`goalx.RunDir(projectRoot, name)` in `config.go` returns:

```text
~/.goalx/runs/{projectID}/{name}
```

This path is consumed broadly by:

- `cli/start.go`
- `cli/runctx.go`
- `cli/global_run_registry.go`
- `cli/sidecar.go`
- `cli/result.go`
- `cli/save.go`
- `cli/integration_state.go`
- `cli/project_registry.go`
- and many tests

### Saved Run Root

`SavedRunDir(projectRoot, runName)` in `cli/storage_paths.go` resolves under:

```text
~/.goalx/runs/{projectID}/saved/{runName}
```

with a legacy project-local compatibility path:

```text
{projectRoot}/.goalx/runs/{runName}
```

### Worktree Root

`worktree_root` already redirects run-root and session worktrees into a project-local directory via the session path helpers. This is working and should remain independent from active/saved durable run state.

## Proposed Config Schema

Add two project-level config fields to `goalx.Config`:

```yaml
run_root: .goalx/runs
saved_run_root: .goalx/saved
```

Rules:

1. Relative paths are resolved against the project root.
2. Absolute paths are allowed.
3. Empty values preserve legacy defaults.
4. These settings are project-scoped only. They are not valid in `~/.goalx/config.yaml`.
5. Resolved values are snapshotted into `run-spec.yaml` for every new run.

The resulting config shape becomes:

```yaml
worktree_root: .worktrees
run_root: .goalx/runs
saved_run_root: .goalx/saved
```

## Proposed Path Model

### Active Runs

If `run_root` is configured, new active runs should resolve to:

```text
{projectRoot}/.goalx/runs/{runName}
```

For example:

```text
/repo/.goalx/runs/my-analysis
```

That directory contains the current active run surfaces:

- `summary.md`
- `reports/`
- `status.json`
- `coordination.json`
- `control/`
- `sessions/`
- `journals/`
- `run-spec.yaml`
- `run-metadata.json`
- `experiments.jsonl`

### Saved Runs

If `saved_run_root` is configured, `goalx save` should write to:

```text
{projectRoot}/.goalx/saved/{runName}
```

### Worktrees

If `worktree_root` is configured, worktrees continue to resolve independently to:

```text
{projectRoot}/.worktrees/{runName}
{projectRoot}/.worktrees/{runName}-{N}
```

## Resolution Model

The main implementation requirement is a unified resolver model. The codebase should stop concatenating paths ad hoc.

Introduce explicit helpers for:

1. active run root resolution
2. saved run root resolution
3. backward-compatible discovery across current and legacy locations

Recommended behavior:

### New Run Creation

- Resolve active run root from merged config.
- Persist the configured values into `run-spec.yaml`.
- Register the actual resolved `RunDir` in the global run registry.

### Existing Run Resolution

When resolving a run by name:

1. Check the configured active root for `run-spec.yaml`.
2. Fall back to the legacy user-scoped path.
3. Use the global registry for cross-project selectors and run IDs.

### Saved Run Resolution

When resolving a saved run:

1. Check the configured saved root.
2. Fall back to current user-scoped saved storage.
3. Fall back to the legacy project-local save path that already exists today.

## Compatibility

This feature must be additive.

### Legacy Runs

Runs already created under `~/.goalx/runs/...` must continue to work for:

- `goalx status`
- `goalx observe`
- `goalx result`
- `goalx report`
- `goalx recover`
- `goalx drop`

### Registry

The global run registry may remain at:

```text
~/.goalx/runs/index.json
```

This change does not require relocating the registry file. The only hard requirement is that registry entries store the actual `RunDir` path, whether user-scoped or project-local.

### Config Drift

If a run was created when `run_root` pointed to one location and the project config later changes, the run must still resolve correctly. The registry and `run-spec.yaml` become the durable source of truth for that run.

## Affected Components

### Config and Path Resolution

- `config.go`
- `config_layers.go`
- `cli/storage_paths.go`

### Run Lifecycle

- `cli/start.go`
- `cli/runctx.go`
- `cli/sidecar.go`
- `cli/control_cleanup.go`
- `cli/wait.go`

### Discovery and Registry

- `cli/global_run_registry.go`
- `cli/project_registry.go`
- `cli/focus.go`
- `cli/list.go`
- `cli/invocation_root.go`
- `cli/claude_hook.go`

### Results and Save/Restore

- `cli/save.go`
- `cli/result.go`
- `cli/integration_state.go`
- any helper using `SavedRunDir`, `ResolveSavedRunLocation`, or `RunDir`

## Risks

### 1. Partial Migration Risk

If `start` writes to the new run root but `ResolveRun` still only checks the legacy root, the feature will appear flaky. This is the primary failure mode to avoid.

### 2. Invocation Root and Hook Detection

`CanonicalProjectRoot()` currently infers run context partly by path shape. Project-local run roots can overlap normal project directories, so run detection must rely more on durable markers and less on the old `.../worktrees/...` assumptions.

### 3. Save/Result Asymmetry

If `save` writes to a configured local saved root but `result` only checks the old location, operators will perceive data loss. Saved-run resolution must change together with save-write behavior.

### 4. Backward Compatibility Noise

Resolver order matters. A project can simultaneously contain:

- new project-local active runs
- new project-local saved runs
- old user-scoped active runs
- old user-scoped saved runs
- older legacy project-local saved runs

Discovery helpers must be explicit and deterministic.

## Testing Strategy

The feature needs path-resolution-focused tests before implementation.

Minimum coverage:

1. config parsing accepts `run_root` and `saved_run_root`
2. `RunDir`-equivalent resolution uses project-local configured root
3. new runs write state under the configured active root
4. `save` writes under the configured saved root
5. `result` finds saved runs in the configured saved root
6. `ResolveRun` still finds legacy user-scoped runs
7. `List` and `Focus` work when active runs are project-local
8. `CanonicalProjectRoot` still maps worktree cwd back to the source root
9. cross-project lookup continues to work because registry entries store actual runDir values

## Rollout Recommendation

Implement this as a compatibility-preserving storage feature:

1. add schema and resolver helpers
2. migrate creation paths
3. migrate readers and discovery
4. update docs/examples
5. keep legacy fallbacks in place

This should not be shipped as a breaking cleanup. The point is to add project-local storage without invalidating existing runs.
