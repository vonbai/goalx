# GoalX Runtime Control

## Observe

```bash
goalx status [NAME] [session-N]
goalx observe [NAME]
goalx attach --run NAME
```

Use:

- `status` for durable/control facts
- `observe` for live transport plus control summary
- `attach` only when you intentionally need the tmux pane

## Redirect

```bash
goalx tell [--run NAME] [master|session-N] "message"
goalx tell --run NAME --urgent master "stop: production is down"
```

- normal tells are durable redirects
- urgent tells escalate immediately

## Add Or Replace Work

```bash
goalx add --run NAME --mode research "investigate the auth boundary"
goalx add --run NAME --mode develop --worktree "implement the fix"
goalx replace --run NAME session-2 --route-profile research_deep
```

Use:

- `add` to create a new durable session
- `replace` to hand the slice to a fresh routed owner

## Live Runtime Adjustments

```bash
goalx dimension --run NAME session-2 --set adversarial,evidence
goalx dimension --run NAME session-2 --add creative
goalx dimension --run NAME session-2 --remove depth
```

Dimensions shape how the agents think about the problem. They do not change the architecture.

## Session Lifecycle

```bash
goalx park --run NAME session-3
goalx resume --run NAME session-3
goalx keep --run NAME session-2
goalx archive --run NAME session-4
```

- `park` pauses a session and keeps its worktree
- `resume` restarts a parked session
- `keep` merges work
- `archive` preserves a session branch

## Finish Or Clean Up

```bash
goalx verify --run NAME
goalx result --run NAME
goalx save NAME
goalx stop --run NAME
goalx drop --run NAME
```

- `stop` preserves the run directory
- `drop` removes the run completely

## What Not To Do

- Do not use direct tmux typing as the normal control path.
- Do not hand-edit machine-consumed run surfaces in place.
- Do not confuse transport success with master/session consumption.
