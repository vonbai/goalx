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
goalx replace --run NAME session-2 --engine claude-code --model opus --effort high
```

Use:

- `add` to create a new durable session
- `add --worktree` to give that session its own git worktree boundary
- if project config sets `worktree_root: .worktrees`, dedicated worktrees are created under that path
- `replace` to hand the slice to a fresh owner, optionally with an explicit engine/model override

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
goalx integrate --run NAME --method partial_adopt --from run-root,session-2
goalx archive --run NAME session-4
```

- `park` pauses a session and keeps its worktree
- `resume` restarts a parked session
- `keep --run NAME session-N` merges a reviewed session branch into the run worktree
- `integrate --run NAME --method ... --from ...` records the lineage of a manual run-root integration that master already completed
- `keep --run NAME` merges the run worktree into the source root when lineage is still valid
- `archive` preserves a session branch
- relocating worktrees with `worktree_root` changes placement only; the same lifecycle and merge rules still apply

## Finish Or Clean Up

```bash
goalx recover --run NAME
goalx verify --run NAME
goalx result --run NAME
goalx save NAME
goalx stop --run NAME
goalx drop --run NAME
```

- `recover` relaunches the same run after `stop`, tmux loss, or a stranded state
- `save` exports durable artifacts for a later phase
- `stop` preserves the run directory
- `drop` removes the run completely
- if `worktree_root` points into the project, `drop` also removes GoalX-managed entries from that directory
- `save` plus `goalx run --from NAME --intent ...` creates a new phase; it does not recover the same run

## What Not To Do

- Do not use direct tmux typing as the normal control path.
- Do not hand-edit machine-consumed run surfaces in place.
- Do not confuse transport success with master/session consumption.
