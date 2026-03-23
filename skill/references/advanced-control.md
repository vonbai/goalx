# Advanced Control

Use this only when the user explicitly asks for operator-level GoalX control.

## Config-First Launch

Choose this path only when the user explicitly wants to inspect or edit config before launch.

```bash
goalx init "goal"
# edit .goalx/goalx.yaml only if the user explicitly asks
goalx start --config .goalx/goalx.yaml
```

Do not choose this path by default. Prefer:

```bash
goalx auto "goal"
goalx research "goal"
goalx develop "goal"
goalx debate --from RUN
goalx implement --from RUN
goalx explore --from RUN
```

## Manual Run Targeting

- `goalx focus --run NAME` pins the default run for commands that omit `--run`
- Bare `--run NAME` resolution is local-first within the current project
- If names collide across projects, use `--run <project-id>/<run>`
- Active runs, new saved runs, focus, and status are user-scoped under `~/.goalx/runs/{projectID}/...`
- `.goalx/config.yaml` is the shared project-scoped config file
- Canonical closeout proof lives at `~/.goalx/runs/{projectID}/{run}/proof/completion.json`

## Manual Intervention

Prefer durable GoalX commands over direct transport input:

- `goalx tell --run NAME "direction"` to redirect the master or a session
- `goalx add --run NAME "direction"` to create a session manually
- `goalx add --run NAME --mode research "question"` to add a temporary research session
- `goalx research "goal"` to start a direct research run with research-role defaults
- `goalx develop "goal"` to start a direct develop run with develop-role defaults
- `goalx debate --from RUN` to start a debate phase from a saved research run
- `goalx implement --from RUN` to start an implementation phase from a saved run
- `goalx explore --from RUN` to start a follow-up research phase from a saved run
- `--master engine/model`, `--research-role engine/model`, and `--develop-role engine/model` to override role defaults
- `--parallel N` to change the initial fan-out for this run or phase; omit it to keep project/preset defaults
- `--write-config` only when the user explicitly wants to generate `.goalx/goalx.yaml` first, then continue with `goalx start --config .goalx/goalx.yaml`
- `goalx park --run NAME session-N` to pause a session without losing its worktree
- `goalx resume --run NAME session-N` to restart a parked session
- `goalx keep --run NAME session-N` to merge a develop session branch

`--parallel` is not a permanent cap. Master may still add or resume more durable sessions later if the goal warrants it.

Only fall back to raw transport intervention when the user explicitly wants pane-level control or the GoalX durable control path is unavailable.
