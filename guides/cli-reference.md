# GoalX CLI Reference

Project-level `.goalx/config.yaml` can set:

```yaml
worktree_root: .worktrees
run_root: .goalx/runs
saved_run_root: .goalx/saved
```

When set, GoalX can place:

- run root and session worktrees under the configured `worktree_root`
- active run state under the configured `run_root`
- saved run artifacts under the configured `saved_run_root`

## Start

```bash
goalx run "goal"
goalx run --objective "goal"
goalx run --objective-file /abs/path/to/objective.txt
goalx run "goal" --intent explore
goalx run "goal" --intent evolve --budget 8h
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
```

## Observe

```bash
goalx list
goalx status [--run NAME]
goalx observe [--run NAME]
goalx context [--run NAME]
goalx afford [--run NAME] [target]
goalx attach [--run NAME] [master|session-N]
goalx wait [--run NAME] [target] --timeout ...
```

## Control

```bash
goalx tell [--run NAME] [master|session-N] "message"
goalx tell --urgent [--run NAME] [master|session-N] "message"
goalx ack-inbox [--run NAME] [master|session-N]
goalx add --run NAME "task"
goalx add --run NAME --worktree "task"
goalx replace --run NAME session-N --engine ENGINE --model MODEL --effort LEVEL
goalx dimension --run NAME session-N --set depth,evidence
goalx budget --run NAME --extend 2h
goalx focus --run NAME
```

- `goalx add --run NAME --mode develop --worktree "goal"` creates a dedicated session worktree.
- with `worktree_root: .worktrees`, that session worktree is created under `project/.worktrees/` instead of `~/.goalx/runs/.../worktrees/`.

## Worktree And Merge

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
goalx verify --run NAME --lane quick
goalx verify --run NAME --lane full
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
goalx recover --run NAME
goalx budget --run NAME --extend 2h
```

Rules:

- `recover` relaunches the same run in place
- fresh runs can briefly show `launching`; use `goalx status`, `goalx observe`, or `goalx wait --run NAME master --timeout 30s` before you decide to recover
- `save + run --from` starts a new phase from saved artifacts
- phase continuation now requires canonical saved surfaces

## Resource Safety

- `goalx status` stays compact under healthy conditions
- `goalx observe` is the full diagnostic path for transport and runtime/resource facts
- GoalX records framework-owned resource facts in `control/resource-state.json`
- `add`, `resume`, `replace`, and `recover` can fail explicitly when new execution would be obviously unsafe
- GoalX does not silently downgrade effort or shrink fan-out

## Durable Surface Schemas

```bash
goalx schema objective-contract
goalx schema obligation-model
goalx schema assurance-plan
goalx schema cognition-state
goalx schema impact-state
goalx schema freshness-state
goalx schema coordination
goalx schema status
goalx schema obligation-log
goalx schema evidence-log
goalx schema experiments
goalx schema compiler-input
goalx schema compiler-report
```

Use `goalx schema <surface>` as the canonical authoring-contract authority.

## Durable Surface Writes

```bash
goalx durable write obligation-model --run NAME --body-file /abs/path.json
goalx durable write assurance-plan --run NAME --body-file /abs/path.json
goalx durable write coordination --run NAME --body-file /abs/path.json
goalx durable write status --run NAME --body-file /abs/path.json

goalx durable write obligation-log --run NAME --kind decision --actor master --body-file /abs/path.json
goalx durable write evidence-log --run NAME --kind scenario.executed --actor master --body-file /abs/path.json
goalx durable write experiments --run NAME --kind experiment.created --actor master --body-file /abs/path.json
```

## Optional Repo Cognition

GoalX always exposes builtin `repo-native` cognition.

GitNexus is optional:

- binary install is preferred
- install: `npm install -g gitnexus@1.5.0`
- verify: `gitnexus status`
- pinned `npx gitnexus@1.5.0` is only exposed when a real probe succeeds
- GoalX does not auto-install it
- `goalx context` records GitNexus per worktree scope with explicit `index_state`
- `goalx afford [--run NAME] [master|session-N]` can expose runnable GitNexus `status`, `query`, `context`, and `impact` commands for the selected scope
- GoalX may best-effort refresh a missing or stale GitNexus index during lifecycle transitions when the provider is available
- optional MCP setup:
  - `codex mcp add gitnexus -- npx -y gitnexus@1.5.0 mcp`
  - `claude mcp add gitnexus -- npx -y gitnexus@1.5.0 mcp`
