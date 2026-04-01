# GoalX CLI Reference

## Start

```bash
goalx run "goal"
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
goalx add --run NAME "task"
goalx add --run NAME --worktree "task"
goalx replace --run NAME session-N --engine ENGINE --model MODEL --effort LEVEL
goalx dimension --run NAME session-N --set depth,evidence
goalx budget --run NAME --extend 2h
goalx focus --run NAME
```

## Worktree And Merge

```bash
goalx park --run NAME session-N
goalx resume --run NAME session-N
goalx keep --run NAME session-N
goalx integrate --run NAME --method partial_adopt --from run-root,session-N
goalx keep --run NAME
goalx archive --run NAME session-N
```

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

## Recovery

```bash
goalx recover --run NAME
goalx budget --run NAME --extend 2h
```

Rules:

- `recover` relaunches the same run in place
- `save + run --from` starts a new phase from saved artifacts
- phase continuation now requires canonical saved surfaces

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
- pinned `npx gitnexus@1.5.0` is only exposed when a real probe succeeds
- GoalX does not auto-install it
