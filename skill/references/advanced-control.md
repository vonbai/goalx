# Advanced Control

Use this only when the user explicitly asks for operator-level GoalX control.

## Config-First Launch

Choose this path only when the user wants to inspect or edit config before launch.

```bash
goalx init "goal"
# edit .goalx/goalx.yaml only if the user explicitly asks
goalx start
```

Do not choose this path by default. Prefer `goalx auto`.

## Manual Run Targeting

- `goalx focus --run NAME` pins the default run for commands that omit `--run`
- `--run NAME` resolution is global when the run name is unique
- If names collide across projects, use `--run <project-id>/<run>`

## Manual Intervention

Prefer durable GoalX commands over direct tmux input:

- `goalx tell --run NAME "direction"` to redirect the master or a session
- `goalx add --run NAME "direction"` to create a session manually
- `goalx add --run NAME --mode research "question"` to add a temporary research session
- `goalx park --run NAME session-N` to pause a session without losing its worktree
- `goalx resume --run NAME session-N` to restart a parked session
- `goalx keep --run NAME session-N` to merge a develop session branch

Only fall back to raw tmux intervention when the user explicitly wants pane-level control or the GoalX durable control path is unavailable.
