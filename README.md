# GoalX

```bash
goalx run "the authentication system is secure and all vulnerabilities are fixed"
# go to sleep
# wake up to results
```

GoalX is autonomous goal execution for codebases. You describe the end state. GoalX launches one master agent that reads the codebase, decides the path, dispatches durable sessions, verifies the result, and closes out the run.

## The Default Path

```bash
goalx run "goal"                 # start the run
goalx observe                    # watch live progress
goalx tell "focus on X first"    # redirect if needed
goalx result                     # read the result
goalx keep                       # merge code when satisfied
```

This is the primary GoalX workflow. Most users do not need more than this.

## Mental Model

- GoalX the framework only provides storage, execution, and connectivity.
- The master agent owns judgment: planning, decomposition, verification, and closeout.
- Sessions own concrete slices of work.
- `goalx verify` records acceptance facts. The master interprets what they mean.
- Control goes through durable GoalX commands, not manual edits to runtime files or tmux panes.

## What GoalX Actually Does

```
                  goalx run "goal"
                        │
                 ┌──────┴──────┐
                 │   master    │
                 │ reads code, │
                 │ decides path│
                 └───┬────┬────┘
                     │    │
               ┌─────┘    └─────┐
               ▼                ▼
           session-1        session-2
           research         develop
               │                │
               └───────┬────────┘
                       ▼
                 master reviews,
                 verifies, and
                  closes out
```

You give GoalX a destination, not a checklist. The master decides whether the run needs research, implementation, adversarial review, more sessions, deeper effort, or a phase transition.

## Install

```bash
git clone https://github.com/vonbai/goalx.git && cd goalx
make install
make skill-sync
```

Requires:

- Go 1.24+
- `tmux`
- at least one of [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex)

Zero config is fine. GoalX auto-detects installed engines and picks the best available preset.

## First Run

Describe the destination, not the route:

```bash
goalx run "the API layer has no N+1 query issues and performs within acceptable limits"
```

Useful launch shapes:

```bash
goalx run "all public endpoints and their auth requirements are documented" --intent research
goalx run "all public endpoints are rate-limited and protected" --intent develop
goalx run "this project is production-ready" --intent evolve --budget 8h
```

Continue an existing run with an explicit next-step intent:

```bash
goalx run --from auth-audit --intent debate
goalx run --from auth-audit --intent implement
goalx run --from auth-audit --intent explore
```

Budget is a user-level time fact. The master sees it and manages its time accordingly. The framework does not enforce it.

## Observe And Redirect

```bash
goalx status
goalx observe
goalx tell "focus on the payment module first"
goalx tell --urgent "stop: production is down"
```

- `goalx status` shows durable/control facts: progress, leases, inbox, coverage, reminders.
- `goalx observe` shows live transport plus the control summary.
- `goalx tell` writes a durable redirect to the master or a session.
- `goalx tell --urgent` escalates immediately through the sidecar.

## Finish The Run

```bash
goalx verify
goalx result
goalx keep
goalx save
```

- `goalx verify` runs the acceptance command and records exit code + output.
- `goalx result` shows the saved result summary and reports.
- `goalx keep` merges session work into the run, or the run into your main worktree.
- `goalx save` exports the run artifacts for durable storage.

GoalX does not auto-judge completion in the framework. The master decides when the goal is actually complete.

## Advanced Guides

Public deep-reference material lives under [`guides/`](guides):

- [Getting Started](guides/getting-started.md)
- [Runtime Control](guides/runtime-control.md)
- [Configuration](guides/configuration.md)
- [Memory](guides/memory.md)
- [CLI Reference](guides/cli-reference.md)

Related material:

- [Deployment / HTTP API](deploy/README.md)
- [Skill advanced control reference](skill/references/advanced-control.md)

## Design Principles

- **Single entrypoint**: `goalx run` is the default path.
- **Facts, not judgments**: framework records and transports facts; agents interpret them.
- **Verify = record, not judge**: verification output is evidence, not an automatic verdict.
- **No silent fallback**: unknown or invalid machine-consumed inputs should fail loudly.

## License
