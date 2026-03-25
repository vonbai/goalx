---
name: goalx
description: Use when the user wants GoalX to autonomously pursue a goal in the current project, observe or redirect a running GoalX run, or explicitly drive research/develop/debate/implement/explore phases without manual transport, subagent, or config micromanagement. Also use when the user mentions goalx commands, run management, autonomous research, multi-agent orchestration, or wants to start/monitor/redirect any goal-driven work — even if they don't say "goalx" explicitly.
---

# GoalX

GoalX launches one master agent that decomposes a goal, dispatches parallel workers across AI engines, challenges findings, rescues stuck sessions, and closes out the run. You give a goal, GoalX gives you results.

## The Default Path

```bash
goalx auto "goal"           # master decides everything
goalx observe               # watch live progress
goalx tell "redirect"       # nudge the master if needed
goalx keep                  # merge results to main
```

For explicit phases:

```bash
goalx research "goal" --effort high
goalx develop "goal" --effort medium
goalx debate --from RUN
goalx implement --from RUN
goalx explore --from RUN
```

This covers 90% of use cases. Only reach for advanced control when the user explicitly asks.

## How to Think About GoalX

**The master is the decision-maker, not you.** When a user says "audit my auth system", your job is to write a clean objective and launch `goalx auto`. The master reads the codebase, decomposes the goal, picks engines, dispatches workers, and verifies results. You don't need to plan the approach — the master will.

**Write objectives as outcomes, not checklists.** "Find and fix all N+1 query issues" is better than "1. scan models 2. check eager loading 3. add includes". The master is an opus-class agent — trust it to decompose.

**Route corrections through the durable control plane.** Use `goalx tell` instead of raw transport input. Use `goalx dimension` to change how agents think. Use `goalx replace` to hand work to a different engine. These are durable — they survive restarts.

**Don't micromanage config or runtime files.** GoalX auto-detects engines and picks presets. Config overrides exist for users who want explicit control, not as a default path.

## Observing and Redirecting

```bash
goalx observe --run NAME    # live transport + control summary
goalx status --run NAME     # progress, lease health, inbox, reminders
```

Report what matters: run status, goal-boundary progress, lease health, unread inbox, delivery failures. Don't dump raw transport noise.

```bash
goalx tell --run NAME "focus on payments first"
goalx tell --urgent --run NAME "stop: production is down"
```

Urgent tells interrupt the master immediately via sidecar escalation (Escape → relaunch if still unread).

## Effort and Dimensions

Effort controls reasoning depth. Dimensions shape the approach.

```bash
goalx auto "goal" --effort high --dimension adversarial,evidence
goalx dimension session-2 --set depth,creative   # change live
goalx replace session-2 --route-profile research_deep
```

| Effort | When |
|--------|------|
| `minimal` | Triage, validation |
| `low` | Fast iteration |
| `medium` | Default for most work |
| `high` | Hard problems, deep analysis |
| `max` | Highest-value slices only |

Routing is config-driven: `routing.profiles` define engine/model/effort bundles, `routing.rules` match role + dimensions + effort to a profile. `preferences.*.guidance` is semantic guidance for the master, not a routing alias.

## Session Management

```bash
goalx add --run NAME --mode develop "task"         # add a worker
goalx add --run NAME --mode develop --worktree "task"  # isolated parallel
goalx park session-3                               # pause, keep worktree
goalx resume session-3                             # restart parked
goalx keep session-1                               # merge session → run
goalx keep                                         # merge run → main
```

`--parallel N` is initial fan-out guidance, not a ceiling. The master may add more workers later.

## Closeout

```bash
goalx verify --run NAME     # run acceptance command, record result
goalx save --run NAME       # export to durable storage
goalx result --run NAME     # view saved results (--full for raw report)
```

`goalx verify` records exit code + output. The master interprets what it means and builds `proof/completion.json`. The framework doesn't judge completion — the master does.

## Multi-Run Projects

```bash
goalx list                  # all runs
goalx focus --run NAME      # pin default for bare commands
goalx stop --run NAME       # kill processes, preserve worktree
goalx drop --run NAME       # kill processes, remove everything
```

Bare `--run NAME` resolves local-first. Cross-project: `--run <project-id>/<run>`.

## What NOT to Do

- Don't ask the user to choose among implementation options unless it changes scope, risk, or cost. Default to the master deciding.
- Don't manually edit GoalX runtime files or config unless explicitly asked.
- Don't use `goalx init` / `goalx start --config` unless the user wants config-first control.
- Don't invent compatibility behavior for missing charter or identity files — that's a broken run, not a fallback opportunity.

## Advanced Control

For config-first launch, manual session control, explicit engine overrides, or transport-level intervention: [references/advanced-control.md](references/advanced-control.md).
