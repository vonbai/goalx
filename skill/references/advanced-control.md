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
goalx run "goal"
goalx run "goal" --intent explore
goalx run "goal" --intent explore --readonly
goalx run "goal" --intent evolve --budget 8h
goalx recover --run RUN
goalx run --from RUN --intent debate
goalx run --from RUN --intent implement
goalx run --from RUN --intent explore
goalx run --from RUN --intent explore --readonly
```

Recommended engine/model policy lives in user-scoped `~/.goalx/config.yaml` under `selection`.
Manual drafts do not persist `selection`; they stay focused on the run-local config the user is reviewing.

User-level selection example:

```yaml
selection:
  disabled_targets:
    - claude-code/sonnet
  master_candidates:
    - codex/gpt-5.4
    - claude-code/opus
  worker_candidates:
    - codex/gpt-5.4
    - claude-code/opus
  master_effort: high
  worker_effort: high
```

Minimal manual draft example:

```yaml
master:
  check_interval: 2m
preferences:
  worker:
    guidance: "Bias toward independent evidence before proposing fixes."
  simple:
    guidance: "Prefer small, mergeable changes."
local_validation:
  command: "go build ./... && go test ./..."
```

## Budget

Budget is a user-level time constraint set at `goalx run`. The master sees it as a time fact and manages accordingly. The framework does not enforce it.

```bash
goalx run "goal" --budget 4h                      # 4-hour budget for any intent
goalx run "goal" --intent evolve                   # evolve defaults to 8h
goalx run "goal" --intent evolve --budget 24h      # override evolve default
goalx run "goal" --intent evolve --budget 0s       # explicit no limit
```

Budget can also be set in config.yaml:

```yaml
budget:
  max_duration: 12h
```

Resolution order: `--budget` CLI flag > config.yaml `budget.max_duration` > intent default (0 for non-evolve intents, 8h for evolve).

## Base-Branch Forking

Fork a new session's worktree from an existing session's branch:

```bash
goalx add --run NAME --base-branch session-1 --worktree "alternative approach"
```

The source session must have its own worktree (created with `--worktree`). If session-1 shares the run root worktree, the command fails fast with an error.

This is useful for:
- Trying an alternative approach starting from where a previous session left off
- Evolve-intent runs where the master wants to fork parallel improvements from the current best result
- Any situation where you want tree-shaped exploration instead of linear iteration

You can also pass a raw branch name instead of a session selector:

```bash
goalx add --run NAME --base-branch goalx/my-run/1 --worktree "fork from branch"
```

## Manual Run Targeting

- `goalx focus --run NAME` pins the default run for commands that omit `--run`
- Bare `--run NAME` resolution is local-first within the current project
- If names collide across projects, use `--run <project-id>/<run>`
- Active runs, new saved runs, focus, and status are user-scoped under `~/.goalx/runs/{projectID}/...`
- `.goalx/config.yaml` is the shared project-scoped config file
- `run-charter.json` and `sessions/session-N/identity.json` are required live-run provenance. Missing files mean the run is broken, not that GoalX should fall back.

## Manual Intervention

Prefer durable GoalX commands over direct transport input:

- `goalx tell --run NAME "direction"` to redirect the master or a session
- `goalx tell --urgent --run NAME "message"` to send an urgent message through the durable inbox
- `goalx recover --run NAME` to relaunch the same stopped or stranded run in place
- `goalx add --run NAME "direction"` to create a worker session manually
- `goalx add --run NAME --effort high "question"` to add a deeper worker using the current selection policy
- `goalx add --run NAME --engine ENGINE --model MODEL --effort LEVEL "task"` only when you intentionally want to bypass the current selection policy
- `goalx run "goal" --effort high` to start a deeper default-deliver run with explicit reasoning depth
- `goalx run "goal" --intent explore` to start a fresh evidence-first run
- `goalx run "goal" --intent explore --readonly` to force an investigation-only run with no repo edits
- `goalx run --from RUN --intent debate` to start a debate phase from a saved run
- `goalx run --from RUN --intent implement` to start an implementation phase from a saved run
- `goalx run --from RUN --intent explore` to start a follow-up exploration phase from a saved run
- `goalx run --from RUN --intent explore --readonly` to keep that follow-up exploration report-first and non-editing
- `--master engine/model` and `--worker engine/model` to override role defaults
- `--effort LEVEL`, `--master-effort LEVEL`, and `--worker-effort LEVEL` to control provider-aware reasoning depth
- `--dimension depth,adversarial` to seed launch-time viewpoints for a new run or phase
- `goalx dimension --run NAME session-2 --set depth,adversarial` to change live runtime dimensions after launch
- `--parallel N` to change the initial fan-out for this run or phase; omit it to keep current defaults
- `--write-config` only when the user explicitly wants to generate `.goalx/goalx.yaml` first, then continue with `goalx start --config .goalx/goalx.yaml`
- `goalx park --run NAME session-N` to pause a session without losing its worktree
- `goalx resume --run NAME session-N` to restart a parked session
- `goalx replace --run NAME session-N --engine ENGINE --model MODEL --effort LEVEL` to hand the same slice to a new explicitly chosen owner
- `goalx keep --run NAME session-N` to merge a reviewed worker session branch
- `goalx integrate --run NAME --method partial_adopt --from run-root,session-N` to record a manual run-root integration after master merged or cherry-picked work itself

`--parallel` is not a permanent cap. Master may still add or resume more durable sessions later if the goal warrants it.

Recovery boundary:

- `goalx recover --run NAME` = same run, same run directory, runtime relaunch after `stop` or stranded failure
- `goalx save --run NAME` plus `goalx run --from NAME --intent ...` = new phase from saved artifacts
- do not substitute `run --from` for same-run recovery

## Effort, Selection, and Runtime Dimensions

- Use `--dimension` at launch and `goalx dimension` at runtime.
- Prefer user-scoped `selection.*` candidate pools and disabled targets for normal engine/model policy.
- Older `routing.profiles` and `routing.rules` config still loads for backward compatibility, but it is not part of the recommended public operator surface.
- Session identity records `requested_effort`, `effective_effort`, resolved dimensions, and replacement lineage for each worker.

## Urgent Delivery and Recovery

- `goalx tell --urgent` marks the inbox message as urgent instead of relying on raw transport nudges.
- The sidecar handles the first escalation by sending tmux `Escape` plus a wake nudge so the master can interrupt its current action and read the durable inbox quickly.
- If the urgent message stays unread for 3 sidecar ticks, the sidecar relaunches the master from durable state. The relaunched master re-reads the charter, inbox, goal, and runtime state before continuing.
- If tmux or the master is gone, or the run was intentionally stopped, use `goalx recover --run NAME` to relaunch that same run in place.
- Use this path when the direction must cut through a stuck or long-running master action; do not bypass the durable inbox with pane typing unless the user explicitly asks for pane-level control.

## Stop and Drop Cleanup

- `goalx stop --run NAME` kills all leased processes and descendant process trees before destroying the tmux session.
- `goalx drop --run NAME` performs the same process cleanup, then removes worktrees, branches, and the run directory.

Only fall back to raw transport intervention when the user explicitly wants pane-level control or the GoalX durable control path is unavailable.
