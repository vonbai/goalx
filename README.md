# GoalX

```bash
goalx run "the authentication system is secure and all vulnerabilities are fixed"
# go to sleep
# wake up to results
```

You describe where you want to end up. GoalX launches a master agent that figures out how to get there — decomposes the goal, spins up parallel workers across AI engines, challenges findings, rescues stuck sessions, and synthesizes everything into a final result. You come back to a research report or a passing test suite — depending on what the goal needed.

## What Actually Happens

```
$ goalx run "the API layer has no N+1 query issues and performs within acceptable limits"

GoalX ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Run:      n1-query-audit
  Master:   claude-code/opus
  Worktree: goalx/n1-query-audit/root

  The master is now in control.
  It will read the codebase, define acceptance criteria,
  dispatch workers, verify results, and close out the run.

  goalx observe    — watch live progress
  goalx status     — check control plane health
  goalx tell       — redirect the master if needed
  goalx keep       — merge results when satisfied

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Behind the scenes:

```
                        goalx run "goal"
                              │
                    ┌─────────┴─────────┐
                    │     Master        │  opus — decomposes goal,
                    │  (orchestrator)   │  dispatches work, verifies results
                    └────┬────┬────┬────┘
                         │    │    │
                   ┌─────┘    │    └─────┐
                   ▼          ▼          ▼
              session-1   session-2   session-3
              codex/5.4   codex/5.4   claude/sonnet
              (research)  (develop)   (adversarial)
                   │          │          │
                   └──────────┴──────────┘
                              │
                    ┌─────────┴─────────┐
                    │   Master merges,  │
                    │   verifies, and   │
                    │   closes the run  │
                    └───────────────────┘
                              │
                        goalx keep
                     (you merge to main)
```

## Install

```bash
git clone https://github.com/vonbai/goalx.git && cd goalx
make install        # /usr/local/bin/goalx
make skill-sync     # installs Claude Code + Codex skills
```

Requires: Go 1.24+, tmux, and at least one of [Claude Code](https://docs.anthropic.com/en/docs/claude-code) or [Codex CLI](https://github.com/openai/codex).

Zero config needed — GoalX detects your engines and picks the best preset:

| Engines Installed | Master | Workers |
|-------------------|--------|---------|
| Claude Code + Codex | opus | gpt-5.4 |
| Claude Code only | opus | sonnet |
| Codex only | gpt-5.4 | gpt-5.4 |

## Quick Start

```bash
# The default. Master decides everything.
goalx run "goal"

# Watch it work
goalx observe

# Nudge the master mid-run
goalx tell "focus on the payment module first"

# Merge results to main when satisfied
goalx keep
```

That's the entire workflow for most goals. Everything below is optional control.

## Intent Overrides

When you want to bias the launch without reintroducing multiple entrypoints:

```bash
goalx run "all API endpoints and their auth requirements are documented" --intent research --effort high
goalx run "all public endpoints are rate-limited and protected" --intent develop --effort medium
goalx run "this project is production-ready" --intent evolve --budget 8h
```

Continue an existing run with an explicit next-step intent:

```bash
goalx run --from auth-audit --intent debate     # challenge findings
goalx run --from auth-audit --intent implement  # build from prior results
goalx run --from auth-audit --intent explore    # dig deeper into one area
```

## Budget

Budget is a user-level time constraint. The master sees it as a fact and manages its time accordingly. The framework does not enforce it.

```bash
goalx run "goal" --budget 4h                    # bounded run with 4h budget
goalx run "goal" --intent evolve                # evolve defaults to 8h
goalx run "goal" --intent evolve --budget 0s    # explicit no limit
```

Non-evolve intents default to no budget — the master stops when the goal is met. Evolve defaults to 8h because open-ended runs need a safety boundary.

## Long-Term Memory

GoalX keeps a user-scoped working-memory store under `~/.goalx/memory/`. It is file-backed, local, and does not require an external database.

- Canonical memory lives under `entries/` and only stores evidence-gated facts, procedures, pitfalls, and secret references.
- Noisy extraction lands in daily `proposals/` shards first; proposals do not become canonical truth until promotion rules pass.
- Each run gets a compiled `control/memory-query.json` and `control/memory-context.json`; the master reads those run-local artifacts instead of scanning canonical memory directly.
- GoalX never persists secret values in memory. It only stores secret references such as where a credential lives or which environment uses it.

## Runtime Control

```bash
# Adjust how agents think about the problem
goalx dimension session-2 --set adversarial,evidence

# Deeper reasoning for a specific session
goalx replace session-2 --route-profile research_deep

# Urgent redirect — interrupts the master immediately
goalx tell --urgent "stop: the payments API is down, pivot to incident response"

# Park a session for later, resume when needed
goalx park session-3
goalx resume session-3
```

## Effort Levels

```bash
goalx run "goal" --intent research --effort high
goalx run "goal" --intent develop --effort minimal
```

| Level | When to Use |
|-------|-------------|
| `minimal` | Triage, validation, lightweight checks |
| `low` | Fast iteration |
| `medium` | Default balanced execution |
| `high` | Hard problems, deep analysis |
| `max` | The hardest, highest-value slices only |

## Goal Dimensions

Dimensions shape how agents approach the objective:

```bash
goalx run "audit security" --dimension adversarial,evidence
goalx dimension session-2 --add creative    # change live
```

| Dimension | Focus |
|-----------|-------|
| `depth` | Go deep on the most impactful area |
| `breadth` | Scan everything, find blind spots |
| `creative` | Non-obvious solutions, challenge assumptions |
| `adversarial` | Find bugs, flaws, edge cases |
| `evidence` | Quantify everything with data |
| `comparative` | Benchmark against industry best practices |
| `feasibility` | Implementation cost, risk, dependencies |
| `user` | End-user perspective, usability |

## Config

Override defaults only when you need to. User-level `~/.goalx/config.yaml`, project-level `.goalx/config.yaml`:

```yaml
master:
  engine: claude-code
  model: opus
roles:
  research: { engine: codex, model: gpt-5.4, effort: high }
  develop:  { engine: codex, model: gpt-5.4, effort: medium }

# Route sessions to different engines based on role + dimensions + effort
routing:
  profiles:
    research_deep:  { engine: claude-code, model: opus, effort: high }
    build_fast:     { engine: codex, model: gpt-5.4-mini, effort: minimal }
  rules:
    - role: research
      any_dimensions: [depth]
      efforts: [high, max]
      profile: research_deep
    - role: develop
      efforts: [minimal, low]
      profile: build_fast

# Semantic guidance for the master (not routing aliases)
preferences:
  research:
    guidance: "Default gpt-5.4 high. Use opus for deep analysis."
  develop:
    guidance: "Default gpt-5.4 medium. Use fast for simple fixes."
```

Project-level config can also define a local validation command that sessions run before handoff:

```yaml
local_validation:
  command: "go build ./... && go test ./... && go vet ./..."
```

## All Commands

| Command | What It Does |
|---------|-------------|
| **Start** | |
| `goalx run` | Primary goal entrypoint. Master decides the path. |
| `goalx run --intent research\|develop` | Bias the initial launch without changing the single-entrypoint model |
| `goalx run --intent evolve --budget 8h` | Open-ended iterative improvement until budget or user stop |
| `goalx run --from RUN --intent debate\|implement\|explore` | Continue an existing run with an explicit next-step intent |
| **Observe** | |
| `goalx observe` | Live transport capture + control summary, including explicit required-coverage facts when present |
| `goalx status` | Progress, lease health, inbox, reminders, and explicit required-coverage facts when present |
| `goalx list` | All runs across states |
| `goalx context` | Run-scoped paths, roster, roles, memory query/context files |
| `goalx afford` | Available commands and paths |
| **Control** | |
| `goalx tell` | Durable instruction to master/session |
| `goalx add` | Add a session to a running run |
| `goalx dimension` | Change runtime dimension assignments |
| `goalx replace` | Hand a slice to a new routed owner |
| `goalx focus` | Pin the default run for bare commands |
| **Session Lifecycle** | |
| `goalx park` | Pause a session, keep its worktree |
| `goalx resume` | Restart a parked session |
| `goalx keep` | Merge worktree (session→run or run→main) |
| `goalx archive` | Tag and preserve a session branch |
| **Closeout** | |
| `goalx verify` | Run acceptance command, record result |
| `goalx save` | Export artifacts to durable storage |
| `goalx result` | Show saved results (`--full` for raw report) |
| `goalx diff` | Diff session code or report outputs |
| `goalx review` | Compare all session outputs |
| `goalx report` | Generate markdown report from journal |
| **Cleanup** | |
| `goalx stop` | Kill processes, preserve worktree |
| `goalx drop` | Kill processes, remove everything |
| **Advanced** | |
| `goalx init` | Generate manual draft config |
| `goalx start --config PATH` | Launch from explicit config |
| `goalx serve` | HTTP API server |
| `goalx attach` | Attach to tmux session |
| `goalx next` | Suggest next pipeline step |

## Architecture

GoalX is **protocol scaffolding infrastructure**. The Go code provides storage (file I/O), execution (process management), and connectivity (git/worktree, inbox/outbox, lease renewal). All interpretation, orchestration, and proof construction belongs to agents.

```
goalx/
├── config.go             # Config loading (one resolver, one path)
├── dimensions.go         # Built-in goal dimensions
├── templates/
│   ├── master.md.tmpl    # Master protocol — the orchestration brain
│   └── program.md.tmpl   # Subagent protocol — hypothesis-driven work
├── cli/                  # All commands
└── cmd/goalx/main.go     # Entry point
```

**Master** reads the immutable `run-charter.json`, maintains `goal.json` as the mutable completion boundary, dispatches parallel work, rescues stuck sessions, and writes the final result to `summary.md`. If the master keeps closeout notes in `proof/completion.json`, it owns the format — the framework does not validate it.

**Subagent** resumes from durable identity + charter, executes hypothesis-driven research or structured TDD, and communicates via journal + inbox.

**Sidecar** renews leases, delivers reminders, and handles urgent interrupt escalation (Escape → relaunch).

## Run Isolation

Every run gets its own git worktree. Sessions can optionally get sub-worktrees for parallel work. Nobody touches main — `goalx keep` is an explicit user decision.

```
~/.goalx/runs/{project}/{run}/     # all runtime state
├── run-charter.json               # immutable doctrine
├── state/                         # mutable run + session state
├── control/                       # inbox, dimensions, guidance
├── summary.md                     # canonical run result
├── reports/                       # supporting research outputs
├── evolution.jsonl                # trial record (evolve intent only)
└── proof/                         # agent-owned closeout evidence
```

## HTTP API

```bash
goalx serve    # starts on configured bind address
```

Full REST API for remote management. Bearer token auth + IP binding. See [deploy/](deploy/) for systemd unit and config.

Key endpoints: `POST /projects/:name/goalx/run`, `/observe`, `/status`, `/tell`, `/keep`, `/stop`. Same action names as the CLI.

## OpenClaw Integration

Manage GoalX from Lark, Telegram, or Web UI via [OpenClaw](https://github.com/openclaw/openclaw):

```bash
cp -r skill/openclaw-skill /path/to/openclaw/workspace/skills/goalx
```

## License

MIT
