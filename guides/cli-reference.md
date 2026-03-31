# GoalX CLI Reference

Project-level `.goalx/config.yaml` can set:

```yaml
worktree_root: .worktrees
```

When set, GoalX still keeps durable run state under `~/.goalx/runs/...`, but places the run root worktree and dedicated session worktrees under the configured path.

## Start

```bash
goalx run "goal"
goalx run "goal" --intent research
goalx run "goal" --intent develop
goalx run "goal" --intent evolve --budget 8h
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
```

## Observe

```bash
goalx status [NAME] [session-N]
goalx observe [NAME]
goalx list
goalx context
goalx afford
goalx attach --run NAME
```

## Control

```bash
goalx tell [--run NAME] [master|session-N] "message"
goalx tell --urgent [--run NAME] [master|session-N] "message"
goalx add --run NAME --mode research "goal"
goalx add --run NAME --mode develop --worktree "goal"
goalx replace --run NAME session-N --engine ENGINE --model MODEL --effort LEVEL
goalx dimension --run NAME session-N --set depth,evidence
goalx focus --run NAME
```

- `goalx add --run NAME --mode develop --worktree "goal"` creates a dedicated session worktree.
- with `worktree_root: .worktrees`, that session worktree is created under `project/.worktrees/` instead of `~/.goalx/runs/.../worktrees/`.

## Session Lifecycle

```bash
goalx park --run NAME session-N
goalx resume --run NAME session-N
goalx keep --run NAME session-N
goalx integrate --run NAME --method partial_adopt --from run-root,session-N
goalx keep --run NAME
goalx archive --run NAME session-N
```

- `goalx keep --run NAME session-N` merges a reviewed develop session branch into the run worktree only.
- `goalx integrate --run NAME --method ... --from ...` records the current run-root result after master manually merged, cherry-picked, or partially adopted work there.
- `goalx keep --run NAME` merges the run worktree into the source root, but only when source-root `HEAD` still descends from the run base revision.
- `park`, `resume`, `keep`, and `drop` continue to work the same way when worktrees are relocated with `worktree_root`.

## Closeout

```bash
goalx verify --run NAME
goalx result [--full]
goalx save NAME
goalx diff
goalx review
goalx report
```

## Cleanup

```bash
goalx stop --run NAME
goalx drop --run NAME
```

- `drop` removes GoalX-managed worktrees from the configured worktree root as well as the run directory.

## Recovery

```bash
goalx recover [--run RUN]
```

## Recovery Semantics

- `goalx recover --run NAME` relaunches the same stopped or stranded run in place.
- `goalx save NAME` plus `goalx run --from NAME --intent ...` starts a new phase from saved artifacts.
- Do not use `goalx run --from ...` as a substitute for same-run recovery.

## Durable Surface Schemas

```bash
goalx schema goal
goalx schema acceptance
goalx schema coordination
goalx schema status
goalx schema goal-log
goalx schema experiments
```

Use `goalx schema <surface>` as the canonical machine-consumed durable authoring-contract authority.

## Durable Surface Writes

```bash
goalx durable write goal --run NAME --body-file /abs/path.json
goalx durable write acceptance --run NAME --body-file /abs/path.json
goalx durable write coordination --run NAME --body-file /abs/path.json
goalx durable write status --run NAME --body-file /abs/path.json

goalx durable write goal-log --run NAME --kind decision --actor master --body-file /abs/path.json
goalx durable write experiments --run NAME --kind experiment.created --actor master --body-file /abs/path.json
```

Inspect the surface first with `goalx schema <surface>`, then use `goalx durable write` with an authoring payload. The framework serializes the canonical storage envelope and timestamps. Do not hand-edit those files in place.
