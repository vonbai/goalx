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
goalx research "goal" --effort high
goalx develop "goal" --effort medium
goalx debate --from RUN
goalx implement --from RUN
goalx explore --from RUN
```

Shared config now uses top-level `preset`, `master`, `roles`, `routing`, and `preferences` fields. A minimal manual draft looks like:

```yaml
preset: codex
master:
  engine: claude-code
  model: opus
roles:
  research:
    engine: codex
    model: gpt-5.4
    effort: high
  develop:
    engine: codex
    model: gpt-5.4
    effort: medium
routing:
  profiles:
    research_deep: { engine: claude-code, model: opus, effort: high }
    build_fast: { engine: codex, model: gpt-5.4-mini, effort: minimal }
  table:
    research: { depth: research_deep }
    develop:  { feasibility: build_fast }
preferences:
  research:
    guidance: "Use high-effort Codex by default; escalate depth work to opus."
```

## Manual Run Targeting

- `goalx focus --run NAME` pins the default run for commands that omit `--run`
- Bare `--run NAME` resolution is local-first within the current project
- If names collide across projects, use `--run <project-id>/<run>`
- Active runs, new saved runs, focus, and status are user-scoped under `~/.goalx/runs/{projectID}/...`
- `.goalx/config.yaml` is the shared project-scoped config file
- Canonical closeout proof lives at `~/.goalx/runs/{projectID}/{run}/proof/completion.json`
- `run-charter.json` and `sessions/session-N/identity.json` are required live-run provenance. Missing files mean the run is broken, not that GoalX should fall back.

## Manual Intervention

Prefer durable GoalX commands over direct transport input:

- `goalx tell --run NAME "direction"` to redirect the master or a session
- `goalx tell --urgent --run NAME "message"` to send an urgent message through the durable inbox
- `goalx add --run NAME "direction"` to create a session manually
- `goalx add --run NAME --mode research "question"` to add a temporary research session
- `goalx research "goal" --effort high` to start a direct research run with research-role defaults plus explicit reasoning depth
- `goalx develop "goal" --effort medium` to start a direct develop run with develop-role defaults plus explicit reasoning depth
- `goalx debate --from RUN` to start a debate phase from a saved research run
- `goalx implement --from RUN` to start an implementation phase from a saved run
- `goalx explore --from RUN` to start a follow-up research phase from a saved run
- `--master engine/model`, `--research-role engine/model`, and `--develop-role engine/model` to override role defaults
- `--effort LEVEL`, `--master-effort LEVEL`, `--research-effort LEVEL`, and `--develop-effort LEVEL` to control provider-aware reasoning depth
- `--dimension depth,adversarial` to seed launch-time hints for a new run or phase
- `goalx dimension --run NAME session-2 --set depth,adversarial` to change live runtime dimensions after launch
- `--parallel N` to change the initial fan-out for this run or phase; omit it to keep project/preset defaults
- `--write-config` only when the user explicitly wants to generate `.goalx/goalx.yaml` first, then continue with `goalx start --config .goalx/goalx.yaml`
- `goalx park --run NAME session-N` to pause a session without losing its worktree
- `goalx resume --run NAME session-N` to restart a parked session
- `goalx keep --run NAME session-N` to merge a develop session branch

`--parallel` is not a permanent cap. Master may still add or resume more durable sessions later if the goal warrants it.

## Effort, Routing, and Runtime Dimensions

- Use `--dimension` at launch and `goalx dimension` at runtime.
- Routing profiles are config entries under `routing.profiles`; `routing.table` maps `role + dimension` to one of those profiles.
- Session identity records `requested_effort`, `effective_effort`, `route_profile`, and the role/mode decision for each worker.

## Urgent Delivery and Recovery

- `goalx tell --urgent` marks the inbox message as urgent instead of relying on raw transport nudges.
- The sidecar handles the first escalation by sending tmux `Escape` plus a wake nudge so the master can interrupt its current action and read the durable inbox quickly.
- If the urgent message stays unread for 3 sidecar ticks, the sidecar relaunches the master from durable state. The relaunched master re-reads the charter, inbox, goal, and runtime state before continuing.
- Use this path when the direction must cut through a stuck or long-running master action; do not bypass the durable inbox with pane typing unless the user explicitly asks for pane-level control.

## Stop and Drop Cleanup

- `goalx stop --run NAME` kills all leased processes and descendant process trees before destroying the tmux session.
- `goalx drop --run NAME` performs the same process cleanup, then removes worktrees, branches, and the run directory.

Only fall back to raw transport intervention when the user explicitly wants pane-level control or the GoalX durable control path is unavailable.
