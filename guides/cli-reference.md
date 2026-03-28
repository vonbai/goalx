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
goalx replace --run NAME session-N --route-profile PROFILE
goalx dimension --run NAME session-N --set depth,evidence
goalx focus --run NAME
```

## Session Lifecycle

```bash
goalx park --run NAME session-N
goalx resume --run NAME session-N
goalx keep --run NAME session-N
goalx archive --run NAME session-N
```

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

## Durable Surface Writes

```bash
goalx durable replace goal --run NAME --file /abs/path.json
goalx durable replace acceptance --run NAME --file /abs/path.json
goalx durable replace coordination --run NAME --file /abs/path.json
goalx durable replace status --run NAME --file /abs/path.json

goalx durable append goal-log --run NAME --file /abs/path.jsonl
goalx durable append evolution --run NAME --file /abs/path.jsonl
```

Use the durable command for machine-consumed run surfaces. Do not hand-edit those files in place.
