---
name: goalx
description: Use when the user wants GoalX to autonomously research, implement, monitor, or redirect a goal in the current project.
allowed-tools: Bash, Read, Glob, Grep, Write, Edit
user-invocable: true
---

# GoalX

Default to one autonomous run. Use low-level GoalX control only when the user explicitly asks for it. Use `goalx focus --run NAME` when a project has multiple active runs and the user wants to pin the default run.

## Default Path

```bash
goalx auto "goal"
```

Use:
- `goalx auto "goal"` for research, investigation, audit
- `goalx auto "goal" --develop` for implementation, fixes, refactors
- `goalx auto "goal" --context /path/a,/path/b` only when external files matter

Treat GoalX as a master-led autonomous run: start it, observe it, redirect only when needed, then review results.

## Operating Rules

1. Write the objective as a goal, not a task checklist. Let the master decompose it.
2. Prefer `goalx auto` over `goalx init` + `goalx start`.
3. Do not manually edit `.goalx/goalx.yaml` unless the user explicitly asks for config-level control.
4. Do not micromanage the master or subagents unless the user explicitly asks for low-level intervention.
5. Route direction changes through the master, not directly to subagent panes.
   When you need a low-level redirect, prefer `goalx tell` over raw `tmux send-keys`.
6. If a project has multiple active runs, use `goalx focus --run NAME` to set the default run instead of relying on implicit selection.
7. Keep history recap short. GoalX resumes from durable run state and current files.
8. Interpret `goalx observe` output. Report the signal, not raw tmux noise.
9. GoalX completion is stricter than "tests passed": `goalx verify` also checks required-item completion provenance and whether the effective acceptance gate was silently narrowed.
10. If a project has multiple active runs, always include `--run NAME` for mutating or inspection commands that target one run.

## Scenario Guide

- Research, investigate, audit: `goalx auto "goal"`
- Fix, implement, refactor: `goalx auto "goal" --develop`
- Reference another repo: `goalx auto "goal" --context /path/to/other-project`
- Check progress: `goalx observe`, `goalx status`, `goalx attach`
- Pin the default run: `goalx focus --run NAME`
- Redirect a running goal: send a short new direction to the master
- Run the acceptance gate explicitly: `goalx verify --run NAME`
- View results: `goalx result` or `goalx result --full`

## Commands

| Command | Purpose |
|---------|---------|
| `goalx auto "goal"` | Default path: init + start one master-led run, then exit. |
| `goalx observe [NAME]` | Live capture from all tmux windows |
| `goalx status [NAME]` | Journal-based progress |
| `goalx focus --run NAME` | Pin the default run for commands that omit `--run` |
| `goalx result [NAME]` | Show summary (`--full` for raw report) |
| `goalx save [NAME]` | Save durable artifacts and `artifacts.json` to `.goalx/runs/` |
| `goalx verify [NAME]` | Run the active run's acceptance command and record the result |
| `goalx keep [NAME] <session>` | Merge a develop session branch into main |
| `goalx stop [NAME]` | Graceful shutdown |
| `goalx drop [NAME]` | Cleanup worktrees and branches; refuses unsaved runs until `goalx save` |
| `goalx init "goal"` | Advanced/manual path only: generate config without starting |
| `goalx start` | Advanced/manual path only: launch from existing config |
| `goalx add "direction"` | Advanced/manual path only: add a subagent session |
| `goalx tell [target] "message"` | Advanced/manual path only: write a durable instruction to master or a session |
| `goalx park [NAME] <session>` | Advanced/manual path only: park a session |
| `goalx resume [NAME] <session>` | Advanced/manual path only: resume a parked session |
| `goalx attach [NAME]` | Attach to tmux session |
| `goalx list` | List all runs |
| `goalx debate` | Generate debate config from prior research |
| `goalx implement` | Generate develop config from consensus |

## Observe and React

- Healthy: summarize progress, wait.
- Stuck 2+ heartbeats: redirect the master. It should rebalance or resume/park sessions instead of just waiting.
- Wrong direction: steer the master, not subagents.
- Need an explicit acceptance check: run `goalx verify` before treating the run as done.
- If a run says "verification only", make sure it did not also merge or keep code changes in the same closeout.
- Complete: `goalx save` then `goalx result` to review. Saved reports are indexed through `artifacts.json`.
- Runtime state lives under `~/.goalx/runs/{projectID}/{run}`. Saved artifacts live under `<project>/.goalx/runs/{run}`.

## Advanced Control

Only use low-level GoalX control when the user explicitly asks for it.

- Manual config flow: see [references/advanced-control.md](references/advanced-control.md)
- Direct session lifecycle or tmux intervention: see [references/advanced-control.md](references/advanced-control.md)
