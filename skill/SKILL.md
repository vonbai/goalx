---
name: goalx
description: Use when the user wants GoalX to autonomously pursue a goal in the current project, observe or redirect a running GoalX run, or avoid manual tmux, subagent, and config micromanagement while GoalX researches, implements, verifies, or closes out work.
---

# GoalX

## Overview

GoalX is the default autonomous path for repo-level goals. Start one master-led run, let the master decide how to decompose and execute, observe the durable control signal, then redirect only when needed.

## When to Use

- The user wants GoalX to research, audit, fix, implement, refactor, or monitor a goal in the current project.
- The user wants to check progress, redirect a run, verify closeout, or review saved results.
- The user wants autonomous orchestration instead of manual tmux, config, or subagent management.
- Do not use the low-level path unless the user explicitly asks for manual control.

## Default Path

```bash
goalx auto "goal"
goalx auto "goal" --develop
```

Common path:

- Start the run with `goalx auto ...`
- Watch progress with `goalx observe` or `goalx status`
- Redirect the master only when the goal or constraints change
- Run `goalx verify` before treating a develop run as done
- Use `goalx save` and `goalx result` for durable closeout

## Autonomy Rules

1. Write the objective as an outcome, not a task checklist. Let the master decompose it.
2. Treat the master as the decision-maker. It can dispatch work, take over work directly, and adjudicate verification and closeout.
3. Default to the master choosing a path and acting on it. Do not ask the user to choose among implementation options unless the choice materially changes scope, risk, acceptance, or irreversible cost.
4. Route course corrections through the master. Prefer `goalx tell --run NAME "direction"` over raw `tmux send-keys`.
5. Do not manually edit GoalX config or runtime files unless the user explicitly asks for config-level control.
6. Keep recaps short. GoalX resumes from durable run state and current files.
7. Interpret `goalx observe` and `goalx status` as control-plane summaries. Report unread inbox items, heartbeat lag, stale state, and contract progress instead of raw tmux noise.
8. `goalx verify` is stricter than "tests passed": it checks the effective acceptance gate, required-item completion, and closeout provenance.
9. When a project has multiple active runs, use `goalx focus --run NAME` to pin the default run. For targeted actions, always pass `--run NAME`; explicit run targeting is global when the name is unique.

## Common Commands

- `goalx auto "goal"`: default autonomous path
- `goalx auto "goal" --develop`: default implementation path
- `goalx observe --run NAME`: live progress plus control-plane summary
- `goalx status --run NAME`: concise status, unread inbox, heartbeat lag, protocol hints
- `goalx tell --run NAME "direction"`: durable redirect to the master or a specific session
- `goalx verify --run NAME`: acceptance gate plus goal/closeout validation
- `goalx save --run NAME`: durable artifacts and run closeout
- `goalx result --run NAME`: saved summary (`--full` for raw report)
- `goalx focus --run NAME`: set the default run for commands that omit `--run`

## Advanced Control

Only enter manual GoalX control when the user explicitly asks for it. For config-first launch, manual session control, or tmux-level intervention, use [references/advanced-control.md](references/advanced-control.md).
