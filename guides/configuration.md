# GoalX Configuration

GoalX works with zero config. Use configuration only when you need explicit control.

## Config Locations

- user-level: `~/.goalx/config.yaml`
- project-level: `.goalx/config.yaml`

## Typical Example

User-level selection policy in `~/.goalx/config.yaml`:

```yaml
selection:
  disabled_targets:
    - claude-code/sonnet

  master_candidates:
    - codex/gpt-5.4
    - claude-code/opus

  research_candidates:
    - claude-code/opus
    - codex/gpt-5.4

  develop_candidates:
    - codex/gpt-5.4
    - codex/gpt-5.4-mini

  master_effort: high
  research_effort: high
  develop_effort: medium
```

Project-level shared config in `.goalx/config.yaml`:

```yaml
worktree_root: .worktrees
run_root: .goalx/runs
saved_run_root: .goalx/saved

master:
  check_interval: 2m

preferences:
  worker:
    guidance: "Prefer broad evidence before proposing a fix plan."
  simple:
    guidance: "Bias toward small, mergeable implementation slices."

local_validation:
  command: "go build ./... && go test ./... && go vet ./..."
```

## Project-Local Storage Roots

GoalX supports three project-scoped storage roots:

| Config Key | Purpose | Default Location |
|------------|---------|------------------|
| `worktree_root` | Git worktrees for runs and sessions | `~/.goalx/runs/<project>/<run>/worktrees/` |
| `run_root` | Active run state (status, reports, journals) | `~/.goalx/runs/<project>/<run>/` |
| `saved_run_root` | Saved run artifacts for phase continuation | `~/.goalx/runs/<project>/saved/<run>/` |

When configured, all three accept:

- **Relative paths** are resolved from the project root (e.g., `.goalx/runs`)
- **Absolute paths** are used directly (e.g., `/mnt/shared/goalx-runs`)
- Configured values are snapshotted into the run spec at launch, so existing runs keep their original layout

### Example Layout

With the configuration above, a run named `demo` looks like:

```text
project-root/
  .worktrees/
    demo          # run-root worktree
    demo-1        # session-1 worktree
    demo-2        # session-2 worktree
  .goalx/
    runs/
      demo/       # active run state
        status.json
        summary.md
        reports/
        sessions/
    saved/
      demo/       # saved run artifacts
```

### Legacy Compatibility

- Runs created before `run_root` and `saved_run_root` were configured continue to use `~/.goalx/runs/...` paths
- Resolution order for saved runs: configured root → user-scoped → legacy project-local
- The global run registry (`~/.goalx/runs/index.json`) remains user-scoped and stores actual run directory paths

### Automatic Git Exclude

Project-local roots are added to `.git/info/exclude` automatically so they do not dirty the source tree.

## Principles

- Keep one resolver path. Do not invent alternate config entrypoints.
- Use overrides only when they clearly improve execution.
- Explicit `--engine/--model` is an override, not the default path.
- Unknown config should fail loudly, not degrade silently.
- `selection` is only supported in `~/.goalx/config.yaml`.

## What Config Is For

- defining long-term candidate pools and disabled engines/targets
- pinning shared validation, context, and check intervals
- choosing a project-local worktree root when you want `.worktrees/`-style layout
- setting local validation

## What Config Is Not For

- encoding orchestration judgment in the framework
- replacing the normal `goalx run "goal"` path
- hand-editing live run state

## Legacy Compatibility

Older config keys such as `preset`, `master`, `roles`, `routing`, and `preferences` still load for backward compatibility.
They are not the recommended public control surface, and the normal CLI no longer exposes `--preset` or `--route-*` flags.

The recommended default path is:

- user-level `selection.*` for engine/model candidate pools
- project-level `.goalx/config.yaml` for shared repo facts such as validation, context, and check interval
