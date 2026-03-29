# GoalX CLI Reference

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

Use `goalx schema <surface>` as the canonical machine-consumed durable contract authority.

## Durable Surface Writes

```bash
goalx durable replace goal --run NAME --file /abs/path.json
goalx durable replace acceptance --run NAME --file /abs/path.json
goalx durable replace coordination --run NAME --file /abs/path.json
goalx durable replace status --run NAME --file /abs/path.json

goalx durable append goal-log --run NAME --file /abs/path.jsonl
goalx durable append experiments --run NAME --file /abs/path.jsonl
```

Inspect the surface first with `goalx schema <surface>`, then use `goalx durable` to write it. Do not hand-edit those files in place.
