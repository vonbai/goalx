---
name: goalx
description: Use when the user wants GoalX to autonomously pursue a goal in the current project, observe or redirect a running GoalX run, or continue a completed run into its next phase. Also use when the user mentions goalx commands, run management, autonomous research, multi-agent orchestration, or wants to start/monitor/redirect any goal-driven work — even if they don't say "goalx" explicitly. Trigger on evolve/iterate/improve-over-time requests too.
---

# GoalX

GoalX launches one master agent that decomposes a goal, dispatches parallel workers across AI engines, challenges findings, rescues stuck sessions, and closes out the run. You give a goal, GoalX gives you results.

## Constructing a Good Goal

The most important thing you do with GoalX is translate the user's intent into a goal. A good goal describes **where the user wants to end up**, not how to get there.

Users rarely arrive with a crisp specification. They say things like "the API is too slow", "this code is a mess", "we need auth". That's fine. Your job is to capture the desired end-state, not to plan the journey.

**Describe the destination, not the route:**

```
"API response times are acceptable for production use"
```
Not: "1. profile the API 2. find bottlenecks 3. add caching 4. test"

```
"the codebase is maintainable and well-structured"
```
Not: "refactor utils.go, split handler.go, add a linter"

```
"users can securely sign in and manage their accounts"
```
Not: "add JWT middleware, write a login page, create the users table"

**Why this matters:** The master agent is opus-class. It reads the codebase, investigates the problem space, compares multiple paths, and picks the highest-value approach. When you hand it a step-by-step recipe, you override its judgment with yours — and its judgment is usually better because it has full codebase context.

**When the user's idea is vague, keep it vague:**

- "just make the deploy work" → `goalx run "the project deploys to production in one step"`
- "something's off here" → `goalx run "find and fix the most pressing issues"`
- "how's this project looking" → `goalx run "full audit of this project with an actionable improvement plan"`

The master will ask clarifying questions or make judgment calls. That's its job.

**When constraints matter, state them as boundaries, not steps:**

```
goalx run "the config system is refactored, go test ./... passes, and there is no silent fallback"
```

Acceptance criteria are verification boundaries — what must be true when done — not a plan.

## The Default Path

```bash
goalx run "goal"                        # master decides everything
goalx status                            # check progress
goalx tell "focus on payments first"    # redirect if needed
goalx result                            # see the outcome
```

This covers 90% of use cases. Only reach for advanced control when the user explicitly asks.

## Intent Hints

When the user wants a specific flavor of work, pass `--intent`:

```bash
goalx run "goal"                                  # default: deliver the outcome
goalx run "goal" --intent research                 # produce findings, not code
goalx run --from RUN --intent debate               # challenge prior findings
goalx run --from RUN --intent implement            # build from prior research
goalx run --from RUN --intent explore              # extend prior findings
goalx run "goal" --intent evolve --budget 8h       # open-ended iterative improvement
```

Intent is a hint to the master about what kind of outcome the user expects. It does not change the runtime architecture.

**Evolve** is for open-ended improvement — the master iteratively finds and executes the highest-value improvements until the budget runs out or the user says stop. It keeps a trial record (`evolution.jsonl`) so context survives relaunches.

## Budget

Budget is a user-level time constraint set at run creation. The master sees it as a fact and respects it. The framework does not enforce it — agents manage their own time.

```bash
goalx run "goal" --budget 4h                # 4-hour time budget
goalx run "goal" --intent evolve            # evolve defaults to 8h if no --budget
goalx run "goal" --intent evolve --budget 0s  # explicit no limit
```

Non-evolve intents default to no budget (master stops when the goal is met). Evolve defaults to 8h because open-ended runs need a safety boundary.

## How to Think About GoalX

**The master is the decision-maker, not you.** When a user says "audit the auth system", your job is to write a clean goal and launch `goalx run`. The master reads the codebase, decomposes, dispatches, and verifies. You don't plan the approach — the master will.

**Route corrections through the durable control plane.** Use `goalx tell` instead of typing into tmux. Use `goalx dimension` to change how agents think. Use `goalx replace` to hand work to a different engine. These are durable — they survive restarts.

**Don't micromanage config or runtime files.** GoalX auto-detects engines and picks presets. Config overrides exist for users who want explicit control, not as a default path.

## Observing and Redirecting

```bash
goalx status --run NAME                # progress, lease health, goal coverage
goalx observe --run NAME               # live transport + control summary
goalx attach --run NAME                # enter the master's tmux pane
```

Report what matters: run status, goal-boundary progress, session health, unread inbox. Don't dump raw transport noise.

```bash
goalx tell --run NAME "focus on payments first"
goalx tell --urgent --run NAME "stop: production is down"
```

Urgent tells interrupt the master immediately via sidecar escalation.

## Effort and Dimensions

Effort controls reasoning depth. Dimensions shape the approach.

```bash
goalx run "goal" --effort high --dimension adversarial,evidence
goalx dimension --run NAME session-2 --set depth,creative
goalx replace --run NAME session-2 --route-profile research_deep
```

| Effort | When |
|--------|------|
| `minimal` | Triage, validation |
| `low` | Fast iteration |
| `medium` | Default for most work |
| `high` | Hard problems, deep analysis |
| `max` | Highest-value slices only |

## Results

The master writes the final user-facing result to `summary.md` and keeps supporting material in `reports/`.

```bash
goalx result                  # show the run result
goalx result --full           # show the full report
goalx save                    # export to durable storage
```

`goalx verify` runs the acceptance command and records exit code + output. The master interprets what it means. The framework records facts, never judges completion.

## Session Management

```bash
goalx add --run NAME --mode develop --worktree "task"   # add an isolated worker
goalx park --run NAME session-3                          # pause, keep worktree
goalx resume --run NAME session-3                        # restart parked
goalx keep --run NAME session-1                          # merge session branch
```

## Multi-Run Projects

```bash
goalx list                    # all runs
goalx focus --run NAME        # pin default for bare commands
goalx stop --run NAME         # graceful shutdown
goalx drop --run NAME         # kill + remove everything
```

## What NOT to Do

- Don't decompose the goal into steps for the master. Describe the end state and let it plan.
- Don't ask the user to choose among implementation options unless it changes scope, risk, or cost.
- Don't manually edit GoalX runtime files or config unless explicitly asked.
- Don't use `goalx init` / `goalx start --config` unless the user wants config-first control.
- Don't invent compatibility behavior for missing charter or identity files — that's a broken run.

## Advanced Control

For config-first launch, manual session control, explicit engine overrides, or transport-level intervention: [references/advanced-control.md](references/advanced-control.md).
