---
name: goalx
description: Use when the user wants GoalX to autonomously pursue a goal in the current project, observe or redirect a running GoalX run, or explicitly drive research/develop/debate/implement/explore phases without manual transport, subagent, or config micromanagement.
---

# GoalX

## Overview

GoalX is the default autonomous path for repo-level goals. Start one master-led run, let the master decide how to decompose and execute, observe the durable control signal, then redirect only when needed. Effort is first-class, routing is config-driven, runtime dimensions live in durable state, and session identity records requested/effective effort plus the selected route profile. Durable identity and control anchors are immutable `run-charter.json`, immutable `sessions/session-N/identity.json`, and derived `control/identity-fence.json`.

## When to Use

- The user wants GoalX to research, audit, fix, implement, refactor, or monitor a goal in the current project.
- The user wants to start a focused `research`, `develop`, `debate`, `implement`, or `explore` phase from a specific saved run.
- The user wants to check progress, redirect a run, verify closeout, or review saved results.
- The user wants autonomous orchestration instead of manual transport control, config, or subagent management.
- Do not use the low-level path unless the user explicitly asks for manual control.

## Default Path

```bash
goalx auto "goal"
goalx research "goal" --effort high
goalx develop "goal" --effort medium
goalx debate --from RUN
goalx implement --from RUN
goalx explore --from RUN
```

Common path:

- Start the run with `goalx auto ...` when you want the master to decide the best approach (`research`, `develop`, or hybrid)
- Use `goalx research ...` or `goalx develop ...` when you want a direct phase-specific run and optionally need explicit effort or role-default overrides.
- Use `goalx debate --from RUN`, `goalx implement --from RUN`, or `goalx explore --from RUN` when you want to continue from a saved run.
- Use `--dimension NAMES` only to seed initial run or phase hints. Change live assignments later with `goalx dimension`.
- Watch progress with `goalx observe` or `goalx status`
- Redirect the master only when the goal or constraints change
- Run `goalx verify` before treating a develop run as done
- Use `goalx save` and `goalx result` for durable closeout in user-scoped saved runs

## Autonomy Rules

1. Write the objective as an outcome, not a task checklist. Let the master decompose it.
2. Treat the master as the decision-maker. It can dispatch work, take over work directly, and adjudicate verification and closeout.
3. Default to the master choosing a path and acting on it. Do not ask the user to choose among implementation options unless the choice materially changes scope, risk, acceptance, or irreversible cost.
4. Route course corrections through the master. Prefer `goalx tell --run NAME "direction"` over raw transport input.
5. Do not manually edit GoalX config or runtime files unless the user explicitly asks for config-level control.
6. Keep recaps short. GoalX resumes from durable run state and current files.
7. Interpret `goalx observe` and `goalx status` as control-plane summaries. Report run status, lease health, unread inbox, reminders, delivery failures, and goal-boundary progress instead of raw transport noise.
8. `goalx verify` is stricter than "tests passed": it checks the effective acceptance gate, required-item completion, and the canonical `proof/completion.json` manifest.
9. `--parallel` is optional. Treat it as initial fan-out, not as a permanent cap on later master dispatch.
10. Role defaults are separate. Use `--master`, `--research-role`, and `--develop-role` only when the user wants to override the run's default engine/model split. Use `--effort`, `--master-effort`, `--research-effort`, and `--develop-effort` when reasoning depth must change too.
11. `goalx research` and `goalx develop` are direct phase entry points. `goalx debate --from RUN`, `goalx implement --from RUN`, and `goalx explore --from RUN` continue from saved runs. Only use `--write-config` when the user explicitly wants config-first/manual control, and pair that with `goalx start --config .goalx/goalx.yaml`.
12. When a project has multiple active runs, use `goalx focus --run NAME` to pin the default run. For targeted actions, always pass `--run NAME`; bare run targeting is local-first, and cross-project targeting should use `--run <project-id>/<run>`.
13. Shared project scope is minimal: `.goalx/config.yaml` is the project-scoped config, while active runs, saved runs, focus, and status live under `~/.goalx/runs/{projectID}/...`.
14. Treat `run-charter.json` as the immutable doctrine anchor, `sessions/session-N/identity.json` as the durable worker identity, and `control/identity-fence.json` as the low-disturbance refresh signal.
15. Treat missing charter or session-identity artifacts as a broken live run. Do not invent compatibility behavior around them.
16. Treat `control/launch-env.json` as the run-scoped launch environment authority. `goalx add` and `goalx resume` should reuse that snapshot, not the caller's current shell env or tmux server env.
17. Use `--dimension` for launch-time hints and `goalx dimension` for runtime mutation.
18. Routing profiles are config-driven. Define reusable bundles under `routing.profiles` and map `role + dimension` to profile names under `routing.table`.
19. `goalx stop` kills all leased processes and their descendant process trees before destroying the tmux session. `goalx drop` does the same cleanup, then removes worktrees, branches, and the run directory.

## Common Commands

- `goalx auto "goal"`: master decides the approach (`research`, `develop`, or hybrid)
- `goalx research "goal" --effort high`: direct research run with research-role defaults plus deeper reasoning
- `goalx develop "goal" --effort medium`: direct develop run with develop-role defaults plus explicit effort
- `--master engine/model`, `--research-role engine/model`, `--develop-role engine/model`: optional role-default overrides for the current run
- `--effort LEVEL`, `--master-effort LEVEL`, `--research-effort LEVEL`, `--develop-effort LEVEL`: provider-aware reasoning-depth overrides (`auto|minimal|low|medium|high|max`)
- `--dimension depth,adversarial`: seed initial dimension hints when creating a run or a follow-up phase
- `goalx dimension [--run NAME] <session-N|all> --set depth,adversarial`: replace current runtime assignments
- `goalx dimension [--run NAME] <session-N> --add creative`: add one live dimension to a session
- `goalx dimension [--run NAME] <session-N> --remove depth`: remove one live dimension from a session
- `--parallel N`: optional initial fan-out only; omit it to keep project/preset defaults
- `goalx debate --from RUN`: start a debate phase from a saved research run
- `goalx implement --from RUN`: start an implementation phase from a saved run
- `goalx explore --from RUN`: start a follow-up research phase from a saved run
- `goalx observe --run NAME`: live progress plus control-plane summary
- `goalx status --run NAME`: concise status, unread inbox, lease health, reminders, delivery failures
- `goalx tell --run NAME "direction"`: durable redirect to the master or a specific session
- `goalx tell --urgent --run NAME "message"`: urgent redirect; sidecar interrupts the master immediately and escalates if it stays unread
- `goalx verify --run NAME`: acceptance gate plus goal/closeout validation
- `goalx save --run NAME`: durable artifacts and run closeout
- `goalx result --run NAME`: saved summary from user-scoped durable storage (`--full` for raw report)
- `goalx focus --run NAME`: set the default run for commands that omit `--run`

## Effort Levels

| Level | Guidance |
|-------|----------|
| `auto` | Let the selected engine or routing profile choose the provider-specific effort |
| `minimal` | Cheapest acceptable pass for lightweight validation or triage |
| `low` | Fast iteration with basic reasoning depth |
| `medium` | Default balanced execution for most work |
| `high` | Deeper reasoning for hard implementation or research slices |
| `max` | Use only when the slice is both difficult and valuable enough to justify the cost |

## Routing and Runtime Dimensions

Routing is config-driven. Keep reusable engine/model/effort bundles in `routing.profiles`, then map `role + dimension` to a profile name in `routing.table`:

```yaml
routing:
  profiles:
    research_deep: { engine: claude-code, model: opus, effort: high }
    build_fast: { engine: codex, model: gpt-5.4-mini, effort: minimal }
  table:
    research: { depth: research_deep, breadth: build_fast }
    develop:  { feasibility: build_fast }
```

- `preferences.*.guidance` remains semantic guidance for the agent; it is not a routing alias.
- Use `--dimension NAMES` only when you want to seed initial hints during `auto`, `research`, `develop`, `debate`, `implement`, or `explore`.
- Runtime assignments live in `control/dimensions.json` and are changed with `goalx dimension`.

```bash
goalx dimension [--run NAME] <session-N|all> --set depth,adversarial
goalx dimension [--run NAME] <session-N> --add creative
goalx dimension [--run NAME] <session-N> --remove depth
```

## Cleanup Semantics

- `goalx stop --run NAME` kills leased processes and descendant process trees before tmux teardown.
- `goalx drop --run NAME` performs the same process cleanup, then removes worktrees, branches, and the run directory.
- Prefer these commands over manually killing panes when MCP, Node, or helper processes may still be attached to the run.

## Advanced Control

Only enter manual GoalX control when the user explicitly asks for it. For config-first launch, manual session control, or transport-level intervention, use [references/advanced-control.md](references/advanced-control.md).
